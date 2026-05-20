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

package workspace

import (
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	envWorkspacePVCSizeDefault   = "MAGOS_WORKSPACE_PVC_SIZE_DEFAULT"
	envWorkspaceJobCPURequest    = "MAGOS_WORKSPACE_JOB_CPU_REQUEST"
	envWorkspaceJobMemoryRequest = "MAGOS_WORKSPACE_JOB_MEMORY_REQUEST"
	envWorkspaceJobCPULimit      = "MAGOS_WORKSPACE_JOB_CPU_LIMIT"
	envWorkspaceJobMemoryLimit   = "MAGOS_WORKSPACE_JOB_MEMORY_LIMIT"
)

// Built-in defaults applied to Terraform plan and apply Job pods when the
// matching MAGOS_WORKSPACE_JOB_* env var is unset. The values keep the pod out
// of the BestEffort QoS class so it is not the first eviction target under
// node pressure.
const (
	defaultJobCPURequest    = "125m"
	defaultJobMemoryRequest = "128Mi"
	defaultJobCPULimit      = "250m"
	defaultJobMemoryLimit   = "256Mi"
)

// loadJobResourcesFromEnv returns the corev1.ResourceRequirements applied to
// Terraform plan and apply Job pods. Each field reads its own MAGOS_WORKSPACE_JOB_*
// env var, falling back to a built-in default when unset.
func loadJobResourcesFromEnv() (corev1.ResourceRequirements, error) {
	parse := func(env, fallback string) (resource.Quantity, error) {
		v := os.Getenv(env)
		if v == "" {
			v = fallback
		}
		q, err := resource.ParseQuantity(v)
		if err != nil {
			return q, fmt.Errorf("%s %q is not a valid quantity: %w", env, v, err)
		}
		return q, nil
	}

	cpuReq, err := parse(envWorkspaceJobCPURequest, defaultJobCPURequest)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	memReq, err := parse(envWorkspaceJobMemoryRequest, defaultJobMemoryRequest)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	cpuLim, err := parse(envWorkspaceJobCPULimit, defaultJobCPULimit)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	memLim, err := parse(envWorkspaceJobMemoryLimit, defaultJobMemoryLimit)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    cpuReq,
			corev1.ResourceMemory: memReq,
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    cpuLim,
			corev1.ResourceMemory: memLim,
		},
	}, nil
}

// loadDefaultPVCSizeFromEnv returns the validated value of
// MAGOS_WORKSPACE_PVC_SIZE_DEFAULT, or "" when unset. The caller falls back to
// the per-Workspace spec value and then to the built-in default in the
// reconciler.
func loadDefaultPVCSizeFromEnv() (string, error) {
	v := os.Getenv(envWorkspacePVCSizeDefault)
	if v == "" {
		return "", nil
	}
	if _, err := resource.ParseQuantity(v); err != nil {
		return "", fmt.Errorf("%s %q is not a valid quantity: %w", envWorkspacePVCSizeDefault, v, err)
	}
	return v, nil
}
