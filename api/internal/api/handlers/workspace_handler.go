package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"

	"github.com/magosproject/magos/api/internal/runs"
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
//	@Security	CookieAuth
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
//	@Security	CookieAuth
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

// Patch godoc
//
//	@Summary	Patch a Workspace resource
//	@Tags		Workspace
//	@Security	CookieAuth
//	@Accept		json
//	@Produce	json
//	@Param		namespace	path		string					true	"Namespace"
//	@Param		name		path		string					true	"Name"
//	@Param		body		body		service.WorkspacePatch	true	"Fields to patch"
//	@Success	200			{object}	Workspace
//	@Failure	400			{object}	ErrorResponse
//	@Failure	404			{object}	ErrorResponse
//	@Failure	500			{object}	ErrorResponse
//	@Router		/apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name} [patch]
func (h *WorkspaceHandler) Patch(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	if namespace == "" || name == "" {
		writeError(w, http.StatusBadRequest, "namespace and name are required")
		return
	}

	var patch service.WorkspacePatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := patch.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	workspace, err := h.service.Patch(r.Context(), namespace, name, patch)
	if err != nil {
		h.logger.Error("failed to patch workspace", "error", err, "namespace", namespace, "name", name)
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "workspace not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to patch workspace")
		return
	}

	writeJSON(w, http.StatusOK, workspace)
}

