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
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// ComponentType defines the type of component in the inference pipeline
// +kubebuilder:validation:Enum=router;prefiller;decoder;worker
type ComponentType string

const (
	ComponentTypeRouter    ComponentType = "router"
	ComponentTypePrefiller ComponentType = "prefiller"
	ComponentTypeDecoder   ComponentType = "decoder"
	ComponentTypeWorker    ComponentType = "worker"
)

// RoutingStrategy defines the inference routing strategy
// +kubebuilder:validation:Enum=prefix-cache;kv-cache-utilization;queue-size;lora-affinity;pd-disaggregation
type RoutingStrategy string

const (
	StrategyPrefixCache      RoutingStrategy = "prefix-cache"
	StrategyKVCacheUtil      RoutingStrategy = "kv-cache-utilization"
	StrategyQueueSize        RoutingStrategy = "queue-size"
	StrategyLoRAffinity      RoutingStrategy = "lora-affinity"
	StrategyPDDisaggregation RoutingStrategy = "pd-disaggregation"
)

// InferenceServiceSpec defines the desired state of InferenceService
type InferenceServiceSpec struct {
	// Roles defines the components of the inference service
	// +kubebuilder:validation:MinItems=1
	Roles []Role `json:"roles"`

	// SchedulingStrategy applies cluster-wide scheduling policies (e.g., Volcano).
	// +optional
	SchedulingStrategy *SchedulingStrategy `json:"schedulingStrategy,omitempty"`
}

// Role defines a component in the inference pipeline
type Role struct {
	// Name is the identifier for this role
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// ComponentType specifies the type of component
	// +kubebuilder:validation:Required
	ComponentType ComponentType `json:"componentType"`

	// Router-specific fields (only for componentType: router)

	// Strategy defines the routing strategy for the router component
	// +optional
	Strategy RoutingStrategy `json:"strategy,omitempty"`

	// HTTPRoute defines the HTTPRoute configuration for routing traffic
	// +optional
	HTTPRoute *gatewayv1.HTTPRouteSpec `json:"httproute,omitempty"`

	// Gateway defines the Gateway configuration
	// +optional
	Gateway *gatewayv1.GatewaySpec `json:"gateway,omitempty"`

	// EndpointPickerConfig is raw YAML for advanced EndpointPickerConfig customization
	// +optional
	EndpointPickerConfig string `json:"endpointPickerConfig,omitempty"`

	// Worker-specific fields (for prefiller/decoder/worker)

	// Replica defines the number of replicas for this component
	// +optional
	Replica *int32 `json:"replica,omitempty"`

	// Multinode enables multi-node distributed inference
	// +optional
	Multinode *Multinode `json:"multinode,omitempty"`

	// Template defines the pod spec for this component
	// +optional
	Template *corev1.PodTemplateSpec `json:"template,omitempty"`
}

// SchedulingStrategy defines pod-level scheduling behavior.
type SchedulingStrategy struct {
	// SchedulerName specifies the Kubernetes scheduler to use (e.g., "volcano").
	// +optional
	SchedulerName string `json:"schedulerName,omitempty"`
}

// MultinodeSpec enables multi-node distributed inference.
type Multinode struct {
	// NodeCount is the number of distinct nodes to distribute this component across.
	// +kubebuilder:validation:Minimum=1
	NodeCount int32 `json:"nodeCount"`
}

// ComponentPhase is a simple, high-level summary of where the component is in its lifecycle.
// +kubebuilder:validation:Enum=Pending;Deploying;Running;Failed;Unknown
type ComponentPhase string

const (
	ComponentPhasePending   ComponentPhase = "Pending"
	ComponentPhaseDeploying ComponentPhase = "Deploying"
	ComponentPhaseRunning   ComponentPhase = "Running"
	ComponentPhaseFailed    ComponentPhase = "Failed"
	ComponentPhaseUnknown   ComponentPhase = "Unknown"
)

// ComponentStatus captures the aggregated runtime state of a single inference component (role).
// For example, with replica=2 and multinode.nodeCount=4:
//   - DesiredReplicas: 2
//   - NodesPerReplica: 4
//   - TotalPods: 8 (2 * 4)
//   - ReadyReplicas: 0/1/2 (a replica is ready only when all its nodes are ready)
//   - ReadyPods: 0-8
type ComponentStatus struct {
	// DesiredReplicas is the number of replicas requested (from spec.roles[].replica).
	DesiredReplicas int32 `json:"desiredReplicas"`

	// ReadyReplicas is the number of replicas that are fully ready.
	// For multi-node replicas, a replica is ready only when all its nodes are ready.
	ReadyReplicas int32 `json:"readyReplicas"`

	// NodesPerReplica is the number of nodes per replica (from spec.roles[].multinode.nodeCount).
	// Defaults to 1 when multinode is not configured.
	NodesPerReplica int32 `json:"nodesPerReplica"`

	// TotalPods is the total number of pods desired (= DesiredReplicas * NodesPerReplica).
	TotalPods int32 `json:"totalPods"`

	// ReadyPods is the total number of ready pods across all replicas.
	ReadyPods int32 `json:"readyPods"`

	// Phase indicates the high-level lifecycle stage of this component.
	// Possible values: Pending, Deploying, Running, Failed, Unknown.
	Phase ComponentPhase `json:"phase"`

	// LastUpdateTime is the timestamp when this component's status was last updated.
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
}

// InferenceServiceStatus defines the observed state of InferenceService.
type InferenceServiceStatus struct {
	// Conditions represent the latest available observations of the service's state.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Components summarizes the current state of each declared role/component.
	// Key is the component's .spec.roles[].name.
	// +optional
	Components map[string]ComponentStatus `json:"components,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// InferenceService is the Schema for the inferenceservices API
type InferenceService struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of InferenceService
	// +required
	Spec InferenceServiceSpec `json:"spec"`

	// status defines the observed state of InferenceService
	// +optional
	Status InferenceServiceStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// InferenceServiceList contains a list of InferenceService
type InferenceServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InferenceService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InferenceService{}, &InferenceServiceList{})
}
