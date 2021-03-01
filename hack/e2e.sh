#!/bin/bash

# Copyright (c) 2020 VMware, Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

################################################################################
# usage: e2e
#  This program deploys a local test environment using Kind and the Cluster API
#  provider for Docker (CAPD).
################################################################################

set -o errexit  # Exits immediately on unexpected errors (does not bypass traps)
set -o nounset  # Errors if variables are used without first being defined
set -o pipefail # Non-zero exit codes in piped commands causes pipeline to fail
                # with that code

# Change directories to the parent directory of the one in which this script is
# located.
cd "$(dirname "${BASH_SOURCE[0]}")/.."

export KIND_EXPERIMENTAL_DOCKER_NETWORK="bridge" # required for CAPD

################################################################################
##                                  usage
################################################################################

usage="$(
  cat <<EOF
usage: ${0} [FLAGS]
  Deploys a local test environment for the Cross-cluster Connectivity using
  Kind and the Cluster API provider for Docker (CAPD).
FLAGS
  -h    show this help and exit
  -u    deploy one kind cluster as the CAPI management cluster, then two CAPD
        clusters ontop.
  -d    destroy CAPD and kind clusters.
Globals
  KIND_MANAGEMENT_CLUSTER
        name of the kind management cluster. default: management
  SKIP_CAPD_IMAGE_LOAD
        skip loading CAPD manager docker image.
  SKIP_CLEANUP_CAPD_CLUSTERS
        skip cleaning up CAPD clusters.
  SKIP_CLEANUP_MGMT_CLUSTER
        skip cleaning up CAPI kind management cluster.
Examples
  Create e2e environment from existing kind cluster with name "my-kind"
        KIND_MANAGEMENT_CLUSTER="my-kind" bash hack/e2e.sh -u
  Destroys all CAPD clusters but not the kind management cluster
        SKIP_CLEANUP_MGMT_CLUSTER="1" bash hack/e2e.sh -d
EOF
)"

################################################################################
##                                   args
################################################################################

KIND_MANAGEMENT_CLUSTER="${KIND_MANAGEMENT_CLUSTER:-management}"
CAPI_VERSION="v0.3.7"
# The capd manager docker image name and tag. These have to be consistent with
# what're used in capd default kustomize manifest.
CAPD_IMAGE="gcr.io/k8s-staging-cluster-api/capd-manager:${CAPI_VERSION}"
CAPD_DEFAULT_IMAGE="gcr.io/k8s-staging-cluster-api/capd-manager:dev"
# Runtime setup
# By default do not skip anything.
SKIP_CAPD_IMAGE_LOAD="${SKIP_CAPD_IMAGE_LOAD:-}"
SKIP_CLEANUP_CAPD_CLUSTERS="${SKIP_CLEANUP_CAPD_CLUSTERS:-}"
SKIP_CLEANUP_MGMT_CLUSTER="${SKIP_CLEANUP_MGMT_CLUSTER:-}"
DEFAULT_IP_ADDR="${DEFAULT_IP_ADDR:-}"
USE_HOST_IP_ADDR="${USE_HOST_IP_ADDR:-}"

ROOT_DIR="${PWD}"
SCRIPTS_DIR="${ROOT_DIR}/hack"

IMAGE_REGISTRY="${IMAGE_REGISTRY:-gcr.io/tanzu-xcc}"
IMAGE_TAG="${IMAGE_TAG:-dev}"

DNS_SERVER_IMAGE=${IMAGE_REGISTRY}/dns-server:${IMAGE_TAG}
CAPI_DNS_CONTROLLER_IMAGE=${IMAGE_REGISTRY}/capi-dns-controller:${IMAGE_TAG}
DNS_CONFIG_PATCHER_IMAGE=${IMAGE_REGISTRY}/dns-config-patcher:${IMAGE_TAG}

DOCKERHUB_PROXY="${DOCKERHUB_PROXY:-docker.io}"

CLUSTER_A="cluster-a"
CLUSTER_B="cluster-b"

################################################################################
##                                  require
################################################################################

