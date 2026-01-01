---
sidebar_position: 1
title: Deployment Guide
---

# Deployment Guide

This guide covers how to deploy LLM inference services using FusionInfer's `InferenceService` CRD.

## Single-Node Deployment

Single-node deployment is the simplest way to serve LLM models. Each replica runs on a single GPU.


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
              image: vllm/vllm-openai:v0.11.0
              args: ["--model", "Qwen/Qwen3-8B"]
              resources:
                limits:
                  nvidia.com/gpu: "1"
```

## Multi-Node Deployment

Multi-node deployment enables tensor parallelism across multiple GPUs/nodes for large models that don't fit in a single GPU's memory.

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
              image: vllm/vllm-openai:v0.13.0
              args: ["--model", "Qwen/Qwen3-8B", "--tensor-parallel-size", "2"]
              resources:
                limits:
                  nvidia.com/gpu: "1"
              volumeMounts:
                - name: shm
                  mountPath: /dev/shm
          volumes:
            # Increase shared memory for NCCL communication between GPUs
            - name: shm
              emptyDir:
                medium: Memory
                sizeLimit: 10Gi
```
