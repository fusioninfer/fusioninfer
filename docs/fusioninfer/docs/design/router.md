---
sidebar_position: 2
title: Router Controller Design
---

# Router Controller Design

## Summary

The FusionInfer Router Controller enables automatic creation and management of Gateway API Inference Extension resources based on `InferenceService` CRDs. This integration transforms FusionInfer-managed inference workloads into optimized inference routing systems by leveraging the Endpoint Picker (EPP) for intelligent request routing, prefix-cache aware scheduling, and load balancing.

When an `InferenceService` is created with a role having `componentType: router`, the FusionInfer Router Controller automatically generates corresponding `InferencePool`, `EndpointPickerConfig`, `HTTPRoute`, `Gateway`, and EPP deployment resources. This provides production-grade traffic management capabilities including prefix-cache optimization, KV-cache utilization based routing, queue-aware scheduling, LoRA adapter affinity, and support for disaggregated prefill/decode architectures.

## Motivation

Self-hosted LLM inference workloads on Kubernetes face significant challenges around request routing, load balancing, and resource utilization. Traditional load balancers are not aware of model-specific characteristics such as KV-cache state, prefix caching opportunities, or the distinction between prefill and decode phases. This leads to suboptimal performance, increased latency, and inefficient GPU utilization.

The Gateway API Inference Extension provides sophisticated routing capabilities specifically designed for inference workloads, but requires manual configuration of multiple CRDs and deep understanding of the plugin ecosystem. FusionInfer simplifies this by providing a higher-level abstraction with predefined routing strategies while still allowing advanced users to customize the full `EndpointPickerConfig` when needed.

### Goals

- **Automatic Router Resource Generation**: Automatically create and manage `InferencePool`, `EndpointPickerConfig`, `HTTPRoute`, `Gateway`, and EPP deployment based on `InferenceService` specifications
- **Predefined Routing Strategies**: Provide simple, declarative routing strategies (`prefix-cache`, `kv-cache-utilization`, `queue-size`, `lora-affinity`, `pd-disaggregation`) for common use cases
- **Advanced Customization**: Allow power users to provide custom `EndpointPickerConfig` for fine-grained control

### Non-Goals

- **Workload Management**: This design focuses only on router/gateway integration; actual pod/deployment management is handled by other FusionInfer components
- **Gateway Implementation**: This does not implement a new gateway, but integrates with existing Gateway API compliant gateways via Envoy ext-proc

### User Stories

#### Story 1: Prefix Cache Aware Routing

As a platform engineer, I want to deploy a Qwen model with prefix cache aware routing so that requests sharing the longest prefixes are routed to the same server to maximize KV-cache reuse.

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
              image: vllm/vllm-openai:latest
              args:
                - --model=Qwen/Qwen2.5-7B-Instruct
```

This would automatically create an `InferencePool` pointing to the inference pods and configure EPP with prefix-cache aware scheduling.

#### Story 2: KV-Cache Utilization Based Load Balancing

As an ML engineer, I want to distribute inference requests based on available KV-cache memory to prevent OOM errors and ensure even resource utilization across all inference servers.

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
              image: vllm/vllm-openai:latest
              args:
                - --model=/models/llama-70b
```

This configuration routes requests to servers with the most available KV-cache memory, preventing memory exhaustion and ensuring balanced utilization.

#### Story 3: Disaggregated Prefill/Decode Architecture

As an infrastructure engineer, I want to deploy a disaggregated serving architecture where prefill (prompt processing) and decode (token generation) are handled by specialized server pools for optimal hardware utilization and latency.

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
              image: vllm/vllm-openai:latest
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
              image: vllm/vllm-openai:latest
              args:
                - --model=meta-llama/Llama-3-70B-Instruct
                - --kv-transfer-config '{"kv_connector":"NixlConnector","kv_role":"kv_both"}'
