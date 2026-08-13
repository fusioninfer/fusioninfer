---
sidebar_position: 2
title: Router 控制器
---

:::warning 旧版 API 设计
本页记录由现有 `InferenceService` v1alpha1 API 生成的路由。在目标 [`InferenceDeployment` 设计](./inference-deployment.md)中，Gateway 关联通过 `spec.endpoint` 配置，Aggregated 路由策略通过 `spec.endpoint.endpointPicker` 配置，而 Prefill/Decode 路由则根据引用的 RuntimeProfile 角色推断。
:::

# Router 控制器 {#router-controller}

## 概述 {#summary}

FusionInfer Router 控制器支持根据 `InferenceService` CRD 自动创建和管理 Gateway API Inference Extension 资源。此集成利用 Endpoint Picker (EPP) 实现智能请求路由、prefix cache 感知调度和负载均衡，将 FusionInfer 管理的推理工作负载转换为经过优化的推理路由系统。

创建的 `InferenceService` 中包含 `componentType: router` 的角色时，FusionInfer Router 控制器会生成 `InferencePool`、`HTTPRoute`，以及 EPP 的 ServiceAccount、RBAC、ConfigMap、Deployment 和 Service 资源。生成的 `HTTPRoute` 通过配置的 `parentRefs` 关联到现有 Gateway；FusionInfer 不会创建 Gateway 本身。EPP 配置支持 prefix cache 优化、基于 KV cache 利用率的路由、queue size 感知调度、LoRA 适配器亲和性，以及分离式 Prefill/Decode 架构。

## 动机 {#motivation}

Kubernetes 上的自托管 LLM 推理工作负载在请求路由、负载均衡和资源利用率方面面临重大挑战。传统负载均衡器不了解模型特有的特征，例如 KV cache 状态、prefix cache 复用机会，或 Prefill 阶段与 Decode 阶段之间的区别。这会导致性能不佳、延迟增加以及 GPU 利用率低下。

Gateway API Inference Extension 提供了专为推理工作负载设计的高级路由能力，但需要手动配置多个 CRD，并深入了解插件生态系统。FusionInfer 提供带有预定义路由策略的更高层抽象，从而简化这一过程，同时仍允许高级用户在需要时自定义完整的 `EndpointPickerConfig`。

### 目标 {#goals}

- **自动生成 Router 资源**：根据 `InferenceService` 创建和管理 `InferencePool`、`HTTPRoute`，以及 EPP 工作负载和配置资源
- **预定义路由策略**：为常见用例提供简单的声明式路由策略（`prefix-cache`、`kv-cache-utilization`、`queue-size`、`lora-affinity`、`pd-disaggregation`）
- **高级自定义**：允许高级用户提供自定义 `EndpointPickerConfig`，以实现精细控制

### 非目标 {#non-goals}

- **工作负载管理**：本设计仅关注 Router/Gateway 集成；实际的 Pod/Deployment 管理由其他 FusionInfer 组件负责
- **Gateway 实现**：本设计不实现新的 Gateway，而是通过 Envoy ext-proc 与现有符合 Gateway API 的 Gateway 集成

### 用户故事 {#user-stories}

#### 故事 1：prefix cache 感知路由 {#story-1-prefix-cache-aware-routing}

作为平台工程师，我希望部署一个采用 prefix cache 感知路由的 Qwen 模型，使共享最长前缀的请求被路由到同一服务器，从而最大限度地复用 KV cache。

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: qwen-service
spec:
  roles:
    - name: router
      componentType: router
      strategy: "prefix-cache"
      httproute:
        parentRefs:
        - name: my-gateway
          namespace: gateway-system
        hostnames:
        - "qwen.example.com"
    - name: inference
      componentType: worker
      replicas: 3
      template:
        spec:
          containers:
            - name: vllm
              image: vllm/vllm-openai:v0.27.1
              args:
                - --model=Qwen/Qwen2.5-7B-Instruct
