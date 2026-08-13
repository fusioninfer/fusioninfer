---
sidebar_position: 1
title: Clientset 生成
description: 为 FusionInfer 自定义资源生成类型化 Kubernetes clientset，并在 Go 中使用它们。
---

# Clientset 生成 {#clientset-generation}

FusionInfer 使用 `controller-runtime` 的 `client.Client` 执行常规操作。如果需要使用 SharedInformers，或与其他 Kubernetes 组件集成，请使用生成的强类型 Clientset。

## 将新 CRD 添加到 Clientset {#add-new-crd-to-clientset}

要为新的 CRD 类型生成 Clientset：

1. 为类型添加 `+genclient` 标记：

```go
// api/core/v1alpha1/your_types.go

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient
type YourResource struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec   YourResourceSpec   `json:"spec,omitempty"`
    Status YourResourceStatus `json:"status,omitempty"`
}
```

2. 重新生成代码：

```bash
./hack/update-codegen.sh
```

生成的代码将存放在 `client-go/` 中。

## 用法 {#usage}

使用强类型 Clientset 执行 CRUD 操作：

```go
import clientset "github.com/fusioninfer/fusioninfer/client-go/clientset/versioned"

config, _ := rest.InClusterConfig()
client, _ := clientset.NewForConfig(config)

// 获取
obj, _ := client.CoreV1alpha1().InferenceServices("default").Get(ctx, "name", metav1.GetOptions{})

// 更新状态
client.CoreV1alpha1().InferenceServices("default").UpdateStatus(ctx, obj, metav1.UpdateOptions{})

// 使用标签选择器列出资源
list, _ := client.CoreV1alpha1().InferenceServices("").List(ctx, metav1.ListOptions{
    LabelSelector: "app=inference",
})
```
