package handlers

import (
	"log/slog"
	"net/http"

	"github.com/magosproject/magos/api/internal/service"
)

// RolloutHandler handles HTTP requests for Rollout resources.
type RolloutHandler struct {
	logger  *slog.Logger
	service service.RolloutService
}

// NewRolloutHandler creates a new RolloutHandler.
func NewRolloutHandler(logger *slog.Logger, svc service.RolloutService) *RolloutHandler {
	return &RolloutHandler{logger: logger, service: svc}
}

// List returns all Rollout resources across all namespaces.
func (h *RolloutHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.List(r.Context())
	if err != nil {
		h.logger.Error("failed to list rollouts", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list rollouts")
		return
	}

	writeJSON(w, http.StatusOK, list)
}

// Get returns a single Rollout resource by namespace and name.
func (h *RolloutHandler) Get(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	if namespace == "" || name == "" {
		writeError(w, http.StatusBadRequest, "namespace and name are required")
		return
	}

	rollout, err := h.service.Get(r.Context(), namespace, name)
	if err != nil {
		h.logger.Error("failed to get rollout", "error", err, "namespace", namespace, "name", name)
		writeError(w, http.StatusNotFound, "rollout not found")
		return
	}

	writeJSON(w, http.StatusOK, rollout)
}
