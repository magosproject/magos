package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/magosproject/magos/api/internal/service"
)

// WorkspaceHandler handles HTTP requests for Workspace resources.
type WorkspaceHandler struct {
	logger  *slog.Logger
	service service.WorkspaceService
}

// NewWorkspaceHandler creates a new WorkspaceHandler.
func NewWorkspaceHandler(logger *slog.Logger, svc service.WorkspaceService) *WorkspaceHandler {
	return &WorkspaceHandler{logger: logger, service: svc}
}

// List returns all Workspace resources across all namespaces.
func (h *WorkspaceHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.List(r.Context())
	if err != nil {
		h.logger.Error("failed to list workspaces", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list workspaces")
		return
	}

	writeJSON(w, http.StatusOK, list)
}

// Get returns a single Workspace resource by namespace and name.
func (h *WorkspaceHandler) Get(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	if namespace == "" || name == "" {
		writeError(w, http.StatusBadRequest, "namespace and name are required")
		return
	}

	workspace, err := h.service.Get(r.Context(), namespace, name)
	if err != nil {
		h.logger.Error("failed to get workspace", "error", err, "namespace", namespace, "name", name)
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}

	writeJSON(w, http.StatusOK, workspace)
}

// Events streams Workspace resource changes as Server-Sent Events.
func (h *WorkspaceHandler) Events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	events := h.service.Watch(r.Context())
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
