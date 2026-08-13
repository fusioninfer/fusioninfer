---
title: InferenceDeployment
description: 绑定 Model 与 RuntimeProfile，并声明副本数、缓存、路由、发布与状态行为。
---

## 资源定位 {#resource-purpose}

`InferenceDeployment` 是 Namespaced 资源，用于创建一个可访问的模型推理服务。它通过显式引用绑定 Base Model、可选的多个 LoRA 和 RuntimeProfile，并声明部署副本数、模型物化时机和 Gateway API 入口。

下面是一个最小的 Aggregated 结构示例。省略 `spec.cache` 时使用默认的 `lazy` 模式。

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceDeployment
metadata:
  name: qwen3-8b-chat
  namespace: team-a
spec:
  modelRef:
    kind: Model
    name: qwen3-8b-r1
  runtimeRef:
    kind: RuntimeProfile
    name: vllm-aggregated
  replicas:
    aggregated: 1
  endpoint:
    gatewayRef:
      name: inference-gateway
      namespace: gateway-system
```

## Spec {#spec}

### API 结构 {#api-structure}

```go
type InferenceDeploymentSpec struct {
    ModelRef   ResourceReference `json:"modelRef"`
    RuntimeRef ResourceReference `json:"runtimeRef"`

    // +listType=map
    // +listMapKey=servedName
    LoRA []LoRABinding `json:"lora,omitempty"`

    // +kubebuilder:default={mode: lazy}
    // +optional
    Cache      *CachePolicy      `json:"cache,omitempty"`
    Replicas   ReplicaSpec       `json:"replicas"`
    Endpoint   EndpointSpec      `json:"endpoint"`
}

type LoRABinding struct {
    ModelRef ResourceReference `json:"modelRef"`

    // +kubebuilder:validation:MinLength=1
    ServedName string `json:"servedName"`
}

type ResourceReference struct {
    Kind string `json:"kind"`
    Name string `json:"name"`
}

type CacheMode string

const (
    CacheModeLazy  CacheMode = "lazy"
    CacheModeEager CacheMode = "eager"
)

type CachePolicy struct {
    Mode CacheMode `json:"mode"`
}

type ReplicaSpec struct {
    Aggregated *int32 `json:"aggregated,omitempty"`
    Prefiller  *int32 `json:"prefiller,omitempty"`
    Decoder    *int32 `json:"decoder,omitempty"`
}

type EndpointSpec struct {
    GatewayRef     GatewayReference     `json:"gatewayRef"`
    Hostnames      []gatewayv1.Hostname `json:"hostnames,omitempty"`
    EndpointPicker *EndpointPickerSpec  `json:"endpointPicker,omitempty"`
}

type EndpointPickerSpec struct {
    // 复用现有的 v1alpha1 RoutingStrategy 类型。
    // +kubebuilder:validation:Enum=prefix-cache;kv-cache-utilization;queue-size
    Strategy RoutingStrategy `json:"strategy"`
}

type GatewayReference struct {
    Name        gatewayv1.ObjectName   `json:"name"`
    Namespace   *gatewayv1.Namespace   `json:"namespace,omitempty"`
    SectionName *gatewayv1.SectionName `json:"sectionName,omitempty"`
}
```

### 资源引用 {#resource-references}

`modelRef` 和 `runtimeRef` 使用显式的 `kind + name`：

- `modelRef.kind` 只允许 `Model` 或 `ClusterModel`。
- `runtimeRef.kind` 只允许 `RuntimeProfile` 或 `ClusterRuntimeProfile`。
- API Group 固定为 `fusioninfer.io`，因此引用中不重复声明 `apiGroup`。
- 引用不提供默认 Kind、selector、Namespace fallback 或同名资源回退。

`modelRef` 必须指向 Base Model。LoRA 通过独立的 `spec.lora` 列表绑定，不能把 LoRA Model 直接作为 `modelRef`。

### LoRA 绑定 {#lora-bindings}

`spec.lora` 可以把多个 LoRA Model 绑定到同一个 Base Model 部署：

```yaml
lora:
  - modelRef:
      kind: Model
      name: qwen3-8b-finance-lora-r1
    servedName: finance
  - modelRef:
      kind: Model
      name: qwen3-8b-support-lora-r1
    servedName: customer-support
