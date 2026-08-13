---
slug: introducing-fusioninfer
title: "Introducing FusionInfer: A Kubernetes-native Platform for LLM Inference"
authors: [fusioninfer-team]
tags: [kubernetes, llm-inference, platform-engineering]
description: Meet FusionInfer, a Kubernetes-native platform for declarative LLM inference orchestration.
---

Running an LLM inference engine is only one part of serving models in production. Platform teams must also coordinate distributed workloads, schedule GPU-backed pods together, and route requests according to inference-specific signals.

FusionInfer brings those responsibilities behind one Kubernetes API.

<!-- truncate -->

## One declarative API {#one-declarative-api}

FusionInfer provides an `InferenceService` custom resource for describing an inference topology. A service can combine workload roles, replica counts, multi-node settings, pod templates, and routing configuration in one manifest.

The controller turns that declaration into the Kubernetes resources needed by the selected topology, including:

- `LeaderWorkerSet` resources for inference workloads
- Volcano `PodGroup` resources for gang scheduling
- `InferencePool` and `HTTPRoute` resources for inference-aware traffic management

This keeps the user-facing workflow Kubernetes-native while leaving the inference engine inside the workload container.

## Three serving topologies {#three-serving-topologies}

FusionInfer is designed around three LLM serving patterns:

1. **Monolithic serving** keeps the full request lifecycle in one inference role.
2. **Prefill/decode disaggregation** separates the compute-intensive prefill phase from the memory-intensive decode phase.
3. **Multi-node inference** uses a leader-worker topology when a model must span multiple nodes.

Each topology uses the same `InferenceService` API, so teams do not need a separate orchestration model for every serving pattern.

## A familiar Kubernetes workflow {#a-familiar-kubernetes-workflow}

The following shortened example declares a routed, single-node service:

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

Users submit the manifest with standard Kubernetes tooling. FusionInfer then reconciles the workload and routing resources represented by the service.

## Inference-aware routing {#inference-aware-routing}

Generic load balancing does not account for signals such as prefix reuse, KV-cache pressure, queue depth, or the split between prefill and decode. FusionInfer's `InferenceService` API exposes routing strategies for:

- Prefix-cache awareness
- KV-cache utilization
- Queue size
- LoRA affinity
- Prefill/decode disaggregation

Advanced users can also provide an `EndpointPickerConfig` when a predefined strategy is not enough.

## A platform, not an inference engine {#a-platform-not-an-inference-engine}

FusionInfer does not implement model execution. Engines such as vLLM continue to run in the workload containers. FusionInfer focuses on the Kubernetes control plane around those engines, and its current scope is LLM inference rather than general-purpose machine learning workloads.

## Start exploring {#start-exploring}

Read the [documentation](/docs/intro) for the architecture and prerequisites, then follow the [deployment guide](/docs/user-guide/deployment) for single-node and multi-node examples.
