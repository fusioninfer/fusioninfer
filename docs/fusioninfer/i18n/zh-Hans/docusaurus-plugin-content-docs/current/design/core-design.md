---
sidebar_position: 1
title: InferenceService CRD
---

:::warning 旧版 API 设计
本文档说明现有 `InferenceService` v1alpha1 API 和 Controller 行为。迁移期间本文档仍有参考价值，但目标用户 API 见[架构](./model-serving.md)。当前的多节点、LeaderWorkerSet 和 Volcano 设计另见[工作负载编排](./workload-orchestration.md)；以下章节保留旧版 `InferenceService.roles` 术语。
::::

# InferenceService CRD {#inferenceservice-crd}

## 概述 {#summary}

本方案引入一个 `InferenceService` CRD，以同时支持大型语言模型（LLM）的**单体**和 **Prefill/Decode（PD）分离**服务拓扑。该设计让用户能够以声明方式定义：

- 按角色划分的部署（`router`、`prefiller`、`decoder`、`worker`）
- 通过可插拔请求调度框架配置的调度策略
- 每个角色的多节点副本和资源约束

## 动机 {#motivation}

现代 LLM 服务系统越来越多地采用**分离**架构（将 Prefill 和 Decode 角色分开），以提高 GPU 利用率、降低尾延迟并支持独立扩缩容。但是，许多使用场景仍受益于**单体部署**（由单个 Pod 处理完整请求生命周期），因为它更简单或适合低流量。

### 目标 {#goals}

- 定义一个同时支持单体和分离推理拓扑的 CRD。
- 允许为每个组件指定副本数、节点数、容器模板和资源。
- 与用于 Gateway 请求调度的 **EPP 调度框架**集成。
- 支持 Prefill/Decode 组件的多节点部署，以跨 GPU/节点扩展。

### 非目标 {#non-goals}

- 实现底层推理引擎（例如 vLLM、TensorRT-LLM）——本方案只负责编排。
- 支持非 LLM 工作负载。

## 用户故事 {#user-stories}

### 故事 1：部署单体 LLM 服务 {#story-1-deploy-a-monolithic-llm-service}

作为开发者，我希望将 Qwen-3 部署为单服务端点。

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
              image: vllm/vllm-openai:v0.27.1
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

### 故事 2：部署 Prefill/Decode 分离服务 {#story-2-deploy-a-disaggregated-prefilldecode-service}

作为开发者，我希望为 Qwen-3 部署 Prefill/Decode 分离推理服务。

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
              image: vllm/vllm-openai:v0.27.1
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
              image: vllm/vllm-openai:v0.27.1
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

### 故事 3：为大模型部署多节点推理服务 {#story-3-deploy-a-multi-node-inference-service-for-large-models}

作为开发者，我希望使用多节点张量并行部署 DeepSeek-R1（671B）。系统将部署 2 个副本 × 4 个节点 = 8 个 Pod，每个 Pod 使用 8 张 GPU（张量并行共使用 64 张 GPU）。

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
              image: vllm/vllm-openai:v0.27.1
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

### 故事 4：部署 Prefill/Decode 分离的多节点服务 {#story-4-deploy-a-disaggregated-multi-node-prefilldecode-service}

作为开发者，我希望使用 Prefill/Decode 分离和多节点并行部署 DeepSeek-R1。系统将部署 Prefill（1 个副本 × 2 个节点 = 2 个 Pod）+ Decode（2 个副本 × 4 个节点 = 8 个 Pod），共 10 个 Pod、80 张 GPU。

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
              image: vllm/vllm-openai:v0.27.1
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
              image: vllm/vllm-openai:v0.27.1
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

## 方案 {#proposal}

`InferenceService` CR 将作为面向用户的主要 LLM 部署 API。
用户声明 **roles**（组件列表），每个角色由用户选择的名称标识，并按其 `componentType` 分类。

### 组件类型 {#component-types}

| componentType | 说明 |
|---------------|------|
| `worker` | 单体推理（完整请求生命周期） |
| `prefiller` | 处理 prompt 摄入和 KV cache 生成 |
| `decoder` | 执行自回归 token 生成 |
| `router` | 使用 EPP 调度插件的请求路由器 |

