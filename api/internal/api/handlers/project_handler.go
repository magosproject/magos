package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/magosproject/magos/api/internal/service"
)

// ProjectHandler handles HTTP requests for Project resources.
type ProjectHandler struct {
	logger  *slog.Logger
	service service.ProjectService
}

// NewProjectHandler creates a new ProjectHandler.
func NewProjectHandler(logger *slog.Logger, svc service.ProjectService) *ProjectHandler {
	return &ProjectHandler{logger: logger, service: svc}
}

// List returns all Project resources across all namespaces.
func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.List(r.Context())
	if err != nil {
		h.logger.Error("failed to list projects", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}

	writeJSON(w, http.StatusOK, list)
}

// Get returns a single Project resource by namespace and name.
func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	if namespace == "" || name == "" {
		writeError(w, http.StatusBadRequest, "namespace and name are required")
		return
	}

	project, err := h.service.Get(r.Context(), namespace, name)
	if err != nil {
		h.logger.Error("failed to get project", "error", err, "namespace", namespace, "name", name)
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	writeJSON(w, http.StatusOK, project)
}

// Events streams Project resource changes as Server-Sent Events.
func (h *ProjectHandler) Events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for event := range h.service.Watch(r.Context()) {
		data, err := json.Marshal(event)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		_ = err
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
