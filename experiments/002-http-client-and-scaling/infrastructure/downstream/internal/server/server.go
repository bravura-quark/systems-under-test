package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const maxDelayMS = 5000

type Config struct {
	Mode        string
	MaxInFlight int
}

type Handler struct {
	config   Config
	capacity chan struct{}
}

func NewHandler(config Config) http.Handler {
	if config.Mode == "" {
		config.Mode = "success"
	}
	handler := &Handler{config: config}
	if config.Mode == "capacity" && config.MaxInFlight > 0 {
		handler.capacity = make(chan struct{}, config.MaxInFlight)
	}
	return handler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/health":
		handleHealth(w, r)
	case "/io":
		h.handleIO(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
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

	delay, err := parseDelay(r.URL.Query().Get("delay"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if h.config.Mode == "error" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "error", "delay_ms": delay})
		return
	}

	if h.capacity != nil {
		select {
		case h.capacity <- struct{}{}:
			defer func() { <-h.capacity }()
		default:
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "over_capacity", "delay_ms": delay})
			return
		}
	}
	if h.config.Mode == "slow" {
		delay += 1000
	}

	timer := time.NewTimer(time.Duration(delay) * time.Millisecond)
	defer stopTimer(timer)

	select {
	case <-timer.C:
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "delay_ms": delay})
	case <-r.Context().Done():
		return
	}
}

func parseDelay(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, &invalidDelayError{"delay is required"}
	}
	delay, err := strconv.Atoi(raw)
	if err != nil {
		return 0, &invalidDelayError{"delay must be an integer"}
	}
	if delay < 0 || delay > maxDelayMS {
		return 0, &invalidDelayError{"delay must be between 0 and " + strconv.Itoa(maxDelayMS) + " ms"}
	}
	return delay, nil
}

type invalidDelayError struct{ message string }

func (e *invalidDelayError) Error() string { return e.message }

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
