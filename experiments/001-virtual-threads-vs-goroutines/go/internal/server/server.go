package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	RequestTimeout time.Duration
	MaxDelay       int
}

type Handler struct {
	config Config
}

func NewHandler(config Config) http.Handler {
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 2 * time.Second
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = 5000
	}
	return &Handler{config: config}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/health":
		h.handleHealth(w, r)
	case "/io":
		h.handleIO(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
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

type invalidDelayError struct {
	message string
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
