package integration

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

const baseAddress = "http://balancer:8090"

var client = http.Client{
	Timeout: 3 * time.Second,
}

func TestBalancer(t *testing.T) {
	if _, exists := os.LookupEnv("INTEGRATION_TEST"); !exists {
		t.Skip("Integration test is not enabled")
	}

	const requestCount = 20
	serverHits := make(map[string]int)

	for i := 0; i < requestCount; i++ {
		resp, err := client.Get(fmt.Sprintf("%s/api/v1/some-data?%d", baseAddress, i))
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		defer resp.Body.Close()

		server := resp.Header.Get("lb-from")
		if server == "" {
			t.Errorf("No lb-from header in response %d", i)
		}
		serverHits[server]++
		t.Logf("Response %d from server: %s", i, server)
	}

	if len(serverHits) < 2 {
		t.Errorf("Expected responses from 2 or more servers, got from: %v", serverHits)
	} else {
		t.Logf("Requests distributed across servers: %v", serverHits)
	}
}

func BenchmarkBalancer(b *testing.B) {
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(fmt.Sprintf("%s/api/v1/some-data?%d", baseAddress, i))
		if err != nil {
			b.Errorf("Request failed: %v", err)
		} else {
			resp.Body.Close()
		}
	}
}