```

这会自动创建指向推理 Pod 的 `InferencePool`，并为 EPP 配置 prefix cache 感知调度。

#### 故事 2：基于 KV cache 利用率的负载均衡 {#story-2-kv-cache-utilization-based-load-balancing}

作为 ML 工程师，我希望根据可用 KV cache 内存分发推理请求，以避免 OOM 错误，并确保所有推理服务器之间的资源利用率均衡。

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: balanced-llm-service
spec:
  roles:
    - name: router
      componentType: router
      strategy: "kv-cache-utilization" 
      httproute:
        parentRefs:
        - name: my-gateway
          namespace: gateway-system
        hostnames:
        - "api.balanced.example.com"
    - name: inference
      componentType: worker
      replicas: 3
      template:
        spec:
          containers:
            - name: vllm
              image: vllm/vllm-openai:v0.27.1
              args:
                - --model=/models/llama-70b
```

此配置会将请求路由到可用 KV cache 内存最多的服务器，从而避免内存耗尽，并确保利用率均衡。

#### 故事 3：分离式 Prefill/Decode 架构 {#story-3-disaggregated-prefilldecode-architecture}

作为基础设施工程师，我希望部署一种分离式服务架构，其中 Prefill（提示词处理）和 Decode（token 生成）由专用服务器池处理，以优化硬件利用率和延迟。

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: disaggregated-llm-service
spec:
  roles:
    - name: router
      componentType: router
      strategy: "pd-disaggregation"
      httproute:
        parentRefs:
        - name: my-gateway
          namespace: gateway-system
        hostnames:
        - "llm.disaggregated.example.com"
    
    - name: prefill-servers
      componentType: prefiller
      replicas: 2
      template:
        spec:
          containers:
            - name: prefill
              image: vllm/vllm-openai:v0.27.1
              args:
                - --model=meta-llama/Llama-3-70B-Instruct
                - --kv-transfer-config '{"kv_connector":"NixlConnector","kv_role":"kv_both"}'
    - name: decode-servers
      componentType: decoder
      replicas: 3
      template:
        spec:
          containers:
            - name: decode
              image: vllm/vllm-openai:v0.27.1
              args:
                - --model=meta-llama/Llama-3-70B-Instruct
                - --kv-transfer-config '{"kv_connector":"NixlConnector","kv_role":"kv_both"}'
```

## 提案 {#proposal}

FusionInfer Router 控制器会监视包含 `componentType: router` 角色的 `InferenceService` 资源。检测到此类资源后，它会自动生成并管理 Gateway API Inference Extension 资源，从而为 LLM 推理工作负载提供智能请求路由和负载均衡。

Router 角色支持两种配置方式：
1. **基于简单策略的配置**：为常见用例使用预定义路由策略（`prefix-cache`、`kv-cache-utilization`、`queue-size`、`lora-affinity`、`pd-disaggregation`）
2. **高级自定义配置**：提供完整的 `endpointPickerConfig`，以精细控制调度插件、Profile 和评分算法

这些配置会转换为 `InferencePool`、`HTTPRoute`，以及包含配套 ServiceAccount、RBAC、Service 和 ConfigMap 的 EPP Deployment。生成的 `HTTPRoute` 引用现有 Gateway，而 EPP 配置则根据 prefix cache 命中、内存利用率和工作负载特征等因素控制路由。

### Go 类型 {#go-types}

```go
// InferenceServiceSpec 定义 InferenceService 的期望状态
type InferenceServiceSpec struct {
    // Roles 定义推理服务的组件
    Roles []RoleSpec `json:"roles"`
}

// Role 定义推理流水线中的一个组件
type RoleSpec struct {
    // Name 是此角色的标识符
    Name string `json:"name"`
    
    // ComponentType 指定组件类型
    // +kubebuilder:validation:Enum=router;prefiller;decoder;worker
    ComponentType ComponentType `json:"componentType"`
    
    // Router 特有字段（仅用于 componentType: router）
    Strategy              RoutingStrategy        `json:"strategy,omitempty"`
    HTTPRoute             *runtime.RawExtension  `json:"httproute,omitempty"`   // Gateway API HTTPRouteSpec
    Gateway               *runtime.RawExtension  `json:"gateway,omitempty"`     // 预留；控制器不调和 Gateway 资源
    EndpointPickerConfig  string                 `json:"endpointPickerConfig,omitempty"`  // 面向高级用户的原始 YAML
    
    // Worker 特有字段（用于 prefiller/decoder/worker）
    Replicas       *int32                 `json:"replicas,omitempty"`
    Template       *runtime.RawExtension  `json:"template,omitempty"`  // corev1.PodTemplateSpec
}

