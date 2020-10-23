#!/usr/bin/env bash
# Copyright (c) 2020 VMware, Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0


set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./hack/tools/vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.
bash "${CODEGEN_PKG}"/generate-groups.sh "client,informer,lister" \
  github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated \
  github.com/vmware-tanzu/cross-cluster-connectivity/apis \
  connectivity:v1alpha1 \
  --go-header-file "${SCRIPT_ROOT}"/hack/boilerplate.go.txt \
  --output-base "${SCRIPT_ROOT}"

find pkg/generated/ -name "*.go" -delete
cp -a "${SCRIPT_ROOT}/github.com/vmware-tanzu/cross-cluster-connectivity/pkg" "${SCRIPT_ROOT}"
git clean -f "${SCRIPT_ROOT}/github.com"

# To use your own boilerplate text append:
#   --go-header-file "${SCRIPT_ROOT}"/hack/custom-boilerplate.go.txt

