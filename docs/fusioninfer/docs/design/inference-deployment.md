---
title: InferenceDeployment
description: Bind a Model to a RuntimeProfile and declare replicas, caching, routing, rollout, and status behavior.
---

## Resource Purpose {#resource-purpose}

`InferenceDeployment` is a Namespaced resource that creates an accessible model inference service. It binds a Base Model, optional LoRAs, and a RuntimeProfile through explicit references, and declares the deployment replica counts, model materialization timing, and Gateway API entry point.

The following example shows a minimal Aggregated topology. Omitting `spec.cache` uses the default `lazy` mode.

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

### API Structure {#api-structure}

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
    // Reuses the existing v1alpha1 RoutingStrategy type.
    // +kubebuilder:validation:Enum=prefix-cache;kv-cache-utilization;queue-size
    Strategy RoutingStrategy `json:"strategy"`
}

type GatewayReference struct {
    Name        gatewayv1.ObjectName   `json:"name"`
    Namespace   *gatewayv1.Namespace   `json:"namespace,omitempty"`
    SectionName *gatewayv1.SectionName `json:"sectionName,omitempty"`
}
```

### Resource References {#resource-references}

`modelRef` and `runtimeRef` use an explicit `kind + name`:

- `modelRef.kind` allows only `Model` or `ClusterModel`.
- `runtimeRef.kind` allows only `RuntimeProfile` or `ClusterRuntimeProfile`.
- The API Group is fixed to `fusioninfer.io`, so references do not repeat the `apiGroup`.
- References do not provide a default Kind, selector, Namespace fallback, or fallback to another resource with the same name.

`modelRef` must reference a Base Model. LoRAs are bound through the separate `spec.lora` list; a LoRA Model cannot be used directly as `modelRef`.

### LoRA Bindings {#lora-bindings}

`spec.lora` can bind multiple LoRA Models to the same Base Model deployment:

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

Each binding contains two pieces of information:

- `modelRef` must resolve to a LoRA Model that has `spec.lora.baseModelRef`.
- `servedName` is the model name used to select the LoRA in an OpenAI-compatible request.

After resolving a LoRA, the Controller must confirm that its `baseModelRef` and the Deployment's `modelRef` point to the same Kind, name, and UID. Bindings with the same `servedName` or duplicate `modelRef` values are rejected. The number of bindings cannot exceed the RuntimeProfile's `lora.maxLoadedAdapters`.

The referenced RuntimeProfile must declare `spec.lora`:

- `loadingMode: preload`: The Controller materializes all LoRAs before creating a workload revision and passes the binding manifest to the backend startup integration. Adding, deleting, or replacing a binding creates a new workload revision.
- `loadingMode: dynamic`: The Controller reconciles loading and unloading against the existing Base Model workload without restarting it. Adding, deleting, or replacing a binding updates only the LoRA binding revision.

In P/D mode, each binding must be loaded into every Prefiller and Decoder logical replica. Through the backend integration, the Controller calls the Pod-local LoRA management endpoint of each logical replica; the backend can have the Leader coordinate loading within the group, or the integration can fan out to all members. A replica counts as Ready only after its Leader and all Workers confirm that the target digest is loaded.

During dynamic loading, Endpoint Picker publishes a `servedName` only after it is Ready on all required roles. When deleting a binding, the Controller first stops publishing the name and waits for in-flight requests to drain before unloading the LoRA. When replacing a LoRA artifact under the same `servedName`, that LoRA name may be temporarily unavailable if the backend does not support atomic replacement; the Base Model and other LoRAs are unaffected.

### Replicas {#replicas}

`replicas` uses the same field combinations as the RuntimeProfile:

- Setting only `aggregated` indicates an Aggregated deployment.
- Setting both `prefiller` and `decoder` indicates a Prefill/Decode deployment.
- Each value represents the number of logical replicas for the corresponding role, not the number of Pods.
- When the RuntimeProfile configures `multinode.nodeCount` for a role, one logical replica expands into one Leader Pod and `nodeCount - 1` Worker Pods.

Changing `replicas` only increases or decreases the number of independently routable logical replicas. It does not change the RuntimeProfile's fixed `multinode.nodeCount`, accelerator resources per Pod, or TP/PP/DP. `replicas` also does not equal the backend's data parallel size: the former creates independent workload groups and serving Endpoints, while the latter is an engine parallelism parameter within a single logical replica.

The Deployment's role combination must exactly match the referenced RuntimeProfile. With an Aggregated Profile, only `replicas.aggregated` can be set; with a P/D Profile, both `replicas.prefiller` and `replicas.decoder` must be set.

### Cache {#cache}

`spec.cache` defaults to `lazy` when omitted. Setting `cache.mode` explicitly selects when the deployment uses the model artifacts:

- `lazy`: After a Pod is scheduled to a node, an injected init container checks the node cache. On a cache miss, it downloads and verifies the model; the inference engine starts only after this completes.
- `eager`: Before creating new inference workloads, a warm-up Job runs on nodes that satisfy the role's scheduling constraints. New workloads are created only after all required copies have been materialized.

This policy applies to both the Base Model and the artifacts referenced by `spec.lora`. `preload` mode requires the Base Model and all LoRAs to be readable before the engine starts. When a LoRA is added in `dynamic` mode, `eager` warms it on all target nodes first, while `lazy` uses a trusted materializer on the target nodes to materialize it on demand. In either cache mode, the backend loading interface is called only after materialization completes.

For each role, `eager` calculates the warming requirement as logical replica count × effective node count. The effective node count comes from the role's `multinode.nodeCount` and defaults to 1 when unset; the Controller prepares one model cache copy on each distinct node that satisfies the Pod template's scheduling constraints.

Both modes use the same content-addressed node cache and derive an immutable cache key from the normalized source URI, the revision or OCI descriptor digest, and the optional `source.digest`.

`eager` warm-up Jobs do not request or reserve GPUs. If the model is warm but GPUs are unavailable, the workload can remain Pending; a failed new version does not prematurely delete the previous version that is still serving.

`pvc://` sources must also be copied to the immutable node cache first. The engine mounts the cached copy read-only and does not use the mutable source PVC contents directly.

