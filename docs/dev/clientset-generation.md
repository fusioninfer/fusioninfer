# Clientset Generation

FusionInfer uses `controller-runtime`'s `client.Client` for standard operations. For SharedInformers or integration with other Kubernetes components, use the generated typed clientset.

## Add New CRD to Clientset

To generate clientset for a new CRD type:

1. Add `+genclient` marker to your type:

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

2. Regenerate:

```bash
./hack/update-codegen.sh
```

Generated code will be placed in `client-go/`.

## Usage

Use the typed clientset for CRUD operations:

```go
import clientset "github.com/fusioninfer/fusioninfer/client-go/clientset/versioned"

config, _ := rest.InClusterConfig()
client, _ := clientset.NewForConfig(config)

// Get
obj, _ := client.CoreV1alpha1().InferenceServices("default").Get(ctx, "name", metav1.GetOptions{})

// Update status
client.CoreV1alpha1().InferenceServices("default").UpdateStatus(ctx, obj, metav1.UpdateOptions{})

// List with label selector
list, _ := client.CoreV1alpha1().InferenceServices("").List(ctx, metav1.ListOptions{
    LabelSelector: "app=inference",
})
```
