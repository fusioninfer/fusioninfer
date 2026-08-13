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

package controller

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	modelAPIVersion = "fusioninfer.io/v1alpha1"
	fullRevision    = "0123456789abcdef0123456789abcdef01234567"
	modelDigest     = "sha256:7f3c9a1e4b2d8f605a7c3e9d1b4f2860c5a8e2d7f1b3096c4e8a5d2f7b1c903e"
	ociDigest       = "sha256:9d2e6b4a8f1c30573a7e9c2d5b608f14e1d4a7c3096b2f855c8e1a6d4f703b29"
)

func modelObject(kind, name, namespace string, spec map[string]any) *unstructured.Unstructured {
	object := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": modelAPIVersion,
			"kind":       kind,
			"metadata": map[string]any{
				"name": name,
			},
			"spec": spec,
		},
	}
	object.SetNamespace(namespace)
	return object
}

func hfModelSpec() map[string]any {
	return map[string]any{
		"source": map[string]any{
			"uri":      "hf://Qwen/Qwen3-8B",
			"revision": fullRevision,
		},
	}
}

func expectInvalidModel(ctx context.Context, object *unstructured.Unstructured) {
	err := k8sClient.Create(ctx, object)
	Expect(err).To(HaveOccurred())
	Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected an Invalid error, got %v", err)
}