### 调和逻辑 {#reconciliation-logic}

以下图示说明不同部署场景的资源拓扑。

#### 单体部署 {#monolithic-deployment}

简单的单角色部署，每个 Pod 处理完整的推理生命周期。

```
┌─────────────────────────────────────────────────────────┐
│                    InferenceService                     │
│                  name: qwen-inference                   │
└─────────────────────────┬───────────────────────────────┘
                          │
                          ▼
              ┌───────────────────────┐
              │    角色: inference    │
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
                  │   [1 张 GPU]  │
                  └───────────────┘

      总计：1 个副本 × 1 个节点 × 1 张 GPU = 1 张 GPU
```

#### Prefill/Decode 分离部署 {#disaggregated-pd-deployment}

Prefill 和 Decode 被拆分为独立角色，以提高资源利用率。

```
┌───────────────────────────────────────────────────────────────────────────┐
│                           InferenceService                                │
│                     name: qwen-inference-service                          │
└─────────────────────────────────┬─────────────────────────────────────────┘
                                  │
                  ┌───────────────┴───────────────┐
                  │          角色 (2)             │
                  └───────────────┬───────────────┘
                                  │
         ┌────────────────────────┼────────────────────────┐
         │                                                 │
         ▼                                                 ▼
┌─────────────────────┐                       ┌─────────────────────┐
│   角色: prefill     │                       │   角色: decode      │
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
  │   [1 张 GPU]  │                           │   [1 张 GPU]  │
  │  ★ Leader-1   │                           │  ★ Leader-1   │
  │   [1 张 GPU]  │                           │   [1 张 GPU]  │
  └───────────────┘                           │  ★ Leader-2   │
                                              │   [1 张 GPU]  │
                                              │  ★ Leader-3   │
                                              │   [1 张 GPU]  │
                                              └───────────────┘

      总计：Prefill（2 × 1 张 GPU）+ Decode（4 × 1 张 GPU）= 6 张 GPU
```

#### 多节点部署 {#multi-node-deployment}

使用 LeaderWorkerSet（LWS）进行多节点张量并行的大模型部署。

```
┌───────────────────────────────────────────────────────────────────────────┐
│                           InferenceService                                │
│                     name: deepseek-r1-inference                           │
└─────────────────────────────────┬─────────────────────────────────────────┘
                                  │
                                  ▼
                      ┌───────────────────────┐
                      │    角色: inference    │
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
      │     (4 个 Pod)      │             │     (4 个 Pod)      │
      │   TP=32，跨         │             │   TP=32，跨         │
      │   32 张 GPU         │             │   32 张 GPU         │
      ├─────────────────────┤             ├─────────────────────┤
      │ ★ Leader Pod-0      │             │ ★ Leader Pod-0      │
      │   [8 张 GPU]        │             │   [8 张 GPU]        │
      │ ● Worker Pod-1      │             │ ● Worker Pod-1      │
      │   [8 张 GPU]        │             │   [8 张 GPU]        │
      │ ● Worker Pod-2      │             │ ● Worker Pod-2      │
      │   [8 张 GPU]        │             │   [8 张 GPU]        │
      │ ● Worker Pod-3      │             │ ● Worker Pod-3      │
      │   [8 张 GPU]        │             │   [8 张 GPU]        │
      └─────────────────────┘             └─────────────────────┘

      总计：inference（2 个副本 × 4 个节点 × 8 张 GPU）= 8 个 Pod、64 张 GPU
```

#### Prefill/Decode 分离多节点部署 {#disaggregated-multi-node-deployment}

将 Prefill/Decode 分离与多节点并行相结合，以获得最大的可扩展性。

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                                    InferenceService                                     │
│                                name: deepseek-r1-disagg                                 │
└────────────────────────────────────────────┬────────────────────────────────────────────┘
                                             │
                              ┌──────────────┴──────────────┐
                              │          角色 (2)           │
                              └──────────────┬──────────────┘
                                             │
                 ┌───────────────────────────┴───────────────────────────┐
                 │                                                       │
                 ▼                                                       ▼
