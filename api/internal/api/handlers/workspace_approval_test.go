package handlers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/magosproject/magos/api/internal/runs"
	"github.com/magosproject/magos/api/internal/service"
	apiv1alpha1 "github.com/magosproject/magos/types/magosproject/v1alpha1"
	"github.com/stretchr/testify/assert"
)

type stubService struct {
	service.WorkspaceService
	approveErr error
	rejectErr  error
	gotNS      string
	gotName    string
	gotRunID   string
	gotReason  string
	returnedWS *apiv1alpha1.Workspace
}

func (s *stubService) Approve(_ context.Context, ns, name, runID, reason string) (*apiv1alpha1.Workspace, error) {
	s.gotNS, s.gotName, s.gotRunID, s.gotReason = ns, name, runID, reason
	if s.approveErr != nil {
		return nil, s.approveErr
	}
	return s.returnedWS, nil
}

func (s *stubService) Reject(_ context.Context, ns, name, runID, reason string) (*apiv1alpha1.Workspace, error) {
	s.gotNS, s.gotName, s.gotRunID, s.gotReason = ns, name, runID, reason
	if s.rejectErr != nil {
		return nil, s.rejectErr
	}
	return s.returnedWS, nil
}

func newApprovalHandler(svc service.WorkspaceService) *WorkspaceHandler {
	return NewWorkspaceHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), svc)
}

func newApprovalRequest(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestApproveRun_HappyPath(t *testing.T) {
	svc := &stubService{returnedWS: &apiv1alpha1.Workspace{}}
	h := newApprovalHandler(svc)

	req := newApprovalRequest(t, http.MethodPost,
		"/apis/magosproject.io/v1alpha1/workspaces/ns/ws/runs/r1/approve",
		`{"reason":"lgtm"}`)
	req.SetPathValue("namespace", "ns")
	req.SetPathValue("name", "ws")
	req.SetPathValue("runID", "r1")
	rr := httptest.NewRecorder()

	h.ApproveRun(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "ns", svc.gotNS)
	assert.Equal(t, "ws", svc.gotName)
	assert.Equal(t, "r1", svc.gotRunID)
	assert.Equal(t, "lgtm", svc.gotReason)
}

func TestApproveRun_EmptyBody(t *testing.T) {
	svc := &stubService{returnedWS: &apiv1alpha1.Workspace{}}
	h := newApprovalHandler(svc)

	req := httptest.NewRequest(http.MethodPost,
		"/apis/magosproject.io/v1alpha1/workspaces/ns/ws/runs/r1/approve",
		bytes.NewReader(nil))
	req.SetPathValue("namespace", "ns")
	req.SetPathValue("name", "ws")
	req.SetPathValue("runID", "r1")
	rr := httptest.NewRecorder()

	h.ApproveRun(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "", svc.gotReason)
}

func TestRejectRun_ReasonRequired(t *testing.T) {
	svc := &stubService{rejectErr: service.ErrReasonRequired}
	h := newApprovalHandler(svc)

	req := newApprovalRequest(t, http.MethodPost,
		"/apis/magosproject.io/v1alpha1/workspaces/ns/ws/runs/r1/reject", `{}`)
	req.SetPathValue("namespace", "ns")
	req.SetPathValue("name", "ws")
	req.SetPathValue("runID", "r1")
	rr := httptest.NewRecorder()

	h.RejectRun(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDecide_ErrorMapping(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		approve  bool
		wantCode int
	}{
		{"approval not pending", service.ErrApprovalNotPending, true, http.StatusConflict},
		{"runID mismatch", service.ErrRunIDMismatch, true, http.StatusNotFound},
		{"reason too long", service.ErrReasonTooLong, false, http.StatusBadRequest},
		{"conflicting decision", runs.ErrConflictingDecision, true, http.StatusConflict},
		{"unknown", errors.New("boom"), true, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &stubService{}
			if tc.approve {
				svc.approveErr = tc.err
			} else {
				svc.rejectErr = tc.err
			}
			h := newApprovalHandler(svc)

			path := "/.../approve"
			method := h.ApproveRun
			if !tc.approve {
				path = "/.../reject"
				method = h.RejectRun
			}
			req := newApprovalRequest(t, http.MethodPost, path, `{"reason":"x"}`)
			req.SetPathValue("namespace", "ns")
			req.SetPathValue("name", "ws")
			req.SetPathValue("runID", "r1")
			rr := httptest.NewRecorder()
			method(rr, req)

			assert.Equal(t, tc.wantCode, rr.Code, "expected %d for %s", tc.wantCode, tc.name)
		})
	}
}