var _ = Describe("Model API contract", func() {
	ctx := context.Background()

	DescribeTable("accepts supported namespaced model sources",
		func(name string, spec map[string]any) {
			object := modelObject("Model", name, "default", spec)
			Expect(k8sClient.Create(ctx, object)).To(Succeed())
			DeferCleanup(func() {
				_ = k8sClient.Delete(ctx, object)
			})
		},
		Entry("Hugging Face", "model-valid-hf", hfModelSpec()),
		Entry("Hugging Face with credentials", "model-valid-hf-credentials", map[string]any{
			"source": map[string]any{
				"uri":            "hf://Qwen/Qwen3-8B",
				"revision":       fullRevision,
				"credentialsRef": map[string]any{"name": "huggingface-token"},
			},
		}),
		Entry("S3", "model-valid-s3", map[string]any{
			"source": map[string]any{
				"uri":    "s3://team-a-models/base/qwen3-8b",
				"digest": modelDigest,
			},
		}),
		Entry("OCI", "model-valid-oci", map[string]any{
			"source": map[string]any{
				"uri": "oci://registry.example.com/models/qwen3-8b@" + ociDigest,
			},
		}),
		Entry("PVC", "model-valid-pvc", map[string]any{
			"source": map[string]any{
				"uri":    "pvc://qwen3-weights/models/qwen3-8b",
				"digest": modelDigest,
			},
		}),
	)

	DescribeTable("rejects invalid namespaced model declarations",
		func(name string, spec map[string]any) {
			expectInvalidModel(ctx, modelObject("Model", name, "default", spec))
		},
		Entry("missing source", "model-missing-source", map[string]any{}),
		Entry("empty URI", "model-empty-uri", map[string]any{
			"source": map[string]any{"uri": ""},
		}),
		Entry("unsupported scheme", "model-unsupported-scheme", map[string]any{
			"source": map[string]any{"uri": "https://example.com/model"},
		}),
		Entry("uppercase scheme", "model-uppercase-scheme", map[string]any{
			"source": map[string]any{
				"uri":      "HF://Qwen/Qwen3-8B",
				"revision": fullRevision,
			},
		}),
		Entry("Hugging Face URI without owner", "model-hf-no-owner", map[string]any{
			"source": map[string]any{
				"uri":      "hf:///Qwen3-8B",
				"revision": fullRevision,
			},
		}),
		Entry("Hugging Face URI with whitespace", "model-hf-whitespace", map[string]any{
			"source": map[string]any{
				"uri":      "hf://Qwen Team/Qwen3-8B",
				"revision": fullRevision,
			},
		}),
		Entry("missing Hugging Face revision", "model-hf-no-revision", map[string]any{
			"source": map[string]any{"uri": "hf://Qwen/Qwen3-8B"},
		}),
		Entry("short Hugging Face revision", "model-hf-short-revision", map[string]any{
			"source": map[string]any{
				"uri":      "hf://Qwen/Qwen3-8B",
				"revision": "main",
			},
		}),
		Entry("missing S3 digest", "model-s3-no-digest", map[string]any{
			"source": map[string]any{"uri": "s3://team-a-models/base/qwen3-8b"},
		}),
		Entry("S3 URI without bucket", "model-s3-no-bucket", map[string]any{
			"source": map[string]any{
				"uri":    "s3:///base/qwen3-8b",
				"digest": modelDigest,
			},
		}),
		Entry("S3 URI with whitespace", "model-s3-whitespace", map[string]any{
			"source": map[string]any{
				"uri":    "s3://team a-models/base/qwen3-8b",
				"digest": modelDigest,
			},
		}),
		Entry("S3 URI without prefix", "model-s3-no-prefix", map[string]any{
			"source": map[string]any{
				"uri":    "s3://team-a-models",
				"digest": modelDigest,
			},
		}),
		Entry("malformed digest", "model-bad-digest", map[string]any{
			"source": map[string]any{
				"uri":    "s3://team-a-models/base/qwen3-8b",
				"digest": "sha256:ABC",
			},
		}),
		Entry("OCI URI without descriptor digest", "model-oci-no-descriptor", map[string]any{
			"source": map[string]any{"uri": "oci://registry.example.com/models/qwen3-8b"},
		}),
		Entry("OCI URI without artifact", "model-oci-no-artifact", map[string]any{
			"source": map[string]any{"uri": "oci://registry.example.com/@" + ociDigest},
		}),
		Entry("OCI URI with whitespace", "model-oci-whitespace", map[string]any{
			"source": map[string]any{"uri": "oci://registry.example.com/models/qwen 3@" + ociDigest},
		}),
		Entry("PVC URI without claim", "model-pvc-no-claim", map[string]any{
			"source": map[string]any{
				"uri":    "pvc:///models/qwen3-8b",
				"digest": modelDigest,
			},
		}),
		Entry("PVC URI with invalid claim name", "model-pvc-invalid-claim", map[string]any{
			"source": map[string]any{
				"uri":    "pvc://NotValid/models/qwen3-8b",
				"digest": modelDigest,
			},
		}),
		Entry("PVC URI with overlong claim name", "model-pvc-overlong-claim", map[string]any{
			"source": map[string]any{
				"uri":    "pvc://" + strings.Repeat("a", 254) + "/models/qwen3-8b",
				"digest": modelDigest,
			},
		}),
		Entry("PVC credentials", "model-pvc-credentials", map[string]any{
			"source": map[string]any{
				"uri":            "pvc://qwen3-weights/models/qwen3-8b",
				"digest":         modelDigest,
				"credentialsRef": map[string]any{"name": "not-allowed"},
			},
		}),
		Entry("empty credential name", "model-empty-credential", map[string]any{
			"source": map[string]any{
				"uri":            "hf://Qwen/Qwen3-8B",
				"revision":       fullRevision,
				"credentialsRef": map[string]any{"name": ""},
			},
		}),
		Entry("invalid credential name", "model-invalid-credential", map[string]any{
			"source": map[string]any{
				"uri":            "hf://Qwen/Qwen3-8B",
				"revision":       fullRevision,
				"credentialsRef": map[string]any{"name": "Not Valid"},
			},
		}),
		Entry("overlong credential name", "model-overlong-credential", map[string]any{
			"source": map[string]any{
				"uri":            "hf://Qwen/Qwen3-8B",
				"revision":       fullRevision,
				"credentialsRef": map[string]any{"name": strings.Repeat("a", 254)},
			},
		}),
		Entry("query string", "model-query", map[string]any{
			"source": map[string]any{
				"uri":      "hf://Qwen/Qwen3-8B?revision=main",
				"revision": fullRevision,
			},
		}),
		Entry("fragment", "model-fragment", map[string]any{
			"source": map[string]any{
				"uri":      "hf://Qwen/Qwen3-8B#weights",
				"revision": fullRevision,
			},
		}),
		Entry("dot path segment", "model-dot-segment", map[string]any{
			"source": map[string]any{
				"uri":      "hf://Qwen/../Qwen3-8B",
				"revision": fullRevision,
			},
		}),
		Entry("missing LoRA base model reference", "model-lora-no-ref", map[string]any{
			"source": hfModelSpec()["source"],
			"lora":   map[string]any{},
		}),
		Entry("invalid LoRA reference kind", "model-lora-kind", map[string]any{
			"source": hfModelSpec()["source"],
			"lora": map[string]any{
				"baseModelRef": map[string]any{
					"kind": "InferenceService",
					"name": "qwen3-8b",
				},
			},
		}),
		Entry("empty LoRA reference name", "model-lora-name", map[string]any{
			"source": hfModelSpec()["source"],
			"lora": map[string]any{
				"baseModelRef": map[string]any{
					"kind": "Model",
					"name": "",
				},
			},
		}),
		Entry("invalid LoRA reference name", "model-lora-invalid-name", map[string]any{
			"source": hfModelSpec()["source"],
			"lora": map[string]any{
				"baseModelRef": map[string]any{
					"kind": "Model",
					"name": "not/a/name",
				},
			},
		}),
		Entry("overlong LoRA reference name", "model-lora-long-name", map[string]any{
			"source": hfModelSpec()["source"],
			"lora": map[string]any{
				"baseModelRef": map[string]any{
					"kind": "Model",
					"name": strings.Repeat("a", 254),
				},
			},
		}),
	)

	DescribeTable("accepts namespaced LoRA references",
		func(name, kind string) {
			object := modelObject("Model", name, "default", map[string]any{
				"source": map[string]any{
					"uri":    "s3://team-a-models/adapters/qwen3-8b-finance",
					"digest": modelDigest,
				},
				"lora": map[string]any{
					"baseModelRef": map[string]any{
						"kind": kind,
						"name": "qwen3-8b",
					},
				},
			})
			Expect(k8sClient.Create(ctx, object)).To(Succeed())
			DeferCleanup(func() {
				_ = k8sClient.Delete(ctx, object)
			})
		},
		Entry("Model", "model-valid-lora-model-ref", "Model"),
		Entry("ClusterModel", "model-valid-lora-cluster-ref", "ClusterModel"),
	)

	It("enforces cluster-scoped source and LoRA reference rules", func() {
		base := modelObject("ClusterModel", "cluster-model-valid", "", hfModelSpec())
		Expect(k8sClient.Create(ctx, base)).To(Succeed())
		DeferCleanup(func() {
			_ = k8sClient.Delete(ctx, base)
		})

		lora := modelObject("ClusterModel", "cluster-model-valid-lora", "", map[string]any{
			"source": map[string]any{
				"uri":    "s3://team-a-models/adapters/qwen3-8b-finance",
				"digest": modelDigest,
			},
			"lora": map[string]any{
				"baseModelRef": map[string]any{
					"kind": "ClusterModel",
					"name": "qwen3-8b",
				},
			},
		})
		Expect(k8sClient.Create(ctx, lora)).To(Succeed())
		DeferCleanup(func() {
			_ = k8sClient.Delete(ctx, lora)
		})

		expectInvalidModel(ctx, modelObject("ClusterModel", "cluster-model-pvc", "", map[string]any{
			"source": map[string]any{
				"uri":    "pvc://qwen3-weights/models/qwen3-8b",
				"digest": modelDigest,
			},
		}))
		expectInvalidModel(ctx, modelObject("ClusterModel", "cluster-model-lora-model-ref", "", map[string]any{
			"source": hfModelSpec()["source"],
			"lora": map[string]any{
				"baseModelRef": map[string]any{
					"kind": "Model",
					"name": "qwen3-8b",
				},
			},
		}))
	})

	DescribeTable("makes the spec immutable while allowing metadata updates",
		func(kind, namespace string) {
			object := modelObject(kind, "immutable-"+strings.ToLower(kind), namespace, hfModelSpec())
			Expect(k8sClient.Create(ctx, object)).To(Succeed())
			DeferCleanup(func() {
				_ = k8sClient.Delete(ctx, object)
			})

			object.SetAnnotations(map[string]string{"fusioninfer.io/test": "metadata-update"})
			Expect(k8sClient.Update(ctx, object)).To(Succeed())

			Expect(unstructured.SetNestedField(
				object.Object,
				"89abcdef0123456789abcdef0123456789abcdef",
				"spec",
				"source",
				"revision",
			)).To(Succeed())
			err := k8sClient.Update(ctx, object)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected an Invalid error, got %v", err)
		},
		Entry("Model", "Model", "default"),
		Entry("ClusterModel", "ClusterModel", ""),
	)

	It("rejects an empty spec from an unstructured client", func() {
		object := &unstructured.Unstructured{}
		object.SetAPIVersion(modelAPIVersion)
		object.SetKind("Model")
		object.SetName("model-no-spec")
		object.SetNamespace("default")

		expectInvalidModel(ctx, object)
	})
})
