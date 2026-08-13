---
sidebar_position: 1
title: 部署指南
description: 在 Kubernetes 集群中部署 FusionInfer 及其依赖项，并验证控制器已就绪。
---

# 部署指南 {#deployment-guide}

本指南介绍如何使用 FusionInfer 的 `InferenceService` CRD 部署大语言模型推理服务。

## 单节点部署 {#single-node-deployment}

单节点部署是提供大语言模型服务的最简单方式。每个副本在单个 GPU 上运行。


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
              image: vllm/vllm-openai:v0.27.1
              args: ["--model", "Qwen/Qwen3-8B"]
              resources:
                limits:
                  nvidia.com/gpu: "1"
```

## 多节点部署 {#multi-node-deployment}

多节点部署支持跨多个 GPU/节点进行张量并行，可用于部署无法装入单个 GPU 显存的大型模型。

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: qwen-multi-node
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
      multinode:
        nodeCount: 2
      template:
        spec:
          containers:
            - name: vllm
              image: vllm/vllm-openai:v0.27.1
              args: ["--model", "Qwen/Qwen3-8B", "--tensor-parallel-size", "2"]
              resources:
                limits:
                  nvidia.com/gpu: "1"
              volumeMounts:
                - name: shm
                  mountPath: /dev/shm
          volumes:
            # 增大共享内存，以支持 GPU 之间的 NCCL 通信
            - name: shm
              emptyDir:
                medium: Memory
                sizeLimit: 10Gi
```
