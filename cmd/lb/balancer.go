package main

import (
	"context"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/roman-mazur/architecture-practice-4-template/httptools"
	"github.com/roman-mazur/architecture-practice-4-template/signal"
)

var (
	port       = flag.Int("port", 8090, "load balancer port")
	timeoutSec = flag.Int("timeout-sec", 3, "request timeout time in seconds")
	https      = flag.Bool("https", false, "whether backends support HTTPs")

	traceEnabled = flag.Bool("trace", false, "whether to include tracing information into responses")
)

var (
	timeout = time.Duration(*timeoutSec) * time.Second

	// Всі сервери
	serversPool = []string{
		"server1:8080",
		"server2:8080",
		"server3:8080",
	}

	mu            sync.RWMutex
	healthyServers []string
)

func scheme() string {
	if *https {
		return "https"
	}
	return "http"
}

func health(dst string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s://%s/health", scheme(), dst), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Проактивне оновлення списку здорових серверів
func updateHealthyServers() {
	for range time.Tick(10 * time.Second) {
		var healthy []string
		for _, s := range serversPool {
			if health(s) {
				healthy = append(healthy, s)
			}
		}

		mu.Lock()
		healthyServers = healthy
		mu.Unlock()

		log.Printf("Updated healthy servers: %v", healthy)
	}
}

// Вибір сервера за хешем шляху
func selectServer(path string) (string, error) {
	mu.RLock()
	defer mu.RUnlock()

	if len(healthyServers) == 0 {
		return "", fmt.Errorf("no healthy servers")
	}

	h := fnv.New32a()
	h.Write([]byte(path))
	idx := int(h.Sum32()) % len(healthyServers)

	return healthyServers[idx], nil
}

func forward(dst string, rw http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	fwdRequest := r.Clone(ctx)
	fwdRequest.RequestURI = ""
	fwdRequest.URL.Host = dst
	fwdRequest.URL.Scheme = scheme()
	fwdRequest.Host = dst

	resp, err := http.DefaultClient.Do(fwdRequest)
	if err == nil {
		for k, values := range resp.Header {
			for _, value := range values {
				rw.Header().Add(k, value)
			}
		}
		if *traceEnabled {
			rw.Header().Set("lb-from", dst)
		}
		log.Println("fwd", resp.StatusCode, resp.Request.URL)
		rw.WriteHeader(resp.StatusCode)
		defer resp.Body.Close()
		_, err := io.Copy(rw, resp.Body)
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
		return nil
	} else {
		log.Printf("Failed to get response from %s: %s", dst, err)
		rw.WriteHeader(http.StatusServiceUnavailable)
		return err
	}
}

func main() {
	flag.Parse()

	// Запускаємо оновлення здорових серверів у фоновому режимі
	go updateHealthyServers()

	frontend := httptools.CreateServer(*port, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		dst, err := selectServer(r.URL.Path)
		if err != nil {
			http.Error(rw, "No healthy servers available", http.StatusServiceUnavailable)
			return
		}
		forward(dst, rw, r)
	}))

	log.Println("Starting load balancer...")
	log.Printf("Tracing support enabled: %t", *traceEnabled)
	frontend.Start()
	signal.WaitForTerminationSignal()
}

