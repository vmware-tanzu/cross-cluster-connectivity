#!/bin/bash
# Copyright (c) 2020 VMware, Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0


set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

MANAGEMENT_CLUSTER="${MANAGEMENT_CLUSTER:-management}"
CLUSTER_A="${CLUSTER_A:-cluster-a}"
CLUSTER_B="${CLUSTER_B:-cluster-b}"

IMAGE_REGISTRY="${IMAGE_REGISTRY:-gcr.io/tanzu-xcc}"
IMAGE_TAG="${IMAGE_TAG:-dev}"
DOCKERHUB_PROXY="${DOCKERHUB_PROXY:-docker.io}"

DNS_SERVER_IMAGE=${IMAGE_REGISTRY}/dns-server:${IMAGE_TAG}
XCC_DNS_CONTROLLER_IMAGE=${IMAGE_REGISTRY}/xcc-dns-controller:${IMAGE_TAG}
DNS_CONFIG_PATCHER_IMAGE=${IMAGE_REGISTRY}/dns-config-patcher:${IMAGE_TAG}

function main() {
  check_dependencies
  check_local_images
  setup_clusters
  install_xcc

  wait_for_all_deployments_ready
}

function check_dependencies() {
  command -v jq >/dev/null 2>&1 || fatal "jq is required"
  command -v kind >/dev/null 2>&1 || fatal "kind is required"
  command -v kubectl >/dev/null 2>&1 || fatal "kubectl is required"
  command -v clusterctl >/dev/null 2>&1 || fatal "clusterctl is required"
  command -v kustomize >/dev/null 2>&1 || fatal "kustomize is required"
  command -v docker >/dev/null 2>&1 || fatal "docker is required"
}

function check_local_images() {
  local images
  local tip_msg=$'\nplease, build the images using make build-images'

  images="$(docker images --format="{{.Repository}}:{{.Tag}}")"

  echo "${images}" | grep -q "${XCC_DNS_CONTROLLER_IMAGE}" || fatal "cannot find local image: ${XCC_DNS_CONTROLLER_IMAGE} ${tip_msg}"
  echo "${images}" | grep -q "${DNS_SERVER_IMAGE}" || fatal "cannot find local image: ${DNS_SERVER_IMAGE} ${tip_msg}"
  echo "${images}" | grep -q "${DNS_CONFIG_PATCHER_IMAGE}" || fatal "cannot find local image: ${DNS_CONFIG_PATCHER_IMAGE} ${tip_msg}"
}

function setup_clusters() {
  echo "Creating management cluster: ${MANAGEMENT_CLUSTER}..."
  kind create cluster --name "${MANAGEMENT_CLUSTER}" --config "${ROOT}/hack/kind/kind-cluster-with-extramounts.yaml"
  kind get kubeconfig --name "${MANAGEMENT_CLUSTER}" > "${MANAGEMENT_CLUSTER}.kubeconfig"

  echo "Init cluster-api..."
  clusterctl init --infrastructure docker --kubeconfig-context "kind-${MANAGEMENT_CLUSTER}" --wait-providers

  echo "Creating workload clusters: ${CLUSTER_A}, ${CLUSTER_B}..."
  kubectl --kubeconfig "${MANAGEMENT_CLUSTER}.kubeconfig" create namespace "dev-team"
  clusterctl generate cluster "${CLUSTER_A}" -n dev-team --kubernetes-version v1.22.0 --flavor development --worker-machine-count 1 | kubectl apply -f -
  clusterctl generate cluster "${CLUSTER_B}" -n dev-team --kubernetes-version v1.22.0 --flavor development --worker-machine-count 1 | kubectl apply -f -

  echo "Waiting for workload clusters to be ready..."
  kubectl wait --timeout=5m --for=condition=Ready=True --all -n dev-team clusters

  clusterctl -n dev-team get kubeconfig "${CLUSTER_A}" > "${CLUSTER_A}.kubeconfig"
  clusterctl -n dev-team get kubeconfig "${CLUSTER_B}" > "${CLUSTER_B}.kubeconfig"

  echo "Installing Calico on workload clusters..."
  update_image_repo_and_kubectl_apply "${CLUSTER_A}" https://docs.projectcalico.org/v3.20/manifests/calico.yaml
  update_image_repo_and_kubectl_apply "${CLUSTER_B}" https://docs.projectcalico.org/v3.20/manifests/calico.yaml

  echo "Installing MetalLB on workload clusters..."
  kubectl --kubeconfig "${CLUSTER_A}.kubeconfig" apply -f https://raw.githubusercontent.com/metallb/metallb/v0.11.0/manifests/namespace.yaml
  kubectl --kubeconfig "${CLUSTER_A}.kubeconfig" apply -f https://raw.githubusercontent.com/metallb/metallb/v0.11.0/manifests/metallb.yaml
  kubectl --kubeconfig "${CLUSTER_A}.kubeconfig" create secret generic -n metallb-system memberlist --from-literal=secretkey="$(openssl rand -base64 128)"
  kubectl --kubeconfig "${CLUSTER_A}.kubeconfig" apply -f "${ROOT}/hack/metallb/config-cluster-a.yaml"

  kubectl --kubeconfig "${CLUSTER_B}.kubeconfig" apply -f https://raw.githubusercontent.com/metallb/metallb/v0.11.0/manifests/namespace.yaml
  kubectl --kubeconfig "${CLUSTER_B}.kubeconfig" apply -f https://raw.githubusercontent.com/metallb/metallb/v0.11.0/manifests/metallb.yaml
  kubectl --kubeconfig "${CLUSTER_B}.kubeconfig" create secret generic -n metallb-system memberlist --from-literal=secretkey="$(openssl rand -base64 128)"
  kubectl --kubeconfig "${CLUSTER_B}.kubeconfig" apply -f "${ROOT}/hack/metallb/config-cluster-b.yaml"

  update_image_repo_and_kubectl_apply "${CLUSTER_A}" "https://projectcontour.io/quickstart/contour.yaml"
  kubectl --kubeconfig "${MANAGEMENT_CLUSTER}.kubeconfig" -n dev-team label cluster "${CLUSTER_A}" hasContour=true --overwrite

  update_image_repo_and_kubectl_apply "${CLUSTER_B}" "https://projectcontour.io/quickstart/contour.yaml"
  kubectl --kubeconfig "${MANAGEMENT_CLUSTER}.kubeconfig" -n dev-team label cluster "${CLUSTER_B}" hasContour=true --overwrite
}