```

每个绑定包含两项信息：

- `modelRef` 必须解析到具有 `spec.lora.baseModelRef` 的 LoRA Model。
- `servedName` 是 OpenAI-compatible 请求中选择该 LoRA 时使用的模型名称。

Controller 解析 LoRA 后必须确认其 `baseModelRef` 与 Deployment 的 `modelRef` 指向相同 Kind、名称和 UID。相同 `servedName` 和重复 `modelRef` 都会被拒绝。绑定数量不能超过 RuntimeProfile 的 `lora.maxLoadedAdapters`。

引用的 RuntimeProfile 必须声明 `spec.lora`：

- `loadingMode: preload`：Controller 在创建 workload revision 前物化全部 LoRA，并把绑定清单交给 backend 的启动集成。增加、删除或替换绑定会创建新的 workload revision。
- `loadingMode: dynamic`：Controller 在现有 Base Model 工作负载上调和加载和卸载，不重启工作负载。增加、删除或替换绑定只更新 LoRA binding revision。

P/D 模式下，同一个绑定必须加载到全部 Prefiller 和 Decoder 逻辑副本。Controller 通过 backend integration 调用各逻辑副本的 Pod-local LoRA management endpoint；backend 可以由 Leader 协调组内加载，也可以由 integration 向全部成员 fan-out。只有该副本的 Leader 和 Worker 都确认目标 digest 已加载后才计为 Ready。

动态加载期间，Endpoint Picker 只发布已经在全部要求角色上 Ready 的 `servedName`。删除绑定时先停止发布并等待在途请求排空，再卸载 LoRA。使用同一个 `servedName` 替换 LoRA 制品时，如果 backend 不支持原子替换，该 LoRA 名称可能短暂不可用；Base Model 和其他 LoRA 不受影响。

### 副本 {#replicas}

`replicas` 使用与 RuntimeProfile 相同的字段组合：

- 只设置 `aggregated` 表示 Aggregated 部署。
- 同时设置 `prefiller` 和 `decoder` 表示 Prefill/Decode 分离部署。
- 每个值表示对应角色的逻辑副本数，而不是 Pod 数量。
- RuntimeProfile 为角色配置 `multinode.nodeCount` 时，一个逻辑副本会展开为一个 Leader Pod 和 `nodeCount - 1` 个 Worker Pod。

修改 `replicas` 只会增加或减少可独立路由的逻辑副本，不会改变 RuntimeProfile 固定的 `multinode.nodeCount`、每个 Pod 的加速器资源或 TP/PP/DP。`replicas` 也不等于 backend 的 data parallel size：前者创建独立的工作负载组和服务 Endpoint，后者是单个逻辑副本内部的引擎并行参数。

Deployment 的角色组合必须与引用的 RuntimeProfile 完全一致。引用 Aggregated Profile 时只能设置 `replicas.aggregated`；引用 P/D Profile 时必须同时设置 `replicas.prefiller` 和 `replicas.decoder`。

### 缓存 {#cache}

`spec.cache` 省略时默认为 `lazy`。显式设置 `cache.mode` 可以选择部署使用模型制品的时机：

- `lazy`：Pod 调度到节点后，由注入的 init container 检查节点缓存。缓存缺失时下载并校验模型，完成后才启动推理引擎。
- `eager`：创建新推理工作负载前，先在满足角色调度约束的节点上运行预热 Job。所需副本全部物化后才创建新工作负载。

该策略同时应用于 Base Model 和 `spec.lora` 引用的制品。`preload` 模式要求 Base Model 与全部 LoRA 在引擎启动前可读。`dynamic` 模式新增 LoRA 时，`eager` 先在全部目标节点预热，`lazy` 则由目标节点上的受信任 materializer 按需物化；无论哪种缓存模式，物化完成后才调用 backend 加载接口。

`eager` 为每个角色按“逻辑副本数 × 有效节点数”计算预热需求。有效节点数来自该角色的 `multinode.nodeCount`，未设置时为 1；Controller 在满足 Pod 模板调度约束的 distinct nodes 上各准备一份模型缓存。

两种模式使用相同的内容寻址节点缓存，并以规范化 source URI、revision 或 OCI descriptor digest，以及可选 `source.digest` 派生不可变 cache key。

`eager` 预热 Job 不申请或预留 GPU。模型已预热但 GPU 不可用时，工作负载可以保持 Pending；新版本失败不会提前删除仍在服务的上一版本。

`pvc://` 来源也必须先复制到不可变节点缓存。引擎只读挂载缓存副本，不直接使用可变的源 PVC 内容。

### Endpoint {#endpoint}

