package handlers

import (
	"log/slog"
	"net/http"

	"github.com/magosproject/magos/api/internal/service"
)

// VariableSetHandler handles HTTP requests for VariableSet resources.
type VariableSetHandler struct {
	logger  *slog.Logger
	service service.VariableSetService
}

// NewVariableSetHandler creates a new VariableSetHandler.
func NewVariableSetHandler(logger *slog.Logger, svc service.VariableSetService) *VariableSetHandler {
	return &VariableSetHandler{logger: logger, service: svc}
}

// List returns all VariableSet resources across all namespaces.
func (h *VariableSetHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.List(r.Context())
	if err != nil {
		h.logger.Error("failed to list variablesets", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list variablesets")
		return
	}

	writeJSON(w, http.StatusOK, list)
}

// Get returns a single VariableSet resource by namespace and name.
func (h *VariableSetHandler) Get(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	if namespace == "" || name == "" {
		writeError(w, http.StatusBadRequest, "namespace and name are required")
		return
	}

	variableSet, err := h.service.Get(r.Context(), namespace, name)
	if err != nil {
		h.logger.Error("failed to get variableset", "error", err, "namespace", namespace, "name", name)
		writeError(w, http.StatusNotFound, "variableset not found")
		return
	}

	writeJSON(w, http.StatusOK, variableSet)
}
