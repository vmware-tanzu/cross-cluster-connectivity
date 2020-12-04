IMAGE_REGISTRY ?= ghcr.io/vmware-tanzu/cross-cluster-connectivity
IMAGE_TAG ?= dev

DNS_SERVER_IMAGE := $(IMAGE_REGISTRY)/dns-server:$(IMAGE_TAG)

CLUSTER_A_KUBECONFIG ?= $(PWD)/cluster-a.kubeconfig

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

.PHONY: check
check: generate fmt vet

.PHONY: e2e-up
e2e-up:
	bash hack/e2e.sh -u

.PHONY: e2e-down
e2e-down:
	bash hack/e2e.sh -d

.PHONY: test
test: test-unit test-cluster-api-dns

.PHONY: test-full
test-full: test-unit build-images e2e-down e2e-up test-cluster-api-dns

.PHONY: test-unit
test-unit:
	ginkgo -race -r $(PWD)/pkg

.PHONY: test-cluster-api-dns
test-cluster-api-dns:
	CLUSTER_A_KUBECONFIG=$(CLUSTER_A_KUBECONFIG) \
	ginkgo -v -p $(PWD)/test/clusterapidns

.PHONY: build-images
build-images: build-dns-server

.PHONY: build-dns-server
build-dns-server:
	docker build -f cmd/dns-server/Dockerfile -t $(DNS_SERVER_IMAGE) .

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: generate
generate: controller-gen
	$(CONTROLLER_GEN) \
		$(CRD_OPTIONS) \
		rbac:roleName=manager-role \
		webhook \
		paths="./apis/..." \
		output:crd:artifacts:config=config/crd/
	$(CONTROLLER_GEN) \
		object:headerFile="hack/boilerplate.go.txt" \
		paths="./..."

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

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
