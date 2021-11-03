#!/bin/bash
# Copyright (c) 2020 VMware, Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0


set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

MANAGEMENT_CLUSTER="${MANAGEMENT_CLUSTER:-management}"
CLUSTER_A="${CLUSTER_A:-cluster-a}"
CLUSTER_B="${CLUSTER_B:-cluster-b}"

function main() {
  kind delete cluster --name "${MANAGEMENT_CLUSTER}"
  kind delete cluster --name "${CLUSTER_A}"
  kind delete cluster --name "${CLUSTER_B}"
  rm -f "${ROOT}/${MANAGEMENT_CLUSTER}.kubeconfig"
  rm -f "${ROOT}/${CLUSTER_A}.kubeconfig"
  rm -f "${ROOT}/${CLUSTER_B}.kubeconfig"
}

main
