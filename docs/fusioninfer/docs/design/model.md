---
title: Model and ClusterModel
description: Define namespaced or cluster-scoped immutable model artifacts and optional LoRA adapter bindings.
---

## Resource scope {#resource-scope}

`Model` and `ClusterModel` declare the source of a model artifact and its version identifier:

- `Model` is a Namespaced resource for models within a Namespace.
- `ClusterModel` is a cluster-scoped resource for models shared across Namespaces.

Both Kinds use the same `ModelSpec`. Setting only `spec.source` represents a Base Model; setting both `spec.source` and `spec.lora.baseModelRef` represents a LoRA artifact.

The following example is a minimal Namespaced Base Model:

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: Model
metadata:
  name: qwen3-8b-r1
  namespace: team-a
spec:
  source:
    uri: hf://Qwen/Qwen3-8B
    revision: 0123456789abcdef0123456789abcdef01234567
```

## Spec {#spec}

### API structure {#api-structure}

`Model` and `ClusterModel` share the following Go API:

```go
type ModelSpec struct {
    Source ModelSource      `json:"source"`
    LoRA   *LoRAArtifactSpec `json:"lora,omitempty"`
}

type LoRAArtifactSpec struct {
    BaseModelRef ModelReference `json:"baseModelRef"`
}

type ModelReference struct {
    Kind string `json:"kind"`
    Name string `json:"name"`
}

type ModelSource struct {
    URI            string                        `json:"uri"`
    Revision       string                        `json:"revision,omitempty"`
    Digest         string                        `json:"digest,omitempty"`
    CredentialsRef *corev1.LocalObjectReference `json:"credentialsRef,omitempty"`
}
```

Omitting `lora` represents a Base Model; including it represents a LoRA. Whether `revision` and `digest` are required depends on the `uri` scheme.

### Scope and references {#scope-and-references}

All references belong to the `fusioninfer.io` API Group and contain only `kind` and `name`:

- A Namespaced LoRA `Model` can reference a `Model` in the same Namespace or a cluster-scoped `ClusterModel`.
- A cluster-scoped LoRA `ClusterModel` can reference only a `ClusterModel`.
- `baseModelRef` must point to a Base Model, not another LoRA.

References do not provide a default Kind, Namespace fallback, or fallback to a resource with the same name.

### Model sources {#model-sources}

`source.uri` is required. v1 supports the following sources:

- `hf://<repository>`: `revision` is required and must be a full commit SHA; `digest` is optional.
- `s3://<bucket>/<prefix>`: `digest` is required.
- `oci://<artifact>@sha256:<digest>`: the URI must include the descriptor digest; `source.digest` is optional.
- `pvc://<claim>/<subpath>`: permitted only for a Namespaced `Model`; `digest` is required, and `credentialsRef` is not allowed.

`source.digest` uses the format `sha256:<64 lowercase hexadecimal characters>`. The URI scheme must be lowercase. Queries, fragments, and `.` or `..` path segments are not allowed.

### Credentials contract {#credentials-contract}

`source.credentialsRef` contains only a Secret name; a Namespace cannot be specified. When omitted, the platform-configured workload identity or anonymous access is used.

- `hf://`: accepts an `Opaque` Secret that must contain `HF_TOKEN`.
- `s3://`: accepts an `Opaque` Secret, uses standard AWS SDK key names, and supports both AWS S3 and S3-compatible storage.
- `oci://`: accepts a `kubernetes.io/dockerconfigjson` Secret that must contain `.dockerconfigjson`.
- `pvc://`: does not allow `credentialsRef`.

- Namespaced `Model`: the Secret is resolved in the Model's Namespace.
- `ClusterModel`: a Secret with the same name is resolved independently in the Namespace of each `InferenceDeployment` that consumes it. Different teams can provide their own credentials; the cluster-scoped object does not read Secrets across Namespaces or create a single global authentication state.

### LoRA artifact semantics {#lora-artifact-semantics}

A LoRA Model uses `source` to declare its adapter files and `lora.baseModelRef` to declare the corresponding Base Model. `baseModelRef` must be an explicit `kind + name` reference. v1 does not allow a LoRA to reference another LoRA.

