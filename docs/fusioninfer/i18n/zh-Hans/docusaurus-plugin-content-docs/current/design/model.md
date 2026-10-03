---
title: Model 与 ClusterModel
description: 定义命名空间级或集群级的模型制品，以及可选的 LoRA 适配器绑定。
---

## 概述 {#overview}

`Model` 和 `ClusterModel` 声明模型制品的来源：

- `Model` 是 Namespaced 资源，用于 Namespace 内的模型。
- `ClusterModel` 是 Cluster-scoped 资源，用于跨 Namespace 共享的模型。

`Model` 和 `ClusterModel` 使用相同的 `ModelSpec`。只有 `spec.source` 表示 Base Model；同时设置 `spec.source` 和 `spec.lora.baseModelRef` 表示 LoRA 制品。

模型文件由 FusionInfer 在每个节点上运行的 model agent 下载到节点缓存，下载时机见[预先下载](#prefetch)。

下面是一个 `Model` 资源的示例：

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

### API 结构 {#api-structure}

`Model` 与 `ClusterModel` 共享以下 Go 接口：

```go
// ModelSpec 声明模型制品以及要预先下载到哪些节点，由 Model 与 ClusterModel 共用。
type ModelSpec struct {
    Source   ModelSource       `json:"source"`
    LoRA     *LoRAArtifactSpec `json:"lora,omitempty"`
    Prefetch *PrefetchSpec     `json:"prefetch,omitempty"`
}

// LoRAArtifactSpec 只在 LoRA 制品上设置，声明这个 LoRA 可以叠加在哪个 Base Model 上。
type LoRAArtifactSpec struct {
    BaseModelRef ModelReference `json:"baseModelRef"`
}

// ModelReference 通过 kind 和 name 引用 Model 或 ClusterModel。
type ModelReference struct {
    Kind string `json:"kind"`
    Name string `json:"name"`
}

// ModelSource 声明制品的存储位置（版本写在 URI 中），以及访问它时使用的 Secret。
type ModelSource struct {
    URI            string           `json:"uri"`
    CredentialsRef *SecretReference `json:"credentialsRef,omitempty"`
}

// SecretReference 按名称引用 Secret，Secret 所在的 Namespace 由资源作用域决定。
type SecretReference struct {
    Name string `json:"name"`
}

// PrefetchSpec 声明模型要预先下载到哪些节点。
type PrefetchSpec struct {
    NodeSelector map[string]string `json:"nodeSelector,omitempty"`
    NodeName     string            `json:"nodeName,omitempty"`
}
```

### 模型来源 {#model-sources}

`source.uri` 指定模型文件的存储位置，URI 的 scheme 决定存储类型，版本也写在 URI 中。该字段必填，支持以下来源，方括号中的部分可以省略：

| 来源 | URI 格式 | 说明 | 示例 |
| --- | --- | --- | --- |
| Hugging Face | `hf://<owner>/<repo>[@<revision>]` | `revision` 可以是分支、tag 或 commit SHA，不写时使用仓库默认分支 | `hf://Qwen/Qwen3-8B@main`<br />`hf://Qwen/Qwen3-8B@b968826d9c46dd6066d109eabc6255188de91218` |
| S3 | `s3://<bucket>/<prefix>` | 不支持指定版本 | `s3://team-a-models/base/qwen3-8b` |
| OCI | `oci://<registry>/<repository>[:<tag>\|@sha256:<digest>]` | 可以写 tag 或 digest，都不写时使用 `latest` | `oci://registry.example.com/models/qwen3-8b:v1`<br />`oci://registry.example.com/models/qwen3-8b@sha256:9d2e…` |

### 访问凭据 {#access-credentials}

`source.credentialsRef` 只包含 Secret 名称，不允许指定 Namespace。省略时不使用凭据，按匿名方式访问，只能下载公开模型。

| 来源 | Secret 类型 | 必需键 | 可选键 |
| --- | --- | --- | --- |
| `hf://` | `Opaque` | `HF_TOKEN` | — |
| `s3://` | `Opaque` | `AWS_ACCESS_KEY_ID`、`AWS_SECRET_ACCESS_KEY` | `AWS_SESSION_TOKEN`、`AWS_REGION`、`AWS_ENDPOINT_URL` |
| `oci://` | `kubernetes.io/dockerconfigjson` | `.dockerconfigjson` | — |

FusionInfer 控制器检查来源能否访问时、model agent 下载时都会读取这个 Secret，它所在的 Namespace 取决于资源作用域：

| 资源 | Secret 所在的 Namespace |
| --- | --- |
| Namespaced `Model` | 该 Model 所在的 Namespace |
| `ClusterModel` | FusionInfer 的系统 Namespace（如 `fusioninfer-system`），由平台管理员维护 |

### LoRA 制品 {#lora-artifacts}

LoRA Model 用 `source` 指定 adapter 文件的位置，用 `lora.baseModelRef` 指定它适配的 Base Model。下面的示例声明了一个 LoRA，它从 S3 读取 adapter 文件，叠加在 `qwen3-8b-hf-r1` 上：

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: Model
metadata:
  name: qwen3-8b-finance-lora-r1
  namespace: team-a
spec:
  source:                # LoRA adapter 文件的位置
    uri: s3://team-a-models/adapters/qwen3-8b-finance-lora-r1
    credentialsRef:
      name: s3-model-reader
  lora:
    baseModelRef:        # 适配的 Base Model
      kind: Model
      name: qwen3-8b-hf-r1
```

`baseModelRef` 需要满足以下规则：

- Namespaced LoRA `Model` 可以引用同一 Namespace 中的 `Model` 或集群级 `ClusterModel`。
- Cluster-scoped LoRA `ClusterModel` 只能引用 `ClusterModel`。
- 必须指向 Base Model，不能指向另一个 LoRA。

LoRA Model 要在 InferenceDeployment 的 `spec.lora[]` 中绑定后才会被加载。Deployment 的 `modelRef` 决定实际运行哪个 Base Model，只有它和 LoRA 的 `baseModelRef` 解析到同一个对象时，InferenceDeployment 控制器才会加载这个 LoRA。下面的示例运行 `qwen3-8b-hf-r1`，并绑定上面的 LoRA：

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceDeployment
metadata:
  name: qwen3-8b-chat
  namespace: team-a
spec:
  modelRef:              # 与 LoRA 的 baseModelRef 指向同一个 Base Model
    kind: Model
    name: qwen3-8b-hf-r1
  lora:
    - modelRef:
        kind: Model
        name: qwen3-8b-finance-lora-r1
      servedName: finance  # 请求中用这个名字选择 LoRA
  # runtimeRef、replicas、endpoint 等字段省略
```

### 预先下载 {#prefetch}

model agent 默认在 InferenceDeployment 需要某个模型时才下载它。`prefetch` 声明模型要预先下载到哪些节点，适合体积大、扩容时现场下载耗时长的模型：

| 配置 | 说明 |
| --- | --- |
| 不设置 `prefetch` | 不主动下载，InferenceDeployment 需要时再按需下载 |
| `prefetch: {}` | 下载到所有运行 model agent 的节点 |
| 设置 `nodeSelector` | 只下载到 label 匹配的节点 |
| 设置 `nodeName` | 只下载到指定的节点 |

下面的示例把 `qwen3-14b-shared-r1` 预先下载到所有 H100 节点：

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

`prefetch` 只负责把模型提前放到节点上，不影响 InferenceDeployment 的发布。InferenceDeployment 使用 `eager` 缓存模式时，发布前仍会确认目标节点的缓存已就绪，已经预先下载的节点会直接通过。

## Status {#status}

FusionInfer 控制器检查来源能否访问，并汇总 model agent 上报的各节点下载结果，写入 Model 的 status：

```go
// ModelStatus 汇总来源的可访问性和预先下载进度。
type ModelStatus struct {
    ObservedGeneration int64              `json:"observedGeneration,omitempty"`
    Prefetch           *PrefetchStatus    `json:"prefetch,omitempty"`
    Conditions         []metav1.Condition `json:"conditions,omitempty"`
}

// PrefetchStatus 记录匹配 prefetch 的节点上的下载进度。
type PrefetchStatus struct {
    DesiredNodes int32 `json:"desiredNodes"`
    ReadyNodes   int32 `json:"readyNodes"`
    FailedNodes  int32 `json:"failedNodes"`
}
```

Conditions 使用标准的 `metav1.Condition`：

- `Accessible`：FusionInfer 控制器使用配置的凭据访问来源的结果，只检查能否访问，不下载模型文件。Model 创建后检查一次，引用的 Secret 变化后重新检查；失败时 `reason` 为 `AuthenticationFailed`、`NotFound` 或 `Unreachable`。
- `Prefetched`：只在设置了 `prefetch` 时出现。所有匹配的节点都完成下载和校验时为 `True`，否则为 `False`，`message` 汇总失败的节点和原因。

下面是[预先下载](#prefetch)中那个 ClusterModel 的 status，4 个匹配节点里有 1 个下载失败：

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

## 示例 {#examples}

### Model：Hugging Face Base Model {#model-hugging-face-base-model}

该对象位于 `team-a`，在 URI 中用完整的 commit SHA 固定仓库版本。`huggingface-token` 从同一 Namespace 中读取。

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

### Model：S3 Base Model {#model-s3-base-model}

该对象从 S3-compatible 存储读取模型文件。

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

### Model：OCI Base Model {#model-oci-base-model}

URI 中的 digest 固定 artifact 版本，Registry 凭据通过 `kubernetes.io/dockerconfigjson` Secret 提供。

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

### ClusterModel：集群级 Base Model {#clustermodel-cluster-scoped-base-model}

ClusterModel 的字段与 Model 相同，但没有 Namespace，可以被多个 Namespace 中的 InferenceDeployment 显式引用。下面的 `meta-llama/Llama-3.1-8B-Instruct` 是 gated 模型，需要使用已获得访问授权的 Hugging Face token 下载。ClusterModel 引用的 Secret 放在 FusionInfer 的系统 Namespace 中，示例中为 `fusioninfer-system`：

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: huggingface-token
  namespace: fusioninfer-system  # ClusterModel 引用的 Secret 所在的 Namespace
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
      name: huggingface-token      # 从 fusioninfer-system 中读取
```