`endpoint` 在 v1 中必填。Controller 根据该字段创建 Endpoint Picker、InferencePool 和 HTTPRoute：

- `gatewayRef.name` 指定 HTTPRoute 连接的 Gateway。
- `gatewayRef.namespace` 省略时使用 `InferenceDeployment` 所在 Namespace。
- `gatewayRef.sectionName` 可选，用于选择 Gateway listener。
- `hostnames` 写入 HTTPRoute 的同名字段，只参与入口请求匹配。

`gatewayRef` 的 API Group 固定为 `gateway.networking.k8s.io`，Kind 固定为 `Gateway`，因此只需要填写名称和可选的 Namespace、SectionName。

`endpointPicker` 只用于 Aggregated 部署：

- 支持 `prefix-cache`、`kv-cache-utilization` 和 `queue-size`。
- 省略时使用 Operator 配置的默认策略。
- P/D 部署必须省略该字段；Controller 根据 `prefiller + decoder` 自动生成 Prefill 和 Decode 调度配置。

Endpoint Picker 的镜像、副本数和端口由 Operator 配置管理，不属于 RuntimeProfile 或 InferenceDeployment 的用户接口。

### 作用域与引用 {#scope-and-references}

`InferenceDeployment` 只存在 Namespaced 形式：

- 引用 `Model` 或 `RuntimeProfile` 时，只在 Deployment 所在 Namespace 查找。
- 引用 `ClusterModel` 或 `ClusterRuntimeProfile` 时，只查找 Cluster-scoped 对象。
- Namespaced 引用不能指定或访问其他 Namespace。
- ClusterModel 的凭据和 ClusterRuntimeProfile 中的 Namespaced 依赖都在 Deployment 所在 Namespace 解析。

跨 Namespace Gateway 是否接受生成的 HTTPRoute，由 Gateway listener 的 `allowedRoutes` 决定。FusionInfer 不修改 Gateway，也不会在引用失败后尝试其他同名对象。

引用解析成功后，Status 记录目标对象的 Kind、名称、UID 和 generation。Base Model 与 Runtime 位于 `resolvedRefs`，LoRA 位于对应的 `status.lora[]` 条目。删除并以相同名称重建对象会产生新的 UID，并被视为引用目标变化。

### 默认值与校验 {#defaults-and-validation}

- `modelRef`、`runtimeRef`、`replicas` 和 `endpoint` 必填。
- `modelRef.kind`、`modelRef.name`、`runtimeRef.kind` 和 `runtimeRef.name` 必填。
- `lora` 省略时不绑定 LoRA；存在时至少包含一个条目。
- 每个 `lora[].modelRef` 和 `servedName` 必填，`servedName` 在列表中必须唯一。
- `lora[].modelRef` 不能重复。
- `replicas` 只能设置 `aggregated`，或者同时设置 `prefiller` 和 `decoder`。
- 每个副本数必须大于等于 1。
- `cache` 省略时默认为 `{mode: lazy}`；显式设置时，`cache.mode` 只允许 `lazy` 或 `eager`。
- `endpoint.gatewayRef.name` 必填。
- `endpoint.endpointPicker.strategy` 只允许用于 Aggregated 部署，且必须属于支持的策略集合。
- RuntimeProfile 角色与 Deployment 副本组合的一致性在引用解析后校验。
- 声明 `lora` 时，RuntimeProfile 必须支持 LoRA，绑定数量不能超过 `maxLoadedAdapters`。
- Controller 必须确认每个绑定引用 LoRA Model，并且其 `baseModelRef` 与 Deployment 的 Base Model 引用解析到相同 UID。
- RuntimeProfile 的 backend adapter 必须支持模板中的镜像和入口参数，并且不能与模板声明的分布式编排参数冲突；不支持时 Controller 不创建新工作负载，并设置 `ReferencesResolved=False`。
- `InferenceDeployment.spec` 可以更新；Model、Runtime、缓存模式或 Endpoint Picker 策略变化会产生新的待提升 revision。LoRA 变化是否重建 workload 由 RuntimeProfile 的 `loadingMode` 决定。

不需要读取其他对象的约束由 CRD OpenAPI、CEL 或 Admission 校验。引用是否存在、角色是否一致、Secret/PVC 是否可用以及 Gateway 是否接受 Route，由 Controller 调和并通过 Conditions 报告，因此资源可以按任意顺序创建。

## Status {#status}

`InferenceDeployment.status` 汇总引用解析、模型物化、工作负载、路由和 rollout 状态：

