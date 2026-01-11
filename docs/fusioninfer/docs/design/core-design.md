---
sidebar_position: 1
title: InferenceService CRD Design
---

# InferenceService CRD Design

## Summary

This proposal introduces a `InferenceService` CRD to support both **monolithic** and **prefill/decode (PD) disaggregated** serving topologies for large language models (LLMs). The design enables users to declaratively define:

- Role-specific deployments (`router`, `prefiller`, `decoder`, `worker`)
- Scheduling policies through a pluggable request scheduling framework
- Multi-node replication and resource constraints per role

## Motivation

Modern LLM serving systems increasingly adopt **disaggregation** (separating prefill and decode role) to improve GPU utilization, reduce latency tail, and enable independent scaling. However, many use cases still benefit from **monolithic deployment** (single pod handling full request lifecycle) due to simplicity or low traffic.

### Goals

- Define a single CRD that supports both monolithic and disaggregated inference topologies.
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
| `router` | Request router with EPP scheduling plugins |

### Reconciliation Logic

The following diagrams illustrate the resource topology for different deployment scenarios.

#### Monolithic Deployment

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

#### Disaggregated PD Deployment

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

#### Multi-Node Deployment

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

#### Disaggregated Multi-Node Deployment

Combines prefill/decode disaggregation with multi-node parallelism for maximum scalability.

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                                    InferenceService                                     │
│                                name: deepseek-r1-disagg                                 │
└────────────────────────────────────────────┬────────────────────────────────────────────┘
                                             │
                              ┌──────────────┴──────────────┐
                              │          Roles (2)          │
                              └──────────────┬──────────────┘
                                             │
                 ┌───────────────────────────┴───────────────────────────┐
                 │                                                       │
                 ▼                                                       ▼
┌───────────────────────────┐                           ┌───────────────────────────┐
│      Role: prefill        │                           │      Role: decode         │
│  componentType: prefiller │                           │  componentType: decoder   │
│  replicas: 1              │                           │  replicas: 2              │
│  multinode:               │                           │  multinode:               │
│    nodeCount: 2           │                           │    nodeCount: 4           │
└─────────────┬─────────────┘                           └─────────────┬─────────────┘
              │                                                       │
              ▼                                           ┌───────────┴───────────┐
┌─────────────────────────┐                               │                       │
│    LeaderWorkerSet-0    │                               ▼                       ▼
│       (2 Pods)          │                 ┌───────────────────────┐ ┌───────────────────────┐
│    TP=16 across         │                 │   LeaderWorkerSet-0   │ │   LeaderWorkerSet-1   │
│    16 GPUs              │                 │       (4 Pods)        │ │       (4 Pods)        │
├─────────────────────────┤                 │    TP=32 across       │ │    TP=32 across       │
│  ★ Leader Pod-0         │                 │    32 GPUs            │ │    32 GPUs            │
│    [8 GPUs]             │                 ├───────────────────────┤ ├───────────────────────┤
│  ● Worker Pod-1         │                 │  ★ Leader Pod-0       │ │  ★ Leader Pod-0       │
│    [8 GPUs]             │                 │    [8 GPUs]           │ │    [8 GPUs]           │
└─────────────────────────┘                 │  ● Worker Pod-1       │ │  ● Worker Pod-1       │
                                            │    [8 GPUs]           │ │    [8 GPUs]           │
                                            │  ● Worker Pod-2       │ │  ● Worker Pod-2       │
                                            │    [8 GPUs]           │ │    [8 GPUs]           │
                                            │  ● Worker Pod-3       │ │  ● Worker Pod-3       │
                                            │    [8 GPUs]           │ │    [8 GPUs]           │
                                            └───────────────────────┘ └───────────────────────┘

      Total: prefill (1 × 2 nodes × 8 GPUs) + decode (2 × 4 nodes × 8 GPUs) = 16 + 64 = 80 GPUs
```

### LeaderWorkerSet (LWS) Workload Management

The controller uses **LeaderWorkerSet (LWS)** for all deployments to provide unified workload management and gang scheduling support.

| Configuration | LWS Mode | LWS Size | Scheduler | Description |
|---------------|----------|----------|-----------|-------------|
| `multinode` not set | Per-replica | `size: 1` | default | Single pod per replica |
| `multinode.nodeCount >= 2` (monolithic) | Per-replica | `size: nodeCount` | volcano | One LWS per replica for independent scheduling |
| PD disaggregated | Per-replica | `size: nodeCount` | volcano | Shared PodGroup across prefill/decode roles |

**LWS Mode:**

The controller always uses **Per-replica mode** (one LWS per replica) to support fine-grained scaling and cleanup.

| Mode | LWS Count | PodGroup Count | Use Case |
|------|-----------|----------------|----------|
| **Per-replica** | N per role (one per replica) | 1 shared (if gang scheduling needed) | All deployment scenarios |

**Labels injected by Controller:**

| Label | Description |
|-------|-------------|
| `fusioninfer.io/service` | InferenceService name |
| `fusioninfer.io/component-type` | Component type (worker/prefiller/decoder) |
| `fusioninfer.io/role-name` | Role name from spec |
| `fusioninfer.io/replica-index` | Replica index (only in per-replica mode) |
| `fusioninfer.io/spec-hash` | Hash of resource spec for change detection |

**Naming Convention:**

The complete naming chain from InferenceService to Pods:

```
InferenceService: <service-name>
         │
         ▼ (Controller creates)
