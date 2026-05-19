/*
Copyright 2026. The Magos Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package v1alpha1 contains API Schema definitions for the  v1alpha1 API group.
// +kubebuilder:object:generate=true
// +groupName=magosproject.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type schemeBuilder struct {
	runtime.SchemeBuilder
}

func (sb *schemeBuilder) Register(objects ...runtime.Object) *schemeBuilder {
	sb.SchemeBuilder.Register(func(scheme *runtime.Scheme) error {
		scheme.AddKnownTypes(GroupVersion, objects...)
		return nil
	})

	return sb
}

func (sb *schemeBuilder) AddToScheme(scheme *runtime.Scheme) error {
	if err := sb.SchemeBuilder.AddToScheme(scheme); err != nil {
		return err
	}

	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "magosproject.io", Version: "v1alpha1"}

	// SchemeGroupVersion is an alias for GroupVersion for compatibility with generated clients.
	SchemeGroupVersion = GroupVersion

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &schemeBuilder{SchemeBuilder: runtime.NewSchemeBuilder()}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// Resource returns a GroupResource for the given resource name in this group.
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}
