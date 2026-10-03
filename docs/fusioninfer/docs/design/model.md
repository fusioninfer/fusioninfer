---
title: Model and ClusterModel
description: Define namespaced or cluster-scoped model artifacts and optional LoRA adapter bindings.
---

## Overview {#overview}

`Model` and `ClusterModel` declare the source of a model artifact:

- `Model` is a Namespaced resource for models within a Namespace.
- `ClusterModel` is a cluster-scoped resource for models shared across Namespaces.

`Model` and `ClusterModel` use the same `ModelSpec`. Setting only `spec.source` represents a Base Model; setting both `spec.source` and `spec.lora.baseModelRef` represents a LoRA artifact.

The model agent that FusionInfer runs on each node downloads model files into the node cache. See [Prefetch](#prefetch) for when downloads happen.

The following is an example of a `Model` resource:

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: Model
metadata:
  name: qwen3-8b-r1
  namespace: team-a
spec:
  source:
    uri: hf://Qwen/Qwen3-8B
```

## Spec {#spec}

### API structure {#api-structure}

`Model` and `ClusterModel` share the following Go API:

```go
// ModelSpec declares a model artifact and the nodes to prefetch it to, and is shared by Model and ClusterModel.
type ModelSpec struct {
    Source   ModelSource       `json:"source"`
    LoRA     *LoRAArtifactSpec `json:"lora,omitempty"`
    Prefetch *PrefetchSpec     `json:"prefetch,omitempty"`
}

// LoRAArtifactSpec is set only on LoRA artifacts and declares which Base Model the LoRA can be applied to.
type LoRAArtifactSpec struct {
    BaseModelRef ModelReference `json:"baseModelRef"`
}

// ModelReference refers to a Model or ClusterModel by kind and name.
type ModelReference struct {
    Kind string `json:"kind"`
    Name string `json:"name"`
}

// ModelSource declares where the artifact is stored, with the version in the URI, and the Secret used to access it.
type ModelSource struct {
    URI            string           `json:"uri"`
    CredentialsRef *SecretReference `json:"credentialsRef,omitempty"`
}

// SecretReference refers to a Secret by name; the resource scope determines the Secret's Namespace.
type SecretReference struct {
    Name string `json:"name"`
}

// PrefetchSpec declares which nodes the model is downloaded to ahead of time.
type PrefetchSpec struct {
    NodeSelector map[string]string `json:"nodeSelector,omitempty"`
    NodeName     string            `json:"nodeName,omitempty"`
}
```

### Model sources {#model-sources}

`source.uri` specifies where the model files are stored. The URI scheme determines the storage type, and the version is also written in the URI. It is required and supports the following sources; parts in square brackets are optional:

| Source | URI format | Description | Example |
| --- | --- | --- | --- |
| Hugging Face | `hf://<owner>/<repo>[@<revision>]` | `revision` can be a branch, tag, or commit SHA; the repository's default branch is used when omitted | `hf://Qwen/Qwen3-8B@main`<br />`hf://Qwen/Qwen3-8B@b968826d9c46dd6066d109eabc6255188de91218` |
| S3 | `s3://<bucket>/<prefix>` | Versions are not supported | `s3://team-a-models/base/qwen3-8b` |
| OCI | `oci://<registry>/<repository>[:<tag>\|@sha256:<digest>]` | Specify a tag or a digest; `latest` is used when neither is specified | `oci://registry.example.com/models/qwen3-8b:v1`<br />`oci://registry.example.com/models/qwen3-8b@sha256:9d2e…` |

### Access credentials {#access-credentials}

`source.credentialsRef` contains only a Secret name; a Namespace cannot be specified. When omitted, no credentials are used and the source is accessed anonymously, so only public models can be downloaded.

| Source | Secret type | Required keys | Optional keys |
| --- | --- | --- | --- |
| `hf://` | `Opaque` | `HF_TOKEN` | — |
| `s3://` | `Opaque` | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` | `AWS_SESSION_TOKEN`, `AWS_REGION`, `AWS_ENDPOINT_URL` |
| `oci://` | `kubernetes.io/dockerconfigjson` | `.dockerconfigjson` | — |

The FusionInfer controller reads the Secret when it checks access to the source, and the model agent reads it when downloading. The Secret's Namespace depends on the resource scope:

| Resource | Namespace of the Secret |
| --- | --- |
| Namespaced `Model` | The Model's Namespace |
| `ClusterModel` | The FusionInfer system Namespace, such as `fusioninfer-system`, maintained by platform administrators |

### LoRA artifacts {#lora-artifacts}

A LoRA Model uses `source` to point to its adapter files and `lora.baseModelRef` to name the Base Model it is built for. The following example declares a LoRA that reads its adapter files from S3 and is applied on top of `qwen3-8b-hf-r1`:

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: Model
metadata:
  name: qwen3-8b-finance-lora-r1
  namespace: team-a
spec:
  source:                # Location of the LoRA adapter files
    uri: s3://team-a-models/adapters/qwen3-8b-finance-lora-r1
    credentialsRef:
      name: s3-model-reader
  lora:
    baseModelRef:        # Base Model the LoRA is built for
      kind: Model
      name: qwen3-8b-hf-r1
```

`baseModelRef` must follow these rules:

- A Namespaced LoRA `Model` can reference a `Model` in the same Namespace or a cluster-scoped `ClusterModel`.
- A cluster-scoped LoRA `ClusterModel` can reference only a `ClusterModel`.
- It must point to a Base Model, not another LoRA.

A LoRA Model is loaded only after it is bound in an InferenceDeployment's `spec.lora[]`. The Deployment's `modelRef` determines which Base Model actually runs, and the InferenceDeployment controller loads the LoRA only when that reference and the LoRA's `baseModelRef` resolve to the same object. The following example runs `qwen3-8b-hf-r1` and binds the LoRA above:

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceDeployment
metadata:
  name: qwen3-8b-chat
  namespace: team-a
spec:
  modelRef:              # Same Base Model as the LoRA's baseModelRef
    kind: Model
    name: qwen3-8b-hf-r1
  lora:
    - modelRef:
        kind: Model
        name: qwen3-8b-finance-lora-r1
      servedName: finance  # Model name that selects the LoRA in requests
  # runtimeRef, replicas, endpoint, and other fields are omitted
```

### Prefetch {#prefetch}

By default, the model agent downloads a model only when an InferenceDeployment needs it. `prefetch` declares which nodes a model is downloaded to ahead of time, which suits large models that take a long time to download during scale-out:

| Configuration | Description |
| --- | --- |
| `prefetch` omitted | No proactive download; the model is downloaded when an InferenceDeployment needs it |
| `prefetch: {}` | Download to every node that runs the model agent |
| `nodeSelector` set | Download only to nodes whose labels match |
| `nodeName` set | Download only to the specified node |

The following example prefetches `qwen3-14b-shared-r1` to all H100 nodes:

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: ClusterModel
metadata:
  name: qwen3-14b-shared-r1
spec:
  source:
    uri: hf://Qwen/Qwen3-14B
  prefetch:
    nodeSelector:
      node.kubernetes.io/instance-type: gpu-h100
```

`prefetch` only places the model on nodes ahead of time and does not affect InferenceDeployment rollouts. When an InferenceDeployment uses the `eager` cache mode, it still confirms that the cache is ready on its target nodes before rollout, and prefetched nodes pass immediately.

## Status {#status}

The FusionInfer controller checks access to the source and aggregates the per-node download results reported by the model agent into the Model status:

```go
// ModelStatus summarizes source accessibility and prefetch progress.
type ModelStatus struct {
    ObservedGeneration int64              `json:"observedGeneration,omitempty"`
    Prefetch           *PrefetchStatus    `json:"prefetch,omitempty"`
    Conditions         []metav1.Condition `json:"conditions,omitempty"`
}

// PrefetchStatus records download progress on the nodes that match prefetch.
type PrefetchStatus struct {
    DesiredNodes int32 `json:"desiredNodes"`
    ReadyNodes   int32 `json:"readyNodes"`
    FailedNodes  int32 `json:"failedNodes"`
}
```

Conditions use the standard `metav1.Condition`:

- `Accessible`: the result of the FusionInfer controller accessing the source with the configured credentials. It checks access only and does not download model files. The check runs when the Model is created and again whenever the referenced Secret changes. On failure, `reason` is `AuthenticationFailed`, `NotFound`, or `Unreachable`.
- `Prefetched`: present only when `prefetch` is set. It is `True` when every matching node has downloaded and verified the model; otherwise it is `False`, and `message` summarizes the failed nodes and reasons.

The following example shows the status of the ClusterModel from [Prefetch](#prefetch), where one of four matching nodes failed to download the model:

```yaml
status:
  observedGeneration: 2
  prefetch:
    desiredNodes: 4
    readyNodes: 3
    failedNodes: 1
  conditions:
    - type: Accessible
      status: "True"
      reason: Verified
    - type: Prefetched
      status: "False"
      reason: DownloadFailed
      message: "gpu-h100-4: insufficient disk space"
```

## Examples {#examples}

### Model: Hugging Face Base Model {#model-hugging-face-base-model}

This object resides in `team-a` and pins the repository version with a full commit SHA in the URI. `huggingface-token` is read from the same Namespace.

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: Model
metadata:
  name: qwen3-8b-hf-r1
  namespace: team-a
spec:
  source:
    uri: hf://Qwen/Qwen3-8B@b968826d9c46dd6066d109eabc6255188de91218
    credentialsRef:
      name: huggingface-token
```

### Model: S3 Base Model {#model-s3-base-model}

This object reads model files from S3-compatible storage.

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: Model
metadata:
  name: qwen3-8b-s3-r1
  namespace: team-a
spec:
  source:
    uri: s3://team-a-models/base/qwen3-8b-r1
    credentialsRef:
      name: s3-model-reader
```

### Model: OCI Base Model {#model-oci-base-model}

The digest in the URI pins the artifact version. Registry credentials are provided through a `kubernetes.io/dockerconfigjson` Secret.

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: Model
metadata:
  name: qwen3-8b-oci-r1
  namespace: team-a
spec:
  source:
    uri: oci://registry.example.com/models/qwen3-8b@sha256:9d2e6b4a8f1c30573a7e9c2d5b608f14e1d4a7c3096b2f855c8e1a6d4f703b29
    credentialsRef:
      name: model-registry-credentials
```

### ClusterModel: Cluster-scoped Base Model {#clustermodel-cluster-scoped-base-model}

ClusterModel has the same fields as Model but no Namespace, and can be explicitly referenced by InferenceDeployments in multiple Namespaces. `meta-llama/Llama-3.1-8B-Instruct` below is a gated model, so it must be downloaded with a Hugging Face token whose account has been granted access. The Secret referenced by a ClusterModel lives in the FusionInfer system Namespace, `fusioninfer-system` in this example:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: huggingface-token
  namespace: fusioninfer-system  # Namespace for Secrets referenced by ClusterModels
type: Opaque
stringData:
  HF_TOKEN: "<hf-token>"
---
apiVersion: fusioninfer.io/v1alpha1
kind: ClusterModel
metadata:
  name: llama-3-1-8b-instruct-r1
spec:
  source:
    uri: hf://meta-llama/Llama-3.1-8B-Instruct
    credentialsRef:
      name: huggingface-token      # Read from fusioninfer-system
```