function check_dependencies() {
  # Ensure Kind 0.7.0+ is available.
  command -v kind >/dev/null 2>&1 || fatal "kind 0.7.0+ is required"
  if [[ 10#"$(kind --version 2>&1 | awk '{print $3}' | tr -d '.' | cut -d '-' -f1)" -lt 10#070 ]]; then
    echo "kind 0.7.0+ is required" && exit 1
  fi

  # Ensure jq 1.3+ is available.
  command -v jq >/dev/null 2>&1 || fatal "jq 1.3+ is required"
  local jq_version="$(jq --version 2>&1 | awk -F- '{print $2}' | tr -d '.')"
  if [[ "${jq_version}" != "master" && "${jq_version}" -lt 13 ]]; then
    echo "jq 1.3+ is required" && exit 1
  fi

  # Require kustomize, kubectl, clusterctl as well.
  command -v kubectl >/dev/null 2>&1 || fatal "kubectl is required"
  command -v clusterctl >/dev/null 2>&1 || fatal "clusterctl is required"
  command -v kustomize >/dev/null 2>&1 || fatal "kustomize is required"
  command -v docker >/dev/null 2>&1 || fatal "docker is required"
}

################################################################################
##                                   funcs
################################################################################

# error stores exit code, writes arguments to STDERR, and returns stored exit code
# fatal is like error except it will exit program if exit code >0
function error() {
  local exit_code="${?}"
  echo "${@}" 1>&2
  return "${exit_code}"
}

function fatal() { error "${@}" || exit "${?}"; }

# base64d decodes STDIN from base64
# safe for linux and darwin
function base64d() { base64 -D 2>/dev/null || base64 -d; }

# base64e encodes STDIN to base64
# safe for linux and darwin
function base64e() { base64 -w0 2>/dev/null || base64; }

# kubectl_mgc executes kubectl against management cluster.
function kubectl_mgc() { kubectl --context "kind-${KIND_MANAGEMENT_CLUSTER}" "${@}"; }

function setup_management_cluster() {
  # Create the management cluster.
  if ! kind get clusters 2>/dev/null | grep -q "${KIND_MANAGEMENT_CLUSTER}"; then
    echo "creating kind management cluster ${KIND_MANAGEMENT_CLUSTER}"
    kind create cluster \
      --config "${SCRIPTS_DIR}/kind/kind-cluster-with-extramounts.yaml" \
      --name "${KIND_MANAGEMENT_CLUSTER}"
  fi

  clusterctl init \
    --kubeconfig-context "kind-${KIND_MANAGEMENT_CLUSTER}"

  kubectl_mgc wait deployment/capi-controller-manager \
    -n capi-system \
    --for=condition=Available --timeout=300s
  kubectl_mgc wait deployment/capi-kubeadm-bootstrap-controller-manager \
    -n capi-kubeadm-bootstrap-system \
    --for=condition=Available --timeout=300s
  kubectl_mgc wait deployment/capi-kubeadm-control-plane-controller-manager \
    -n capi-kubeadm-control-plane-system \
    --for=condition=Available --timeout=300s

  kustomize build "https://github.com/kubernetes-sigs/cluster-api/test/infrastructure/docker/config/?ref=${CAPI_VERSION}" |
    sed 's~'"${CAPD_DEFAULT_IMAGE}"'~'"${CAPD_IMAGE}"'~g' |
    kubectl_mgc apply -f -

  kubectl_mgc wait deployment/capd-controller-manager \
    -n capd-system \
    --for=condition=Available --timeout=300s

  kind load docker-image "${CAPI_DNS_CONTROLLER_IMAGE}" --name "${KIND_MANAGEMENT_CLUSTER}"
  kubectl_mgc apply -f "./manifests/crds/connectivity.tanzu.vmware.com_gatewaydns.yaml"
  kubectl_mgc apply -f "./manifests/capi-dns-controller/deployment.yaml"

  local kubeconfig_path="${ROOT_DIR}/${KIND_MANAGEMENT_CLUSTER}.kubeconfig"
  kind get kubeconfig --name management > "${kubeconfig_path}"
}

function msg() {
  echo
  echo "### ${1} ###"
  echo
}

function create_cluster() {
  local simple_cluster_yaml="./hack/kind/simple-cluster.yaml"
  local namespace="${1}"
  local clustername="${2}"

  msg "Creating cluster resource for ${clustername} on management cluster in namespace ${namespace}"
  # Create per cluster deployment manifest, replace the original resource
  # names with our desired names, parameterized by clustername.
  if ! kind get clusters 2>/dev/null | grep -q "${clustername}"; then
    cat ${simple_cluster_yaml} |
      sed -e 's~my-cluster~'"${clustername}"'~g' \
	-e 's~controlplane-0~'"${clustername}"'-controlplane-0~g' \
	-e 's~worker-0~'"${clustername}"'-worker-0~g' |
      kubectl_mgc apply -n "${namespace}" -f -
  else
    echo "kind cluster ${clustername} already exists"
  fi
}

function wait_and_patch_kubeconfig() {
  local namespace="${1}"
  local clustername="${2}"

  msg "Waiting for kubeconfig secret for ${clustername} to patch"
  while ! kubectl_mgc -n "${namespace}" get secret "${clustername}"-kubeconfig; do
    sleep 5s
  done

  local kubeconfig_path="${ROOT_DIR}/${clustername}.kubeconfig"
  kubectl_mgc -n "${namespace}" get secret "${clustername}"-kubeconfig -o json | \
    jq -cr '.data.value' | \
    base64d >"${clustername}.kubeconfig"

  # Get the API server port for the cluster.
  local api_server_port
  api_server_port="$(docker port "${clustername}"-lb 6443/tcp | cut -d ':' -f 2)"

  # We need to patch the kubeconfig fetched from CAPD:
  #   1. replace the cluster IP with host IP, 6443 with LB POD port;
  #   2. disable SSL by removing the CA and setting insecure to true;
  # Note: we're assuming the lb pod is ready at this moment. If it's not,
  # we'll error out because of the script global settings.
  kubectl --kubeconfig="${clustername}.kubeconfig" config \
    set clusters."${clustername}".server "https://127.0.0.1:${api_server_port}"
  kubectl --kubeconfig="${clustername}.kubeconfig" config \
    unset clusters."${clustername}".certificate-authority-data
  kubectl --kubeconfig="${clustername}.kubeconfig" config \
    set clusters."${clustername}".insecure-skip-tls-verify true
}

function wait_for_lb_and_control_plane() {
  local clustername="${1}"

  msg "Waiting for cluster lb and control plane to be ready for ${clustername}"
  while ! kubectl --kubeconfig "${clustername}.kubeconfig" get nodes; do
    sleep 5s
  done
}

function wait_for_ready_nodes() {
  local clustername="${1}"

  msg "Waiting for ${clustername} nodes to be Ready"
  for node in $(kubectl --kubeconfig "${clustername}.kubeconfig" get nodes -o json | jq -cr '.items[].metadata.name'); do
    kubectl --kubeconfig "${clustername}.kubeconfig" wait \
    --for=condition=Ready --timeout=300s node/"${node}"
  done
}

function wait_for_external_ips() {
  local namespace="${1}"
  local clustername="${2}"

  msg "Waiting for every machine to have an ExternalIP in its status for ${clustername}"
  local machines="$(kubectl_mgc get machine -n "${namespace}" \
    -l "cluster.x-k8s.io/cluster-name=${clustername}" \
    -o json | jq -cr '.items[].metadata.name')"
  for machine in $machines; do
    while [[ -z "$(kubectl_mgc get machine "${machine}" -n "${namespace}" -o json -o=jsonpath='{.status.addresses[?(@.type=="ExternalIP")].address}')" ]]; do
      sleep 5s;
    done
  done
}

function install_cert_manager_and_metatallb() {
  local clustername="${1}"

  msg "Deploying cert-manager on cluster ${clustername}"
  kubectl --kubeconfig "${clustername}.kubeconfig" apply -f https://github.com/jetstack/cert-manager/releases/download/v1.0.3/cert-manager.yaml

  msg "Deploying metallb on cluster ${clustername}"
  update_image_repo_and_kubectl_apply "${clustername}" https://raw.githubusercontent.com/metallb/metallb/v0.9.5/manifests/namespace.yaml
  update_image_repo_and_kubectl_apply "${clustername}" https://raw.githubusercontent.com/metallb/metallb/v0.9.5/manifests/metallb.yaml
  kubectl --kubeconfig "${clustername}.kubeconfig" create secret generic -n metallb-system memberlist --from-literal=secretkey="$(openssl rand -base64 128)"
  # Each cluster needs to give metallb a config with a CIDR for services it exposes.
  # The clusters are sharing the address space, so they need non-overalpping
  # ranges assigned to each cluster.
  kubectl --kubeconfig "${clustername}.kubeconfig" apply -f "${SCRIPTS_DIR}/metallb/config-${clustername}.yaml"
}

function e2e_up() {
  check_dependencies
  setup_management_cluster
  kubectl_mgc create namespace dev-team

  for cluster in ${CLUSTER_A} ${CLUSTER_B}; do
    create_cluster "dev-team" "${cluster}"
  done

  for cluster in ${CLUSTER_A} ${CLUSTER_B}; do
    wait_and_patch_kubeconfig "dev-team" "${cluster}"
    wait_for_lb_and_control_plane "${cluster}"
    msg "Installing calico on ${cluster}"
    update_image_repo_and_kubectl_apply "${cluster}" https://docs.projectcalico.org/v3.8/manifests/calico.yaml
  done


  for cluster in ${CLUSTER_A} ${CLUSTER_B}; do
    wait_for_ready_nodes "${cluster}"
    wait_for_external_ips "dev-team" "${cluster}"
    install_cert_manager_and_metatallb "${cluster}"
    msg "Loading docker images on ${cluster}"
    kind load docker-image "${DNS_SERVER_IMAGE}" --name "${cluster}"
    kind load docker-image "${DNS_CONFIG_PATCHER_IMAGE}" --name "${cluster}"
    msg "Installing multi-cluster DNS on ${cluster}"
    kubectl --kubeconfig "${cluster}.kubeconfig" apply -f manifests/dns-server/
    kubectl --kubeconfig "${cluster}.kubeconfig" apply -f manifests/dns-config-patcher/
  done

  for cluster in ${CLUSTER_A} ${CLUSTER_B}; do
    msg "Installing Contour on ${cluster}"
    cat manifests/contour/*.yaml | \
      sed "s~image: docker.io/\(.*\)$~image: ${DOCKERHUB_PROXY}/\1~" | \
      kubectl --kubeconfig "${cluster}.kubeconfig" apply -f -
    kubectl_mgc -n dev-team label cluster "${cluster}" hasContour=true --overwrite
  done

  for cluster in ${CLUSTER_A} ${CLUSTER_B}; do
    cat <<EOF
################################################################################
cluster artifacts:
  name: ${cluster}
  manifest: ${ROOT_DIR}/${cluster}.yaml
  kubeconfig: ${ROOT_DIR}/${cluster}.kubeconfig
EOF
  done
}

function e2e_down() {
  # clean up CAPD clusters
  if [[ -z "${SKIP_CLEANUP_CAPD_CLUSTERS}" ]]; then
    # our management cluster has to be available to cleanup CAPD
    # clusters.
    for cluster in ${CLUSTER_A} ${CLUSTER_B}; do
      # ignore status
      kind delete cluster --name "${cluster}" ||
        echo "cluster ${cluster} deleted."
      rm -fv "${ROOT_DIR}/${cluster}".kubeconfig* 2>&1 ||
        echo "${ROOT_DIR}/${cluster}.kubeconfig* deleted"
    done
  fi
  # clean up kind cluster
  if [[ -z "${SKIP_CLEANUP_MGMT_CLUSTER}" ]]; then
    # ignore status
    kind delete cluster --name "${KIND_MANAGEMENT_CLUSTER}" ||
      echo "kind cluster ${KIND_MANAGEMENT_CLUSTER} deleted."
  fi
  return 0
}

function update_image_repo_and_kubectl_apply() {
  cluster=$1
  resourceURL=$2
  curl -L "${resourceURL}"| \
    sed "s~image: \(.*\)$~image: ${DOCKERHUB_PROXY}/\1~" | \
    kubectl --kubeconfig "${cluster}.kubeconfig" apply -f -
}

################################################################################
##                                   main
################################################################################

# Parse the command-line arguments.
while getopts ":hud" opt; do
  case ${opt} in
    h)
      error "${usage}" && exit 1
      ;;
    u)
      e2e_up
      exit 0
      ;;
    d)
      e2e_down
      exit 0
      ;;
    \?)
      error "invalid option: -${OPTARG} ${usage}" && exit 1
      ;;
  esac
done

error "${usage}" && exit 1
