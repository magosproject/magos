/*
Copyright 2026. The Magos Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package refwatcher

import (
	"context"
	"testing"

	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestIsApprovalPendingGate(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, v1alpha1.AddToScheme(scheme))

	ws := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "ws",
			Namespace:   "default",
			Annotations: map[string]string{},
		},
		Spec:   v1alpha1.WorkspaceSpec{AutoApply: false},
		Status: v1alpha1.WorkspaceStatus{Phase: v1alpha1.PhasePlanned},
	}

	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ws).Build()

	got := &v1alpha1.Workspace{}
	assert.NoError(t, cli.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "ws"}, got))
	assert.True(t, v1alpha1.IsApprovalPending(got), "parked workspace should be approval-pending")

	if got.Annotations == nil {
		got.Annotations = map[string]string{}
	}
	got.Annotations[v1alpha1.WorkspaceApprovalDecisionAnnotation] = v1alpha1.ApprovalDecisionApproved
	assert.False(t, v1alpha1.IsApprovalPending(got), "decision in flight should not be approval-pending")
}
