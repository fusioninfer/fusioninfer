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

package cel

import (
	"fmt"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/core/v1alpha1"
)

const (
	// Valid sources that the test objects start from.
	hfModelURI   = "hf://Qwen/Qwen3-8B"
	s3AdapterURI = "s3://team-a-models/adapters/qwen3-8b-finance"
	ociModelURI  = "oci://registry.example.com/models/qwen3-8b"
	commitSHA    = "b968826d9c46dd6066d109eabc6255188de91218"
	ociDigest    = "sha256:9d2e6b4a8f1c30573a7e9c2d5b608f14e1d4a7c3096b2f855c8e1a6d4f703b29"

	// Messages of the URI validation rules; the tests match them as substrings of the API error.
	msgScheme      = "uri must use a supported lowercase scheme"
	msgCharacters  = "uri must use valid URI characters"
	msgQuery       = "uri must not contain a query string or fragment"
	msgDotSegments = "uri must not contain dot path segments"
	msgPercent     = "uri must not percent-encode dots or slashes"
	msgHF          = "hf uri must be hf://<owner>/<repo>"
	msgS3          = "s3 uri must be s3://<bucket>/<prefix>"
	msgOCI         = "oci uri must be oci://<registry>/<repository>"
)

// modelObject returns a Model in the default namespace or a cluster-scoped ClusterModel.
func modelObject(kind, name string, spec fusioninferiov1alpha1.ModelSpec) client.Object {
	switch kind {
	case "Model":
		return &fusioninferiov1alpha1.Model{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       spec,
		}
	case "ClusterModel":
		return &fusioninferiov1alpha1.ClusterModel{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       spec,
		}
	default:
		panic(fmt.Sprintf("unknown kind %q", kind))
	}
}

// specOf returns the spec of a Model or ClusterModel, which share ModelSpec.
func specOf(object client.Object) *fusioninferiov1alpha1.ModelSpec {
	switch object := object.(type) {
	case *fusioninferiov1alpha1.Model:
		return &object.Spec
	case *fusioninferiov1alpha1.ClusterModel:
		return &object.Spec
	default:
		panic(fmt.Sprintf("%T is not a Model or ClusterModel", object))
	}
}

// sourceSpec returns a spec that only sets source.uri.
func sourceSpec(uri string) fusioninferiov1alpha1.ModelSpec {
	return fusioninferiov1alpha1.ModelSpec{Source: fusioninferiov1alpha1.ModelSource{URI: uri}}
}

// credentialsSpec returns a spec with a valid URI that reads its credentials from the named Secret.
func credentialsSpec(secretName string) fusioninferiov1alpha1.ModelSpec {
	spec := sourceSpec(hfModelURI)
	spec.Source.CredentialsRef = &fusioninferiov1alpha1.SecretReference{Name: secretName}
	return spec
}

// prefetchSpec returns a spec with a valid URI and the given prefetch.
func prefetchSpec(prefetch fusioninferiov1alpha1.PrefetchSpec) fusioninferiov1alpha1.ModelSpec {
	spec := sourceSpec(hfModelURI)
	spec.Prefetch = &prefetch
	return spec
}

// loraSpec returns a LoRA adapter spec whose base model is the given kind and name.
func loraSpec(kind, name string) fusioninferiov1alpha1.ModelSpec {
	spec := sourceSpec(s3AdapterURI)
	spec.LoRA = &fusioninferiov1alpha1.LoRAArtifactSpec{
		BaseModelRef: fusioninferiov1alpha1.ModelReference{Kind: kind, Name: name},
	}
	return spec
}

// TestModelAcceptsSupportedSourceURIs checks that valid hf, s3 and oci URIs are accepted.
func TestModelAcceptsSupportedSourceURIs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc string
		name string
		uri  string
	}{
		{"Hugging Face without a revision", "uri-hf", hfModelURI},
		{"Hugging Face branch", "uri-hf-branch", hfModelURI + "@main"},
		{"Hugging Face commit SHA", "uri-hf-commit", hfModelURI + "@" + commitSHA},
		{"Hugging Face ref with slashes", "uri-hf-ref", hfModelURI + "@refs/pr/1"},
		{"S3", "uri-s3", "s3://team-a-models/base/qwen3-8b"},
		{"S3 key with an encoded space", "uri-s3-encoded", "s3://team-a-models/base/qwen3%208b"},
		{"OCI without a version", "uri-oci", ociModelURI},
		{"OCI tag", "uri-oci-tag", ociModelURI + ":v1"},
		{"OCI digest", "uri-oci-digest", ociModelURI + "@" + ociDigest},
		{"OCI registry with a port", "uri-oci-port", "oci://localhost:5000/qwen3-8b:v1"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			createObject(t, modelObject("Model", tt.name, sourceSpec(tt.uri)))
		})
	}
}

