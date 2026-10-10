package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseDelay(t *testing.T) {
	if got, err := parseDelay("50", 5000); err != nil || got != 50 {
		t.Fatalf("parseDelay(50) = %d, %v; want 50, nil", got, err)
	}
	for _, raw := range []string{"", "not-a-number", "-1", "5001"} {
		if _, err := parseDelay(raw, 5000); err == nil {
			t.Fatalf("parseDelay(%q) returned nil error", raw)
		}
	}
}

func TestHealthAndIOSuccessHaveStableJSON(t *testing.T) {
	handler := NewHandler(Config{RequestTimeout: 2 * time.Second, MaxDelay: 5000})

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/health", nil))
	if health.Code != http.StatusOK || health.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("health response = %d, %q", health.Code, health.Body.String())
	}
	var healthBody map[string]any
	if err := json.Unmarshal(health.Body.Bytes(), &healthBody); err != nil || healthBody["status"] != "ok" {
		t.Fatalf("health JSON = %q, error = %v", health.Body.String(), err)
	}

	io := httptest.NewRecorder()
	handler.ServeHTTP(io, httptest.NewRequest(http.MethodGet, "/io?delay=50", nil))
	if io.Code != http.StatusOK {
		t.Fatalf("io status = %d, body = %q", io.Code, io.Body.String())
	}
	var ioBody struct {
		Status  string `json:"status"`
		DelayMS int    `json:"delay_ms"`
	}
	if err := json.Unmarshal(io.Body.Bytes(), &ioBody); err != nil || ioBody.Status != "ok" || ioBody.DelayMS != 50 {
		t.Fatalf("io JSON = %q, error = %v", io.Body.String(), err)
	}
}

func TestIOCancellationStopsWaiting(t *testing.T) {
	handler := NewHandler(Config{RequestTimeout: 2 * time.Second, MaxDelay: 5000})
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/io?delay=5000", nil).WithContext(ctx)
	response := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(response, request)
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handler did not stop after request cancellation")
	}
}

func TestIOTimeoutReturnsGatewayTimeout(t *testing.T) {
	handler := NewHandler(Config{RequestTimeout: 20 * time.Millisecond, MaxDelay: 5000})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/io?delay=50", nil))
	if response.Code != http.StatusGatewayTimeout {
		t.Fatalf("timeout status = %d, body = %q", response.Code, response.Body.String())
	}
}