```go
type InferenceDeploymentStatus struct {
    ObservedGeneration int64                               `json:"observedGeneration,omitempty"`
    ResolvedRefs       *ResolvedReferences                 `json:"resolvedRefs,omitempty"`
    ModelCache         *ModelCacheStatus                   `json:"modelCache,omitempty"`

    // +listType=map
    // +listMapKey=servedName
    LoRA               []LoRABindingStatus                 `json:"lora,omitempty"`

    Components         map[string]InferenceComponentStatus `json:"components,omitempty"`
    Endpoint           *EndpointStatus                     `json:"endpoint,omitempty"`
    ActiveRevision     *DeploymentRevisionStatus           `json:"activeRevision,omitempty"`
    PendingRevision    *DeploymentRevisionStatus           `json:"pendingRevision,omitempty"`

    // +listType=map
    // +listMapKey=type
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type ResolvedReferenceStatus struct {
    Kind       string    `json:"kind"`
    Name       string    `json:"name"`
    UID        types.UID `json:"uid"`
    Generation int64     `json:"generation"`
    Digest     string    `json:"digest,omitempty"`
}

type ModelCacheStatus struct {
    Mode          CacheMode `json:"mode"`
    Digest        string    `json:"digest,omitempty"`
    DesiredCopies int32     `json:"desiredCopies,omitempty"`
    ReadyCopies   int32     `json:"readyCopies,omitempty"`
}

type InferenceComponentStatus struct {
    DesiredReplicas int32 `json:"desiredReplicas"`
    ReadyReplicas   int32 `json:"readyReplicas"`
    NodesPerReplica int32 `json:"nodesPerReplica"`
    ReadyPods       int32 `json:"readyPods"`
}

type LoRABindingStatus struct {
    ServedName string                              `json:"servedName"`
    Model      *ResolvedReferenceStatus            `json:"model,omitempty"`
    Cache      *ModelCacheStatus                   `json:"cache,omitempty"`
    Components map[string]LoRAComponentStatus      `json:"components,omitempty"`

    // +listType=map
    // +listMapKey=type
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type LoRAComponentStatus struct {
    DesiredReplicas int32 `json:"desiredReplicas"`
    ReadyReplicas   int32 `json:"readyReplicas"`
}
```

`components` 使用稳定的 `endpointPicker`、`aggregated`、`prefiller` 和 `decoder` 键。`nodesPerReplica` 记录 RuntimeProfile 中生效的 `multinode.nodeCount`，未设置时为 1；`readyReplicas` 只统计该逻辑副本全部 `nodesPerReplica` 个 Pod 都 Ready 的副本。`ModelCacheStatus.desiredCopies` 和 `readyCopies` 分别记录当前 revision 期望和已经完成的 distinct node 缓存副本数。`activeRevision` 表示当前接收流量的版本；`pendingRevision` 表示正在物化、调度或等待提升的新版本。

`status.lora` 以 `servedName` 为列表键，并为每个绑定汇总解析后的 Model、缓存副本以及各角色的加载进度。它不公开 backend management endpoint 的地址或逐 Pod 错误正文。`preload` 模式的 Ready 状态随 workload revision 提升；`dynamic` 模式直接反映当前 active revision 中的加载状态。

Conditions 使用标准 `metav1.Condition`，不增加单一 `phase`：

- `ReferencesResolved`：Base Model、全部 LoRA Model 和 RuntimeProfile 引用已经解析并通过消费方校验。
- `ModelMaterialized`：当前 revision 所需的 Base Model 缓存副本已经满足；LoRA 物化状态记录在各绑定中。
- `LoRAReady`：所有声明的 LoRA 已完成解析、物化，并加载到全部要求的逻辑副本；没有 LoRA 时为 `True`。
- `WorkloadsReady`：所有角色的期望逻辑副本已经 Ready；多节点逻辑副本要求 Leader 和全部 Worker Pod 都 Ready。
- `RouteReady`：Service、InferencePool、Endpoint Picker 和 HTTPRoute 已经就绪。
- `Available`：当前 active revision 可以接受请求。
- `Progressing`：预热、创建、扩缩容或更新仍在进行。
- `Degraded`：调和失败并需要用户或平台管理员介入。

动态模式下，单个 LoRA 失败会使 `LoRAReady=False`，但只要 active Base Model 仍可服务，`Available` 可以保持 `True`。失败的 served name 不会发布到 Endpoint Picker。