// TestModelRejectsInvalidSourceURIs checks that each URI rule rejects a malformed URI with its own message.
func TestModelRejectsInvalidSourceURIs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc    string
		name    string
		uri     string
		message string
	}{
		{"unsupported scheme", "bad-uri-https", "https://example.com/model", msgScheme},
		{"pvc scheme", "bad-uri-pvc", "pvc://qwen3-weights/models/qwen3-8b", msgScheme},
		{"uppercase scheme", "bad-uri-uppercase", "HF://Qwen/Qwen3-8B", msgScheme},
		{"whitespace", "bad-uri-space", "hf://Qwen Team/Qwen3-8B", msgCharacters},
		{"query string", "bad-uri-query", hfModelURI + "?revision=main", msgQuery},
		{"fragment", "bad-uri-fragment", hfModelURI + "#weights", msgQuery},
		{"dot path segment", "bad-uri-dots", "s3://team-a-models/base/../qwen3-8b", msgDotSegments},
		{"encoded dot segment", "bad-uri-encoded-dots", "s3://team-a-models/base/%2e%2e/qwen3-8b", msgPercent},
		{"encoded slash", "bad-uri-encoded-slash", "s3://team-a-models/base%2Fqwen3-8b", msgPercent},
		{"Hugging Face without owner", "bad-hf-owner", "hf:///Qwen3-8B", msgHF},
		{"Hugging Face with an extra path", "bad-hf-path", hfModelURI + "/extra", msgHF},
		{"Hugging Face with credentials", "bad-hf-credentials", "hf://user:token@Qwen/Qwen3-8B", msgHF},
		{"Hugging Face with an empty revision", "bad-hf-empty-revision", hfModelURI + "@", msgHF},
		{"Hugging Face revision with ..", "bad-hf-revision", hfModelURI + "@v1..v2", msgHF},
		{"S3 without bucket", "bad-s3-bucket", "s3:///base/qwen3-8b", msgS3},
		{"S3 without prefix", "bad-s3-prefix", "s3://team-a-models", msgS3},
		{"S3 with a trailing slash", "bad-s3-slash", "s3://team-a-models/base/qwen3-8b/", msgS3},
		{"S3 with credentials", "bad-s3-credentials", "s3://key:secret@team-a-models/base/qwen3-8b", msgS3},
		{"OCI without repository", "bad-oci-repository", "oci://registry.example.com", msgOCI},
		{"OCI with credentials", "bad-oci-credentials", "oci://user:pass@registry.example.com/qwen3-8b:v1", msgOCI},
		{"OCI uppercase repository", "bad-oci-uppercase", "oci://registry.example.com/Models/Qwen3-8B:v1", msgOCI},
		{"OCI tag and digest", "bad-oci-tag-digest", ociModelURI + ":v1@" + ociDigest, msgOCI},
		{"OCI short digest", "bad-oci-digest", ociModelURI + "@sha256:abc", msgOCI},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			object := modelObject("Model", tt.name, sourceSpec(tt.uri))
			expectInvalid(t, k8sClient.Create(t.Context(), object), tt.message)
		})
	}
}

// TestModelAcceptsCredentialsAndPrefetch checks the valid forms of credentialsRef and prefetch.
func TestModelAcceptsCredentialsAndPrefetch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc string
		name string
		spec fusioninferiov1alpha1.ModelSpec
	}{
		{"credentials", "spec-credentials", credentialsSpec("huggingface-token")},
		{"prefetch to every node", "spec-prefetch-all", prefetchSpec(fusioninferiov1alpha1.PrefetchSpec{})},
		{"prefetch by node selector", "spec-prefetch-selector", prefetchSpec(fusioninferiov1alpha1.PrefetchSpec{
			NodeSelector: map[string]string{"node.kubernetes.io/instance-type": "gpu-h100"},
		})},
		{"prefetch by node name", "spec-prefetch-node",
			prefetchSpec(fusioninferiov1alpha1.PrefetchSpec{NodeName: "gpu-node-1"})},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			createObject(t, modelObject("Model", tt.name, tt.spec))
		})
	}
}

