package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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

func TestDownstreamSuccessPreservesResponseContract(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/io" || r.URL.Query().Get("delay") != "50" {
			t.Errorf("downstream request = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","delay_ms":50}`))
	}))
	defer downstream.Close()

	handler := NewHandler(Config{
		RequestTimeout:    2 * time.Second,
		MaxDelay:          5000,
		DownstreamURL:     downstream.URL + "/io",
		DownstreamTimeout: 500 * time.Millisecond,
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/downstream?delay=50", nil))

	var payload struct {
		Status  string `json:"status"`
		DelayMS int    `json:"delay_ms"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != http.StatusOK || payload.Status != "ok" || payload.DelayMS != 50 {
		t.Fatalf("response = %d %q, want successful downstream contract", response.Code, response.Body.String())
	}
}

func TestDownstreamNonSuccessReturnsBadGateway(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer downstream.Close()

	handler := NewHandler(Config{DownstreamURL: downstream.URL + "/io"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/downstream?delay=50", nil))

	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadGateway)
	}
}

func TestDownstreamTimeoutReturnsBadGateway(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer downstream.Close()

	handler := NewHandler(Config{
		DownstreamURL:     downstream.URL + "/io",
		DownstreamTimeout: 10 * time.Millisecond,
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/downstream?delay=50", nil))

	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadGateway)
	}
}

func TestDownstreamCancellationStopsClientRequest(t *testing.T) {
	requestStarted := make(chan struct{})
	downstreamCancelled := make(chan struct{})
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-r.Context().Done()
		close(downstreamCancelled)
	}))
	defer downstream.Close()

	handler := NewHandler(Config{
		DownstreamURL:     downstream.URL + "/io",
		DownstreamTimeout: time.Second,
	})
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/downstream?delay=50", nil).WithContext(ctx)
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(response, request)
		close(done)
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("downstream request did not start")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler did not stop after request cancellation")
	}
	select {
	case <-downstreamCancelled:
	case <-time.After(time.Second):
		t.Fatal("downstream request was not cancelled")
	}
	if response.Body.Len() != 0 {
		t.Fatalf("body = %q, want no response after cancellation", response.Body.String())
	}
}

func TestFanoutMakesOneDownstreamRequestPerBranch(t *testing.T) {
	var requests atomic.Int32
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/io" || r.URL.Query().Get("delay") != "50" {
			t.Errorf("downstream request = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer downstream.Close()

	handler := NewHandler(Config{DownstreamURL: downstream.URL + "/io"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/fanout?delay=50&branches=4", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	if got := requests.Load(); got != 4 {
		t.Fatalf("downstream requests = %d, want 4", got)
	}
}

func TestFanoutRejectsInvalidBranchCounts(t *testing.T) {
	handler := NewHandler(Config{})
	for _, branches := range []string{"", "0", "17", "not-a-number"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/fanout?delay=50&branches="+branches, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("branches=%q status = %d, want %d", branches, response.Code, http.StatusBadRequest)
		}
	}
}

func TestFanoutDownstreamFailureReturnsBadGateway(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer downstream.Close()

	handler := NewHandler(Config{DownstreamURL: downstream.URL + "/io"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/fanout?delay=50&branches=4", nil))

	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadGateway)
	}
}

func TestFanoutCancellationStopsDownstreamRequests(t *testing.T) {
	started := make(chan struct{}, 4)
	cancelled := make(chan struct{}, 4)
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-r.Context().Done()
		cancelled <- struct{}{}
	}))
	defer downstream.Close()

	handler := NewHandler(Config{DownstreamURL: downstream.URL + "/io", DownstreamTimeout: time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/fanout?delay=50&branches=4", nil).WithContext(ctx)
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(response, request)
		close(done)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("fanout downstream request did not start")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("fanout handler did not stop after request cancellation")
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("fanout downstream request was not cancelled")
	}
	if response.Body.Len() != 0 {
		t.Fatalf("body = %q, want no response after cancellation", response.Body.String())
	}
}

func TestDiagnosticsHandlerServesPprofIndex(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	NewDiagnosticsHandler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("pprof index status = %d, body = %q", response.Code, response.Body.String())
	}
}
