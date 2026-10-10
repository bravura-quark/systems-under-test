package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthReturnsOK(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)

	NewHandler(Config{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if recorder.Body.String() != `{"status":"ok"}`+"\n" {
		t.Fatalf("body = %q, want health response", recorder.Body.String())
	}
}

func TestIOReturnsDelayedSuccess(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/io?delay=0", nil)

	NewHandler(Config{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != "ok" || payload["delay_ms"] != float64(0) {
		t.Fatalf("payload = %#v, want successful zero-delay response", payload)
	}
}

func TestIORejectsInvalidDelay(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/io?delay=not-a-number", nil)

	NewHandler(Config{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestIOStopsWhenRequestIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/io?delay=50", nil).WithContext(ctx)

	NewHandler(Config{}).ServeHTTP(recorder, request)

	if recorder.Body.Len() != 0 {
		t.Fatalf("body = %q, want no response after cancellation", recorder.Body.String())
	}
}

func TestErrorModeReturnsServiceUnavailable(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/io?delay=50", nil)

	NewHandler(Config{Mode: "error"}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestSlowModeCanBeCancelledBeforeCompletion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/io?delay=0", nil).WithContext(ctx)

	NewHandler(Config{Mode: "slow"}).ServeHTTP(recorder, request)

	if recorder.Body.Len() != 0 {
		t.Fatalf("body = %q, want no response after slow request cancellation", recorder.Body.String())
	}
}

func TestCapacityModeRejectsWhenLimitIsFull(t *testing.T) {
	handler := NewHandler(Config{Mode: "capacity", MaxInFlight: 1})
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/io?delay=100", nil))
	}()
	time.Sleep(10 * time.Millisecond)

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/io?delay=0", nil))
	if second.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", second.Code, http.StatusServiceUnavailable)
	}
	<-firstDone
}
