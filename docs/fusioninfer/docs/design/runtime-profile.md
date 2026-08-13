---
title: RuntimeProfile and ClusterRuntimeProfile
description: Define reusable runtime templates for aggregated, Prefill/Decode-disaggregated, and multi-node inference.
---

## Resource Definition {#resource-definition}

`RuntimeProfile` and `ClusterRuntimeProfile` declare reusable inference runtime templates, including the backend, inference image, startup arguments, LoRA loading capabilities, Pod topology, and Aggregated or Prefill/Decode roles:

- `RuntimeProfile` is a namespaced resource that can be reused within a Namespace.
- `ClusterRuntimeProfile` is a cluster-scoped resource that can be shared across Namespaces.

Both Kinds use the same `RuntimeProfileSpec`. A Profile describes one logical replica per role; it neither specifies deployment replica counts nor binds to a specific Model. The following example shows a minimal Aggregated structure:

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: RuntimeProfile
metadata:
  name: vllm-aggregated
  namespace: team-a
spec:
  backend: vllm
  aggregated:
    podTemplate:
      spec:
        containers:
          - name: engine
            image: vllm/vllm-openai:v0.27.1
            args:
              - $(FUSION_MODEL_PATH)
            ports:
              - name: http
                containerPort: 8000
```

## Spec {#spec}

### API Structure {#api-structure}

`RuntimeProfile` and `ClusterRuntimeProfile` share the following Go interfaces:

```go
// +kubebuilder:validation:Enum=vllm;sglang;trtllm
type RuntimeBackend string

const (
    RuntimeBackendVLLM   RuntimeBackend = "vllm"
    RuntimeBackendSGLang RuntimeBackend = "sglang"
    RuntimeBackendTRTLLM RuntimeBackend = "trtllm"
)

type RuntimeProfileSpec struct {
    Backend    RuntimeBackend        `json:"backend"`
    LoRA       *RuntimeLoRASpec       `json:"lora,omitempty"`
    Aggregated *RuntimeComponentSpec `json:"aggregated,omitempty"`
    Prefiller  *RuntimeComponentSpec `json:"prefiller,omitempty"`
    Decoder    *RuntimeComponentSpec `json:"decoder,omitempty"`
}

// +kubebuilder:validation:Enum=preload;dynamic
type LoRALoadingMode string

const (
    LoRALoadingModePreload LoRALoadingMode = "preload"
    LoRALoadingModeDynamic LoRALoadingMode = "dynamic"
)

type RuntimeLoRASpec struct {
    LoadingMode LoRALoadingMode `json:"loadingMode"`

    // Control-plane limit for one InferenceDeployment.
    // +kubebuilder:validation:Minimum=1
    MaxLoadedAdapters int32 `json:"maxLoadedAdapters"`
}

type RuntimeComponentSpec struct {
    // Decoded and validated as corev1.PodTemplateSpec.
    // +kubebuilder:pruning:PreserveUnknownFields
    PodTemplate runtime.RawExtension `json:"podTemplate"`

    Multinode *MultinodeSpec `json:"multinode,omitempty"`
}

