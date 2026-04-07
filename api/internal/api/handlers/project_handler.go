package handlers

import (
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

// List godoc
//
//	@Summary	List Project resources
//	@Tags		Project
//	@Produce	json
//	@Success	200	{array}		Project
//	@Failure	500	{object}	ErrorResponse
//	@Router		/apis/magosproject.io/v1alpha1/projects [get]
func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.List(r.Context())
	if err != nil {
		h.logger.Error("failed to list projects", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}

	writeJSON(w, http.StatusOK, list)
}

// Get godoc
//
//	@Summary	Get Project resource
//	@Tags		Project
//	@Produce	json
//	@Param		namespace	path		string	true	"Namespace"
//	@Param		name		path		string	true	"Name"
//	@Success	200			{object}	Project
//	@Failure	400			{object}	ErrorResponse
//	@Failure	404			{object}	ErrorResponse
//	@Router		/apis/magosproject.io/v1alpha1/projects/{namespace}/{name} [get]
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

// Events godoc
//
//	@Summary		Stream Project events
//	@Description	Server-Sent Events stream of Project changes. Each event is a JSON-encoded ProjectEvent.
//	@Tags			Project
//	@Produce		text/event-stream
//	@Success		200	{object}	service.ProjectEvent
//	@Router			/apis/magosproject.io/v1alpha1/projects/events [get]
func (h *ProjectHandler) Events(w http.ResponseWriter, r *http.Request) {
	StreamSSE(w, r, h.service.Watch)
}
