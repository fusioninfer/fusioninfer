/*
Copyright 2025.

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModelSpec declares an immutable model artifact.
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable"
type ModelSpec struct {
	// Source identifies the immutable model artifact.
	// +required
	Source ModelSource `json:"source"`

	// LoRA identifies this artifact as a LoRA and names its base model.
	// +optional
	LoRA *LoRAArtifactSpec `json:"lora,omitempty"`
}

// ModelSource identifies the storage location and immutable version of a model artifact.
// +kubebuilder:validation:XValidation:rule="self.uri.matches('^(hf|s3|oci|pvc)://.+$')",message="uri must use a supported lowercase scheme and identify an artifact"
// +kubebuilder:validation:XValidation:rule="self.uri.matches('^[A-Za-z0-9._~:/@!$&()*+,;=%-]+$')",message="uri must use valid URI characters; encode spaces and non-ASCII characters"
// +kubebuilder:validation:XValidation:rule="!self.uri.startsWith('hf://') || self.uri.matches('^hf://[^/?#]+/[^/?#]+$')",message="hf uri must identify a repository as hf://<owner>/<name>"
// +kubebuilder:validation:XValidation:rule="!self.uri.startsWith('s3://') || self.uri.matches('^s3://[^/?#]+/[^/?#]+(/[^/?#]+)*$')",message="s3 uri must identify a bucket and prefix"
// +kubebuilder:validation:XValidation:rule="!self.uri.startsWith('pvc://') || self.uri.matches('^pvc://[a-z0-9]([-a-z0-9]*[a-z0-9])?([.][a-z0-9]([-a-z0-9]*[a-z0-9])?)*/[^/?#]+(/[^/?#]+)*$')",message="pvc uri must identify a Kubernetes claim name and subpath"
// +kubebuilder:validation:XValidation:rule="!self.uri.startsWith('pvc://') || self.uri.matches('^pvc://[^/]{1,253}/.+$')",message="pvc claim name must not exceed 253 characters"
// +kubebuilder:validation:XValidation:rule="!self.uri.contains('?') && !self.uri.contains('#')",message="uri must not contain a query string or fragment"
// +kubebuilder:validation:XValidation:rule="!self.uri.contains('/./') && !self.uri.contains('/../') && !self.uri.endsWith('/.') && !self.uri.endsWith('/..')",message="uri must not contain dot path segments"
// +kubebuilder:validation:XValidation:rule="!self.uri.startsWith('hf://') || (has(self.revision) && self.revision.matches('^[0-9a-f]{40}$'))",message="hf sources require a full lowercase commit SHA revision"
// +kubebuilder:validation:XValidation:rule="!(self.uri.startsWith('s3://') || self.uri.startsWith('pvc://')) || has(self.digest)",message="s3 and pvc sources require a digest"
// +kubebuilder:validation:XValidation:rule="!self.uri.startsWith('oci://') || self.uri.matches('^oci://[^/?#]+/[^/?#@]+(/[^/?#@]+)*@sha256:[0-9a-f]{64}$')",message="oci uri must identify an artifact and include its descriptor digest"
// +kubebuilder:validation:XValidation:rule="!self.uri.startsWith('pvc://') || !has(self.credentialsRef)",message="pvc sources do not allow credentialsRef"
// +kubebuilder:validation:XValidation:rule="!has(self.credentialsRef) || (has(self.credentialsRef.name) && size(self.credentialsRef.name) <= 253 && self.credentialsRef.name.matches('^[a-z0-9]([-a-z0-9]*[a-z0-9])?([.][a-z0-9]([-a-z0-9]*[a-z0-9])?)*$'))",message="credentialsRef.name must be a valid Kubernetes object name"
type ModelSource struct {
	// URI is the model artifact location.
	// +kubebuilder:validation:MinLength=1
	// +required
	URI string `json:"uri"`

	// Revision is the full commit SHA for a Hugging Face source.
	// +kubebuilder:validation:Pattern="^[0-9a-f]{40}$"
	// +optional
	Revision string `json:"revision,omitempty"`

	// Digest verifies artifact content.
	// +kubebuilder:validation:Pattern="^sha256:[0-9a-f]{64}$"
	// +optional
	Digest string `json:"digest,omitempty"`

	// CredentialsRef names a Secret used to access the source.
	// +optional
	CredentialsRef *corev1.LocalObjectReference `json:"credentialsRef,omitempty"`
}

// LoRAArtifactSpec declares the base model required by a LoRA artifact.
type LoRAArtifactSpec struct {
	// BaseModelRef identifies the compatible base model.
	// +required
	BaseModelRef ModelReference `json:"baseModelRef"`
}

// ModelReference identifies a Model or ClusterModel in the fusioninfer.io API group.
type ModelReference struct {
	// Kind is the referenced resource kind.
	// +kubebuilder:validation:Enum=Model;ClusterModel
	// +required
	Kind string `json:"kind"`

	// Name is the referenced resource name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern="^[a-z0-9]([-a-z0-9]*[a-z0-9])?([.][a-z0-9]([-a-z0-9]*[a-z0-9])?)*$"
	// +required
	Name string `json:"name"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +genclient
// +genclient:noStatus

// Model is a namespaced immutable model artifact declaration.
type Model struct {
	metav1.TypeMeta `json:",inline"`

	// Metadata is the standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// Spec declares the model artifact.
	// +required
	Spec ModelSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// ModelList contains a list of Model objects.
type ModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Model `json:"items"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:validation:XValidation:rule="!self.spec.source.uri.startsWith('pvc://')",message="cluster-scoped models do not allow pvc sources"
// +kubebuilder:validation:XValidation:rule="!has(self.spec.lora) || self.spec.lora.baseModelRef.kind == 'ClusterModel'",message="cluster-scoped LoRA artifacts must reference a ClusterModel"
// +genclient
// +genclient:nonNamespaced
// +genclient:noStatus

// ClusterModel is a cluster-scoped immutable model artifact declaration.
type ClusterModel struct {
	metav1.TypeMeta `json:",inline"`

	// Metadata is the standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// Spec declares the model artifact.
	// +required
	Spec ModelSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// ClusterModelList contains a list of ClusterModel objects.
type ClusterModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterModel `json:"items"`
}

func init() {
	SchemeBuilder.Register(
		&Model{},
		&ModelList{},
		&ClusterModel{},
		&ClusterModelList{},
	)
}