┌───────────────────────────┐                           ┌───────────────────────────┐
│      角色: prefill        │                           │      角色: decode         │
│  componentType: prefiller │                           │  componentType: decoder   │
│  replicas: 1              │                           │  replicas: 2              │
│  multinode:               │                           │  multinode:               │
│    nodeCount: 2           │                           │    nodeCount: 4           │
└─────────────┬─────────────┘                           └─────────────┬─────────────┘
              │                                                       │
              ▼                                           ┌───────────┴───────────┐
┌─────────────────────────┐                               │                       │
│    LeaderWorkerSet-0    │                               ▼                       ▼
│       (2 个 Pod)        │                 ┌───────────────────────┐ ┌───────────────────────┐
│    TP=16，跨            │                 │   LeaderWorkerSet-0   │ │   LeaderWorkerSet-1   │
│    16 张 GPU            │                 │       (4 个 Pod)      │ │       (4 个 Pod)      │
├─────────────────────────┤                 │    TP=32，跨          │ │    TP=32，跨          │
│  ★ Leader Pod-0         │                 │    32 张 GPU          │ │    32 张 GPU          │
│    [8 张 GPU]           │                 ├───────────────────────┤ ├───────────────────────┤
│  ● Worker Pod-1         │                 │  ★ Leader Pod-0       │ │  ★ Leader Pod-0       │
│    [8 张 GPU]           │                 │    [8 张 GPU]         │ │    [8 张 GPU]         │
└─────────────────────────┘                 │  ● Worker Pod-1       │ │  ● Worker Pod-1       │
                                            │    [8 张 GPU]         │ │    [8 张 GPU]         │
                                            │  ● Worker Pod-2       │ │  ● Worker Pod-2       │
                                            │    [8 张 GPU]         │ │    [8 张 GPU]         │
                                            │  ● Worker Pod-3       │ │  ● Worker Pod-3       │
                                            │    [8 张 GPU]         │ │    [8 张 GPU]         │
                                            └───────────────────────┘ └───────────────────────┘

      总计：Prefill（1 × 2 个节点 × 8 张 GPU）+ Decode（2 × 4 个节点 × 8 张 GPU）= 16 + 64 = 80 张 GPU
```

### LeaderWorkerSet（LWS）工作负载管理 {#leaderworkerset-lws-workload-management}

Controller 对所有部署使用 **LeaderWorkerSet（LWS）**，以提供统一的工作负载管理和 Gang Scheduling 支持。

| 配置 | LWS 模式 | LWS 大小 | 调度器 | 说明 |
|------|----------|----------|--------|------|
| 未设置 `multinode` | 每副本 | `size: 1` | default | 每个副本一个 Pod |
| `multinode.nodeCount >= 2`（单体） | 每副本 | `size: nodeCount` | volcano | 每个副本一个 LWS，以便独立调度 |
| PD 分离 | 每副本 | `size: nodeCount` | volcano | Prefill/Decode 角色共享 PodGroup |

**LWS 模式：**

Controller 始终使用**每副本模式**（每个副本一个 LWS），以支持细粒度扩缩容和清理。

| 模式 | LWS 数量 | PodGroup 数量 | 使用场景 |
|------|----------|---------------|----------|
| **每副本** | 每个角色 N 个（每个副本一个） | 1 个共享 PodGroup（如果需要 Gang Scheduling） | 所有部署场景 |

**Controller 注入的 Label：**

| Label | 说明 |
|-------|------|
| `fusioninfer.io/service` | InferenceService 名称 |
| `fusioninfer.io/component-type` | 组件类型（worker/prefiller/decoder） |
| `fusioninfer.io/role-name` | spec 中的角色名称 |
| `fusioninfer.io/replica-index` | 副本索引（仅限每副本模式） |
| `fusioninfer.io/spec-hash` | 用于变更检测的资源 spec 哈希值 |

**命名约定：**

从 InferenceService 到 Pod 的完整命名链如下：

```
InferenceService: <service-name>
         │
         ▼（Controller 创建）