LWS: <service>-<role>-<fusioninfer-replica>
         │
         ▼ (LWS creates)
Pods:
  ├── <lws-name>-<lws-replica>              (Leader, no worker suffix)
  └── <lws-name>-<lws-replica>-<worker>     (Workers, index starts from 1)
```

| Resource | Naming Pattern | Example |
|----------|----------------|---------|
| LWS | `{service}-{role}-{replica}` | `qwen-inference-inference-0` |
| Leader Pod | `{lws-name}-{lws-replica}` | `qwen-inference-inference-0-0` |
| Worker Pod | `{lws-name}-{lws-replica}-{worker}` | `qwen-inference-inference-0-0-1` |

> **Note**: The Leader pod does not have a worker index suffix. Worker pods have indices starting from 1.

**Example: Pod Naming for Multi-Node Deployment**

For an InferenceService named `deepseek-r1` with role `inference`, `replicas: 2`, and `nodeCount: 4`:

```
deepseek-r1-inference-0          (LWS for replica 0)
  ├── deepseek-r1-inference-0-0      (Leader)
  ├── deepseek-r1-inference-0-0-1    (Worker 1)
  ├── deepseek-r1-inference-0-0-2    (Worker 2)
  └── deepseek-r1-inference-0-0-3    (Worker 3)

deepseek-r1-inference-1          (LWS for replica 1)
  ├── deepseek-r1-inference-1-0      (Leader)
  ├── deepseek-r1-inference-1-0-1    (Worker 1)
  ├── deepseek-r1-inference-1-0-2    (Worker 2)
  └── deepseek-r1-inference-1-0-3    (Worker 3)
```

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

For multi-node deployments (`replicas: 2, nodeCount: 4`), the **InferenceService Controller** creates:
- **1 shared PodGroup** with `minTaskMember` for each replica
- **Separate LWS per replica** to enable fine-grained scheduling

```
InferenceService (replicas: 2, nodeCount: 4)
    │
    ├── PodGroup: deepseek-r1-inference (shared)
    │   └── minTaskMember: {inference-0: 4, inference-1: 4}
    │
    ├── LWS: deepseek-r1-inference-inference-0
    │   └── replicas: 1, size: 4, task-spec: inference-0
    │
    └── LWS: deepseek-r1-inference-inference-1
        └── replicas: 1, size: 4, task-spec: inference-1
```

This allows partial deployment when cluster resources are limited (e.g., only one replica can be scheduled).

```yaml
# Shared PodGroup for the InferenceService
apiVersion: scheduling.volcano.sh/v1beta1
kind: PodGroup
metadata:
  name: deepseek-r1-inference
spec:
  minMember: 8                       # 4 + 4 = 8 pods total
  minTaskMember:
    inference-0: 4                   # All 4 pods in replica 0
    inference-1: 4                   # All 4 pods in replica 1
---
# Per-replica LWS (Controller creates one for each replica)
apiVersion: leaderworkerset.x-k8s.io/v1
kind: LeaderWorkerSet
metadata:
  name: deepseek-r1-inference-inference-0  # {service}-{role}-{replica}
  labels:
    fusioninfer.io/service: deepseek-r1-inference
    fusioninfer.io/component-type: worker
    fusioninfer.io/role-name: inference
    fusioninfer.io/replica-index: "0"
