#!/usr/bin/env bash

# Copyright 2025.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

cd "$(dirname "${0}")/.."
REPO_ROOT="$(pwd)"

# Get code-generator from go module cache
CODEGEN_VERSION=$(go list -m -f '{{.Version}}' k8s.io/code-generator)
CODEGEN_PKG=$(go env GOMODCACHE)/k8s.io/code-generator@${CODEGEN_VERSION}

source "${CODEGEN_PKG}/kube_codegen.sh"

# Workaround for code-generator directory resolution
mkdir -p github.com && ln -s ../.. github.com/fusioninfer
trap "rm -rf github.com" EXIT

# Generate deepcopy, defaulter, conversion functions
kube::codegen::gen_helpers github.com/fusioninfer/fusioninfer/api \
    --boilerplate "${REPO_ROOT}/hack/boilerplate.go.txt"

# Generate client, listers, informers and apply configurations
kube::codegen::gen_client github.com/fusioninfer/fusioninfer/api \
    --with-watch \
    --with-applyconfig \
    --output-dir "${REPO_ROOT}/client-go" \
    --output-pkg github.com/fusioninfer/fusioninfer/client-go \
    --boilerplate "${REPO_ROOT}/hack/boilerplate.go.txt"

echo "Clientset generated successfully in ${REPO_ROOT}/client-go"
