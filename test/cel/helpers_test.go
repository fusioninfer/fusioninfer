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
	"context"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// createObject creates object, fails the test if the API server rejects it, and deletes it when
// the test ends.
func createObject(t *testing.T, object client.Object) {
	t.Helper()
	if err := k8sClient.Create(t.Context(), object); err != nil {
		t.Fatalf("create %T %q: %v", object, object.GetName(), err)
	}
	// t.Context is canceled before cleanups run, so delete with a fresh context.
	t.Cleanup(func() {
		_ = k8sClient.Delete(context.Background(), object)
	})
}

// updateObject reads the stored object into latest, applies mutate, and writes it back. latest
// must be a new object with only its name and namespace set, because decoding does not clear the
// fields that the stored copy lacks.
func updateObject(ctx context.Context, latest client.Object, mutate func(client.Object)) error {
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(latest), latest); err != nil {
		return err
	}
	mutate(latest)
	return k8sClient.Update(ctx, latest)
}

// expectInvalid checks that the API server rejected the request as Invalid and mentioned message.
func expectInvalid(t *testing.T, err error, message string) {
	t.Helper()
	if !apierrors.IsInvalid(err) {
		t.Errorf("expected an Invalid error, got %v", err)
		return
	}
	if !strings.Contains(err.Error(), message) {
		t.Errorf("expected the error to contain %q, got %v", message, err)
	}
}