type MultinodeSpec struct {
    // +kubebuilder:validation:Minimum=2
    NodeCount int32 `json:"nodeCount"`
}
```

`podTemplate` uses `runtime.RawExtension` to avoid expanding the complete Kubernetes Pod schema again in the CRD. Admission and Controller must strictly decode it as the currently supported `corev1.PodTemplateSpec`.

### Role Fields {#role-fields}

The valid role field combinations are:

- Setting only `aggregated` selects Aggregated inference.
- Setting both `prefiller` and `decoder` selects Prefill/Decode disaggregation.
- `aggregated` cannot coexist with `prefiller` or `decoder`.
- `prefiller` and `decoder` must appear together.

Each role uses the same `RuntimeComponentSpec`:

- When `multinode` is not set, `podTemplate` represents one complete single-node logical replica.
- When `multinode` is set, `nodeCount` specifies the total number of nodes used by each logical replica, including one Leader and `nodeCount - 1` Workers.
- The Leader and Workers are all derived from the same `podTemplate` and must be placed on different Kubernetes Nodes.
- A single-node, multi-GPU engine should not set `multinode`; it should request multiple GPUs in one Pod instead.

For example, `multinode.nodeCount: 4` means that one logical replica consists of one Leader Pod and three Worker Pods. If the corresponding `InferenceDeployment` sets `replicas.aggregated: 2`, the Controller creates two such logical replicas: two Leader Pods and six Worker Pods, for a total of eight Pods.

### Distributed Backend Execution {#distributed-backend-execution}

`backend` is required and supports `vllm`, `sglang`, and `trtllm`. It selects only the engine adapter; the role field combination still determines whether the mode is Aggregated or P/D. All roles in the same RuntimeProfile use the same backend.

When `multinode` is set, the Controller generates the startup configuration according to the backend and the Pod's role within the logical replica:

- The RuntimeProfile author declares the image, engine parallelism parameters such as TP/PP/DP, resources, and scheduling constraints once, rather than maintaining separate Leader and Worker templates.
- `multinode.nodeCount`, the accelerator resources for each Pod, and the engine parallelism parameters are fixed in the RuntimeProfile.
- The Leader establishes the distributed runtime environment and starts the inference service; each Worker only joins the logical replica's distributed runtime environment.
- The backend adapter may wrap or rewrite the `command` and `args` of the `engine` container in the generated Pod, but only to inject Leader/Worker orchestration differences such as the multiprocessing executor, address, rank, and `nnodes`.
- The adapter does not derive or modify the TP/PP/DP declared by the RuntimeProfile based on `nodeCount`; these engine parallelism parameters remain unchanged across all logical replicas.
- The adapter handles only the differences required for distributed startup; user-declared environment variables, resources, volumes, probes, and scheduling constraints continue to apply to every Pod.
- The Controller does not infer the backend from the image name or arbitrary command strings.

Multinode vLLM always uses the multiprocessing executor. The backend adapter injects `--distributed-executor-backend mp` and the group startup arguments. SGLang uses its native distributed launcher. For Leader/Worker commands and managed parameters, see [Workload Orchestration: Distributed Backend Execution](./workload-orchestration.md#backend-distributed-execution).

The supported image contract, entrypoint forms, and adapter-reserved orchestration parameters for each backend must be documented and tested for every Operator version. A RuntimeProfile cannot predeclare the executor, address, rank, `nnodes`, or headless parameters managed by the adapter. If a conflict occurs or a custom entrypoint cannot be handled, the Controller rejects creation of a new workload when it consumes the RuntimeProfile. The adapter recognizes only the limited parameters defined by the versioned contract; it does not parse arbitrary CLI commands or shell scripts, nor does it validate the mathematical compatibility of the Model with TP/PP/DP. The Profile author is responsible for validating these parameters.

### LoRA Loading Capabilities {#lora-loading-capabilities}

`spec.lora` declares whether the Profile can consume `InferenceDeployment.spec.lora` and fixes the LoRA loading lifecycle:

- When `lora` is omitted, the Profile does not accept LoRA bindings.
- `loadingMode: preload` materializes and mounts all LoRAs before the engine starts. A change to the binding set produces a new workload revision.
- `loadingMode: dynamic` loads or unloads LoRAs after the Base Model workload is running and does not restart the Base Model when the binding set changes.
- `maxLoadedAdapters` limits the number of LoRAs that one InferenceDeployment can bind at the same time.

`maxLoadedAdapters` is a control-plane limit across backends. It does not replace the engine's own GPU memory, CPU cache, maximum LoRA rank, or batch concurrency settings. The Profile author continues to fix these backend-specific parameters in `podTemplate`; the LoRA integration for the corresponding backend validates that known parameters are compatible with the control-plane limit without modifying TP/PP/DP.

The LoRA loading configuration is defined at the top level of the Profile, so Aggregated, Prefiller, and Decoder use the same mode. A P/D deployment must load every LoRA into all routable logical replicas of both the Prefiller and Decoder; it cannot select different loading modes for the two roles.

In dynamic mode, the `InferenceDeployment` Controller calls a Pod-local LoRA management endpoint through the backend integration built into the Operator. The Controller is the only Reconciler and owns the desired bindings, retries, and status; the management endpoint performs only idempotent load, unload, and list operations. If the backend already provides a management interface that satisfies the contract, the Operator uses it directly. Otherwise, the Operator may inject a thin stateless proxy as a backend-specific implementation detail instead of running a second control loop. The management port is not added to the inference Service, InferencePool, or HTTPRoute.

Multinode dynamic loading operates at the logical replica level. Depending on the engine's capabilities, the backend integration calls the Leader coordination interface or performs a controlled fan-out to all members of the logical replica. The logical replica is considered Ready only after the Leader and every Worker confirm that the target digest is loaded. Member selection, request formats, and error normalization remain internal to the backend integration and are not exposed through the RuntimeProfile interface.

### PodTemplate {#podtemplate}

The PodTemplate is a complete `corev1.PodTemplateSpec`, but only one template layer is allowed:

- Each role's `podTemplate` must contain a container named `engine`.
- `engine` must declare exactly one named port called `http`; in multinode mode, only the Leader is registered as a service Endpoint.
- All Pods use the Volcano scheduler configured by the Operator. The template's `schedulerName` must be empty or match that configuration.
- The metadata, containers, resources, and scheduling constraints from the same template apply to the Leader and Workers; the backend adapter generates only role-specific startup configuration and reserved environment variables.
- `InferenceDeployment` does not provide a second layer of Pod overrides.
- Template `metadata` may contain only labels and annotations. Resource names, Namespace, OwnerReference, finalizers, and other server-side metadata are managed by the Controller.
- When `lora` is configured, the template entrypoint must conform to the LoRA contract for the corresponding backend. The Profile declares the engine's LoRA enablement, rank, and backend-specific capacity parameters; the Deployment cannot override them.
- The Operator injects the management port, volume, mount, and environment variables required by dynamic mode, and the template cannot use these reserved names. A thin stateless proxy is injected only when the backend's native interface cannot satisfy the internal lifecycle contract; the proxy does not own desired state or perform independent reconciliation.

The Operator provides a uniform Model mount contract in every `engine` container:

```text
volume: fusioninfer-model
volume: fusioninfer-model-metadata
initContainer: fusioninfer-model-init
env: FUSION_MODEL_PATH
env: FUSION_MODEL_METADATA_PATH
volume: fusioninfer-lora
env: FUSION_LORA_ROOT
env: FUSION_LORA_MANIFEST
```

Model content is mounted read-only at the fixed path `/models`, and the following values are injected:

```text
FUSION_MODEL_PATH=/models
FUSION_MODEL_METADATA_PATH=/var/run/fusioninfer/model/model.json
```

Runtime commands should read the Model through `$(FUSION_MODEL_PATH)` rather than hard-coding the cache root. A Profile cannot declare the reserved fields above or override the materializer or Endpoint Picker images managed by the Operator.

When LoRA bindings are declared, the Operator also mounts the current Deployment's adapter projection read-only at `/adapters` and generates `/var/run/fusioninfer/lora/adapters.json`. The Manifest uses an internal binding key to map `servedName`, the resolved Model UID, the digest, and the in-container path; paths do not directly use the user-provided served name. The engine container can see only the LoRAs bound to the current Deployment and cannot browse the node cache root.

Inference images in production must be pinned by OCI digest. For readability, the following examples use the official versioned image `vllm/vllm-openai:v0.27.1`; replace it with the corresponding digest-pinned image when deploying.

### Scope and References {#scope-and-references}

`RuntimeProfile` does not contain a `modelRef`. The specific Model, runtime template, and replica counts are bound by `InferenceDeployment`.

A PodTemplate can reference a ServiceAccount, Secret, ConfigMap, and PVC:

- Namespaced dependencies in a `RuntimeProfile` are resolved in the Profile's Namespace.
- Namespaced dependency names in a `ClusterRuntimeProfile` are resolved in the Namespace of the `InferenceDeployment` that consumes it.
- A `ClusterRuntimeProfile` cannot pin dependencies in another Namespace.
- When a `ClusterRuntimeProfile` is created, only the reference structure is validated. The consumer reconciles whether each dependency exists and reports the result through `InferenceDeployment.status`.

The Profile neither owns nor modifies these dependencies. ConfigMaps and Secrets that affect startup behavior should use immutable objects or versioned names.

### Defaults and Validation {#defaults-and-validation}

- `backend` is required and must be `vllm`, `sglang`, or `trtllm`.
- `lora.loadingMode` must be `preload` or `dynamic`; `maxLoadedAdapters` must be at least 1.
- A Profile can be consumed only when the current Operator version implements the selected LoRA mode for the specified backend and template entrypoint.
- `aggregated` must be set, or both `prefiller` and `decoder` must be set.
- Every declared role must provide a `podTemplate`.
- When `multinode` is set, `nodeCount` must be at least 2; when it is omitted, the role is treated as single-node.
- RawExtension must strictly decode as `corev1.PodTemplateSpec`.
- The template must contain an `engine` container and exactly one named `http` port.
- The template cannot use Operator-reserved volumes, init containers, environment variables, mount paths, labels, or annotations.
- The backend adapter must support the image and entrypoint arguments declared in the template.
- The template cannot declare executor, address, rank, `nnodes`, or headless parameters reserved by the backend adapter.
- The template image must be pinned by OCI digest.
- `RuntimeProfile.spec` and `ClusterRuntimeProfile.spec` are immutable in v1. Changing the backend, image, command, resources, `multinode`, or PodTemplate requires a new object.

## Status {#status}

`RuntimeProfile` and `ClusterRuntimeProfile` do not provide a status subresource and do not require a dedicated Controller. Admission validates constraints within the object, while the consuming `InferenceDeployment.status` holds the state of Namespaced dependencies and the actual runtime.

## Examples {#examples}

### RuntimeProfile: Single-Node Aggregated {#runtimeprofile-single-node-aggregated}

This Profile describes an Aggregated logical replica that uses one GPU.

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: RuntimeProfile
metadata:
  name: vllm-aggregated-a10-r1
  namespace: team-a
spec:
  backend: vllm
  aggregated:
    podTemplate:
      metadata:
        labels:
          example.fusioninfer.io/runtime: vllm
      spec:
        containers:
          - name: engine
            image: vllm/vllm-openai:v0.27.1
            args:
              - $(FUSION_MODEL_PATH)
            ports:
              - name: http
                containerPort: 8000
            readinessProbe:
              httpGet:
                path: /health
                port: http
            resources:
              requests:
                cpu: "4"
                memory: 16Gi
              limits:
                nvidia.com/gpu: "1"
        nodeSelector:
          accelerator: a10
```

