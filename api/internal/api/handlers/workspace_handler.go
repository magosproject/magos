package handlers

import (
	"log/slog"
	"net/http"

	"github.com/magosproject/magos/api/internal/service"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// RequestReconcile godoc
//
//	@Summary	Request Workspace reconcile
//	@Tags		Workspace
//	@Param		namespace	path		string	true	"Namespace"
//	@Param		name		path		string	true	"Name"
//	@Success	200			{object}	Workspace
//	@Failure	400			{object}	ErrorResponse
//	@Failure	404			{object}	ErrorResponse
//	@Failure	500			{object}	ErrorResponse
//	@Router		/apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name}/reconcile [post]
func (h *WorkspaceHandler) RequestReconcile(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	if namespace == "" || name == "" {
		writeError(w, http.StatusBadRequest, "namespace and name are required")
		return
	}

	workspace, err := h.service.RequestReconcile(r.Context(), namespace, name)
	if err != nil {
		h.logger.Error("failed to request workspace reconcile", "error", err, "namespace", namespace, "name", name)
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "workspace not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to request workspace reconcile")
		return
	}

	writeJSON(w, http.StatusOK, workspace)
}

// Events godoc
//
//	@Summary		Stream Workspace events
//	@Description	Server-Sent Events stream of Workspace changes. Each event is a JSON-encoded WorkspaceEvent. Use ?projectRef=name to filter by project.
//	@Tags			Workspace
//	@Produce		text/event-stream
//	@Param			projectRef	query		string	false	"Filter by project name"
//	@Success		200			{object}	service.WorkspaceEvent
//	@Router			/apis/magosproject.io/v1alpha1/workspaces/events [get]
func (h *WorkspaceHandler) Events(w http.ResponseWriter, r *http.Request) {
	projectRef := r.URL.Query().Get("projectRef")
	if projectRef == "" {
		StreamSSE(w, r, h.service.Watch)
		return
	}

	FilteredStreamSSE(w, r, h.service.Watch, func(e service.WorkspaceEvent) bool {
		return e.Object != nil && e.Object.Spec.ProjectRef.Name == projectRef
	})
}