每个 Condition 必须填写 `observedGeneration`、稳定的 `reason` 和可操作的 `message`。下面展示一个已经就绪的 Aggregated 部署状态：

```yaml
status:
  observedGeneration: 2
  resolvedRefs:
    model:
      kind: ClusterModel
      name: qwen3-8b-r1
      uid: 8bb42ec6-95a5-4dc8-904f-6a69ee9bb8c8
      generation: 1
      digest: sha256:7f3c9a1e4b2d8f605a7c3e9d1b4f2860c5a8e2d7f1b3096c4e8a5d2f7b1c903e
    runtime:
      kind: ClusterRuntimeProfile
      name: vllm-aggregated-h100-r1
      uid: f42e664a-bf65-4690-8e0a-93568e854e3d
      generation: 1
  modelCache:
    mode: eager
    digest: sha256:7f3c9a1e4b2d8f605a7c3e9d1b4f2860c5a8e2d7f1b3096c4e8a5d2f7b1c903e
    desiredCopies: 2
    readyCopies: 2
  components:
    aggregated:
      desiredReplicas: 2
      readyReplicas: 2
      nodesPerReplica: 1
      readyPods: 2
    endpointPicker:
      desiredReplicas: 1
      readyReplicas: 1
      nodesPerReplica: 1
      readyPods: 1
  endpoint:
    routeRef:
      name: qwen3-8b-chat
      namespace: team-a
    addresses:
      - hostname: qwen3-8b.example.com
        url: https://qwen3-8b.example.com
  conditions:
    - type: Available
      status: "True"
      observedGeneration: 2
      reason: Ready
      message: 当前 active revision 已就绪，可以接收请求。
```

## 示例 {#examples}

### Namespaced Aggregated：默认 lazy 缓存 {#namespaced-aggregated-default-lazy-cache}

Model 和 RuntimeProfile 都从 `team-a` Namespace 解析。该部署省略 `spec.cache` 以使用默认的 `lazy` 模式，创建两个 Aggregated 逻辑副本，并使用 prefix-cache Endpoint Picker。

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceDeployment
metadata:
  name: llama-3-8b-chat
  namespace: team-a
spec:
  modelRef:
    kind: Model
    name: llama-3-8b-r1
  runtimeRef:
    kind: RuntimeProfile
    name: vllm-aggregated-a10-r1
  replicas:
    aggregated: 2
  endpoint:
    gatewayRef:
      name: inference-gateway
      namespace: gateway-system
      sectionName: https
    hostnames:
      - llama-3-8b.example.com
    endpointPicker:
      strategy: prefix-cache
```

### Cluster Prefill/Decode：eager 缓存 {#cluster-prefilldecode-eager-cache}

Model 和 RuntimeProfile 使用 Cluster-scoped 对象。Controller 在启动两个 Prefiller 和四个 Decoder 逻辑副本前预热模型；P/D 部署不声明 `endpointPicker`。

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceDeployment
metadata:
  name: qwen3-32b-chat
  namespace: team-b
spec:
  modelRef:
    kind: ClusterModel
    name: qwen3-32b-r1
  runtimeRef:
    kind: ClusterRuntimeProfile
    name: vllm-pd-h100-r1
  cache:
    mode: eager
  replicas:
    prefiller: 2
    decoder: 4
  endpoint:
    gatewayRef:
      name: inference-gateway
      namespace: gateway-system
      sectionName: https
    hostnames:
      - qwen3-32b.example.com
```

### Namespaced Aggregated：多个动态 LoRA {#namespaced-aggregated-multiple-dynamic-loras}

该部署使用支持动态 LoRA 的 RuntimeProfile。两个 LoRA 分别以 `finance` 和 `customer-support` 作为请求中的模型名称。

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceDeployment
metadata:
  name: qwen3-8b-chat
  namespace: team-a
spec:
  modelRef:
    kind: Model
    name: qwen3-8b-r1
  runtimeRef:
    kind: RuntimeProfile
    name: vllm-aggregated-lora-dynamic-r1
  lora:
    - modelRef:
        kind: Model
        name: qwen3-8b-finance-lora-r1
      servedName: finance
    - modelRef:
        kind: Model
        name: qwen3-8b-support-lora-r1
      servedName: customer-support
  cache:
    mode: eager
  replicas:
    aggregated: 2
  endpoint:
    gatewayRef:
      name: inference-gateway
      namespace: gateway-system
      sectionName: https
    hostnames:
      - qwen3-8b.example.com
    endpointPicker:
      strategy: prefix-cache
```
