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

IMAGE_REGISTRY="${IMAGE_REGISTRY:-ghcr.io/vmware-tanzu/cross-cluster-connectivity}"
IMAGE_TAG="${IMAGE_TAG:-dev}"

DNS_SERVER_IMAGE=${IMAGE_REGISTRY}/dns-server:${IMAGE_TAG}
CAPI_DNS_CONTROLLER_IMAGE=${IMAGE_REGISTRY}/capi-dns-controller:${IMAGE_TAG}

CLUSTER_A="cluster-a"
CLUSTER_B="cluster-b"
CLUSTER_A_KUBECONFIG="cluster-a.kubeconfig"
CLUSTER_B_KUBECONFIG="cluster-b.kubeconfig"

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
    --kubeconfig-context "kind-${KIND_MANAGEMENT_CLUSTER}" \
    --core "cluster-api:v0.3.11"

  # Enable the ClusterResourceSet feature in cluster API
  kubectl_mgc -n capi-webhook-system patch deployment capi-controller-manager \
    --type=strategic --patch="$(
      cat <<EOF
spec:
  template:
    spec:
      containers:
      - name: manager
        args:
        - --metrics-addr=127.0.0.1:8080
        - --webhook-port=9443
        - --feature-gates=ClusterResourceSet=true
EOF
    )"

  kubectl_mgc -n capi-system patch deployment capi-controller-manager \
    --type=strategic --patch="$(
      cat <<EOF
spec:
  template:
    spec:
      containers:
      - name: manager
        args:
        - --metrics-addr=127.0.0.1:8080
        - --feature-gates=ClusterResourceSet=true
EOF
    )"

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


function create_cluster() {
  local simple_cluster_yaml="./hack/kind/simple-cluster.yaml"
  local clustername="${1}"

  local kubeconfig_path="${ROOT_DIR}/${clustername}.kubeconfig"

  local ip_addr="127.0.0.1"

  # Create per cluster deployment manifest, replace the original resource
  # names with our desired names, parameterized by clustername.
  if ! kind get clusters 2>/dev/null | grep -q "${clustername}"; then
    cat ${simple_cluster_yaml} |
      sed -e 's~my-cluster~'"${clustername}"'~g' \
	-e 's~controlplane-0~'"${clustername}"'-controlplane-0~g' \
	-e 's~worker-0~'"${clustername}"'-worker-0~g' |
      kubectl_mgc apply -f -
    while ! kubectl_mgc -n default get secret "${clustername}"-kubeconfig; do
      sleep 5s
    done
  fi

  kubectl_mgc -n default get secret "${clustername}"-kubeconfig -o json | \
    jq -cr '.data.value' | \
    base64d >"${kubeconfig_path}"

  # Do not quote clusterkubectl when using it to allow for the correct
  # expansion.
  clusterkubectl="kubectl --kubeconfig=${kubeconfig_path}"

  # Get the API server port for the cluster.
  local api_server_port
  api_server_port="$(docker port "${clustername}"-lb 6443/tcp | cut -d ':' -f 2)"

  # We need to patch the kubeconfig fetched from CAPD:
  #   1. replace the cluster IP with host IP, 6443 with LB POD port;
  #   2. disable SSL by removing the CA and setting insecure to true;
  # Note: we're assuming the lb pod is ready at this moment. If it's not,
  # we'll error out because of the script global settings.
  ${clusterkubectl} config set clusters."${clustername}".server "https://${ip_addr}:${api_server_port}"
  ${clusterkubectl} config unset clusters."${clustername}".certificate-authority-data
  ${clusterkubectl} config set clusters."${clustername}".insecure-skip-tls-verify true

  # Ensure CAPD cluster lb and control plane being ready by querying nodes.
  while ! ${clusterkubectl} get nodes; do
    sleep 5s
  done

  # Deploy Calico cni into CAPD cluster.
  ${clusterkubectl} apply -f https://docs.projectcalico.org/v3.8/manifests/calico.yaml

  # Deploy cert-manager
  ${clusterkubectl} apply -f https://github.com/jetstack/cert-manager/releases/download/v1.0.3/cert-manager.yaml

  # Deploy metallb
  ${clusterkubectl} apply -f https://raw.githubusercontent.com/metallb/metallb/v0.9.5/manifests/namespace.yaml
  ${clusterkubectl} apply -f https://raw.githubusercontent.com/metallb/metallb/v0.9.5/manifests/metallb.yaml
  ${clusterkubectl} create secret generic -n metallb-system memberlist --from-literal=secretkey="$(openssl rand -base64 128)"
  # Each cluster needs to give metallb a config with a CIDR for services it exposes.
  # The clusters are sharing the address space, so they need non-overalpping
  # ranges assigned to each cluster.
  ${clusterkubectl} apply -f "${SCRIPTS_DIR}/metallb/config-${clustername}.yaml"

  # Wait until every node is in Ready condition.
  for node in $(${clusterkubectl} get nodes -o json | jq -cr '.items[].metadata.name'); do
    ${clusterkubectl} wait --for=condition=Ready --timeout=300s node/"${node}"
  done

  # Wait until every machine has ExternalIP in status
  local machines="$(kubectl_mgc get machine \
    -l "cluster.x-k8s.io/cluster-name=${clustername}" \
    -o json | jq -cr '.items[].metadata.name')"
  for machine in $machines; do
    while [[ -z "$(kubectl_mgc get machine "${machine}" -o json -o=jsonpath='{.status.addresses[?(@.type=="ExternalIP")].address}')" ]]; do
      sleep 5s;
    done
  done
  mkdir -p "${ROOT_DIR}/kubeconfig"
}

