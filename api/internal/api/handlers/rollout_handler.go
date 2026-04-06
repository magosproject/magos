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

// List godoc
//
//	@Summary	List Rollout resources
//	@Tags		Rollout
//	@Produce	json
//	@Success	200	{array}		Rollout
//	@Failure	500	{object}	ErrorResponse
//	@Router		/apis/magosproject.io/v1alpha1/rollouts [get]
func (h *RolloutHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.List(r.Context())
	if err != nil {
		h.logger.Error("failed to list rollouts", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list rollouts")
		return
	}

	writeJSON(w, http.StatusOK, list)
}

// Get godoc
//
//	@Summary	Get Rollout resource
//	@Tags		Rollout
//	@Produce	json
//	@Param		namespace	path		string	true	"Namespace"
//	@Param		name		path		string	true	"Name"
//	@Success	200			{object}	Rollout
//	@Failure	400			{object}	ErrorResponse
//	@Failure	404			{object}	ErrorResponse
//	@Router		/apis/magosproject.io/v1alpha1/rollouts/{namespace}/{name} [get]
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