### ClusterRuntimeProfile: Prefill/Decode Disaggregation {#clusterruntimeprofile-prefilldecode-disaggregation}

The Prefiller and Decoder declare their KV transfer roles separately. The Profile does not contain replica counts or an Endpoint Picker policy.

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: ClusterRuntimeProfile
metadata:
  name: vllm-pd-h100-r1
spec:
  backend: vllm
  prefiller:
    podTemplate:
      spec:
        containers:
          - name: engine
            image: vllm/vllm-openai:v0.27.1
            args:
              - $(FUSION_MODEL_PATH)
              - --kv-transfer-config
              - '{"kv_connector":"NixlConnector","kv_role":"kv_producer"}'
            ports:
              - name: http
                containerPort: 8000
            resources:
              limits:
                nvidia.com/gpu: "2"
        nodeSelector:
          accelerator: h100
  decoder:
    podTemplate:
      spec:
        containers:
          - name: engine
            image: vllm/vllm-openai:v0.27.1
            args:
              - $(FUSION_MODEL_PATH)
              - --kv-transfer-config
              - '{"kv_connector":"NixlConnector","kv_role":"kv_consumer"}'
            ports:
              - name: http
                containerPort: 8000
            resources:
              limits:
                nvidia.com/gpu: "1"
        nodeSelector:
          accelerator: h100