// RequestReconcile godoc
//
//	@Summary	Request Workspace reconcile
//	@Tags		Workspace
//	@Security	CookieAuth
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
//	@Security	CookieAuth
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
		if errors.Is(err, runs.ErrInvalidCursor) {
			writeError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
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
//	@Security	CookieAuth
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

// ApprovalDecisionRequest is the request body for the approve and reject
// endpoints. Reason is required when rejecting and optional when approving.
type ApprovalDecisionRequest struct {
	Reason string `json:"reason,omitempty"`
}

type recordRunPhaseRequest struct {
	Run apiv1alpha1.Run `json:"run"`
}

func (h *WorkspaceHandler) RecordRun(w http.ResponseWriter, r *http.Request) {
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

	var payload recordRunPhaseRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if payload.Run.ID != runID {
		writeError(w, http.StatusBadRequest, "runID in path and payload do not match")
		return
	}

	if err := h.service.RecordRun(r.Context(), namespace, name, payload.Run); err != nil {
		h.logger.Error("failed to record workspace run", "error", err, "namespace", namespace, "name", name, "runID", runID)
		writeError(w, http.StatusInternalServerError, "failed to record workspace run")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *WorkspaceHandler) RecordRunPhase(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	runID := r.PathValue("runID")
	phase, ok := parseRequiredRunPhase(r.PathValue("phase"))
	if namespace == "" || name == "" || runID == "" {
		writeError(w, http.StatusBadRequest, "namespace, name, and runID are required")
		return
	}
	if !isValidRunID(runID) {
		writeError(w, http.StatusBadRequest, "invalid runID")
		return
	}
	if !ok {
		writeError(w, http.StatusBadRequest, "phase must be plan or apply")
		return
	}

	var payload recordRunPhaseRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if payload.Run.ID != runID {
		writeError(w, http.StatusBadRequest, "runID in path and payload do not match")
		return
	}
	if phase == apiv1alpha1.RunPhasePlan && payload.Run.Plan == nil {
		writeError(w, http.StatusBadRequest, "plan phase summary is required")
		return
	}
	if phase == apiv1alpha1.RunPhaseApply && payload.Run.Apply == nil {
		writeError(w, http.StatusBadRequest, "apply phase summary is required")
		return
	}

	if err := h.service.RecordRunPhase(r.Context(), namespace, name, runID, phase, payload.Run); err != nil {
		h.logger.Error("failed to record workspace run phase", "error", err, "namespace", namespace, "name", name, "runID", runID, "phase", phase)
		writeError(w, http.StatusInternalServerError, "failed to record workspace run phase")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// StreamCurrentRunLog godoc
//
//	@Summary	Stream live logs from the in-progress plan and apply run
//	@Tags		Workspace
//	@Security	CookieAuth
//	@Produce	text/event-stream
//	@Param		namespace	path		string	true	"Namespace"
//	@Param		name		path		string	true	"Name"
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

	StreamSSE(w, r, func(ctx context.Context) <-chan service.RunLogStreamEvent {
		return h.service.StreamCurrentRunLogs(ctx, namespace, name)
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

func parseRequiredRunPhase(raw string) (apiv1alpha1.RunPhase, bool) {
	switch raw {
	case string(apiv1alpha1.RunPhasePlan):
		return apiv1alpha1.RunPhasePlan, true
	case string(apiv1alpha1.RunPhaseApply):
		return apiv1alpha1.RunPhaseApply, true
	default:
		return "", false
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

// ApproveRun godoc
//
//	@Summary	Approve a parked plan for a workspace
//	@Tags		Workspace
//	@Security	CookieAuth
//	@Accept		json
//	@Produce	json
//	@Param		namespace	path		string					true	"Namespace"
//	@Param		name		path		string					true	"Name"
//	@Param		runID		path		string					true	"Run ID"
//	@Param		body		body		ApprovalDecisionRequest	false	"Optional reason"
//	@Success	200			{object}	Workspace
//	@Failure	400			{object}	ErrorResponse
//	@Failure	404			{object}	ErrorResponse
//	@Failure	409			{object}	ErrorResponse
//	@Failure	500			{object}	ErrorResponse
//	@Router		/apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name}/runs/{runID}/approve [post]
func (h *WorkspaceHandler) ApproveRun(w http.ResponseWriter, r *http.Request) {
	// TODO: enforce approver identity when RBAC is introduced.
	h.decide(w, r, true)
}

// RejectRun godoc
//
//	@Summary	Reject a parked plan for a workspace
//	@Tags		Workspace
//	@Security	CookieAuth
//	@Accept		json
//	@Produce	json
//	@Param		namespace	path		string					true	"Namespace"
//	@Param		name		path		string					true	"Name"
//	@Param		runID		path		string					true	"Run ID"
//	@Param		body		body		ApprovalDecisionRequest	true	"Reason (required)"
//	@Success	200			{object}	Workspace
//	@Failure	400			{object}	ErrorResponse
//	@Failure	404			{object}	ErrorResponse
//	@Failure	409			{object}	ErrorResponse
//	@Failure	500			{object}	ErrorResponse
//	@Router		/apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name}/runs/{runID}/reject [post]
func (h *WorkspaceHandler) RejectRun(w http.ResponseWriter, r *http.Request) {
	// TODO: enforce approver identity when RBAC is introduced.
	h.decide(w, r, false)
}

func (h *WorkspaceHandler) decide(w http.ResponseWriter, r *http.Request, approve bool) {
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	runID := r.PathValue("runID")
	if namespace == "" || name == "" || runID == "" {
		writeError(w, http.StatusBadRequest, "namespace, name, and runID are required")
		return
	}

	var body ApprovalDecisionRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}

	var (
		ws  *apiv1alpha1.Workspace
		err error
	)
	if approve {
		ws, err = h.service.Approve(r.Context(), namespace, name, runID, body.Reason)
	} else {
		ws, err = h.service.Reject(r.Context(), namespace, name, runID, body.Reason)
	}

	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, ws)
	case errors.Is(err, service.ErrApprovalNotPending):
		writeError(w, http.StatusConflict, "no approval pending")
	case errors.Is(err, service.ErrRunIDMismatch):
		writeError(w, http.StatusNotFound, "run not found")
	case errors.Is(err, service.ErrReasonRequired):
		writeError(w, http.StatusBadRequest, "reason is required for reject")
	case errors.Is(err, service.ErrReasonTooLong):
		writeError(w, http.StatusBadRequest, "reason exceeds 1024 characters")
	case errors.Is(err, runs.ErrConflictingDecision):
		writeError(w, http.StatusConflict, "already decided")
	case apierrors.IsNotFound(err):
		writeError(w, http.StatusNotFound, "workspace not found")
	default:
		h.logger.Error("approval decision failed", "error", err, "namespace", namespace, "name", name, "runID", runID, "approve", approve)
		writeError(w, http.StatusInternalServerError, "approval decision failed")
	}
}

// Events godoc
//
//	@Summary		Stream Workspace events
//	@Description	Server-Sent Events stream of Workspace changes. Each event is a JSON-encoded WorkspaceEvent. Use ?namespace=ns&name=n to scope to a single workspace, or ?projectRef=name to filter by project.
//	@Tags			Workspace
//	@Security		CookieAuth
//	@Produce		text/event-stream
//	@Param			namespace	query		string	false	"Filter by namespace and name (both required)"
//	@Param			name		query		string	false	"Filter by namespace and name (both required)"
//	@Param			projectRef	query		string	false	"Filter by project name"
//	@Success		200			{object}	service.WorkspaceEvent
//	@Router			/apis/magosproject.io/v1alpha1/workspaces/events [get]
func (h *WorkspaceHandler) Events(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	name := r.URL.Query().Get("name")
	if namespace != "" && name != "" {
		FilteredStreamSSE(w, r, h.service.Watch, func(e service.WorkspaceEvent) bool {
			return e.Object != nil && e.Object.Namespace == namespace && e.Object.Name == name
		})
		return
	}

	projectRef := r.URL.Query().Get("projectRef")
	if projectRef != "" {
		FilteredStreamSSE(w, r, h.service.Watch, func(e service.WorkspaceEvent) bool {
			return e.Object != nil && e.Object.Spec.ProjectRef.Name == projectRef
		})
		return
	}

	StreamSSE(w, r, h.service.Watch)
}
