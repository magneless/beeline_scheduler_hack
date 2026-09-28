package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/progress"
)

type calculationResult struct {
	value any
	err   error
}

var calculationHeartbeatInterval = 2 * time.Second

// respondCalculation preserves JSON responses unless the client requests SSE.
// Only this goroutine writes to ResponseWriter; the worker reports through a channel.
func respondCalculation(w http.ResponseWriter, r *http.Request, status int, total int, calculate func(context.Context) (any, error)) {
	if !strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream") {
		value, err := calculate(r.Context())
		respond(w, status, value, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		failure(w, invalid("Потоковый ответ недоступен"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	writeSSE(w, flusher, "progress", progress.Update{Stage: "preparing", Message: "Подготавливаем данные", Completed: 0, Total: total})
	updates := make(chan progress.Update, 16)
	finished := make(chan calculationResult, 1)
	ctx := progress.WithReporter(r.Context(), func(update progress.Update) {
		select {
		case updates <- update:
		case <-r.Context().Done():
		}
	})
	go func() {
		value, err := calculate(ctx)
		finished <- calculationResult{value: value, err: err}
	}()
	heartbeat := time.NewTicker(calculationHeartbeatInterval)
	defer heartbeat.Stop()
	for {
		select {
		case update := <-updates:
			writeSSE(w, flusher, "progress", update)
		case result := <-finished:
			for len(updates) > 0 {
				writeSSE(w, flusher, "progress", <-updates)
			}
			if result.err != nil {
				_, ce := errorResponse(result.err)
				writeSSE(w, flusher, "error", ce)
			} else {
				writeSSE(w, flusher, "result", result.value)
			}
			return
		case <-heartbeat.C:
			writeSSE(w, flusher, "heartbeat", struct{}{})
		case <-r.Context().Done():
			return
		}
	}
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	flusher.Flush()
}