LWS: <service>-<role>-<fusioninfer-replica>
         │
         ▼（LWS 创建）
Pod：
  ├── <lws-name>-<lws-replica>              （Leader，无 worker 后缀）
  └── <lws-name>-<lws-replica>-<worker>     （Worker，索引从 1 开始）
```

| 资源 | 命名模式 | 示例 |
|------|----------|------|
| LWS | `{service}-{role}-{replica}` | `qwen-inference-inference-0` |
| Leader Pod | `{lws-name}-{lws-replica}` | `qwen-inference-inference-0-0` |
| Worker Pod | `{lws-name}-{lws-replica}-{worker}` | `qwen-inference-inference-0-0-1` |

> **注意**：Leader Pod 没有 worker 索引后缀。Worker Pod 的索引从 1 开始。

**示例：多节点部署的 Pod 命名**

对于名为 `deepseek-r1`、角色为 `inference`、`replicas: 2` 且 `nodeCount: 4` 的 InferenceService：

```
deepseek-r1-inference-0          （副本 0 的 LWS）
  ├── deepseek-r1-inference-0-0      （Leader）
  ├── deepseek-r1-inference-0-0-1    （Worker 1）
  ├── deepseek-r1-inference-0-0-2    （Worker 2）
  └── deepseek-r1-inference-0-0-3    （Worker 3）

deepseek-r1-inference-1          （副本 1 的 LWS）
  ├── deepseek-r1-inference-1-0      （Leader）
  ├── deepseek-r1-inference-1-0-1    （Worker 1）
  ├── deepseek-r1-inference-1-0-2    （Worker 2）
  └── deepseek-r1-inference-1-0-3    （Worker 3）
```

**示例 1：单节点 LWS**

```yaml
apiVersion: leaderworkerset.x-k8s.io/v1
kind: LeaderWorkerSet
metadata:
  name: qwen-inference
spec:
  replicas: 2
  leaderWorkerTemplate:
    size: 1                    # 每个副本一个 Pod
    workerTemplate:
      spec:
        containers:
          - name: vllm
            image: vllm/vllm-openai:v0.27.1
            args: ["Qwen/Qwen3-8B"]
            ports:
              - containerPort: 8000
            resources:
              limits:
                nvidia.com/gpu: "1"
```

**示例 2：使用 Gang Scheduling 的多节点 LWS**

对于多节点部署（`replicas: 2, nodeCount: 4`），**InferenceService Controller** 会创建：
- **1 个共享 PodGroup**，其中包含每个副本的 `minTaskMember`
- **每个副本独立的 LWS**，以支持细粒度调度

```
InferenceService（replicas: 2, nodeCount: 4）
    │
    ├── PodGroup: deepseek-r1-inference（共享）
    │   └── minTaskMember: {inference-0: 4, inference-1: 4}
    │
    ├── LWS: deepseek-r1-inference-inference-0
    │   └── replicas: 1, size: 4, task-spec: inference-0
    │
    └── LWS: deepseek-r1-inference-inference-1
        └── replicas: 1, size: 4, task-spec: inference-1
```

这样可在集群资源有限时进行部分部署（例如只能调度一个副本）。

```yaml
# InferenceService 的共享 PodGroup
apiVersion: scheduling.volcano.sh/v1beta1
kind: PodGroup
metadata:
  name: deepseek-r1-inference
spec:
  minMember: 8                       # 4 + 4 = 共 8 个 Pod
  minTaskMember:
    inference-0: 4                   # 副本 0 中的全部 4 个 Pod
    inference-1: 4                   # 副本 1 中的全部 4 个 Pod
