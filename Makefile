
IMAGE_REGISTRY ?= ghcr.io/vmware-tanzu/cross-cluster-connectivity
IMAGE_TAG ?= dev

CONNECTIVITY_PUBLISHER_IMAGE := $(IMAGE_REGISTRY)/connectivity-publisher:$(IMAGE_TAG)
CONNECTIVITY_BINDER_IMAGE := $(IMAGE_REGISTRY)/connectivity-binder:$(IMAGE_TAG)
CONNECTIVITY_REGISTRY_IMAGE := $(IMAGE_REGISTRY)/connectivity-registry:$(IMAGE_TAG)
CONNECTIVITY_DNS_IMAGE := $(IMAGE_REGISTRY)/connectivity-dns:$(IMAGE_TAG)

SHARED_SERVICE_CLUSTER_KUBECONFIG ?= $(PWD)/shared-services.kubeconfig
WORKLOAD_CLUSTER_KUBECONFIG ?= $(PWD)/workloads.kubeconfig

# Directories.
TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin
BIN_DIR := bin
CONTROLLER_GEN := $(abspath $(TOOLS_BIN_DIR)/controller-gen)

.PHONY: e2e-up
e2e-up:
	bash hack/e2e.sh -u

.PHONY: e2e-down
e2e-down:
	bash hack/e2e.sh -d

.PHONY: test
test: test-unit test-connectivity

.PHONY: test-unit
test-unit:
	ginkgo -v -r $(PWD)/pkg $(PWD)/cmd/connectivity-registry

.PHONY: test-connectivity
test-connectivity:
	SHARED_SERVICE_CLUSTER_KUBECONFIG=$(SHARED_SERVICE_CLUSTER_KUBECONFIG) \
	WORKLOAD_CLUSTER_KUBECONFIG=$(WORKLOAD_CLUSTER_KUBECONFIG) \
	ginkgo -v $(PWD)/test/connectivity

.PHONY: build-images
build-images: build-connectivity-publisher build-connectivity-binder build-connectivity-registry build-connectivity-dns

.PHONY: build-connectivity-publisher
build-connectivity-publisher:
	docker build -f cmd/connectivity-publisher/Dockerfile -t $(CONNECTIVITY_PUBLISHER_IMAGE) .

.PHONY: build-connectivity-binder
build-connectivity-binder:
	docker build -f cmd/connectivity-binder/Dockerfile -t $(CONNECTIVITY_BINDER_IMAGE) .

.PHONY: build-connectivity-registry
build-connectivity-registry:
	docker build -f cmd/connectivity-registry/Dockerfile -t $(CONNECTIVITY_REGISTRY_IMAGE) .

.PHONY: build-connectivity-dns
build-connectivity-dns:
	docker build -f cmd/connectivity-dns/Dockerfile -t $(CONNECTIVITY_DNS_IMAGE) .

.PHONY: generate
generate: tools-vendor $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) \
		crd \
		paths=./apis/... \
		output:dir=manifests/crds/
	$(CONTROLLER_GEN) \
		object \
		"object:headerFile=./hack/boilerplate.go.txt" \
		paths=./apis/...
	./hack/update-codegen.sh

tools-vendor: $(TOOLS_DIR)/vendor

$(TOOLS_DIR)/vendor:
	cd $(TOOLS_DIR); go mod vendor

$(CONTROLLER_GEN): # Build controller-gen from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/controller-gen sigs.k8s.io/controller-tools/cmd/controller-gen

.PHONY: addlicense
addlicense:
	# requires https://github.com/google/addlicense
	addlicense -f ./hack/license.txt $(shell find . -name *.go)
