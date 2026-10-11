package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	RequestTimeout            time.Duration
	MaxDelay                  int
	DownstreamURL             string
	DownstreamTimeout         time.Duration
	FanoutMaxInFlight         int
	FanoutMaxBranchesInFlight int
}

type Handler struct {
	config           Config
	downstreamClient *http.Client
	fanoutAdmission  chan struct{}
	branchAdmission  chan struct{}
}

func NewHandler(config Config) http.Handler {
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 2 * time.Second
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = 5000
	}
	if config.DownstreamURL == "" {
		config.DownstreamURL = "http://downstream:8080/io"
	}
	if config.DownstreamTimeout <= 0 {
		config.DownstreamTimeout = 500 * time.Millisecond
	}
	if config.FanoutMaxInFlight <= 0 {
		config.FanoutMaxInFlight = 256
	}
	if config.FanoutMaxBranchesInFlight <= 0 {
		config.FanoutMaxBranchesInFlight = 256
	}
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     30 * time.Second,
		ForceAttemptHTTP2:   false,
	}
	return &Handler{
		config: config,
		downstreamClient: &http.Client{
			Transport: transport,
			Timeout:   config.DownstreamTimeout,
		},
		fanoutAdmission: make(chan struct{}, config.FanoutMaxInFlight),
		branchAdmission: make(chan struct{}, config.FanoutMaxBranchesInFlight),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/health":
		h.handleHealth(w, r)
	case "/io":
		h.handleIO(w, r)
	case "/downstream":
		h.handleDownstream(w, r)
	case "/fanout":
		h.handleFanout(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (h *Handler) handleFanout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	delay, err := parseDelay(r.URL.Query().Get("delay"), h.config.MaxDelay)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	branches, err := parseBranches(r.URL.Query().Get("branches"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	select {
	case h.fanoutAdmission <- struct{}{}:
		defer func() { <-h.fanoutAdmission }()
	default:
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":   "capacity",
			"delay_ms": delay,
		})
		return
	}
	if !tryAcquireBranches(h.branchAdmission, branches) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":   "capacity",
			"delay_ms": delay,
		})
		return
	}
	defer releaseBranches(h.branchAdmission, branches)

	ctx, cancel := context.WithTimeout(r.Context(), h.config.RequestTimeout)
	defer cancel()
	results := make(chan error, branches)
	var group sync.WaitGroup
	group.Add(branches)
	for i := 0; i < branches; i++ {
		go func() {
			defer group.Done()
			target, targetErr := downstreamTarget(h.config.DownstreamURL, delay)
			if targetErr != nil {
				results <- targetErr
				cancel()
				return
			}
			request, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
			if requestErr != nil {
				results <- requestErr
				cancel()
				return
			}
			response, requestErr := h.downstreamClient.Do(request)
			if requestErr == nil {
				_, _ = io.Copy(io.Discard, response.Body)
				_ = response.Body.Close()
				if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
					requestErr = &downstreamStatusError{statusCode: response.StatusCode}
				}
			}
			results <- requestErr
			if requestErr != nil {
				cancel()
			}
		}()
	}
	group.Wait()
	close(results)

	if r.Context().Err() != nil {
		return
	}
	for branchErr := range results {
		if branchErr != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"status": "bad_gateway", "delay_ms": delay})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "delay_ms": delay})
}

func tryAcquireBranches(admission chan struct{}, branches int) bool {
	acquired := 0
	for acquired < branches {
		select {
		case admission <- struct{}{}:
			acquired++
		default:
			releaseBranches(admission, acquired)
			return false
		}
	}
	return true
}

func releaseBranches(admission chan struct{}, branches int) {
	for i := 0; i < branches; i++ {
		<-admission
	}
}

type downstreamStatusError struct {
	statusCode int
}

func (e *downstreamStatusError) Error() string {
	return "downstream returned status " + strconv.Itoa(e.statusCode)
}

func (h *Handler) handleDownstream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	delay, err := parseDelay(r.URL.Query().Get("delay"), h.config.MaxDelay)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	target, err := downstreamTarget(h.config.DownstreamURL, delay)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"status": "bad_gateway", "delay_ms": delay})
		return
	}
	requestContext, cancel := context.WithTimeout(r.Context(), h.config.RequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, target, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"status": "bad_gateway", "delay_ms": delay})
		return
	}

	response, err := h.downstreamClient.Do(request)
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]any{"status": "bad_gateway", "delay_ms": delay})
		return
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		writeJSON(w, http.StatusBadGateway, map[string]any{"status": "bad_gateway", "delay_ms": delay})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "delay_ms": delay})
}

func downstreamTarget(base string, delay int) (string, error) {
	target, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	query := target.Query()
	query.Set("delay", strconv.Itoa(delay))
	target.RawQuery = query.Encode()
	return target.String(), nil
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleIO(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	delay, err := parseDelay(r.URL.Query().Get("delay"), h.config.MaxDelay)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.config.RequestTimeout)
	defer cancel()

	timer := time.NewTimer(time.Duration(delay) * time.Millisecond)
	defer stopTimer(timer)

	select {
	case <-timer.C:
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "delay_ms": delay})
	case <-ctx.Done():
		// A client disconnect is cancellation, not a gateway timeout response.
		if r.Context().Err() != nil {
			return
		}
		writeJSON(w, http.StatusGatewayTimeout, map[string]any{"status": "timeout", "delay_ms": delay})
	}
}

func parseDelay(raw string, maxDelay int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, &invalidDelayError{"delay is required"}
	}
	delay, err := strconv.Atoi(raw)
	if err != nil {
		return 0, &invalidDelayError{"delay must be an integer"}
	}
	if delay < 0 || delay > maxDelay {
		return 0, &invalidDelayError{"delay must be between 0 and " + strconv.Itoa(maxDelay) + " ms"}
	}
	return delay, nil
}

func parseBranches(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, &invalidBranchesError{"branches is required"}
	}
	branches, err := strconv.Atoi(raw)
	if err != nil {
		return 0, &invalidBranchesError{"branches must be an integer"}
	}
	if branches < 1 || branches > 16 {
		return 0, &invalidBranchesError{"branches must be between 1 and 16"}
	}
	return branches, nil
}

type invalidDelayError struct {
	message string
}

type invalidBranchesError struct {
	message string
}

func (e *invalidBranchesError) Error() string {
	return e.message
}

func (e *invalidDelayError) Error() string {
	return e.message
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