---
# 每副本 LWS（Controller 为每个副本创建一个）
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
  replicas: 1                         # 每副本模式中始终为 1
  leaderWorkerTemplate:
    size: 4                           # 每个副本 4 个 Pod
    # leaderTemplate：Leader 启动 Ray head 并运行 vLLM
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
            image: vllm/vllm-openai:v0.27.1
            command: ["/bin/sh", "-c"]
            args:
              - "ray start --head --port=6379 && vllm serve deepseek-ai/DeepSeek-R1 --tensor-parallel-size 32 --distributed-executor-backend ray"
            ports:
              - containerPort: 8000
              - containerPort: 6379
            resources:
              limits:
                nvidia.com/gpu: "8"
    # workerTemplate：Worker 加入 Ray 集群
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
            image: vllm/vllm-openai:v0.27.1
            command: ["/bin/sh", "-c"]
            args:
              - "ray start --address=$LWS_LEADER_ADDRESS:6379 --block"
            resources:
              limits:
                nvidia.com/gpu: "8"
```

> **注意**：对于多节点部署，Controller 会自动生成独立的 `leaderTemplate` 和 `workerTemplate`：
> - **Leader**：`ray start --head && <original command> --distributed-executor-backend ray`
> - **Worker**：`ray start --address=$LWS_LEADER_ADDRESS:6379 --block`

### Gang Scheduling 行为 {#gang-scheduling-behavior}

**InferenceService Controller** 为每个 InferenceService 创建**一个共享 PodGroup**。`minTaskMember` 字段使用 `{roleName}-{replicaIndex}` 格式的键，以实现细粒度 Gang Scheduling，并确保：

1. **副本内原子性**：单个副本中的所有 Pod 一起调度（全有或全无）
2. **跨角色协调**（适用于 PD 分离）：必须至少同时调度一个 Prefill 副本和一个 Decode 副本

| 场景 | LWS 数量 | PodGroup 数量 | minTaskMember 键 |
|------|----------|---------------|-------------------|
| 单体（单节点） | 每个副本 1 个 | 0 | N/A（无 Gang Scheduling） |
| 单体（多节点） | 每个副本 1 个 | 1 个共享 PodGroup | `{role}-0`、`{role}-1`，... |
| PD 分离 | 每个副本 1 个 | 1 个共享 PodGroup | `prefill-0`、`decode-0`、`decode-1`，... |

**示例：PD 分离多节点部署（故事 4）**

对于 `Prefill（1 个副本 × 2 个节点）` + `Decode（2 个副本 × 4 个节点）`：

```yaml
# 整个 InferenceService 使用一个 PodGroup
apiVersion: scheduling.volcano.sh/v1beta1
kind: PodGroup
metadata:
  name: deepseek-r1-disagg
spec:
  minMember: 10              # 2 + 4 + 4 = 共 10 个 Pod
  minTaskMember:
    prefill-0: 2             # Prefill 副本 0 中的全部 2 个 Pod
    decode-0: 4              # Decode 副本 0 中的全部 4 个 Pod
    decode-1: 4              # Decode 副本 1 中的全部 4 个 Pod
```

**调度行为表：**

| 集群 GPU | prefill-0（16 张 GPU） | decode-0（32 张 GPU） | decode-1（32 张 GPU） | 服务状态 |
|----------|------------------------|-----------------------|-----------------------|----------|
| 80 张 GPU | ✅ | ✅ | ✅ | 全部容量 |
| 64 张 GPU | ✅ | ✅ | ⏳ | 部分可用（1P + 1D） |
| 48 张 GPU | ✅ | ✅ | ⏳ | 部分可用（1P + 1D） |
| 32 张 GPU | ⏳ | ⏳ | ⏳ | ❌ 阻塞（无法以原子方式满足 1P + 1D） |
| 16 张 GPU | ⏳ | ⏳ | ⏳ | ❌ 阻塞（资源只够 Prefill） |

> **注意**：Volcano 确保每个任务的 Pod 以原子方式调度。使用 `minTaskMember` 时，调度器会等待，直到任务内的**所有** Pod 都能一起调度，从而避免副本内出现部分部署。

#### PodGroup 管理 {#podgroup-management}

**InferenceService Controller** 为**每个 InferenceService 创建一个 PodGroup**。`minTaskMember` 的键使用 `{roleName}-{replicaIndex}` 格式来标识每个副本的任务：

```yaml
# PD 分离部署的 PodGroup：Prefill（2 个副本 × 1 个节点）+ Decode（4 个副本 × 1 个节点）
apiVersion: scheduling.volcano.sh/v1beta1
kind: PodGroup
metadata:
  name: qwen3-inference              # 以 InferenceService 命名
  namespace: default