```

## Proposal

The FusionInfer Router Controller watches for `InferenceService` resources that include roles with `componentType: router`. When detected, it automatically generates and manages Gateway API Inference Extension resources to enable intelligent request routing and load balancing for LLM inference workloads.

The router role supports two configuration approaches:
1. **Simple strategy-based**: Use predefined routing strategies (`prefix-cache`, `kv-cache-utilization`, `queue-size`, `lora-affinity`, `pd-disaggregation`) for common use cases
2. **Advanced custom configuration**: Provide a complete `endpointPickerConfig` for fine-grained control over scheduling plugins, profiles, and scoring algorithms

These configurations are translated into `InferencePool`, `EndpointPickerConfig`, `HTTPRoute`, `Gateway`, and EPP deployment resources that work together to optimize inference request routing based on factors like prefix cache hits, memory utilization, and workload characteristics.

### Go Types

```go
// InferenceServiceSpec defines the desired state of InferenceService
type InferenceServiceSpec struct {
    // Roles defines the components of the inference service
    Roles []RoleSpec `json:"roles"`
}

// Role defines a component in the inference pipeline
type RoleSpec struct {
    // Name is the identifier for this role
    Name string `json:"name"`
    
    // ComponentType specifies the type of component
    // +kubebuilder:validation:Enum=router;prefiller;decoder;worker
    ComponentType ComponentType `json:"componentType"`
    
    // Router-specific fields (only for componentType: router)
    Strategy              RoutingStrategy        `json:"strategy,omitempty"`
    HTTPRoute             *runtime.RawExtension  `json:"httproute,omitempty"`   // Gateway API HTTPRouteSpec
    Gateway               *runtime.RawExtension  `json:"gateway,omitempty"`     // Gateway API GatewaySpec
    EndpointPickerConfig  string                 `json:"endpointPickerConfig,omitempty"`  // Raw YAML for advanced users
    
    // Worker-specific fields (for prefiller/decoder/worker)
    Replicas       *int32                 `json:"replicas,omitempty"`
    Template       *runtime.RawExtension  `json:"template,omitempty"`  // corev1.PodTemplateSpec
}

// ComponentType defines the type of component
type ComponentType string

const (
    ComponentTypeRouter    ComponentType = "router"
    ComponentTypePrefiller ComponentType = "prefiller"
    ComponentTypeDecoder   ComponentType = "decoder"
    ComponentTypeWorker    ComponentType = "worker"
)

// RoutingStrategy defines the inference routing strategy
type RoutingStrategy string

const (
    StrategyPrefixCache      RoutingStrategy = "prefix-cache"
    StrategyKVCacheUtil      RoutingStrategy = "kv-cache-utilization"
    StrategyQueueSize        RoutingStrategy = "queue-size"
    StrategyLoRAffinity      RoutingStrategy = "lora-affinity"
    StrategyPDDisaggregation RoutingStrategy = "pd-disaggregation"
)
```

### Configuration Examples

#### Example 1: Simple Strategy-based Configuration

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
            image: vllm/vllm-openai:latest
            args:
            - --model=meta-llama/Llama-3-8B-Instruct
```

FusionInfer automatically generates the following resources:

```yaml
# 1. HTTPRoute - Routes traffic to the InferencePool
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
# 2. InferencePool - Manages the inference backend pods
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
# 3. EndpointPickerConfig (ConfigMap) - EPP scheduling configuration
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
# 4. EPP Deployment - Endpoint Picker deployment
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
# 5. EPP Service - Exposes the EPP for Envoy ext-proc
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

#### Example 2: Advanced Configuration with Custom EndpointPickerConfig

For advanced users with specific tuning requirements, FusionInfer allows direct configuration of the EndpointPickerConfig. This provides full control over scheduling behavior, plugin parameters, and scoring weights. 

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: advanced-service
spec:
  roles:
    - name: gateway
      componentType: router
      # Direct control over EPP configuration for advanced tuning
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
            image: vllm/vllm-openai:latest
            args:
            - --model=meta-llama/Llama-3-8B-Instruct
```

#### Example 3: Disaggregated Prefill/Decode Architecture

For large-scale LLM deployments, disaggregated prefill/decode architecture separates the compute-intensive prompt processing (prefill) phase from the memory-intensive token generation (decode) phase. This allows independent scaling and optimization of each phase:

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
              image: vllm/vllm-openai:latest
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
              image: vllm/vllm-openai:latest
              args:
                - --model=meta-llama/Llama-3-70B-Instruct
                - --kv-transfer-config '{"kv_connector":"NixlConnector","kv_role":"kv_both"}'