// ComponentType 定义组件类型
type ComponentType string

const (
    ComponentTypeRouter    ComponentType = "router"
    ComponentTypePrefiller ComponentType = "prefiller"
    ComponentTypeDecoder   ComponentType = "decoder"
    ComponentTypeWorker    ComponentType = "worker"
)

// RoutingStrategy 定义推理路由策略
type RoutingStrategy string

const (
    StrategyPrefixCache      RoutingStrategy = "prefix-cache"
    StrategyKVCacheUtil      RoutingStrategy = "kv-cache-utilization"
    StrategyQueueSize        RoutingStrategy = "queue-size"
    StrategyLoRAffinity      RoutingStrategy = "lora-affinity"
    StrategyPDDisaggregation RoutingStrategy = "pd-disaggregation"
)
```

### 配置示例 {#configuration-examples}

#### 示例 1：基于简单策略的配置 {#example-1-simple-strategy-based-configuration}

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: my-service
spec:
  roles:
    - name: gateway
      componentType: router
      strategy: "prefix-cache"
      httproute:
        parentRefs:
        - name: my-gateway
          namespace: gateway-system
        hostnames:
        - "api.example.com"
    - name: inference
      componentType: worker
      replicas: 3
      template:
        spec:
          containers:
          - name: vllm
            image: vllm/vllm-openai:v0.27.1
            args:
            - --model=meta-llama/Llama-3-8B-Instruct
```

FusionInfer 会自动生成以下资源：

```yaml
# 1. HTTPRoute - 将流量路由到 InferencePool
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-service-httproute
spec:
  parentRefs:
  - name: my-gateway
    namespace: gateway-system
  hostnames:
  - "api.example.com"
  rules:
  - backendRefs:
    - group: inference.networking.k8s.io
      kind: InferencePool
      name: my-service-pool

---
# 2. InferencePool - 管理推理后端 Pod
apiVersion: inference.networking.k8s.io/v1
kind: InferencePool
metadata:
  name: my-service-pool
spec:
  selector:
    matchLabels:
      fusioninfer.io/service: my-service
      fusioninfer.io/component-type: worker
  targetPorts:
  - number: 8000
  endpointPickerRef:
    name: my-service-epp
    port:
      number: 9002

---
# 3. EndpointPickerConfig (ConfigMap) - EPP 调度配置
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-service-epp-config
data:
  config.yaml: |
    apiVersion: inference.networking.x-k8s.io/v1alpha1
    kind: EndpointPickerConfig
    plugins:
    - type: prefix-cache-scorer
      parameters:
        blockSize: 5
        maxPrefixBlocksToMatch: 256
        lruCapacityPerServer: 31250
    - type: max-score-picker
    schedulingProfiles:
    - name: default
      plugins:
      - pluginRef: max-score-picker
      - pluginRef: prefix-cache-scorer
        weight: 100

---
# 4. EPP Deployment - Endpoint Picker 部署
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-service-epp
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: my-service-epp
  template:
    metadata:
      labels:
        app: my-service-epp
    spec:
      serviceAccountName: my-service-epp
      containers:
      - name: epp
        image: registry.k8s.io/gateway-api-inference-extension/epp:v1.2.1
        args:
        - --pool-name=my-service-pool
        - --pool-namespace=default
        - --config-file=/config/config.yaml
        ports:
        - name: grpc
          containerPort: 9002
        - name: grpc-health
          containerPort: 9003
        - name: metrics
          containerPort: 9090
        livenessProbe:
          grpc:
            port: 9003
            service: inference-extension
          initialDelaySeconds: 5
          periodSeconds: 10
        readinessProbe:
          grpc:
            port: 9003
            service: inference-extension
          periodSeconds: 2
        env:
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        volumeMounts:
        - name: plugins-config-volume
          mountPath: /config
      volumes:
      - name: plugins-config-volume
        configMap:
          name: my-service-epp-config

---
# 5. EPP Service - 通过 Envoy ext-proc 暴露 EPP
apiVersion: v1
kind: Service
metadata:
  name: my-service-epp
spec:
  selector:
    app: my-service-epp
  type: ClusterIP
  ports:
  - name: grpc-ext-proc
    port: 9002
    protocol: TCP
  - name: grpc-health
    port: 9003
    protocol: TCP
  - name: http-metrics
    port: 9090
    protocol: TCP
```

