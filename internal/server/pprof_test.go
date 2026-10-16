package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPprofMuxServesGoroutineProfile(t *testing.T) {
	server := NewPprofServer(0)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/debug/pprof/goroutine?debug=2", nil)

	server.server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), "goroutine ") {
		t.Fatalf("response does not look like a goroutine profile: %q", response.Body.String())
	}
}

func TestPprofServerStartsAndStops(t *testing.T) {
	server := NewPprofServer(0)
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Stop()

	response, err := http.Get("http://" + server.listener.Addr().String() + "/debug/pprof/goroutine?debug=2")
	if err != nil {
		t.Fatalf("GET goroutine profile: %v", err)
	}
	body, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil {
		t.Fatalf("read response: %v", readErr)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if !strings.Contains(string(body), "goroutine ") {
		t.Fatalf("response does not look like a goroutine profile: %q", body)
	}

	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if _, err := http.Get("http://" + server.listener.Addr().String() + "/debug/pprof/goroutine?debug=2"); err == nil {
		t.Fatal("GET after Stop() succeeded, want connection error")
	}
}