spec:
  minMember: 6                       # 2 + 4 = 6 个 Pod
  minTaskMember:                     # 通过 Pod annotation volcano.sh/task-spec 匹配
    prefill-0: 1                     # annotation 为 "volcano.sh/task-spec: prefill-0" 的 Pod
    prefill-1: 1                     # annotation 为 "volcano.sh/task-spec: prefill-1" 的 Pod
    decode-0: 1                      # annotation 为 "volcano.sh/task-spec: decode-0" 的 Pod
    decode-1: 1                      # ... 依此类推
    decode-2: 1
    decode-3: 1
```

每个 LWS 都按副本创建，并通过 annotation 加入共享 PodGroup：

```yaml
# 副本 0 的 Prefill LWS（Controller 为每个副本创建一个 LWS）
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
  replicas: 1                        # 每副本模式中始终为 1
  leaderWorkerTemplate:
    size: 1                          # 每个副本 1 个 Pod（单节点）
    workerTemplate:
      metadata:
        labels:
          fusioninfer.io/replica-index: "0"
        annotations:
          scheduling.k8s.io/group-name: qwen3-inference   # 加入共享 PodGroup
          volcano.sh/task-spec: prefill-0                 # 任务：{roleName}-{replicaIndex}
      spec:
        schedulerName: volcano
        containers:
          - name: vllm
            image: vllm/vllm-openai:v0.27.1
            # ... Prefill 配置
---
# 副本 0 的 Decode LWS
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
            image: vllm/vllm-openai:v0.27.1
            # ... Decode 配置
```

#### Volcano Gang Scheduling 的关键 Annotation {#key-annotations-for-volcano-gang-scheduling}

| Annotation | 定义位置 | 用途 |
|------------|----------|------|
| `scheduling.k8s.io/group-name` | `volcano.sh/apis/pkg/apis/scheduling/v1beta1` | 标识 Pod 所属的 PodGroup |
| `volcano.sh/task-spec` | `volcano.sh/apis/pkg/apis/batch/v1alpha1` | 标识 PodGroup 内的任务（与 `minTaskMember` 的键匹配） |

**Task-spec 格式：** `{roleName}-{replicaIndex}`（例如 `prefill-0`、`decode-1`）

**参考资料：**
- [Volcano MinTaskMember 设计](https://github.com/volcano-sh/volcano/blob/master/docs/design/task-minavailable.md)
- [Volcano Gang Scheduling](https://volcano.sh/en/docs/gang_scheduling/)

### CRD 结构概览 {#crd-structure-overview}

```go
// ComponentType 定义推理流水线中的组件类型
// +kubebuilder:validation:Enum=router;prefiller;decoder;worker
type ComponentType string

const (
    ComponentTypeRouter    ComponentType = "router"
    ComponentTypePrefiller ComponentType = "prefiller"
    ComponentTypeDecoder   ComponentType = "decoder"
    ComponentTypeWorker    ComponentType = "worker"
)

// InferenceServiceSpec 定义 InferenceService 的期望状态。
type InferenceServiceSpec struct {
    // Roles 是推理拓扑中的逻辑组件列表。
    // 每个角色都由用户定义的 Name 标识，并按 ComponentType 分类。
    Roles []Role `json:"roles"`
    
    // SchedulingStrategy 应用集群范围的调度策略（例如 Volcano）。
    // +optional
    SchedulingStrategy *SchedulingStrategy `json:"schedulingStrategy,omitempty"`
}

// SchedulingStrategy 定义 Pod 级别的调度行为。
type SchedulingStrategy struct {
    // SchedulerName 指定要使用的 Kubernetes 调度器（例如 "volcano"）。
    // +optional
    SchedulerName string `json:"schedulerName,omitempty"`
}

