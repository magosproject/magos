package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WatchFunc returns a channel of events bound to the given context's lifetime.
type WatchFunc[T any] func(ctx context.Context) <-chan T

// StreamSSE writes Server-Sent Events to w for all events from watchFn.
func StreamSSE[T any](w http.ResponseWriter, r *http.Request, watchFn WatchFunc[T]) {
	FilteredStreamSSE(w, r, watchFn, nil)
}

// FilteredStreamSSE is like StreamSSE but skips events where keep returns false.
// When keep is nil every event is sent. Each HTTP connection gets its own
// subscriber channel and filter goroutine, so concurrent users are independent.
func FilteredStreamSSE[T any](w http.ResponseWriter, r *http.Request, watchFn WatchFunc[T], keep func(T) bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	events := watchFn(r.Context())
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			if keep != nil && !keep(event) {
				continue
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			if _, err = fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprintf(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