#### 示例 2：使用自定义 EndpointPickerConfig 的高级配置 {#example-2-advanced-configuration-with-custom-endpointpickerconfig}

对于有特定调优需求的高级用户，FusionInfer 允许直接配置 EndpointPickerConfig。这样可以完全控制调度行为、插件参数和评分权重。

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: advanced-service
spec:
  roles:
    - name: gateway
      componentType: router
      # 直接控制 EPP 配置以进行高级调优
      endpointPickerConfig: |
        apiVersion: inference.networking.x-k8s.io/v1alpha1
        kind: EndpointPickerConfig
        plugins:
        - type: prefix-cache-scorer
          parameters:
            blockSize: 10
            maxPrefixBlocksToMatch: 512
            lruCapacityPerServer: 50000 
        - type: kv-cache-utilization-scorer
        - type: max-score-picker
        schedulingProfiles:
        - name: default
          plugins:
          - pluginRef: max-score-picker
          - pluginRef: prefix-cache-scorer
            weight: 70 
          - pluginRef: kv-cache-utilization-scorer
            weight: 30
      httproute:
        parentRefs:
        - name: my-gateway
          namespace: gateway-system
        hostnames:
        - "advanced.example.com"
    - name: inference
      componentType: worker
      replicas: 5
      template:
        spec:
          containers:
          - name: vllm
            image: vllm/vllm-openai:v0.27.1
            args:
            - --model=meta-llama/Llama-3-8B-Instruct
```

#### 示例 3：分离式 Prefill/Decode 架构 {#example-3-disaggregated-prefilldecode-architecture}

对于大规模 LLM 部署，分离式 Prefill/Decode 架构将计算密集型提示词处理（Prefill）阶段与内存密集型 token 生成（Decode）阶段分开。这样可以独立扩缩和优化每个阶段：

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: disaggregated-llm-service
spec:
  roles:
    - name: router
      componentType: router
      strategy: "pd-disaggregation"
      httproute:
        parentRefs:
        - name: my-gateway
          namespace: gateway-system
        hostnames:
        - "llm.disaggregated.example.com"
    
    - name: prefill-servers
      componentType: prefiller
      replicas: 2
      template:
        spec:
          containers:
            - name: prefill
              image: vllm/vllm-openai:v0.27.1
              args:
                - --model=meta-llama/Llama-3-70B-Instruct
                - --kv-transfer-config '{"kv_connector":"NixlConnector","kv_role":"kv_both"}'
    - name: decode-servers
      componentType: decoder
      replicas: 2
      template:
        spec:
          containers:
            - name: decode
              image: vllm/vllm-openai:v0.27.1
              args:
                - --model=meta-llama/Llama-3-70B-Instruct
                - --kv-transfer-config '{"kv_connector":"NixlConnector","kv_role":"kv_both"}'
```

**生成的资源：**

FusionInfer Router 控制器会为分离式 Prefill/Decode 架构创建以下 Gateway API 和 EPP 资源。生成的 `HTTPRoute` 所引用的 Gateway 必须已经存在。

> **注意**：在完整的 P/D 部署中，Decode Pod 通常包含一个用于编排与 Prefill Pod 通信的 sidecar。该 sidecar 的部署由工作负载组件管理，而不是由 Gateway 组件管理。