// Role 描述推理流水线中的逻辑组件。
type Role struct {
    // Name 是该组件由用户定义的唯一标识符（例如 "inference"）。
    Name string `json:"name"`

    // ComponentType 表示语义角色。有效值：
    // - "worker"：单体推理
    // - "prefiller"：prompt 处理
    // - "decoder"：token 生成
    // - "router"：使用调度插件的请求路由器
    ComponentType ComponentType `json:"componentType"`

    // Router 特有字段（仅用于 componentType: router）
    
    // Strategy 定义 Router 组件的路由策略
    // +optional
    Strategy RoutingStrategy `json:"strategy,omitempty"`
    
    // HTTPRoute 定义用于路由流量的 HTTPRoute 规范（Gateway API）
    // +optional
    HTTPRoute *runtime.RawExtension `json:"httproute,omitempty"`
    
    // Gateway 定义该 Router 的 Gateway 规范（Gateway API GatewaySpec）
    // +optional
    Gateway *runtime.RawExtension `json:"gateway,omitempty"`
    
    // EndpointPickerConfig 是用于 EPP 高级定制的原始 YAML
    // +optional
    EndpointPickerConfig string `json:"endpointPickerConfig,omitempty"`

    // Worker 特有字段（用于 prefiller/decoder/worker）
    
    // Replicas 指定要创建多少个相互独立的分布式实例。
    // 默认值：1
    // +optional
    Replicas *int32 `json:"replicas,omitempty"`

    // Multinode 使用内置的 Leader + Worker 拓扑启用分布式推理。
    // +optional
    Multinode *Multinode `json:"multinode,omitempty"`

    // Template 定义该组件的 Pod spec。
    // 使用 runtime.RawExtension 避免 CRD 大小限制。
    // +optional
    Template *runtime.RawExtension `json:"template,omitempty"`
}

// Multinode 启用多节点分布式推理。
type Multinode struct {
    // NodeCount 是分布该组件所跨的不同节点数量。
    NodeCount int32 `json:"nodeCount"`
}

// InferenceServiceStatus 反映观察到的 InferenceService 状态。
type InferenceServiceStatus struct {
    // ObservedGeneration 是 Controller 最近观察到的 generation。
    // +optional
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`
    
    // Conditions 表示对服务状态的最新可用观察结果。
    Conditions []metav1.Condition `json:"conditions,omitempty"`
    
    // Components 汇总每个已声明角色/组件的当前状态。
    // 键为组件的 .spec.roles[].name。
    // +optional
    Components map[string]ComponentStatus `json:"components,omitempty"`
}

// ComponentStatus 捕获单个推理组件（角色）的聚合运行时状态。
// 例如，replica=2 且 multinode.nodeCount=4 时：
//   - DesiredReplicas：2
//   - NodesPerReplica：4
//   - TotalPods：8（2 * 4）
//   - ReadyReplicas：0/1/2（仅当一个副本的所有节点均 Ready 时，该副本才 Ready）
//   - ReadyPods：0-8
type ComponentStatus struct {
    // DesiredReplicas 是请求的副本数（来自 spec.roles[].replica）。
    DesiredReplicas int32 `json:"desiredReplicas"`
    
    // ReadyReplicas 是完全 Ready 的副本数。
    // 对于多节点副本，仅当其所有节点均 Ready 时，该副本才 Ready。
    ReadyReplicas int32 `json:"readyReplicas"`
    
    // NodesPerReplica 是每个副本的节点数（来自 spec.roles[].multinode.nodeCount）。
    // 未配置 multinode 时默认为 1。
    NodesPerReplica int32 `json:"nodesPerReplica"`
    
    // TotalPods 是期望的 Pod 总数（= DesiredReplicas * NodesPerReplica）。
    TotalPods int32 `json:"totalPods"`
    
    // ReadyPods 是所有副本中 Ready Pod 的总数。
    ReadyPods int32 `json:"readyPods"`
    
    // Phase 表示该组件的高层生命周期阶段。
    // 可能的值：Pending、Deploying、Running、Failed、Unknown。
    Phase ComponentPhase `json:"phase"`
    
    // LastUpdateTime 是上次更新该组件状态时的时间戳。
    // +optional
    LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
}

// ComponentPhase 是组件所处生命周期阶段的简单高层摘要。
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
