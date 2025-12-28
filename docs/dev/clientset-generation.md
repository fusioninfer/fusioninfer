# Clientset Generation for Kubebuilder Projects

> **Note**: This is optional. FusionInfer currently uses `controller-runtime`'s `client.Client` which is sufficient for most use cases. Only generate clientset if you need SharedInformers or deep integration with other k8s components.

## Prerequisites

Ensure `code-generator` version matches your `client-go` version.

## Steps

### 1. Add code-generator dependency

```bash
K8S_VERSION=v0.31.0
go get k8s.io/code-generator@$K8S_VERSION
go mod vendor
chmod +x vendor/k8s.io/code-generator/generate-groups.sh
```

### 2. Update API type definition

Add `+genclient` marker to your CRD type:

```go
// api/v1alpha1/inferenceservice_types.go

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type InferenceService struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec   InferenceServiceSpec   `json:"spec,omitempty"`
    Status InferenceServiceStatus `json:"status,omitempty"`
}
```

### 3. Create `hack/update-codegen.sh`

```bash
#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

MODULE=github.com/fusioninfer/fusioninfer
APIS_PKG=api
GROUP=fusioninfer.io
VERSION=v1alpha1
GROUP_VERSION=${GROUP}:${VERSION}
OUTPUT_PKG=client-go

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

bash "${CODEGEN_PKG}"/generate-groups.sh "client,lister,informer" \
  "${MODULE}/${OUTPUT_PKG}" "${MODULE}/${APIS_PKG}" \
  "${GROUP_VERSION}" \
  --go-header-file "${SCRIPT_ROOT}"/hack/boilerplate.go.txt \
  --output-base "${SCRIPT_ROOT}"
```

### 4. Create `hack/boilerplate.go.txt`

```go
/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
```

### 5. Run generation

```bash
chmod +x hack/update-codegen.sh
./hack/update-codegen.sh
```

## Generated Directory Structure

```
client-go/
├── clientset/
│   └── versioned/
│       ├── clientset.go
│       └── typed/
│           └── fusioninfer.io/
│               └── v1alpha1/
│                   ├── inferenceservice.go
│                   └── doc.go
├── informers/
│   └── externalversions/
│       └── fusioninfer.io/
│           └── v1alpha1/
│               └── inferenceservice.go
└── listers/
    └── fusioninfer.io/
        └── v1alpha1/
            └── inferenceservice.go
```

## Usage Example

```go
import (
    clientset "github.com/fusioninfer/fusioninfer/client-go/clientset/versioned"
)

// Create clientset
config, _ := rest.InClusterConfig()
client, _ := clientset.NewForConfig(config)

// Use typed client
inferSvc, err := client.FusioninferV1alpha1().
    InferenceServices("default").
    Get(ctx, "my-service", metav1.GetOptions{})

// Update status
client.FusioninferV1alpha1().
    InferenceServices("default").
    UpdateStatus(ctx, inferSvc, metav1.UpdateOptions{})
```

## Comparison: controller-runtime vs clientset

| Feature | controller-runtime | clientset |
|---------|-------------------|-----------|
| Style | Generic `client.Client` | Typed interface |
| Usage | `r.Get(ctx, key, &obj)` | `client.Get(ctx, name, opts)` |
| Return | Fills object via parameter | Returns object directly |
| Generation | Not needed | Requires `client-gen` |
| Best for | Simple CRUD, kubebuilder | SharedInformers, complex integration |

## When to Use Clientset

- Need SharedInformer for efficient watch/list
- Integration with other k8s components expecting clientset
- Need fine-grained control over API calls
- Building tools that work with multiple clusters

## When NOT to Use Clientset

- Simple controller logic (use controller-runtime)
- Already using kubebuilder patterns
- Want to minimize generated code

