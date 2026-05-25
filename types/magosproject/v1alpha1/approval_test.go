package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsApprovalPending(t *testing.T) {
	cases := []struct {
		name string
		ws   *Workspace
		want bool
	}{
		{name: "nil workspace", ws: nil, want: false},
		{
			name: "autoApply true never pending",
			ws: &Workspace{
				Spec:   WorkspaceSpec{AutoApply: true},
				Status: WorkspaceStatus{Phase: PhasePlanned},
			},
			want: false,
		},
		{
			name: "wrong phase",
			ws: &Workspace{
				Spec:   WorkspaceSpec{AutoApply: false},
				Status: WorkspaceStatus{Phase: PhasePlanning},
			},
			want: false,
		},
		{
			name: "planned and parked",
			ws: &Workspace{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       WorkspaceSpec{AutoApply: false},
				Status:     WorkspaceStatus{Phase: PhasePlanned},
			},
			want: true,
		},
		{
			name: "decision in flight (approved)",
			ws: &Workspace{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
					WorkspaceApprovalDecisionAnnotation: ApprovalDecisionApproved,
				}},
				Spec:   WorkspaceSpec{AutoApply: false},
				Status: WorkspaceStatus{Phase: PhasePlanned},
			},
			want: false,
		},
		{
			name: "decision in flight (rejected)",
			ws: &Workspace{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
					WorkspaceApprovalDecisionAnnotation: ApprovalDecisionRejected,
				}},
				Spec:   WorkspaceSpec{AutoApply: false},
				Status: WorkspaceStatus{Phase: PhasePlanned},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsApprovalPending(tc.ws))
		})
	}
}