```yaml
# 1. InferencePool - 管理 Prefill 和 Decode Pod 的单一池
apiVersion: inference.networking.k8s.io/v1
kind: InferencePool
metadata:
  name: disaggregated-llm-service-pool
spec:
  selector:
    matchLabels:
      fusioninfer.io/service: disaggregated-llm-service
      leaderworkerset.sigs.k8s.io/worker-index: "0"
  endpointPickerRef:
    name: disaggregated-llm-service-epp
    port:
      number: 9002
  targetPorts:
  - number: 8000

---
# 2. ConfigMap - P/D 感知的 Endpoint Picker 配置
apiVersion: v1
kind: ConfigMap
metadata:
  name: disaggregated-llm-service-epp-config
data:
  config.yaml: |
    apiVersion: inference.networking.x-k8s.io/v1alpha1
    kind: EndpointPickerConfig
    plugins:
    - type: pd-profile-handler
      parameters:
        threshold: 0
        hashBlockSize: 5
        primaryPort: 8000
    - type: prefill-header-handler
    - type: by-label
      name: prefill-pods
      parameters:
        label: "fusioninfer.io/component-type"
        validValues: ["prefiller"]
    - type: by-label
      name: decode-pods
      parameters:
        label: "fusioninfer.io/component-type"
        validValues: ["decoder"]
    - type: prefix-cache-scorer
      parameters:
        hashBlockSize: 5
        maxPrefixBlocksToMatch: 256
        lruCapacityPerServer: 31250
    - type: max-score-picker
    schedulingProfiles:
    - name: prefill
      plugins:
      - pluginRef: prefill-pods
      - pluginRef: max-score-picker
      - pluginRef: prefix-cache-scorer
        weight: 50
    - name: decode
      plugins:
      - pluginRef: decode-pods
      - pluginRef: max-score-picker
      - pluginRef: prefix-cache-scorer
        weight: 50

---
# 3. HTTPRoute - 将流量路由到 EPP
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: disaggregated-llm-service-httproute
spec:
  parentRefs:
  - name: my-gateway
    namespace: gateway-system
  hostnames:
  - "llm.disaggregated.example.com"
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /
    backendRefs:
    - name: disaggregated-llm-service-pool
      group: inference.networking.k8s.io
      kind: InferencePool

---
# 4. EPP Deployment 和 Service（自动配置）
apiVersion: apps/v1
kind: Deployment
metadata:
  name: disaggregated-llm-service-epp
spec:
  replicas: 1
  selector:
    matchLabels:
      app: disaggregated-llm-service-epp
  template:
    metadata:
      labels:
        app: disaggregated-llm-service-epp
    spec:
      containers:
      - name: epp
        image: registry.k8s.io/gateway-api-inference-extension/epp:v1.2.1
        args:
        - --pool-name=disaggregated-llm-service-pool
        - --pool-namespace=default
        - --config-file=/config/config.yaml
        ports:
        - containerPort: 9002
        - containerPort: 9003
        - containerPort: 9090
        volumeMounts:
        - name: config
          mountPath: /config
      volumes:
      - name: config
        configMap:
          name: disaggregated-llm-service-epp-config

---
apiVersion: v1
kind: Service
metadata:
  name: disaggregated-llm-service-epp
spec:
  selector:
    app: disaggregated-llm-service-epp
  ports:
  - name: grpc-ext-proc
    port: 9002
  - name: grpc-health
    port: 9003
  - name: http-metrics
    port: 9090
```

## 路由策略到 EndpointPickerConfig 的映射 {#strategy-to-endpointpickerconfig-mapping}

FusionInfer 会根据路由策略自动生成相应的 EndpointPickerConfig：

