---
slug: introducing-fusioninfer
title: "推出 FusionInfer：面向 LLM 推理的 Kubernetes 原生平台"
authors: [fusioninfer-team]
tags: [kubernetes, llm-inference, platform-engineering]
description: 了解 FusionInfer——一个以声明式方式编排 LLM 推理的 Kubernetes 原生平台。
---

运行 LLM 推理引擎只是生产环境中提供模型服务的一环。平台团队还必须协调分布式工作负载、对使用 GPU 的 Pod 进行协同调度，并依据推理特有的信号路由请求。

FusionInfer 将这些职责统一到一个 Kubernetes API 之下。

<!-- truncate -->

## 一个声明式 API {#one-declarative-api}

FusionInfer 提供 `InferenceService` 自定义资源，用于描述推理拓扑。一个服务可以在一份清单中组合工作负载角色、副本数量、多节点设置、Pod 模板和路由配置。

控制器将这份声明转换为所选拓扑所需的 Kubernetes 资源，包括：

- 用于推理工作负载的 `LeaderWorkerSet` 资源
- 用于 Gang 调度的 Volcano `PodGroup` 资源
- 用于推理感知流量管理的 `InferencePool` 和 `HTTPRoute` 资源

这样既能让面向用户的工作流保持 Kubernetes 原生，又能让推理引擎继续运行在工作负载容器内。

## 三种服务拓扑 {#three-serving-topologies}

FusionInfer 围绕三种 LLM 服务模式而设计：

1. **一体式服务**在单一推理角色内完成请求的整个生命周期。
2. **Prefill/Decode 分离**将计算密集型的 Prefill 阶段与内存密集型的 Decode 阶段分开。
3. **多节点推理**在模型必须跨多个节点运行时采用 Leader/Worker 拓扑。

每种拓扑都使用相同的 `InferenceService` API，因此团队无需为每种服务模式采用不同的编排模型。

## 熟悉的 Kubernetes 工作流 {#a-familiar-kubernetes-workflow}

下面的精简示例声明了一个启用路由的单节点服务：

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: qwen
spec:
  roles:
    - name: router
      componentType: router
      strategy: prefix-cache
      httproute:
        parentRefs:
          - name: inference-gateway
    - name: inference
      componentType: worker
      replicas: 2
      template:
        spec:
          containers:
            - name: vllm
              image: vllm/vllm-openai:v0.26.0
              args: ["--model", "Qwen/Qwen3-8B"]
              resources:
                limits:
                  nvidia.com/gpu: "1"
```

用户可以使用标准 Kubernetes 工具提交该清单。随后，FusionInfer 会调和该服务所表示的工作负载和路由资源。

## 推理感知路由 {#inference-aware-routing}

通用负载均衡无法考虑前缀复用、KV cache 压力、队列深度，以及 Prefill 与 Decode 的分离等信号。FusionInfer 的 `InferenceService` API 提供以下路由策略：

- Prefix cache 感知
- KV cache 利用率
- 队列大小
- LoRA 亲和性
- Prefill/Decode 分离

预定义策略不足时，高级用户还可以提供 `EndpointPickerConfig`。

## 平台，而非推理引擎 {#a-platform-not-an-inference-engine}

FusionInfer 不负责模型执行。vLLM 等引擎仍然在工作负载容器中运行。FusionInfer 专注于这些引擎周围的 Kubernetes 控制平面，目前的范围是 LLM 推理，而非通用机器学习工作负载。

## 开始探索 {#start-exploring}

阅读[文档](/docs/intro)以了解架构和前置要求，然后按照[部署指南](/docs/user-guide/deployment)完成单节点和多节点示例。
