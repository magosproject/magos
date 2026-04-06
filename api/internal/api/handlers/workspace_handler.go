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

// List godoc
//
//	@Summary	List Workspace resources
//	@Tags		Workspace
//	@Produce	json
//	@Success	200	{array}		Workspace
//	@Failure	500	{object}	ErrorResponse
//	@Router		/apis/magosproject.io/v1alpha1/workspaces [get]
func (h *WorkspaceHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.List(r.Context())
	if err != nil {
		h.logger.Error("failed to list workspaces", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list workspaces")
		return
	}

	writeJSON(w, http.StatusOK, list)
}

// Get godoc
//
//	@Summary	Get Workspace resource
//	@Tags		Workspace
//	@Produce	json
//	@Param		namespace	path		string	true	"Namespace"
//	@Param		name		path		string	true	"Name"
//	@Success	200			{object}	Workspace
//	@Failure	400			{object}	ErrorResponse
//	@Failure	404			{object}	ErrorResponse
//	@Router		/apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name} [get]
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

// Events godoc
//
//	@Summary		Stream Workspace events
//	@Description	Server-Sent Events stream of Workspace changes. Each event is a JSON-encoded WorkspaceEvent.
//	@Tags			Workspace
//	@Produce		text/event-stream
//	@Success		200	{object}	service.WorkspaceEvent
//	@Router			/apis/magosproject.io/v1alpha1/workspaces/events [get]
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