spec:
  replicas: 1                         # Always 1 in per-replica mode
  leaderWorkerTemplate:
    size: 4                           # 4 pods per replica
    # LeaderTemplate: Leader starts Ray head and runs vLLM
    leaderTemplate:
      metadata:
        labels:
          fusioninfer.io/replica-index: "0"
        annotations:
          scheduling.k8s.io/group-name: deepseek-r1-inference
          volcano.sh/task-spec: inference-0
      spec:
        schedulerName: volcano
        containers:
          - name: vllm
            image: vllm/vllm-openai:v0.11.0
            command: ["/bin/sh", "-c"]
            args:
              - "ray start --head --port=6379 && vllm serve deepseek-ai/DeepSeek-R1 --tensor-parallel-size 32 --distributed-executor-backend ray"
            ports:
              - containerPort: 8000
              - containerPort: 6379
            resources:
              limits:
                nvidia.com/gpu: "8"
    # WorkerTemplate: Workers join Ray cluster
    workerTemplate:
      metadata:
        labels:
          fusioninfer.io/replica-index: "0"
        annotations:
          scheduling.k8s.io/group-name: deepseek-r1-inference
          volcano.sh/task-spec: inference-0
      spec:
        schedulerName: volcano
        containers:
          - name: vllm
            image: vllm/vllm-openai:v0.11.0
            command: ["/bin/sh", "-c"]
            args:
              - "ray start --address=$LWS_LEADER_ADDRESS:6379 --block"
            resources:
              limits:
                nvidia.com/gpu: "8"
```

> **Note**: For multi-node deployments, the Controller automatically generates separate `leaderTemplate` and `workerTemplate`:
> - **Leader**: `ray start --head && <original command> --distributed-executor-backend ray`
> - **Worker**: `ray start --address=$LWS_LEADER_ADDRESS:6379 --block`

### Gang Scheduling Behavior

The **InferenceService Controller** creates a **single shared PodGroup** per InferenceService. The `minTaskMember` field uses keys in the format `{roleName}-{replicaIndex}` to enable fine-grained gang scheduling that ensures:

1. **Intra-replica atomicity**: All pods within a single replica are scheduled together (all-or-nothing)
2. **Cross-role coordination** (for PD disaggregated): At least one prefill AND one decode replica must be scheduled together

| Scenario | LWS Count | PodGroup Count | minTaskMember Keys |
|----------|-----------|----------------|--------------------|
| Monolithic (single-node) | 1 per replica | 0 | N/A (no gang scheduling) |
| Monolithic (multi-node) | 1 per replica | 1 shared | `{role}-0`, `{role}-1`, ... |
| PD disaggregated | 1 per replica | 1 shared | `prefill-0`, `decode-0`, `decode-1`, ... |

**Example: PD Disaggregated Multi-Node (Story 4)**

For `prefill (1 replica × 2 nodes)` + `decode (2 replicas × 4 nodes)`:

```yaml
# Single PodGroup for the entire InferenceService
apiVersion: scheduling.volcano.sh/v1beta1
kind: PodGroup
metadata:
  name: deepseek-r1-disagg
spec:
  minMember: 10              # 2 + 4 + 4 = 10 pods total
  minTaskMember:
    prefill-0: 2             # All 2 pods in prefill replica-0
    decode-0: 4              # All 4 pods in decode replica-0
    decode-1: 4              # All 4 pods in decode replica-1
```

**Scheduling Behavior Table:**

| Cluster GPUs | prefill-0 (16 GPUs) | decode-0 (32 GPUs) | decode-1 (32 GPUs) | Service Status |
|--------------|---------------------|--------------------|--------------------|----------------|
| 80 GPUs | ✅ | ✅ | ✅ | Full capacity |
| 64 GPUs | ✅ | ✅ | ⏳ | Partial (1P + 1D) |
| 48 GPUs | ✅ | ✅ | ⏳ | Partial (1P + 1D) |
| 32 GPUs | ⏳ | ⏳ | ⏳ | ❌ Blocked (can't satisfy 1P + 1D atomically) |
| 16 GPUs | ⏳ | ⏳ | ⏳ | ❌ Blocked (only enough for prefill) |

> **Note**: Volcano ensures each task's pods are scheduled atomically. With `minTaskMember`, the scheduler blocks until **all** pods within each task can be scheduled together, preventing partial deployments within a replica.

#### PodGroup Management

The **InferenceService Controller** creates **one PodGroup per InferenceService**. The `minTaskMember` keys use the format `{roleName}-{replicaIndex}` to identify each replica's task:

```yaml
# PodGroup for PD disaggregated: prefill (2 replicas × 1 node) + decode (4 replicas × 1 node)
apiVersion: scheduling.volcano.sh/v1beta1
kind: PodGroup
metadata:
  name: qwen3-inference              # Named after InferenceService
  namespace: default
spec:
  minMember: 6                       # 2 + 4 = 6 pods
  minTaskMember:                     # Matched by pod annotation: volcano.sh/task-spec
    prefill-0: 1                     # Pods with annotation "volcano.sh/task-spec: prefill-0"
    prefill-1: 1                     # Pods with annotation "volcano.sh/task-spec: prefill-1"
    decode-0: 1                      # Pods with annotation "volcano.sh/task-spec: decode-0"
    decode-1: 1                      # ... and so on
    decode-2: 1
    decode-3: 1