// TestModelRejectsInvalidDeclarations checks the name patterns of credentialsRef, prefetch and lora,
// and the CEL rule that makes the prefetch targets mutually exclusive.
func TestModelRejectsInvalidDeclarations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc    string
		name    string
		spec    fusioninferiov1alpha1.ModelSpec
		message string
	}{
		{"invalid credential name", "bad-credentials-invalid",
			credentialsSpec("Not Valid"), "spec.source.credentialsRef.name"},
		{"prefetch with nodeSelector and nodeName", "bad-prefetch-both", prefetchSpec(fusioninferiov1alpha1.PrefetchSpec{
			NodeSelector: map[string]string{"node.kubernetes.io/instance-type": "gpu-h100"},
			NodeName:     "gpu-node-1",
		}), "nodeSelector and nodeName are mutually exclusive"},
		{"prefetch with an invalid nodeName", "bad-prefetch-node",
			prefetchSpec(fusioninferiov1alpha1.PrefetchSpec{NodeName: "Not Valid"}), "spec.prefetch.nodeName"},
		{"invalid LoRA reference name", "bad-lora-invalid",
			loraSpec("Model", "not/a/name"), "spec.lora.baseModelRef.name"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			object := modelObject("Model", tt.name, tt.spec)
			expectInvalid(t, k8sClient.Create(t.Context(), object), tt.message)
		})
	}
}

// TestModelAcceptsNamespacedLoRAReferences checks that a namespaced LoRA can reference a Model or a ClusterModel.
func TestModelAcceptsNamespacedLoRAReferences(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc string
		name string
		kind string
	}{
		{"Model", "lora-model-ref", "Model"},
		{"ClusterModel", "lora-cluster-ref", "ClusterModel"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			createObject(t, modelObject("Model", tt.name, loraSpec(tt.kind, "qwen3-8b")))
		})
	}
}

// TestClusterModelLoRAMustReferenceClusterModel checks that a cluster-scoped LoRA can only reference a ClusterModel.
func TestClusterModelLoRAMustReferenceClusterModel(t *testing.T) {
	t.Parallel()
	createObject(t, modelObject("ClusterModel", "cluster-lora-cluster-ref", loraSpec("ClusterModel", "qwen3-8b")))

	object := modelObject("ClusterModel", "cluster-lora-model-ref", loraSpec("Model", "qwen3-8b"))
	expectInvalid(t, k8sClient.Create(t.Context(), object), "cluster-scoped LoRA artifacts must reference a ClusterModel")
}

// TestModelUpdatesKeepURIAndLoRAImmutable checks which fields can change after creation:
// metadata, credentialsRef and prefetch can; uri and lora cannot.
func TestModelUpdatesKeepURIAndLoRAImmutable(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"Model", "ClusterModel"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			suffix := strings.ToLower(kind)
			base := "update-base-" + suffix
			createObject(t, modelObject(kind, base, sourceSpec(hfModelURI)))
			adapter := "update-lora-" + suffix
			createObject(t, modelObject(kind, adapter, loraSpec("ClusterModel", "qwen3-8b")))

			// Updates that must succeed, applied to base in order.
			allowed := []struct {
				desc   string
				mutate func(client.Object)
			}{
				{"metadata", func(object client.Object) {
					object.SetAnnotations(map[string]string{"fusioninfer.io/test": "metadata-update"})
				}},
				{"credentialsRef", func(object client.Object) {
					specOf(object).Source.CredentialsRef = &fusioninferiov1alpha1.SecretReference{Name: "huggingface-token"}
				}},
				{"prefetch", func(object client.Object) {
					specOf(object).Prefetch = &fusioninferiov1alpha1.PrefetchSpec{NodeName: "gpu-node-1"}
				}},
				{"prefetch removal", func(object client.Object) { specOf(object).Prefetch = nil }},
			}
			for _, update := range allowed {
				latest := modelObject(kind, base, fusioninferiov1alpha1.ModelSpec{})
				if err := updateObject(ctx, latest, update.mutate); err != nil {
					t.Errorf("update %s: %v", update.desc, err)
				}
			}

			// Updates that the immutability rules must reject.
			rejected := []struct {
				name    string
				mutate  func(client.Object)
				message string
			}{
				{base, func(object client.Object) { specOf(object).Source.URI = hfModelURI + "@main" }, "uri is immutable"},
				{base, func(object client.Object) { specOf(object).LoRA = loraSpec("ClusterModel", "qwen3-8b").LoRA },
					"lora cannot be added or removed"},
				{adapter, func(object client.Object) { specOf(object).LoRA.BaseModelRef.Name = "qwen3-14b" },
					"lora is immutable"},
				{adapter, func(object client.Object) { specOf(object).LoRA = nil }, "lora cannot be added or removed"},
			}
			for _, update := range rejected {
				latest := modelObject(kind, update.name, fusioninferiov1alpha1.ModelSpec{})
				expectInvalid(t, updateObject(ctx, latest, update.mutate), update.message)
			}
		})
	}
}
