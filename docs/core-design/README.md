# FusionInfer InferenceService CRD: Unified Support for LLM Serving

<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [User Stories](#user-stories)
  - [Story 1: Deploy a monolithic LLM service](#story-1-deploy-a-monolithic-llm-service)
  - [Story 2: Deploy a disaggregated prefill/decode service](#story-2-deploy-a-disaggregated-prefilldecode-service)
  - [Story 3: Deploy a multi-node inference service for large models](#story-3-deploy-a-multi-node-inference-service-for-large-models)
  - [Story 4: Deploy a disaggregated multi-node prefill/decode service](#story-4-deploy-a-disaggregated-multi-node-prefilldecode-service)
- [Proposal](#proposal)
  - [Component Types](#component-types)
  - [Reconciliation Logic](#reconciliation-logic)
    - [Monolithic Deployment (Story 1)](#monolithic-deployment-story-1)
    - [Disaggregated PD Deployment (Story 2)](#disaggregated-pd-deployment-story-2)
    - [Multi-Node Deployment (Story 3)](#multi-node-deployment-story-3)
    - [Disaggregated Multi-Node Deployment (Story 4)](#disaggregated-multi-node-deployment-story-4)
  - [LeaderWorkerSet (LWS) Workload Management](#leaderworkerset-lws-workload-management)
  - [Gang Scheduling Behavior](#gang-scheduling-behavior)
    - [PodGroup Management](#podgroup-management)
    - [Key Annotations for Volcano Gang Scheduling](#key-annotations-for-volcano-gang-scheduling)
    - [InferenceService Controller responsibilities](#inferenceservice-controller-responsibilities)
  - [CRD Structure Overview](#crd-structure-overview)
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

## User Stories

### Story 1: Deploy a monolithic LLM service

As a developer, I want to deploy Qwen-3 as a single-service endpoint.

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: qwen-inference
spec:
  roles:
    - name: inference
      componentType: worker
      replicas: 1
      template:
        spec:
          containers:
            - name: vllm
              image: vllm/vllm-openai:v0.11.0
              args:
                - "--model"
                - "Qwen/Qwen3-8B"
              ports:
                - containerPort: 8000
                  name: http
              resources:
                limits:
                  nvidia.com/gpu: "1"
```

### Story 2: Deploy a disaggregated prefill/decode service

As a developer, I want to deploy a prefill/decode disaggregated inference service for Qwen-3.

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: qwen-inference-service
spec:
  roles:
    - name: prefill
      componentType: prefiller
      replicas: 2
      template:
        spec:
          containers:
            - name: vllm
              image: vllm/vllm-openai:v0.11.0
              args:
                - "--model"
                - "Qwen/Qwen3-8B"
                - "--kv-transfer-config"
                - '{"kv_connector":"PyNcclConnector","kv_role":"kv_producer"}'
              ports:
                - containerPort: 8000
                  name: http
              resources:
                limits:
                  nvidia.com/gpu: "1"
    - name: decode
      componentType: decoder
      replicas: 4
      template:
        spec:
          containers:
            - name: vllm
              image: vllm/vllm-openai:v0.11.0
              args:
                - "--model"
                - "Qwen/Qwen3-8B"
                - "--kv-transfer-config"
                - '{"kv_connector":"PyNcclConnector","kv_role":"kv_consumer"}'
              ports:
                - containerPort: 8000
                  name: http
              resources:
                limits:
                  nvidia.com/gpu: "1"
```

### Story 3: Deploy a multi-node inference service for large models

As a developer, I want to deploy DeepSeek-R1 (671B) using multi-node tensor parallelism. System deploys 2 replicas × 4 nodes = 8 pods, each with 8 GPUs (total 64 GPUs for tensor parallelism).

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: deepseek-r1-inference
spec:
  roles:
    - name: inference
      componentType: worker
      replicas: 2
      multinode:
        nodeCount: 4
      template:
        spec:
          containers:
            - name: vllm
              image: vllm/vllm-openai:v0.11.0
              args:
                - "--model"
                - "deepseek-ai/DeepSeek-R1"
                - "--tensor-parallel-size"
                - "32"
              ports:
                - containerPort: 8000
                  name: http
              resources:
                limits:
                  nvidia.com/gpu: "8"
```

### Story 4: Deploy a disaggregated multi-node prefill/decode service

As a developer, I want to deploy DeepSeek-R1 with prefill/decode disaggregation and multi-node parallelism. System deploys prefill (1 replica × 2 nodes = 2 pods) + decode (2 replicas × 4 nodes = 8 pods), total 10 pods with 80 GPUs.

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: deepseek-r1-disagg
spec:
  roles:
    - name: prefill
      componentType: prefiller
      replicas: 1
      multinode:
        nodeCount: 2
      template:
        spec:
          containers:
            - name: vllm
              image: vllm/vllm-openai:v0.11.0
              args:
                - "--model"
                - "deepseek-ai/DeepSeek-R1"
                - "--tensor-parallel-size"
                - "16"
                - "--kv-transfer-config"
                - '{"kv_connector":"PyNcclConnector","kv_role":"kv_producer"}'
              ports:
                - containerPort: 8000
                  name: http
              resources:
                limits:
                  nvidia.com/gpu: "8"
    - name: decode
      componentType: decoder
      replicas: 2
      multinode:
        nodeCount: 4
      template:
        spec:
          containers:
            - name: vllm
              image: vllm/vllm-openai:v0.11.0
              args:
                - "--model"
                - "deepseek-ai/DeepSeek-R1"
                - "--tensor-parallel-size"
                - "32"
                - "--kv-transfer-config"
                - '{"kv_connector":"PyNcclConnector","kv_role":"kv_consumer"}'
              ports:
                - containerPort: 8000
                  name: http
              resources:
                limits:
                  nvidia.com/gpu: "8"
```

## Proposal

The `InferenceService` CR will serve as the primary user-facing API for LLM deployment. 
Users declare **roles** (a list of components), each identified by a user-chosen name and classified by its componentType.

### Component Types

| componentType | Description |
|---------------|-------------|
| `worker` | Monolithic inference (full request lifecycle) |
| `prefiller` | Handles prompt ingestion and KV cache generation |
| `decoder` | Performs autoregressive token generation |

### Reconciliation Logic

The following diagrams illustrate the resource topology for different deployment scenarios.

#### Monolithic Deployment (Story 1)

A simple single-role deployment where each pod handles the full inference lifecycle.

```
┌─────────────────────────────────────────────────────────┐
│                    InferenceService                     │
│                  name: qwen-inference                   │
└─────────────────────────┬───────────────────────────────┘
                          │
                          ▼
              ┌───────────────────────┐
              │    Role: inference    │
              │  componentType: worker│
              │     replicas: 1       │
              └───────────┬───────────┘
                          │
                          ▼
                  ┌───────────────┐
                  │     LWS       │
                  │  (size=1)     │
                  ├───────────────┤
                  │  ★ Leader-0   │
                  │    [1 GPU]    │
                  └───────────────┘

      Total: 1 replica × 1 node × 1 GPU = 1 GPU
```

#### Disaggregated PD Deployment (Story 2)

Prefill and decode are separated into independent roles for better resource utilization.

```
┌───────────────────────────────────────────────────────────────────────────┐
│                           InferenceService                                │
│                     name: qwen-inference-service                          │
└─────────────────────────────────┬─────────────────────────────────────────┘
                                  │
                  ┌───────────────┴───────────────┐
                  │         Roles (2)             │
                  └───────────────┬───────────────┘
                                  │
         ┌────────────────────────┼────────────────────────┐
         │                                                 │
         ▼                                                 ▼
┌─────────────────────┐                       ┌─────────────────────┐
│   Role: prefill     │                       │   Role: decode      │
│ componentType:      │                       │ componentType:      │
│   prefiller         │                       │   decoder           │
│ replicas: 2         │                       │ replicas: 4         │
└─────────┬───────────┘                       └─────────┬───────────┘
          │                                             │
          ▼                                             ▼
  ┌───────────────┐                           ┌───────────────┐
  │     LWS       │                           │     LWS       │
  │   (size=1)    │                           │   (size=1)    │
  │  replicas: 2  │                           │  replicas: 4  │
  ├───────────────┤                           ├───────────────┤
  │  ★ Leader-0   │                           │  ★ Leader-0   │
  │    [1 GPU]    │                           │    [1 GPU]    │
  │  ★ Leader-1   │                           │  ★ Leader-1   │
  │    [1 GPU]    │                           │    [1 GPU]    │
  └───────────────┘                           │  ★ Leader-2   │
                                              │    [1 GPU]    │
                                              │  ★ Leader-3   │
                                              │    [1 GPU]    │
                                              └───────────────┘

      Total: prefill (2 × 1 GPU) + decode (4 × 1 GPU) = 6 GPUs
```

#### Multi-Node Deployment (Story 3)

Large model deployment using LeaderWorkerSet (LWS) for multi-node tensor parallelism.

```
┌───────────────────────────────────────────────────────────────────────────┐
│                           InferenceService                                │
│                     name: deepseek-r1-inference                           │
└─────────────────────────────────┬─────────────────────────────────────────┘
                                  │
                                  ▼
                      ┌───────────────────────┐
                      │    Role: inference    │
                      │  componentType: worker│
                      │     replicas: 2       │
                      │  multinode:           │
                      │    nodeCount: 4       │
                      └───────────┬───────────┘
                                  │
                ┌─────────────────┴─────────────────┐
                │                                   │
                ▼                                   ▼
      ┌─────────────────────┐             ┌─────────────────────┐
      │  LeaderWorkerSet-0  │             │  LeaderWorkerSet-1  │
      │     (4 Pods)        │             │     (4 Pods)        │
      │   TP=32 across      │             │   TP=32 across      │
      │   32 GPUs           │             │   32 GPUs           │
      ├─────────────────────┤             ├─────────────────────┤
      │ ★ Leader Pod-0      │             │ ★ Leader Pod-0      │
      │   [8 GPUs]          │             │   [8 GPUs]          │
      │ ● Worker Pod-1      │             │ ● Worker Pod-1      │
      │   [8 GPUs]          │             │   [8 GPUs]          │
      │ ● Worker Pod-2      │             │ ● Worker Pod-2      │
      │   [8 GPUs]          │             │   [8 GPUs]          │
      │ ● Worker Pod-3      │             │ ● Worker Pod-3      │
      │   [8 GPUs]          │             │   [8 GPUs]          │
      └─────────────────────┘             └─────────────────────┘

      Total: inference (2 replicas × 4 nodes × 8 GPUs) = 8 pods, 64 GPUs
```

#### Disaggregated Multi-Node Deployment (Story 4)

Combines prefill/decode disaggregation with multi-node parallelism for maximum scalability.

```
┌───────────────────────────────────────────────────────────────────────────────────────┐
│                                  InferenceService                                     │
│                              name: deepseek-r1-disagg                                 │
└─────────────────────────────────────────┬─────────────────────────────────────────────┘
                                          │
                          ┌───────────────┴───────────────┐
                          │          Roles (2)            │
                          └───────────────┬───────────────┘
                        │
             ┌────────────────────────────┼────────────────────────────┐
             │                                                         │
             ▼                                                         ▼
┌───────────────────────┐                             ┌───────────────────────┐
│    Role: prefill      │                             │    Role: decode       │
│ componentType:        │                             │ componentType:        │
│   prefiller           │                             │   decoder             │
│ replicas: 1           │                             │ replicas: 2           │
│ multinode:            │                             │ multinode:            │
│   nodeCount: 2        │                             │   nodeCount: 4        │
└───────────┬───────────┘                             └───────────┬───────────┘
            │                                                     │
            ▼                                         ┌───────────┴───────────┐
┌─────────────────────┐                               │                       │
│  LeaderWorkerSet-0  │                               ▼                       ▼
│     (2 Pods)        │                 ┌─────────────────────┐ ┌─────────────────────┐
│   TP=16 across      │                 │  LeaderWorkerSet-0  │ │  LeaderWorkerSet-1  │
│   16 GPUs           │                 │     (4 Pods)        │ │     (4 Pods)        │
├─────────────────────┤                 │   TP=32 across      │ │   TP=32 across      │
│ ★ Leader Pod-0      │                 │   32 GPUs           │ │   32 GPUs           │
│   [8 GPUs]          │                 ├─────────────────────┤ ├─────────────────────┤
│ ● Worker Pod-1      │                 │ ★ Leader Pod-0      │ │ ★ Leader Pod-0      │
│   [8 GPUs]          │                 │   [8 GPUs]          │ │   [8 GPUs]          │
└─────────────────────┘                 │ ● Worker Pod-1      │ │ ● Worker Pod-1      │
                                        │   [8 GPUs]          │ │   [8 GPUs]          │
                                        │ ● Worker Pod-2      │ │ ● Worker Pod-2      │
                                        │   [8 GPUs]          │ │   [8 GPUs]          │
                                        │ ● Worker Pod-3      │ │ ● Worker Pod-3      │
                                        │   [8 GPUs]          │ │   [8 GPUs]          │
                                        └─────────────────────┘ └─────────────────────┘

      Total: prefill (1 × 2 nodes × 8 GPUs) + decode (2 × 4 nodes × 8 GPUs) = 16 + 64 = 80 GPUs
```

### LeaderWorkerSet (LWS) Workload Management

The controller uses **LeaderWorkerSet (LWS)** for all deployments to provide unified workload management and gang scheduling support.

| Configuration | LWS Size | Scheduler | Description |
|---------------|----------|-----------|-------------|
| `multinode` not set | `size: 1` | default | Single pod per replica |
| `multinode.nodeCount >= 2` | `size: nodeCount` | volcano | Multi-node with gang scheduling |

**Example 1: Single-Node LWS**

```yaml
apiVersion: leaderworkerset.x-k8s.io/v1
kind: LeaderWorkerSet
metadata:
  name: qwen-inference
spec:
  replicas: 2
  leaderWorkerTemplate:
    size: 1                    # Single pod per replica
    workerTemplate:
      spec:
        containers:
          - name: vllm
            image: vllm/vllm-openai:v0.11.0
            args: ["vllm", "serve", "Qwen/Qwen3-8B"]
            ports:
              - containerPort: 8000
            resources:
              limits:
                nvidia.com/gpu: "1"
```

**Example 2: Multi-Node LWS with Gang Scheduling**

For multi-node deployments, the **InferenceService Controller** creates a custom PodGroup and injects annotations into LWS pod templates. Set `schedulerName: volcano` in the pod spec.

```yaml
apiVersion: leaderworkerset.x-k8s.io/v1
kind: LeaderWorkerSet
metadata:
  name: deepseek-r1-inference
spec:
  replicas: 2
  leaderWorkerTemplate:
    size: 4                    # 4 pods per replica
    workerTemplate:
      metadata:
        annotations:
          scheduling.k8s.io/group-name: deepseek-r1-inference  # Injected by Controller
          volcano.sh/task-spec: worker                         # Injected by Controller
      spec:
        schedulerName: volcano
        containers:
          - name: vllm
            image: vllm/vllm-openai:v0.11.0
            command: ["ray"]
            args:
              - "symmetric-run"
              - "--address"
              - "$(LWS_LEADER_ADDRESS):6379"
              - "--min-nodes"
              - "4"
              - "--num-gpus"
              - "8"
              - "--"
              - "vllm"
              - "serve"
              - "deepseek-ai/DeepSeek-R1"
              - "--tensor-parallel-size"
              - "32"
            ports:
              - containerPort: 8000
              - containerPort: 6379
            resources:
              limits:
                nvidia.com/gpu: "8"
```

### Gang Scheduling Behavior

The **InferenceService Controller** creates and manages PodGroups for all scenarios. Gang scheduling behavior depends on the `minTaskMember` configuration in the PodGroup.

**Monolithic (Story 3):** Controller creates a PodGroup per LWS replica. Each replica's pods are scheduled atomically, but different replicas are scheduled independently.

| Cluster GPUs | Replica-0 (32 GPUs) | Replica-1 (32 GPUs) | Capacity |
|--------------|---------------------|---------------------|----------|
| 64 GPUs | ✅ Scheduled | ✅ Scheduled | 100% |
| 48 GPUs | ✅ Scheduled | ⏳ Pending | 50% |
| 24 GPUs | ⏳ Pending | ⏳ Pending | 0% |

**PD Disaggregated (Story 4):** With custom PodGroup (`minTaskMember: {prefill: 1, decode: 1}`), Volcano ensures at least 1 prefill AND 1 decode are scheduled together.

Example: `prefill (1×2 nodes=16 GPUs)` + `decode (2×4 nodes=64 GPUs)` = 80 GPUs total

| Cluster GPUs | Prefill (16 GPUs) | Decode-0 (32 GPUs) | Decode-1 (32 GPUs) | Service Status |
|--------------|-------------------|--------------------|--------------------|----------------|
| 80 GPUs | ✅ | ✅ | ✅ | Full capacity |
| 64 GPUs | ✅ | ✅ | ⏳ | Partial (1P + 1D) |
| 48 GPUs | ✅ | ✅ | ⏳ | Partial (1P + 1D) |
| 32 GPUs | ⏳ | ⏳ | ⏳ | ❌ Blocked (can't satisfy 1P + 1D together) |
| 16 GPUs | ⏳ | ⏳ | ⏳ | ❌ Blocked (only enough for prefill) |
| 0 GPUs | ⏳ | ⏳ | ⏳ | Service unavailable |

> **Note**: With `minTaskMember: {prefill: 1, decode: 1}`, Volcano blocks scheduling unless **both** prefill (16 GPUs) AND decode (32 GPUs) can be satisfied simultaneously (requires ≥48 GPUs).

#### PodGroup Management

The **InferenceService Controller** creates and manages PodGroups for all deployment scenarios:

- **Monolithic**: One PodGroup per LWS replica (intra-replica gang scheduling)
- **PD Disaggregated**: One PodGroup spanning all roles (cross-role gang scheduling)

For PD disaggregated scenarios, the Controller creates a PodGroup that ensures prefill and decode are scheduled together:

```yaml
# InferenceService Controller creates this PodGroup
apiVersion: scheduling.volcano.sh/v1beta1
kind: PodGroup
metadata:
  name: qwen3-inference              # Shared by all roles
  namespace: default
spec:
  minMember: 2                       # At least 1 prefill + 1 decode
  minTaskMember:                     # Matched by pod annotation: volcano.sh/task-spec
    prefill: 1                       # Pods with annotation "volcano.sh/task-spec: prefill"
    decode: 1                        # Pods with annotation "volcano.sh/task-spec: decode"
  minResources:
    nvidia.com/gpu: "16"
```

Each LWS pod template includes **two annotations** to join this PodGroup:

```yaml
# Prefill LWS (created by InferenceService Controller)
apiVersion: leaderworkerset.x-k8s.io/v1
kind: LeaderWorkerSet
metadata:
  name: qwen3-prefill
spec:
  replicas: 2
  leaderWorkerTemplate:
    size: 1
    workerTemplate:
      metadata:
        annotations:
          scheduling.k8s.io/group-name: qwen3-inference  # Join shared PodGroup
          volcano.sh/task-spec: prefill                  # Task identifier
      spec:
        schedulerName: volcano
        containers:
          - name: vllm
            image: vllm/vllm-openai:v0.8.5
            # ... prefill config
---
# Decode LWS (created by InferenceService Controller)
apiVersion: leaderworkerset.x-k8s.io/v1
kind: LeaderWorkerSet
metadata:
  name: qwen3-decode
spec:
  replicas: 4
  leaderWorkerTemplate:
    size: 1
    workerTemplate:
      metadata:
        annotations:
          scheduling.k8s.io/group-name: qwen3-inference  # Join shared PodGroup
          volcano.sh/task-spec: decode                   # Task identifier
      spec:
        schedulerName: volcano
        containers:
          - name: vllm
            image: vllm/vllm-openai:v0.8.5
            # ... decode config
```

#### Key Annotations for Volcano Gang Scheduling

| Annotation | Defined In | Purpose |
|------------|------------|---------|
| `scheduling.k8s.io/group-name` | `volcano.sh/apis/pkg/apis/scheduling/v1beta1` | Identifies which PodGroup the pod belongs to |
| `volcano.sh/task-spec` | `volcano.sh/apis/pkg/apis/batch/v1alpha1` | Identifies which task within the PodGroup (matches `minTaskMember` keys) |

**How Volcano Scheduler uses these annotations:**

```
┌─────────────────────────────────────────────────────────────────┐
│                         Pod Annotations                          │
│                                                                  │
│  scheduling.k8s.io/group-name: qwen3-inference                  │
│  volcano.sh/task-spec: prefill                                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Volcano Scheduler                            │
│                                                                  │
│  1. getJobID() → Find PodGroup by group-name annotation          │
│  2. getTaskName() → Get task name from task-spec annotation      │
│  3. Check PodGroup.spec.minTaskMember[taskName] >= required      │
│  4. Gang schedule only if ALL tasks meet minTaskMember           │
└─────────────────────────────────────────────────────────────────┘
```

**References:**
- [Volcano MinTaskMember Design](https://github.com/volcano-sh/volcano/blob/master/docs/design/task-minavailable.md)
- [Volcano Gang Scheduling](https://volcano.sh/en/docs/gang_scheduling/)

**Benefits of this approach:**

| Aspect | Solution |
|--------|----------|
| Pod lifecycle | LWS manages (failure recovery, env injection, multi-node coordination) |
| Gang scheduling | Controller-managed PodGroup for both intra-replica and cross-role scenarios |
| Independent scaling | Each role's replicas can be adjusted independently |
| Code reuse | Leverages LWS instead of reimplementing pod management |

#### InferenceService Controller responsibilities

1. **Create PodGroup** - One per InferenceService, with `minTaskMember` for gang scheduling constraints
2. **Create LWS per role** - Inject PodGroup annotations (`scheduling.k8s.io/group-name`, `volcano.sh/task-spec`) into pod templates
3. **Update PodGroup** - Adjust `minTaskMember` when role replicas change
4. **Aggregate status** - Monitor all LWS and PodGroup states, update InferenceService status

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