function create_resource_set_secret() {
  local name="${1}"
  local file="${2}"
  kubectl_mgc create secret generic "${name}" \
    -n default \
    --from-file="${file}" \
    --type=addons.cluster.x-k8s.io/resource-set \
    --dry-run --save-config -o yaml | kubectl_mgc apply -f -
}

function deploy_cluster_resource_set() {
  create_resource_set_secret "dns-server" "manifests/dns-server"

  cat <<EOF | kubectl_mgc apply -f -
apiVersion: addons.cluster.x-k8s.io/v1alpha3
kind: ClusterResourceSet
metadata:
  name: connectivity
  namespace: default
spec:
  strategy: ApplyOnce
  clusterSelector:
    matchExpressions:
    - key: cross-cluster-connectivity
      operator: Exists
  resources:
    - name: dns-server
      kind: Secret
EOF
}

function load_cluster_images() {
  kind load docker-image "${DNS_SERVER_IMAGE}" --name "${CLUSTER_A}"
  kind load docker-image "${DNS_SERVER_IMAGE}" --name "${CLUSTER_B}"
}

function patch_kube_system_coredns() {
  local kubeconfig="${1}"
  local dns_server_service_ip="$(kubectl get service \
    --kubeconfig ${kubeconfig} \
    -n capi-dns \
    dns-server -o=jsonpath='{.spec.clusterIP}')"

  kubectl patch configmap coredns \
    --kubeconfig "${kubeconfig}" \
    -n kube-system \
    --type=strategic --patch="$(
      cat <<EOF
data:
  Corefile: |
    .:53 {
        errors
        health {
           lameduck 5s
        }
        ready
        kubernetes cluster.local in-addr.arpa ip6.arpa {
           pods insecure
           fallthrough in-addr.arpa ip6.arpa
           ttl 30
        }
        prometheus :9153
        forward . /etc/resolv.conf
        cache 30
        loop
        reload
        loadbalance
    }
    xcc.test {
        forward . ${dns_server_service_ip}
        reload
    }
EOF
    )"
}

function e2e_up() {
  check_dependencies

  setup_management_cluster

  # Create two clusters
  create_cluster "${CLUSTER_A}"
  create_cluster "${CLUSTER_B}"

  # deploy addons for cluster-a
  kubectl --kubeconfig ${CLUSTER_A_KUBECONFIG} apply -f manifests/contour/

  # deploy addons for cluster-b
  kubectl --kubeconfig ${CLUSTER_B_KUBECONFIG} apply -f manifests/contour/

  # Label the clusters so we can install our stuff with ClusterResourceSet
  kubectl_mgc -n default label cluster "${CLUSTER_A}" cross-cluster-connectivity=true --overwrite
  kubectl_mgc -n default label cluster "${CLUSTER_B}" cross-cluster-connectivity=true --overwrite

  deploy_cluster_resource_set
  load_cluster_images

  for cluster in ${CLUSTER_A} ${CLUSTER_B}; do
    cat <<EOF
################################################################################
cluster artifacts:
  name: ${cluster}
  manifest: ${ROOT_DIR}/${cluster}.yaml
  kubeconfig: ${ROOT_DIR}/${cluster}.kubeconfig
EOF

  patch_kube_system_coredns "${CLUSTER_A_KUBECONFIG}"
  patch_kube_system_coredns "${CLUSTER_B_KUBECONFIG}"
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
