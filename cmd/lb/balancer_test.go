package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Tests for health

func TestHealthFunction(t *testing.T) {
	result := health("server1:8080")
	assert.IsType(t, true, result)
}

func TestHealth_Positive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	assert.True(t, health(host))
}

func TestHealth_Negative(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	assert.False(t, health(host))
}

// Tests for Select
func TestSelectServer_ReturnsConsistentResult(t *testing.T) {
	mu.Lock()
	healthyServers = []string{"server1:8080", "server2:8080", "server3:8080"}
	mu.Unlock()

	req := httptest.NewRequest("GET", "/api/test?foo=bar", nil)
	s1, err1 := selectServer(req)
	s2, err2 := selectServer(req)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, s1, s2, "Same path and query should be routed to the same server")
}

func TestSelectServer_EmptyHealthyList(t *testing.T) {
	mu.Lock()
	healthyServers = []string{}
	mu.Unlock()

	req := httptest.NewRequest("GET", "/anything", nil)
	_, err := selectServer(req)
	assert.Error(t, err)
}

// Tests Forward
func TestForward_Success(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "success")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello!"))
	}))
	defer backend.Close()

	dst := strings.TrimPrefix(backend.URL, "http://")

	req := httptest.NewRequest("GET", "/some/path", nil)
	rr := httptest.NewRecorder()

	err := forward(dst, rr, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "success", rr.Header().Get("X-Test"))
	assert.Equal(t, "Hello!", rr.Body.String())
}

func TestForward_Failure(t *testing.T) {
	req := httptest.NewRequest("GET", "/some/path", nil)
	rr := httptest.NewRecorder()

	err := forward("localhost:9999", rr, req)

	assert.Error(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestUpdateHealthyServersOnce(t *testing.T) {
	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer s1.Close()

	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer s2.Close()

	serversPool = []string{
		strings.TrimPrefix(s1.URL, "http://"),
		strings.TrimPrefix(s2.URL, "http://"),
	}

	updateHealthyServersOnce()

	mu.RLock()
	defer mu.RUnlock()

	assert.Equal(t, 1, len(healthyServers))
	assert.Equal(t, strings.TrimPrefix(s1.URL, "http://"), healthyServers[0])
}