### Endpoint {#endpoint}

`endpoint` is required in v1. The Controller creates Endpoint Picker, InferencePool, and HTTPRoute resources from this field:

- `gatewayRef.name` specifies the Gateway to which the HTTPRoute attaches.
- When `gatewayRef.namespace` is omitted, it uses the `InferenceDeployment`'s Namespace.
- `gatewayRef.sectionName` is optional and selects a Gateway listener.
- `hostnames` is written to the field of the same name in the HTTPRoute and participates only in matching incoming requests.

The `gatewayRef` API Group is fixed to `gateway.networking.k8s.io` and its Kind is fixed to `Gateway`, so only the name and optional Namespace and SectionName are required.

`endpointPicker` applies only to Aggregated deployments:

- It supports `prefix-cache`, `kv-cache-utilization`, and `queue-size`.
- When omitted, it uses the default strategy configured by the Operator.
- P/D deployments must omit this field; the Controller automatically generates Prefill and Decode scheduling configuration from `prefiller + decoder`.

The Endpoint Picker image, replica count, and port are managed by Operator configuration and are not part of the RuntimeProfile or InferenceDeployment user interface.

### Scope and References {#scope-and-references}

`InferenceDeployment` exists only in Namespaced form:

- When referencing a `Model` or `RuntimeProfile`, lookup is limited to the Deployment's Namespace.
- When referencing a `ClusterModel` or `ClusterRuntimeProfile`, lookup is limited to Cluster-scoped objects.
- Namespaced references cannot specify or access another Namespace.
- ClusterModel credentials and Namespaced dependencies in a ClusterRuntimeProfile are resolved in the Deployment's Namespace.

Whether a cross-Namespace Gateway accepts the generated HTTPRoute is determined by the Gateway listener's `allowedRoutes`. FusionInfer does not modify the Gateway or try another object with the same name after reference resolution fails.

After a reference is resolved successfully, Status records the target object's Kind, name, UID, and generation. The Base Model and Runtime are stored in `resolvedRefs`, while LoRAs are stored in the corresponding `status.lora[]` entries. Deleting and recreating an object with the same name produces a new UID and is treated as a change to the reference target.

### Defaults and Validation {#defaults-and-validation}

