---
title: Model 与 ClusterModel
description: 定义命名空间级或集群级的不可变模型制品，以及可选的 LoRA 适配器绑定。
---

## 资源定位 {#resource-scope}

`Model` 和 `ClusterModel` 声明模型制品的来源及其版本标识：

- `Model` 是 Namespaced 资源，用于 Namespace 内的模型。
- `ClusterModel` 是 Cluster-scoped 资源，用于跨 Namespace 共享的模型。

两个 Kind 使用相同的 `ModelSpec`。只有 `spec.source` 表示 Base Model；同时设置 `spec.source` 和 `spec.lora.baseModelRef` 表示 LoRA 制品。

下面是一个最小的 Namespaced Base Model：

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

### API 结构 {#api-structure}

`Model` 与 `ClusterModel` 共享以下 Go 接口：

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

`lora` 省略时表示 Base Model，存在时表示 LoRA。`revision` 和 `digest` 是否必填由 `uri` 的 scheme 决定。

### 作用域与引用 {#scope-and-references}

所有引用固定属于 `fusioninfer.io` API Group，只包含 `kind` 和 `name`：

- Namespaced LoRA `Model` 可以引用同一 Namespace 中的 `Model` 或集群级 `ClusterModel`。
- Cluster-scoped LoRA `ClusterModel` 只能引用 `ClusterModel`。
- `baseModelRef` 必须指向 Base Model，不能指向另一个 LoRA。

引用不提供默认 Kind、Namespace fallback 或同名资源回退。

### 模型来源 {#model-sources}

`source.uri` 必填，v1 支持以下来源：

- `hf://<repository>`：`revision` 必填，并使用完整 commit SHA；`digest` 可选。
- `s3://<bucket>/<prefix>`：`digest` 必填。
- `oci://<artifact>@sha256:<digest>`：URI 必须包含 descriptor digest；`source.digest` 可选。
- `pvc://<claim>/<subpath>`：只允许用于 Namespaced `Model`，`digest` 必填，不允许设置 `credentialsRef`。

`source.digest` 使用 `sha256:<64 个小写十六进制字符>` 格式。URI scheme 必须为小写，不允许 query、fragment 或 `.`、`..` 路径段。

### 凭据契约 {#credentials-contract}

`source.credentialsRef` 只包含 Secret 名称，不允许指定 Namespace。省略时使用平台配置的 workload identity 或匿名访问。

- `hf://`：接受 `Opaque` Secret，要求包含 `HF_TOKEN`。
- `s3://`：接受 `Opaque` Secret，使用 AWS SDK 标准键名，同时支持 AWS S3 和 S3-compatible 存储。
- `oci://`：接受 `kubernetes.io/dockerconfigjson` Secret，要求包含 `.dockerconfigjson`。
- `pvc://`：不允许设置 `credentialsRef`。

- Namespaced `Model`：在该 Model 所在 Namespace 中解析 Secret。
- `ClusterModel`：在每个消费它的 `InferenceDeployment` Namespace 中独立解析同名 Secret。不同团队可以提供各自的凭据，Cluster-scoped 对象不会跨 Namespace 读取 Secret，也不会产生一份全局认证状态。

### LoRA 制品语义 {#lora-artifact-semantics}

LoRA Model 使用 `source` 声明 adapter 文件，并使用 `lora.baseModelRef` 声明对应的 Base Model。`baseModelRef` 必须是显式的 `kind + name` 引用，v1 不允许 LoRA 引用另一个 LoRA。

LoRA Model 通过 `InferenceDeployment.spec.lora[]` 绑定到推理服务。制品中的 `baseModelRef` 表示兼容性约束，Deployment 的 `modelRef` 表示实际运行的 Base Model；Controller 只有在两者解析到相同 Kind、名称和 UID 时才加载 LoRA。加载模式、目标逻辑副本和逐角色状态由消费它的 InferenceDeployment 与 RuntimeProfile 管理。

### 默认值与校验 {#defaults-and-validation}

- `spec.source` 必填。
- `revision`、`digest` 和 `credentialsRef` 必须符合对应 URI scheme 的约束。
- `lora` 存在时，`baseModelRef.kind` 和 `baseModelRef.name` 必填。
- `ClusterModel` 不允许使用 `pvc://`。
- Cluster-scoped LoRA 不能引用 Namespaced `Model`。
- `Model.spec` 和 `ClusterModel.spec` 在 v1 中不可变。修改来源、版本、凭据或 `baseModelRef` 时需要创建新对象。

## Status {#status}

`Model` 和 `ClusterModel` 不提供 status subresource。Model 对象存在只表示声明有效，不表示模型已经下载或加载。

## 示例 {#examples}

### Model：Hugging Face Base Model {#model-hugging-face-base-model}

该对象位于 `team-a`，使用完整 HF commit SHA 固定仓库版本。`huggingface-token` 在同一 Namespace 中解析。

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

### Model：PVC Base Model {#model-pvc-base-model}

该对象及 `qwen3-weights` PVC 都位于 `team-a`，`digest` 用于校验 PVC 中的模型内容。

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

### Model：S3 Base Model {#model-s3-base-model}

该对象从 S3-compatible 存储读取模型文件，并使用 `digest` 校验物化后的内容。

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

### Model：OCI Base Model {#model-oci-base-model}

OCI URI 中的 descriptor digest 固定 artifact 版本，Registry 凭据通过 `kubernetes.io/dockerconfigjson` Secret 提供。

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

### Model：LoRA {#model-lora}

该 LoRA 位于 `team-a`，adapter 文件来自 S3，`baseModelRef` 显式引用同一 Namespace 中的 Base Model。

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

### ClusterModel：共享 Base Model {#clustermodel-shared-base-model}

ClusterModel 的字段与 Model 相同，但没有 Namespace。该普通 Base Model 可以被多个 Namespace 中的 InferenceDeployment 显式引用。

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

