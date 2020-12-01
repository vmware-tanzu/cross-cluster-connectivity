
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

.PHONY: test-full
test-full: test-unit build-images e2e-down e2e-up test-connectivity test-cluster-api-dns

.PHONY: test-unit
test-unit:
	ginkgo -v -race -r $(PWD)/pkg $(PWD)/cmd/connectivity-registry

.PHONY: test-connectivity
test-connectivity:
	CLUSTER_ONE_KUBECONFIG=$(WORKLOAD_CLUSTER_KUBECONFIG) \
	CLUSTER_TWO_KUBECONFIG=$(SHARED_SERVICE_CLUSTER_KUBECONFIG) \
	ginkgo -v -p $(PWD)/test/connectivity

.PHONY: test-cluster-api-dns
test-cluster-api-dns:
	CLUSTER_ONE_KUBECONFIG=$(WORKLOAD_CLUSTER_KUBECONFIG) \
	CLUSTER_TWO_KUBECONFIG=$(SHARED_SERVICE_CLUSTER_KUBECONFIG) \
	ginkgo -v -p $(PWD)/test/clusterapidns

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

.PHONY: push-images
push-images: push-connectivity-publisher push-connectivity-binder push-connectivity-registry push-connectivity-dns

.PHONY: push-connectivity-publisher
push-connectivity-publisher:
	docker push $(CONNECTIVITY_PUBLISHER_IMAGE)

.PHONY: push-connectivity-binder
push-connectivity-binder:
	docker push $(CONNECTIVITY_BINDER_IMAGE)

.PHONY: push-connectivity-registry
push-connectivity-registry:
	docker push $(CONNECTIVITY_REGISTRY_IMAGE)

.PHONY: push-connectivity-dns
push-connectivity-dns:
	docker push $(CONNECTIVITY_DNS_IMAGE)

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
	addlicense -f ./hack/license.txt $(shell find . -path ./hack/tools/vendor -prune -false -o -name *.go)
	addlicense -f ./hack/license.txt $(shell find . -path ./hack/tools/vendor -prune -false -o -name *.sh)
	addlicense -f ./hack/license.txt $(shell find . -path ./hack/tools/vendor -prune -false -o -name Dockerfile)

.PHONY: checklicense
checklicense:
	# requires https://github.com/google/addlicense
	addlicense -check -f ./hack/license.txt $(shell find . -path ./hack/tools/vendor -prune -false -o -name *.go)
	addlicense -check -f ./hack/license.txt $(shell find . -path ./hack/tools/vendor -prune -false -o -name *.sh)
	addlicense -check -f ./hack/license.txt $(shell find . -path ./hack/tools/vendor -prune -false -o -name Dockerfile)

.PHONY: example-deploy-nginx
example-deploy-nginx:
	kubectl --kubeconfig ./shared-services.kubeconfig apply -f ./manifests/example/nginx/certs.yaml
	kubectl --kubeconfig ./shared-services.kubeconfig apply -f ./manifests/example/nginx/nginx.yaml
	kubectl --kubeconfig ./shared-services.kubeconfig apply -f ./manifests/example/nginx/exported_http_proxy.yaml

.PHONY: example-curl-nginx
example-curl-nginx:
	kubectl --kubeconfig ./workloads.kubeconfig run -it --rm --restart=Never \
		--image=curlimages/curl curl -- \
		curl -v -k "https://nginx.xcc.test"
