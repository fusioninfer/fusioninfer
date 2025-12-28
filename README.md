# FusionInfer

A Kubernetes controller for unified LLM inference orchestration, supporting both monolithic and prefill/decode (PD) disaggregated serving topologies.

## Description

FusionInfer provides a single `InferenceService` CRD that enables:

- **Monolithic deployment**: Single-pod inference handling full request lifecycle
- **PD disaggregated deployment**: Separate prefill and decode roles for better GPU utilization
- **Multi-node deployment**: Distributed inference across multiple nodes using tensor parallelism
- **Gang scheduling**: Atomic scheduling via Volcano PodGroup integration
- **Intelligent routing**: Gateway API integration with EPP (Endpoint Picker) for request scheduling

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      InferenceService CRD                        │
│   (roles: worker/prefiller/decoder, replicas, multinode)        │
└─────────────────────────────────┬───────────────────────────────┘
                                  │
                    ┌─────────────┴─────────────┐
                    │  InferenceService Controller │
                    └─────────────┬─────────────┘
                                  │
        ┌─────────────────────────┼─────────────────────────┐
        │                         │                         │
        ▼                         ▼                         ▼
┌───────────────┐       ┌─────────────────┐       ┌─────────────────┐
│   PodGroup    │       │ LeaderWorkerSet │       │  Router (EPP)   │
│  (Volcano)    │       │     (LWS)       │       │  InferencePool  │
│               │       │                 │       │  HTTPRoute      │
└───────────────┘       └─────────────────┘       └─────────────────┘
```

## Getting Started

### Prerequisites

- Go version v1.24.0+
- Docker version 17.03+
- kubectl version v1.11.3+
- Access to a Kubernetes v1.11.3+ cluster

### Install Dependencies

FusionInfer requires the following components:

**1. LeaderWorkerSet (LWS)** - For multi-node workload management

```bash
kubectl create -f https://github.com/kubernetes-sigs/lws/releases/download/v0.7.0/manifests.yaml
```

> Source: [LWS Installation Guide](https://lws.sigs.k8s.io/docs/installation/) | [Releases](https://github.com/kubernetes-sigs/lws/releases)

**2. Volcano** - For gang scheduling

```bash
kubectl apply -f https://raw.githubusercontent.com/volcano-sh/volcano/v1.13.1/installer/volcano-development.yaml
```

> Source: [Volcano Installation Guide](https://volcano.sh/en/docs/installation/) | [Releases](https://github.com/volcano-sh/volcano/releases)

**3. Gateway API** - For service routing

```bash
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.4.1/standard-install.yaml
```

> Source: [Gateway API Installation Guide](https://gateway-api.sigs.k8s.io/guides/#installing-gateway-api) | [Releases](https://github.com/kubernetes-sigs/gateway-api/releases)

**4. Gateway API Inference Extension** - For intelligent inference request routing

```bash
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/releases/download/v1.2.1/manifests.yaml
```

> Source: [Inference Extension Docs](https://gateway-api-inference-extension.sigs.k8s.io/) | [Releases](https://github.com/kubernetes-sigs/gateway-api-inference-extension/releases)

### Install the Gateway

Set the Kgateway version and install the Kgateway CRDs:

```bash
KGTW_VERSION=v2.1.0
helm upgrade -i --create-namespace --namespace kgateway-system --version $KGTW_VERSION kgateway-crds oci://cr.kgateway.dev/kgateway-dev/charts/kgateway-crds
```

Install Kgateway:

```bash
helm upgrade -i --namespace kgateway-system --version $KGTW_VERSION kgateway oci://cr.kgateway.dev/kgateway-dev/charts/kgateway --set inferenceExtension.enabled=true
```

Deploy the Inference Gateway:

```bash
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api-inference-extension/raw/main/config/manifests/gateway/kgateway/gateway.yaml
```

### Quick Start (Local Development)

```bash
# 1. Create a kind cluster (optional)
kind create cluster --name fusioninfer

# 2. Install dependencies (see above)

# 3. Install FusionInfer CRDs
make install

# 4. Run the controller locally
make run
```

### Deploy to Cluster

**Build and push your image:**

```bash
make docker-build docker-push IMG=<some-registry>/fusioninfer:tag
```

**Deploy the controller:**

```bash
make deploy IMG=<some-registry>/fusioninfer:tag
```

**Apply sample resources:**

```bash
kubectl apply -k config/samples/
```

### Uninstall

```bash
# Delete sample resources
kubectl delete -k config/samples/

# Uninstall CRDs
make uninstall

# Undeploy controller
make undeploy
```

## Usage Examples

### Monolithic LLM Service

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
              image: vllm/vllm-openai:v0.11.0
              args: ["--model", "Qwen/Qwen3-8B"]
              resources:
                limits:
                  nvidia.com/gpu: "1"
```

### PD Disaggregated Service

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: qwen-pd-service
spec:
  roles:
    - name: router
      componentType: router
      strategy: pd-disaggregation
      httproute:
        parentRefs:
          - name: inference-gateway
    - name: prefill
      componentType: prefiller
      replicas: 2
      template:
        spec:
          containers:
            - name: vllm
              image: vllm/vllm-openai:v0.11.0
              args:
                - "--model"
                - "Qwen/Qwen3-8B"
                - "--kv-transfer-config"
                - '{"kv_connector":"PyNcclConnector","kv_role":"kv_producer"}'
    - name: decode
      componentType: decoder
      replicas: 4
      template:
        spec:
          containers:
            - name: vllm
              image: vllm/vllm-openai:v0.11.0
              args:
                - "--model"
                - "Qwen/Qwen3-8B"
                - "--kv-transfer-config"
                - '{"kv_connector":"PyNcclConnector","kv_role":"kv_consumer"}'
```

### Multi-Node Inference

```yaml
apiVersion: fusioninfer.io/v1alpha1
kind: InferenceService
metadata:
  name: deepseek-r1
spec:
  roles:
    - name: inference
      componentType: worker
      replicas: 2
      multinode:
        nodeCount: 4
      template:
        spec:
          containers:
            - name: vllm
              image: vllm/vllm-openai:v0.11.0
              args:
                - "--model"
                - "deepseek-ai/DeepSeek-R1"
                - "--tensor-parallel-size"
                - "32"
              resources:
                limits:
                  nvidia.com/gpu: "8"
```

## Send Request

```bash
# You can use minikube tunnel to assign IP address to an LoadBalancer Type Service
GATEWAY_IP=$(kubectl get gateway inference-gateway -o jsonpath='{.status.addresses[0].value}')

curl -X POST "http://${GATEWAY_IP}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "Qwen/Qwen3-8B",
    "messages": [
      {"role": "user", "content": "Hello, how are you?"}
    ]
  }'

```