```

Each LWS is created per-replica with annotations to join the shared PodGroup:

```yaml
# Prefill LWS for replica 0 (Controller creates one LWS per replica)
apiVersion: leaderworkerset.x-k8s.io/v1
kind: LeaderWorkerSet
metadata:
  name: qwen3-inference-prefill-0    # {service}-{role}-{replica}
  labels:
    fusioninfer.io/service: qwen3-inference
    fusioninfer.io/component-type: prefiller
    fusioninfer.io/role-name: prefill
    fusioninfer.io/replica-index: "0"
spec:
  replicas: 1                        # Always 1 in per-replica mode
  leaderWorkerTemplate:
    size: 1                          # 1 pod per replica (single-node)
    workerTemplate:
      metadata:
        labels:
          fusioninfer.io/replica-index: "0"
        annotations:
          scheduling.k8s.io/group-name: qwen3-inference   # Join shared PodGroup
          volcano.sh/task-spec: prefill-0                 # Task: {roleName}-{replicaIndex}
      spec:
        schedulerName: volcano
        containers:
          - name: vllm
            image: vllm/vllm-openai:v0.8.5
            # ... prefill config
---
# Decode LWS for replica 0
apiVersion: leaderworkerset.x-k8s.io/v1
kind: LeaderWorkerSet
metadata:
  name: qwen3-inference-decode-0
  labels:
    fusioninfer.io/service: qwen3-inference
    fusioninfer.io/component-type: decoder
    fusioninfer.io/role-name: decode
    fusioninfer.io/replica-index: "0"
spec:
  replicas: 1
  leaderWorkerTemplate:
    size: 1
    workerTemplate:
      metadata:
        labels:
          fusioninfer.io/replica-index: "0"
        annotations:
          scheduling.k8s.io/group-name: qwen3-inference
          volcano.sh/task-spec: decode-0
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

**Task-spec format:** `{roleName}-{replicaIndex}` (e.g., `prefill-0`, `decode-1`)

**References:**
- [Volcano MinTaskMember Design](https://github.com/volcano-sh/volcano/blob/master/docs/design/task-minavailable.md)
- [Volcano Gang Scheduling](https://volcano.sh/en/docs/gang_scheduling/)

### CRD Structure Overview

```go
// ComponentType defines the type of component in the inference pipeline
// +kubebuilder:validation:Enum=router;prefiller;decoder;worker
type ComponentType string

const (
    ComponentTypeRouter    ComponentType = "router"
    ComponentTypePrefiller ComponentType = "prefiller"
    ComponentTypeDecoder   ComponentType = "decoder"
    ComponentTypeWorker    ComponentType = "worker"
)

// InferenceServiceSpec defines the desired state of InferenceService.
type InferenceServiceSpec struct {
    // Roles is a list of logical components in the inference topology.
    // Each role is identified by a user-defined Name and classified by ComponentType.
    Roles []Role `json:"roles"`
    
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
    ComponentType ComponentType `json:"componentType"`

    // Router-specific fields (only for componentType: router)
    
    // Strategy defines the routing strategy for the router component
    // +optional
    Strategy RoutingStrategy `json:"strategy,omitempty"`
    
    // HTTPRoute defines the HTTPRoute spec for routing traffic (Gateway API)
    // +optional
    HTTPRoute *runtime.RawExtension `json:"httproute,omitempty"`
    
    // Gateway defines the Gateway spec for this router (Gateway API GatewaySpec)
    // +optional
    Gateway *runtime.RawExtension `json:"gateway,omitempty"`
    
    // EndpointPickerConfig is raw YAML for advanced EPP customization
    // +optional
    EndpointPickerConfig string `json:"endpointPickerConfig,omitempty"`

    // Worker-specific fields (for prefiller/decoder/worker)
    
    // Replicas specifies how many independent distributed instances to create.
    // Default: 1
    // +optional
    Replicas *int32 `json:"replicas,omitempty"`

    // Multinode enables distributed inference with a built-in Leader + Worker topology.
    // +optional
    Multinode *Multinode `json:"multinode,omitempty"`

    // Template defines the pod spec for this component.
    // Uses runtime.RawExtension to avoid CRD size limits.
    // +optional
    Template *runtime.RawExtension `json:"template,omitempty"`
}

// Multinode enables multi-node distributed inference.
type Multinode struct {
    // NodeCount is the number of distinct nodes to distribute this component across.
    NodeCount int32 `json:"nodeCount"`
}

// InferenceServiceStatus reflects the observed state of the InferenceService.
type InferenceServiceStatus struct {
    // ObservedGeneration is the most recent generation observed by the controller.
    // +optional
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`
    
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
