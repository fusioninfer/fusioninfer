---
sidebar_position: 1
slug: /intro
description: FusionInfer 是一个用于部署、编排和路由 LLM 推理工作负载的 Kubernetes 原生平台。
---

# FusionInfer {#fusioninfer}

FusionInfer 是用于统一编排 LLM 推理的 Kubernetes 控制器，同时支持一体式与 Prefill/Decode（PD）分离式服务拓扑。

## 说明 {#description}

FusionInfer 提供统一的 `InferenceService` CRD，支持：

- **一体式部署**：由单个 Pod 执行推理并处理请求的完整生命周期
- **PD 分离式部署**：将 Prefill 和 Decode 拆分为独立角色，提高 GPU 利用率
- **多节点部署**：通过张量并行跨多个节点执行分布式推理
- **Gang 调度**：集成 Volcano PodGroup，实现原子调度
- **智能路由**：集成 Gateway API，并使用 EPP（Endpoint Picker）进行请求调度

## 演示 {#demo}

观看[前缀缓存感知路由演示](https://github.com/user-attachments/assets/1743bf67-2abd-42cd-a0f3-d7b65281f8cb)。

## 架构 {#architecture}

```
┌─────────────────────────────────────────────────────────────────┐
│                      InferenceService CRD                       │
│   (roles: worker/prefiller/decoder, replicas, multinode)        │
└─────────────────────────────────┬───────────────────────────────┘
                                  │
                    ┌───────────────────────────────┐
                    │      InferenceService 控制器   │
                    └─────────────┬─────────────────┘
                                  │
        ┌─────────────────────────┼─────────────────────────┐
        │                         │                         │
        ▼                         ▼                         ▼
┌───────────────┐       ┌─────────────────┐       ┌─────────────────┐
│   PodGroup    │       │ LeaderWorkerSet │       │   路由器 (EPP)   │
│  (Volcano)    │       │     (LWS)       │       │  InferencePool  │
│               │       │                 │       │  HTTPRoute      │
└───────────────┘       └─────────────────┘       └─────────────────┘
```

## 入门 {#getting-started}

### 安装依赖项 {#install-dependencies}

FusionInfer 需要以下组件：

**1. LeaderWorkerSet (LWS)** - 用于管理多节点工作负载

```bash
kubectl create -f https://github.com/kubernetes-sigs/lws/releases/download/v0.7.0/manifests.yaml
```

参考：[LWS 安装指南](https://lws.sigs.k8s.io/docs/installation/) | [版本发布](https://github.com/kubernetes-sigs/lws/releases)

**2. Volcano** - 用于 Gang 调度

```bash
kubectl apply -f https://raw.githubusercontent.com/volcano-sh/volcano/v1.13.1/installer/volcano-development.yaml
```

参考：[Volcano 安装指南](https://volcano.sh/en/docs/installation/) | [版本发布](https://github.com/volcano-sh/volcano/releases)

**3. Gateway API** - 用于服务路由

```bash
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.4.1/standard-install.yaml
```

参考：[Gateway API 安装指南](https://gateway-api.sigs.k8s.io/guides/#installing-gateway-api) | [版本发布](https://github.com/kubernetes-sigs/gateway-api/releases)

**4. Gateway API Inference Extension** - 用于智能路由推理请求

```bash
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/releases/download/v1.2.1/manifests.yaml
```

参考：[Inference Extension 文档](https://gateway-api-inference-extension.sigs.k8s.io/) | [版本发布](https://github.com/kubernetes-sigs/gateway-api-inference-extension/releases)

### 安装 Gateway {#install-the-gateway}

设置 Kgateway 版本并安装 Kgateway CRD：

```bash
KGTW_VERSION=v2.1.0
helm upgrade -i --create-namespace --namespace kgateway-system --version $KGTW_VERSION kgateway-crds oci://cr.kgateway.dev/kgateway-dev/charts/kgateway-crds
```

安装 Kgateway：

```bash
helm upgrade -i --namespace kgateway-system --version $KGTW_VERSION kgateway oci://cr.kgateway.dev/kgateway-dev/charts/kgateway --set inferenceExtension.enabled=true
```

部署 Inference Gateway：

```bash
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/raw/main/config/manifests/gateway/kgateway/gateway.yaml
```

### 快速开始（本地开发） {#quick-start-local-development}

```bash
# 1. 创建 kind 集群（可选）
kind create cluster --name fusioninfer

# 2. 安装 FusionInfer CRD
make install

# 3. 在本地运行控制器
make run
```

## 使用示例 {#usage-examples}

### 一体式 LLM 服务 {#monolithic-llm-service}

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: qwen-inference
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
      replicas: 1
      template:
        spec:
          containers:
            - name: vllm
              image: vllm/vllm-openai:v0.27.1
              args: ["--model", "Qwen/Qwen3-8B"]
              resources:
                limits:
                  nvidia.com/gpu: "1"
```

## 发送请求 {#send-request}

```bash
# 可使用 minikube tunnel 为 LoadBalancer 类型的 Service 分配 IP 地址
GATEWAY_IP=$(kubectl get gateway inference-gateway -o jsonpath='{.status.addresses[0].value}')

curl -X POST "http://${GATEWAY_IP}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "Qwen/Qwen3-8B",
    "messages": [
      {"role": "user", "content": "你好，最近怎么样？"}
    ]
  }'
```