| 路由策略 | 生成的插件 | 用例 |
|-----------------|------------------|----------|
| `prefix-cache` | • `prefix-cache-scorer`（使用优化参数）<br/>• `max-score-picker`<br/>• 单个 `default` 调度 Profile | 将具有最长共享前缀的请求路由到同一服务器，同时兼顾 KV cache 利用率和 queue size |
| `kv-cache-utilization` | • `kv-cache-utilization-scorer`<br/>• `max-score-picker`<br/>• 单个 `default` 调度 Profile | 根据各推理服务器的内存使用情况均衡负载 |
| `queue-size` | • `queue-scorer`<br/>• `max-score-picker`<br/>• 单个 `default` 调度 Profile | 将请求路由到负载最低的服务器，从而最大限度缩短请求等待时间 |
| `lora-affinity` | • `lora-affinity-scorer`<br/>• `max-score-picker`<br/>• 单个 `default` 调度 Profile | 将请求路由到具有匹配 LoRA 适配器的服务器，以支持多适配器服务 |
| `pd-disaggregation` | • `pd-profile-handler`<br/>• `prefill-header-handler`<br/>• `by-label` 过滤器（用于 prefiller/decoder）<br/>• `prefix-cache-scorer`<br/>• `max-score-picker`<br/>• 两个 Profile：`prefill` 和 `decode` | 将计算密集型 Prefill 阶段与内存密集型 Decode 阶段分离 |

**prefix-cache：**
```yaml
plugins:
- type: prefix-cache-scorer
  parameters:
    blockSize: 5
    maxPrefixBlocksToMatch: 256
    lruCapacityPerServer: 31250
- type: max-score-picker
schedulingProfiles:
- name: default
  plugins:
  - pluginRef: max-score-picker
  - pluginRef: prefix-cache-scorer
    weight: 100
```

**kv-cache-utilization：**
```yaml
plugins:
- type: kv-cache-utilization-scorer
- type: max-score-picker
schedulingProfiles:
- name: default
  plugins:
  - pluginRef: max-score-picker
  - pluginRef: kv-cache-utilization-scorer
    weight: 100
```

**queue-size：**
```yaml
plugins:
- type: queue-scorer
- type: max-score-picker
schedulingProfiles:
- name: default
  plugins:
  - pluginRef: max-score-picker
  - pluginRef: queue-scorer
    weight: 100
```

**lora-affinity：**
```yaml
plugins:
- type: lora-affinity-scorer
- type: max-score-picker
schedulingProfiles:
- name: default
  plugins:
  - pluginRef: max-score-picker
  - pluginRef: lora-affinity-scorer
    weight: 100
```

**pd-disaggregation：**
```yaml
plugins:
- type: pd-profile-handler
  parameters:
    threshold: 0
    hashBlockSize: 5
    primaryPort: 8000
- type: prefill-header-handler
- type: by-label
  name: prefill-pods
  parameters:
    label: "fusioninfer.io/component-type"
    validValues: ["prefiller"]
- type: by-label
  name: decode-pods
  parameters:
    label: "fusioninfer.io/component-type"
    validValues: ["decoder"]
- type: prefix-cache-scorer
  parameters:
    hashBlockSize: 5
    maxPrefixBlocksToMatch: 256
    lruCapacityPerServer: 31250
- type: max-score-picker
schedulingProfiles:
- name: prefill
  plugins:
  - pluginRef: prefill-pods
  - pluginRef: max-score-picker
  - pluginRef: prefix-cache-scorer
    weight: 50
- name: decode
  plugins:
  - pluginRef: decode-pods
  - pluginRef: max-score-picker
  - pluginRef: prefix-cache-scorer
    weight: 50
```

## 实施阶段 {#implementation-phases}

### 阶段 1：核心 Router 集成 {#phase-1-core-router-integration}

第一阶段侧重于为标准推理工作负载建立基础 Router 能力：

**交付成果：**
- 自动生成 `InferencePool`、`HTTPRoute`，以及 EPP 工作负载和配置资源
- 支持基本路由策略：
  - `prefix-cache`：将具有共享前缀的请求路由到同一服务器，以优化 prefix cache 利用率
  - `kv-cache-utilization`：根据内存使用情况均衡负载
  - `queue-size`：最大限度缩短请求等待时间
  - `lora-affinity`：路由到具有匹配 LoRA 适配器的服务器
- 为高级用户提供自定义 `endpointPickerConfig` 支持

### 阶段 2：分离式 Prefill/Decode 支持 {#phase-2-disaggregated-prefilldecode-support}

第二阶段增加对高级分离式 Prefill/Decode 架构的支持，将计算密集型 Prefill 操作与内存密集型 Decode 操作分开。