function install_xcc() {
  echo "Install Cross-cluster-connectivity..."
  kind load docker-image "${XCC_DNS_CONTROLLER_IMAGE}" --name "${MANAGEMENT_CLUSTER}"
  kubectl --kubeconfig "${MANAGEMENT_CLUSTER}.kubeconfig" apply -f "${ROOT}/manifests/crds/connectivity.tanzu.vmware.com_gatewaydns.yaml"
  kubectl --kubeconfig "${MANAGEMENT_CLUSTER}.kubeconfig" apply -f "${ROOT}/manifests/xcc-dns-controller/deployment.yaml"

  kind load docker-image "${DNS_SERVER_IMAGE}" --name "${CLUSTER_A}"
  kind load docker-image "${DNS_CONFIG_PATCHER_IMAGE}" --name "${CLUSTER_A}"
  kubectl --kubeconfig "${CLUSTER_A}.kubeconfig" apply -f "${ROOT}/manifests/dns-server/"
  kubectl --kubeconfig "${CLUSTER_A}.kubeconfig" apply -f "${ROOT}/manifests/dns-config-patcher/"

  kind load docker-image "${DNS_SERVER_IMAGE}" --name "${CLUSTER_B}"
  kind load docker-image "${DNS_CONFIG_PATCHER_IMAGE}" --name "${CLUSTER_B}"
  kubectl --kubeconfig "${CLUSTER_B}.kubeconfig" apply -f "${ROOT}/manifests/dns-server/"
  kubectl --kubeconfig "${CLUSTER_B}.kubeconfig" apply -f "${ROOT}/manifests/dns-config-patcher/"
}

function wait_for_all_deployments_ready() {
  echo "Waiting for all xcc-dns deployments to be ready on ${MANAGEMENT_CLUSTER} cluster..."
  kubectl --kubeconfig "${MANAGEMENT_CLUSTER}.kubeconfig" wait --timeout=5m --for=condition=Available --all -n xcc-dns deployments
  echo "Waiting for all xcc-dns deployments to be ready on ${CLUSTER_A} cluster..."
  kubectl --kubeconfig "${CLUSTER_A}.kubeconfig" wait --timeout=1m --for=condition=Available --all -n xcc-dns deployments
  echo "Waiting for all xcc-dns deployments to be ready on ${CLUSTER_B} cluster..."
  kubectl --kubeconfig "${CLUSTER_B}.kubeconfig" wait --timeout=1m --for=condition=Available --all -n xcc-dns deployments
  echo "Deployments are ready!"
}

function fatal() {
  echo "${@}"
  exit 1
}

function update_image_repo_and_kubectl_apply() {
  local cluster="${1}"
  local resourceURL="${2}"
  curl -L "${resourceURL}"| \
    sed "s~image: docker.io/\(.*\)$~image: ${DOCKERHUB_PROXY}/\1~" | \
    kubectl --kubeconfig "${cluster}.kubeconfig" apply -f -
}

main