```

### RuntimeProfile: Multinode Aggregated {#runtimeprofile-multinode-aggregated}

Each logical replica consists of one Leader Pod and three Worker Pods, using four nodes in total.

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: RuntimeProfile
metadata:
  name: vllm-aggregated-4node-r1
  namespace: team-a
spec:
  backend: vllm
  aggregated:
    multinode:
      nodeCount: 4
    podTemplate:
      spec:
        containers:
          - name: engine
            image: vllm/vllm-openai:v0.27.1
            args:
              - $(FUSION_MODEL_PATH)
              - --port
              - "8000"
              - --tensor-parallel-size
              - "8"
              - --pipeline-parallel-size
              - "4"
              - --data-parallel-size
              - "1"
            ports:
              - name: http
                containerPort: 8000
            resources:
              limits:
                nvidia.com/gpu: "8"
        nodeSelector:
          accelerator: h100
```

The `backend: vllm` adapter injects the multiprocessing executor, node count, address, and rank for the Leader and Workers based on `nodeCount: 4`. It preserves the `TP=8`, `PP=4`, and `DP=1` values fixed in the Profile, so the user maintains only one set of vLLM arguments and one PodTemplate.

### RuntimeProfile: Dynamic LoRA {#runtimeprofile-dynamic-lora}

This Profile allows a Deployment to bind up to eight LoRAs dynamically. vLLM's LoRA enablement and backend-specific capacity remain fixed in the PodTemplate. The Operator configures the protected Pod-local management endpoint and the environment variables required for runtime updates, while the `InferenceDeployment` Controller reconciles loading state through the backend integration.

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: RuntimeProfile
metadata:
  name: vllm-aggregated-lora-dynamic-r1
  namespace: team-a
spec:
  backend: vllm
  lora:
    loadingMode: dynamic
    maxLoadedAdapters: 8
  aggregated:
    podTemplate:
      spec:
        containers:
          - name: engine
            image: vllm/vllm-openai:v0.27.1
            args:
              - $(FUSION_MODEL_PATH)
              - --enable-lora
              - --max-loras
              - "8"
              - --max-cpu-loras
              - "8"
            ports:
              - name: http
                containerPort: 8000
            resources:
              limits:
                nvidia.com/gpu: "1"
        nodeSelector:
          accelerator: h100
```
