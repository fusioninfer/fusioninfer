# FusionInfer InferenceService CRD: Unified Support for LLM Serving

<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [User Stories](#user-stories)
    - [Story 1: Deploy a monolithic LLM service](#story-1-deploy-a-monolithic-llm-service)
    - [Story 2: Deploy a disaggregated prefill/decode service with custom scheduling](#story-2-deploy-a-disaggregated-prefilldecode-service-with-custom-scheduling)
  - [Notes/Constraints/Caveats](#notesconstraintscaveats)
  - [Risks and Mitigations](#risks-and-mitigations)
- [Design Details](#design-details)
  - [CRD Structure Overview](#crd-structure-overview)
  - [Role-Based](#role-based-topology)
  - [Plugin System Integration](#plugin-system-integration)
  - [Backward Compatibility](#backward-compatibility)
  - [Test Plan](#test-plan)
    - [Unit Tests](#unit-tests)
    - [Integration Tests](#integration-tests)
  - [Graduation Criteria](#graduation-criteria)
- [Implementation History](#implementation-history)
- [Drawbacks](#drawbacks)
- [Alternatives](#alternatives)
<!-- /toc -->

## Summary

This proposal introduces a `InferenceService` CRD to support both **monolithic** and **prefill/decode (PD) disaggregated** serving topologies for large language models (LLMs). The design enables users to declaratively define:

- Role-specific deployments (`router`, `prefiller`, `decoder`, `worker`)
- Scheduling policies through a pluggable request scheduling framework
- Multi-node replication and resource constraints per role

## Motivation

Modern LLM serving systems increasingly adopt **disaggregation** (separating prefill and decode role) to improve GPU utilization, reduce latency tail, and enable independent scaling. However, many use cases still benefit from **monolithic deployment** (single pod handling full request lifecycle) due to simplicity or low traffic.

### Goals

- Define a single CRD that supports both monolithic and disaggregated inference topologies.
- Use a componentType field to express the logical role (`worker`, `prefiller`, `decoder`, `router`), while allowing flexible name.
- Allow per component specification of replicas, node count, container templates, and resources.
- Integrate with an **EPP scheduling framework** for request scheduling at the gateway.
- Enable multi-node deployment for prefill/decode components to scale across GPUs/nodes.

### Non-Goals

- Implement the underlying inference engine (e.g., vLLM, TensorRT-LLM) — only orchestrate it.
- Support non-LLM workloads.

## Proposal

The `InferenceService` CR will serve as the primary user-facing API for LLM deployment. 
Users declare **roles** (a list of components), each identified by a user-chosen name and classified by its componentType.

### User Stories

#### Story 1: Deploy a monolithic LLM service

> As a developer, I want to deploy Qwen-3 as a single-service endpoint.

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: qwen-inference
spec:
  roles:
    ...
    - name: inference
      componentType: worker
      roleReplicas: 1  # 1 independent roles
      multinode:       # each instance is split into: 1 header pod + 1 worker pod (total 2 pods)
        nodeCount: 2
      template: { ... }
```

→ System deploys 2 pods (across 2 nodes) running full vLLM servers.

#### Story 2: Deploy a disaggregated prefill/decode service with custom scheduling

> As a developer, I want to run prefill on A100 nodes and decode on cheaper L4 nodes, with prefix caching request scheduling.

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: qwen-inference-service
spec:
  schedulingStrategy:
    schedulerName: volcano
  roles:
    ...
    - name: prefill-servers
      componentType: prefiller
      roleReplicas: 2         # ← 2 independent roles
      podReplicas: 4          # ← 4 pods per role
      template: { ... }
    - name: decode-servers
      componentType: decoder
      podReplicas: 2          # 1 role × 2 pods
      template: { ... }
```

→ System deploys separate prefill/decode pods.

### Component Diagram


```bash
┌───────────────────────────────────────────────────────┐
│                  InferenceService                     │
│  scheduler: volcano                                   │
└───────────────────────┬───────────────────────────────┘
                        │
        ┌───────────────▼───────────────┐
        │          Roles (2)            │
        └───────────────┬───────────────┘
                        │
   ┌────────────────────┼────────────────────┐
   │                                         │
   ▼                                         ▼
┌─────────────────┐               ┌────────────────────┐
│ prefill-servers │               │ decode-servers     │
│ componentType:  │               │ componentType:     │
│   prefiller     │               │   decoder          │
│   replicas: 2   │               │   replicas: 1      │
│  mulit node: 4  │               │   mulit node: 2    │
└────────┬────────┘               └─────────┬──────────┘
         │                                  │
   ┌─────┴─────┐                      ┌─────┴─────┐
   │ Group 0   │                      │ Group 0   │
   │ (4 Pods)  │                      │ (2 Pods)  │
   ├───────────┤                      ├───────────┤
   │ ● Pod-0   │                      │ ● Pod-0   │
   │ ● Pod-1   │                      │ ● Pod-1   │
   │ ● Pod-2   │                      └───────────┘
   │ ● Pod-3   │
   ├───────────┤
   │ Group 1   │
   │ (4 Pods)  │
   ├───────────┤
   │ ● Pod-0   │
   │ ● Pod-1   │
   │ ● Pod-2   │
   │ ● Pod-3   │
   └───────────┘

> Note: All Pods belong to the same Volcano PodGroup to ensure they can be scheduled synchronously.
```

### Notes/Constraints/Caveats

- Valid values for componentType are:
  - `worker`: Monolithic inference (full request lifecycle)
  - `prefiller`: Handles prompt ingestion and KV cache generation
  - `decoder`: Performs autoregressive token generation using received KV cache
  - `router`: Request entrypoint with scheduling logic (optional)
- Topology validation rules:
    - Monolithic mode: Exactly one component with componentType: `worker`. No `prefiller` or `decoder` allowed.
    - Disaggregated mode: At least one `prefiller` AND one `decoder`.
- Multi-node `nodeCount` implies **distributed execution** (e.g., vLLM tensor/pipeline parallelism).

### Summary of Workload Mapping

| Condition                                      | Workload Type           | Reason                                                                 |
|-----------------------------------------------|-------------------------|------------------------------------------------------------------------|
| `Multinode == nil`                            | `Statefulset`           | Simple, replicated serving (monolithic or PD role on a single node).   |
| `Multinode != nil` and `NodeCount >= 2`       | `LeaderWorkerSet (LWS)` | Fixed header-worker topology requiring coordinated lifecycle management. |

### Risks and Mitigations

## Design Details

### CRD Structure Overview

```go
// InferenceServiceSpec defines the desired state of InferenceService.
type InferenceServiceSpec struct {
    // Roles is a list of logical components in the inference topology.
    // Each role is identified by a user-defined Name and classified by ComponentType.
    Roles []Role `json:"roles,omitempty"`
    
    // SchedulingStrategy applies cluster-wide scheduling policies (e.g., Volcano).
    // +optional
    SchedulingStrategy *SchedulingStrategy `json:"schedulingStrategy,omitempty"`
}

// SchedulingStrategy defines pod-level scheduling behavior.
type SchedulingStrategy struct {
    // SchedulerName specifies the Kubernetes scheduler to use (e.g., "volcano").
    // +optional
    SchedulerName string `json:"schedulerName,omitempty"`
}

// Role describes a logical component in the inference pipeline.
type Role struct {
    // Name is a user-defined, unique identifier for this component (e.g., "inference").
    Name string `json:"name"`

    // ComponentType indicates the semantic role. Valid values:
    // - "worker": monolithic inference
    // - "prefiller": prompt processing
    // - "decoder": token generation
    // - "router": request router with scheduling plugins
    ComponentType string `json:"componentType"`

    // Replicas specifies how many independent distributed instances to create.
    // Each instance is a self-contained unit (either a single Deployment or a Header+Worker pair).
    // Default: 1
    // +optional
    Replicas int32 `json:"replicas,omitempty"`

    // Multinode enables distributed inference with a built-in Header + Worker topology.
    // When Multinode is specified (and NodeCount >= 2), each roleReplica is implemented as
    // a LeaderWorkerSet (LWS) consisting of:
    //   - 1 Leader pod (header)
    //   - (NodeCount - 1) Worker pods
    // The controller will use LWS as the underlying workload instead of Deployment.
    // If Multinode is nil, a standard Deployment is used (single).
    // +optional
    Multinode *Multinode `json:"multinode,omitempty"`

    // Template defines the pod spec for this component.
    // +optional
    Template *corev1.PodTemplateSpec `json:"template,omitempty"`
}

// Multinode enables multi-node distributed inference.
type Multinode struct {
    // NodeCount is the number of distinct nodes to distribute this component across.
    NodeCount int32 `json:"nodeCount"`
}

// InferenceServiceStatus reflects the observed state of the InferenceService.
type InferenceServiceStatus struct {
    // Conditions represent the latest available observations of the service's state.
    Conditions []metav1.Condition `json:"conditions,omitempty"`
    
    // Components summarizes the current state of each declared role/component.
    // Key is the component's .spec.roles[].name.
    // +optional
    Components map[string]ComponentStatus `json:"components,omitempty"`
}

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
```

### Test Plan

#### Unit Tests

TBD

#### Integration Tests

TBD

### Graduation Criteria

TBD


## Implementation History

TBD

## Drawbacks

TBD

## Alternatives