- `modelRef`, `runtimeRef`, `replicas`, and `endpoint` are required.
- `modelRef.kind`, `modelRef.name`, `runtimeRef.kind`, and `runtimeRef.name` are required.
- When `lora` is omitted, no LoRA is bound; when present, it must contain at least one entry.
- Every `lora[].modelRef` and `servedName` is required, and `servedName` must be unique within the list.
- `lora[].modelRef` values cannot be duplicated.
- `replicas` can contain only `aggregated`, or it can contain both `prefiller` and `decoder`.
- Each replica count must be at least 1.
- When `cache` is omitted, it defaults to `{mode: lazy}`; when set explicitly, `cache.mode` allows only `lazy` or `eager`.
- `endpoint.gatewayRef.name` is required.
- `endpoint.endpointPicker.strategy` is allowed only for Aggregated deployments and must be one of the supported strategies.
- Compatibility between the RuntimeProfile roles and the Deployment replica combination is validated after reference resolution.
- When `lora` is declared, the RuntimeProfile must support LoRA, and the number of bindings cannot exceed `maxLoadedAdapters`.
- The Controller must confirm that every binding references a LoRA Model and that its `baseModelRef` and the Deployment's Base Model reference resolve to the same UID.
- The RuntimeProfile's backend adapter must support the images and entrypoint arguments in the template and must not conflict with the distributed orchestration parameters declared by the template. If unsupported, the Controller does not create new workloads and sets `ReferencesResolved=False`.
- `InferenceDeployment.spec` can be updated. Changes to the Model, Runtime, cache mode, or Endpoint Picker strategy create a new revision pending promotion. Whether a LoRA change rebuilds the workload is determined by the RuntimeProfile's `loadingMode`.

Constraints that do not require reading other objects are validated by CRD OpenAPI, CEL, or Admission. Whether references exist, roles match, Secret/PVC resources are available, and the Gateway accepts the Route is reconciled by the Controller and reported through Conditions, so resources can be created in any order.

## Status {#status}

`InferenceDeployment.status` summarizes reference resolution, model materialization, workload, routing, and rollout status:

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

`components` uses the stable `endpointPicker`, `aggregated`, `prefiller`, and `decoder` keys. `nodesPerReplica` records the effective `multinode.nodeCount` from the RuntimeProfile and defaults to 1 when unset; `readyReplicas` counts a logical replica only when all of its `nodesPerReplica` Pods are Ready. `ModelCacheStatus.desiredCopies` and `readyCopies` record the required and completed distinct-node cache copies for the current revision, respectively. `activeRevision` identifies the version currently receiving traffic; `pendingRevision` identifies the new version being materialized, scheduled, or awaiting promotion.

`status.lora` uses `servedName` as the list key and summarizes the resolved Model, cache copies, and loading progress for each role for every binding. It does not expose backend management endpoint addresses or per-Pod error bodies. In `preload` mode, Ready status advances with promotion of the workload revision; in `dynamic` mode, it directly reflects loading status in the current active revision.

Conditions use the standard `metav1.Condition` without adding a single `phase` field:

- `ReferencesResolved`: The references to the Base Model, all LoRA Models, and the RuntimeProfile have been resolved and passed consumer-side validation.
- `ModelMaterialized`: The Base Model cache copies required by the current revision are available; LoRA materialization status is recorded in each binding.
- `LoRAReady`: All declared LoRAs have been resolved, materialized, and loaded into every required logical replica; it is `True` when there are no LoRAs.
- `WorkloadsReady`: The desired logical replicas for all roles are Ready; a multinode logical replica requires the Leader and all Worker Pods to be Ready.
- `RouteReady`: The Service, InferencePool, Endpoint Picker, and HTTPRoute are ready.
- `Available`: The current active revision can accept requests.
- `Progressing`: Warming, creation, scaling, or updates are still in progress.
- `Degraded`: Reconciliation failed and requires intervention from the user or platform administrator.

In dynamic mode, a failure in one LoRA sets `LoRAReady=False`, but `Available` can remain `True` as long as the active Base Model can still serve requests. The failed served name is not published to Endpoint Picker.

Every Condition must include `observedGeneration`, a stable `reason`, and an actionable `message`. The following example shows the status of a ready Aggregated deployment:

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
      message: The active revision is ready to serve requests.
```

## Examples {#examples}

### Namespaced Aggregated: Default lazy cache {#namespaced-aggregated-default-lazy-cache}

Both the Model and RuntimeProfile are resolved from the `team-a` Namespace. This deployment omits `spec.cache` to use the default `lazy` mode, creates two Aggregated logical replicas, and uses the prefix-cache Endpoint Picker.

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

### Cluster Prefill/Decode: eager cache {#cluster-prefilldecode-eager-cache}

The Model and RuntimeProfile use Cluster-scoped objects. The Controller warms the model before starting two Prefiller and four Decoder logical replicas; the P/D deployment does not declare `endpointPicker`.

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

### Namespaced Aggregated: Multiple dynamic LoRAs {#namespaced-aggregated-multiple-dynamic-loras}

This deployment uses a RuntimeProfile that supports dynamic LoRA. The two LoRAs use `finance` and `customer-support` as their model names in requests.

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