```

**Generated Resources:**

The FusionInfer Router Controller will automatically create the following Gateway API and routing resources for Disaggregated Prefill/Decode architecture:

> **Note**: In a complete P/D deployment, decode pods typically include a sidecar for orchestrating communication with prefill pods. This sidecar deployment is managed by the workload component, not the gateway component.

```yaml
# 1. InferencePool - Single pool managing both prefill and decode pods
apiVersion: inference.networking.k8s.io/v1
kind: InferencePool
metadata:
  name: disaggregated-llm-service-pool
spec:
  selector:
    matchLabels:
      fusioninfer.io/service: disaggregated-llm-service
  endpointPickerRef:
    name: disaggregated-llm-service-epp
    port:
      number: 9002
  targetPorts:
  - number: 8000

---
# 2. EndpointPickerConfig - P/D-aware scheduling configuration
apiVersion: inference.networking.x-k8s.io/v1alpha1
kind: EndpointPickerConfig
metadata:
  name: disaggregated-llm-service-epp-config
spec:
  plugins:
  # Pre-request handler to set P/D headers
  - type: prefill-header-handler
  
  # Filters for role-based pod selection
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
  
  # Prefix cache optimization
  - type: prefix-cache-scorer
    parameters:
      hashBlockSize: 5
      maxPrefixBlocksToMatch: 256
      lruCapacityPerServer: 31250
  
  # Picker
  - type: max-score-picker
  
  # P/D profile handler for request routing
  - type: pd-profile-handler
    parameters:
      threshold: 0
      hashBlockSize: 5
      primaryPort: 8000
      decodeProfile: "decode"  # Optional, defaults to "decode"
      prefillProfile: "prefill"  # Optional, defaults to "prefill"
  
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
# 3. HTTPRoute - Routes traffic to the EPP
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
      kind: Service
      port: 8000

---
# 4. EPP Deployment and Service (automatically configured)
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
        - --config-file=/config/epp-config.yaml
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

## Strategy to EndpointPickerConfig Mapping

FusionInfer automatically generates the appropriate EndpointPickerConfig based on the routing strategy:

| Routing Strategy | Generated Plugins | Use Case |
|-----------------|------------------|----------|
| `prefix-cache` | • `prefix-cache-scorer` (with optimized parameters)<br/>• `max-score-picker`<br/>• Single `default` scheduling profile | Route requests with longest shared prefixes to same server while balancing with KV-cache utilization and queue depth |
| `kv-cache-utilization` | • `kv-cache-utilization-scorer`<br/>• `max-score-picker`<br/>• Single `default` scheduling profile | Balance load based on memory usage across inference servers |
| `queue-size` | • `queue-scorer`<br/>• `max-score-picker`<br/>• Single `default` scheduling profile | Minimize request waiting time by routing to least loaded servers |
| `lora-affinity` | • `lora-affinity-scorer`<br/>• `max-score-picker`<br/>• Single `default` scheduling profile | Route requests to servers with matching LoRA adapters for multi-adapter serving |
| `pd-disaggregation` | • `pd-profile-handler`<br/>• `prefill-header-handler`<br/>• `by-label` filters (for prefiller/decoder)<br/>• `prefix-cache-scorer`<br/>• `max-score-picker`<br/>• Two profiles: `prefill` and `decode` | Separate compute-intensive prefill from memory-intensive decode phases |

**prefix-cache:**
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

**kv-cache-utilization:**
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

**queue-size:**
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

**lora-affinity:**
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

**pd-disaggregation:**
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

## Implementation Phases

### Phase 1: Core Router Integration

The first phase focuses on establishing the fundamental router capabilities for standard inference workloads:

**Deliverables:**
- Automatic generation of `InferencePool`, `EndpointPickerConfig`, `HTTPRoute`, `Gateway`, and EPP deployment resources
- Support for basic routing strategies:
  - `prefix-cache`: Route requests with shared prefixes to optimize cache utilization
  - `kv-cache-utilization`: Balance load based on memory usage
  - `queue-size`: Minimize request waiting time
  - `lora-affinity`: Route to servers with matching LoRA adapters
- Custom `endpointPickerConfig` support for advanced users

### Phase 2: Disaggregated Prefill/Decode Support

The second phase adds support for advanced Disaggregated Prefill/Decode architectures that separate compute-intensive prefill from memory-intensive decode operations.