A LoRA Model is bound to an inference service through `InferenceDeployment.spec.lora[]`. The artifact's `baseModelRef` represents a compatibility constraint, while the Deployment's `modelRef` represents the Base Model that actually runs. The Controller loads the LoRA only when both references resolve to the same Kind, name, and UID. The consuming InferenceDeployment and RuntimeProfile manage the loading mode, target logical replicas, and per-role status.

### Defaults and validation {#defaults-and-validation}

- `spec.source` is required.
- `revision`, `digest`, and `credentialsRef` must satisfy the constraints for the corresponding URI scheme.
- When `lora` is present, `baseModelRef.kind` and `baseModelRef.name` are required.
- `ClusterModel` does not allow `pvc://`.
- A cluster-scoped LoRA cannot reference a Namespaced `Model`.
- `Model.spec` and `ClusterModel.spec` are immutable in v1. Changing the source, version, credentials, or `baseModelRef` requires creating a new object.

## Status {#status}

`Model` and `ClusterModel` do not provide a status subresource. The existence of a Model object indicates only that its declaration is valid; it does not indicate that the model has been downloaded or loaded.

## Examples {#examples}

### Model: Hugging Face Base Model {#model-hugging-face-base-model}

This object resides in `team-a` and pins the repository version with a full HF commit SHA. `huggingface-token` is resolved in the same Namespace.

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: Model
metadata:
  name: qwen3-8b-hf-r1
  namespace: team-a
spec:
  source:
    uri: hf://Qwen/Qwen3-8B
    revision: 0123456789abcdef0123456789abcdef01234567
    credentialsRef:
      name: huggingface-token
```

### Model: PVC Base Model {#model-pvc-base-model}

This object and the `qwen3-weights` PVC both reside in `team-a`. `digest` verifies the model content in the PVC.

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: Model
metadata:
  name: qwen3-8b-pvc-r1
  namespace: team-a
spec:
  source:
    uri: pvc://qwen3-weights/models/qwen3-8b
    digest: sha256:7f3c9a1e4b2d8f605a7c3e9d1b4f2860c5a8e2d7f1b3096c4e8a5d2f7b1c903e
```

### Model: S3 Base Model {#model-s3-base-model}

This object reads model files from S3-compatible storage and uses `digest` to verify the materialized content.

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: Model
metadata:
  name: qwen3-8b-s3-r1
  namespace: team-a
spec:
  source:
    uri: s3://team-a-models/base/qwen3-8b-r1
    digest: sha256:4a7d1ed414474e4033ac29ccb8653d9b8f9f45278a28a74cc2d8f7f31e4b6c90
    credentialsRef:
      name: s3-model-reader
```

### Model: OCI Base Model {#model-oci-base-model}

The descriptor digest in the OCI URI pins the artifact version. Registry credentials are provided through a `kubernetes.io/dockerconfigjson` Secret.

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

### Model: LoRA {#model-lora}

This LoRA resides in `team-a`. Its adapter files come from S3, and `baseModelRef` explicitly references a Base Model in the same Namespace.

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: Model
metadata:
  name: qwen3-8b-finance-lora-r1
  namespace: team-a
spec:
  source:
    uri: s3://team-a-models/adapters/qwen3-8b-finance-lora-r1
    digest: sha256:c14a8d6f2e905b378f4c1a7d3e6b2095d2f8a4c7091e5b636a3d9f2e8c1b7045
    credentialsRef:
      name: s3-model-reader
  lora:
    baseModelRef:
      kind: Model
      name: qwen3-8b-hf-r1
```

### ClusterModel: Shared Base Model {#clustermodel-shared-base-model}

ClusterModel has the same fields as Model but no Namespace. This regular Base Model can be explicitly referenced by InferenceDeployments in multiple Namespaces.

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: ClusterModel
metadata:
  name: qwen3-14b-shared-r1
spec:
  source:
    uri: hf://Qwen/Qwen3-14B
    revision: 89abcdef0123456789abcdef0123456789abcdef
```

