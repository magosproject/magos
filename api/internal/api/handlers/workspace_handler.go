package handlers

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"

	"github.com/magosproject/magos/api/internal/service"
	apiv1alpha1 "github.com/magosproject/magos/types/magosproject/v1alpha1"
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

// ListRuns godoc
//
//	@Summary	List runs for a Workspace
//	@Tags		Workspace
//	@Produce	json
//	@Param		namespace	path		string	true	"Namespace"
//	@Param		name		path		string	true	"Name"
//	@Param		limit		query		int		false	"Page size (default 20, max 100)"
//	@Param		cursor		query		string	false	"Pagination cursor from a previous response"
//	@Success	200			{object}	service.RunListResponse
//	@Failure	400			{object}	ErrorResponse
//	@Failure	404			{object}	ErrorResponse
//	@Router		/apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name}/runs [get]
func (h *WorkspaceHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	if namespace == "" || name == "" {
		writeError(w, http.StatusBadRequest, "namespace and name are required")
		return
	}

	limit, err := parseListLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	items, err := h.service.ListRuns(r.Context(), namespace, name, limit, r.URL.Query().Get("cursor"))
	if err != nil {
		h.logger.Error("failed to list workspace reconcile runs", "error", err, "namespace", namespace, "name", name)
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "workspace not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list workspace reconcile runs")
		return
	}

	writeJSON(w, http.StatusOK, items)
}

// GetRunPhaseLog godoc
//
//	@Summary	Get the archived log for one phase of a plan and apply run
//	@Tags		Workspace
//	@Produce	plain
//	@Param		namespace	path		string	true	"Namespace"
//	@Param		name		path		string	true	"Name"
//	@Param		runID		path		string	true	"Run ID"
//	@Param		phase		query		string	false	"Phase to retrieve: plan or apply (defaults to apply)"
//	@Success	200			{string}	string	"Plain text log content"
//	@Failure	400			{object}	ErrorResponse
//	@Failure	404			{object}	ErrorResponse
//	@Router		/apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name}/runs/{runID}/log [get]
func (h *WorkspaceHandler) GetRunPhaseLog(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	runID := r.PathValue("runID")
	if namespace == "" || name == "" || runID == "" {
		writeError(w, http.StatusBadRequest, "namespace, name, and runID are required")
		return
	}
	if !isValidRunID(runID) {
		writeError(w, http.StatusBadRequest, "invalid runID")
		return
	}

	body, err := h.service.GetRunPhaseLog(r.Context(), namespace, name, runID, parseRunPhase(r.URL.Query().Get("phase")))
	if err != nil {
		h.logger.Error("failed to get workspace run log", "error", err, "namespace", namespace, "name", name, "runID", runID)
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "workspace not found")
			return
		}
		writeError(w, http.StatusNotFound, "workspace run log not found")
		return
	}
	defer func() {
		_ = body.Close()
	}()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, body); err != nil {
		h.logger.Error("failed to stream workspace run log", "error", err, "namespace", namespace, "name", name, "runID", runID)
	}
}

// StreamCurrentRunLog godoc
//
//	@Summary	Stream live logs from the active phase of the in-progress plan and apply run
//	@Tags		Workspace
//	@Produce	text/event-stream
//	@Param		namespace	path		string	true	"Namespace"
//	@Param		name		path		string	true	"Name"
//	@Param		phase		query		string	false	"Phase to stream: plan or apply (defaults to apply)"
//	@Success	200			{object}	service.RunLogStreamEvent
//	@Failure	400			{object}	ErrorResponse
//	@Failure	404			{object}	ErrorResponse
//	@Router		/apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name}/runs/current/log/stream [get]
func (h *WorkspaceHandler) StreamCurrentRunLog(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	if namespace == "" || name == "" {
		writeError(w, http.StatusBadRequest, "namespace and name are required")
		return
	}

	if _, err := h.service.Get(r.Context(), namespace, name); err != nil {
		h.logger.Error("failed to get workspace for live log stream", "error", err, "namespace", namespace, "name", name)
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}

	phase := parseRunPhase(r.URL.Query().Get("phase"))
	StreamSSE(w, r, func(ctx context.Context) <-chan service.RunLogStreamEvent {
		return h.service.StreamCurrentRunLogs(ctx, namespace, name, phase)
	})
}

func parseRunPhase(raw string) apiv1alpha1.RunPhase {
	switch raw {
	case string(apiv1alpha1.RunPhasePlan):
		return apiv1alpha1.RunPhasePlan
	case string(apiv1alpha1.RunPhaseApply), "":
		return apiv1alpha1.RunPhaseApply
	default:
		return apiv1alpha1.RunPhaseApply
	}
}

// runIDPattern matches the format produced by newRunID: a UTC timestamp
// followed by 8 lowercase hex characters. Rejecting anything that doesn't
// match prevents path traversal via ".." segments in the S3 key.
var runIDPattern = regexp.MustCompile(`^\d{8}T\d{6}-[0-9a-f]{8}$`)

func isValidRunID(id string) bool {
	return runIDPattern.MatchString(id)
}

func parseListLimit(raw string) (int, error) {
	if raw == "" {
		return 20, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("limit must be a positive integer")
	}
	if value > 100 {
		return 100, nil
	}
	return value, nil
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
