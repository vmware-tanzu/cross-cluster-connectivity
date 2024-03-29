IMAGE_REGISTRY ?= gcr.io/tanzu-xcc
IMAGE_TAG ?= dev

GOBUILD_VERSION ?= latest

DNS_SERVER_IMAGE := $(IMAGE_REGISTRY)/dns-server:$(IMAGE_TAG)
XCC_DNS_CONTROLLER_IMAGE := $(IMAGE_REGISTRY)/xcc-dns-controller:$(IMAGE_TAG)
DNS_CONFIG_PATCHER_IMAGE := $(IMAGE_REGISTRY)/dns-config-patcher:$(IMAGE_TAG)

CLUSTER_A := "cluster-a"
CLUSTER_B := "cluster-b"
MANAGEMENT := "management"
CLUSTER_A_KUBECONFIG ?= $(PWD)/$(CLUSTER_A).kubeconfig
CLUSTER_B_KUBECONFIG ?= $(PWD)/$(CLUSTER_B).kubeconfig
MANAGEMENT_KUBECONFIG ?= $(PWD)/$(MANAGEMENT).kubeconfig

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

.PHONY: check
check: generate test-unit

.PHONY: e2e-up
e2e-up:
	bash hack/e2e-up.sh

.PHONY: e2e-down
e2e-down:
	bash hack/e2e-down.sh

.PHONY: test
test: test-unit test-cluster-api-dns

.PHONY: test-full
test-full: test-unit build-images e2e-down e2e-up test-cluster-api-dns

.PHONY: test-unit
test-unit:
	ginkgo -race -p -r $(PWD)/pkg

.PHONY: test-cluster-api-dns
test-cluster-api-dns:
	CLUSTER_A_KUBECONFIG=$(CLUSTER_A_KUBECONFIG) \
	CLUSTER_B_KUBECONFIG=$(CLUSTER_B_KUBECONFIG) \
	MANAGEMENT_KUBECONFIG=$(MANAGEMENT_KUBECONFIG) \
	ginkgo -v $(PWD)/test/clusterapidns

.PHONY: build-images
build-images: build-dns-server build-xcc-dns-controller build-dns-config-patcher

.PHONY: build-dns-server
build-dns-server:
	docker build --build-arg GOVERSION=$(GOBUILD_VERSION) -f cmd/dns-server/Dockerfile -t $(DNS_SERVER_IMAGE) .

.PHONY: build-dns-config-patcher
build-dns-config-patcher:
	docker build --build-arg GOVERSION=$(GOBUILD_VERSION) -f cmd/dns-config-patcher/Dockerfile -t $(DNS_CONFIG_PATCHER_IMAGE) .

.PHONY: build-xcc-dns-controller
build-xcc-dns-controller:
	docker build --build-arg GOVERSION=$(GOBUILD_VERSION) -f cmd/xcc-dns-controller/Dockerfile -t $(XCC_DNS_CONTROLLER_IMAGE) .

.PHONY: e2e-load-images
e2e-load-images: e2e-load-dns-server-image e2e-load-xcc-dns-controller-image e2e-load-dns-config-patcher-image

.PHONY: e2e-load-dns-config-patcher-image
e2e-load-dns-config-patcher-image:
	kind load docker-image $(DNS_CONFIG_PATCHER_IMAGE) --name $(CLUSTER_A)
	kind load docker-image $(DNS_CONFIG_PATCHER_IMAGE) --name $(CLUSTER_B)

.PHONY: e2e-load-dns-server-image
e2e-load-dns-server-image:
	kind load docker-image $(DNS_SERVER_IMAGE) --name $(CLUSTER_A)
	kind load docker-image $(DNS_SERVER_IMAGE) --name $(CLUSTER_B)
	kubectl --kubeconfig $(CLUSTER_A_KUBECONFIG) get pod \
		-n xcc-dns \
		-l app=dns-server \
		-o jsonpath={.items[0].metadata.name} \
		| xargs -n1 kubectl --kubeconfig $(CLUSTER_A_KUBECONFIG) -n xcc-dns delete pod
	kubectl --kubeconfig $(CLUSTER_B_KUBECONFIG) get pod \
		-n xcc-dns \
		-l app=dns-server \
		-o jsonpath={.items[0].metadata.name} \
		| xargs -n1 kubectl --kubeconfig $(CLUSTER_B_KUBECONFIG) -n xcc-dns delete pod

.PHONY: e2e-load-xcc-dns-controller-image
e2e-load-xcc-dns-controller-image:
	kind load docker-image $(XCC_DNS_CONTROLLER_IMAGE) --name $(MANAGEMENT)
	kubectl --kubeconfig $(MANAGEMENT_KUBECONFIG) get pod \
		-n xcc-dns \
		-l app=xcc-dns-controller \
		-o jsonpath={.items[0].metadata.name} \
		| xargs -n1 kubectl --kubeconfig $(MANAGEMENT_KUBECONFIG) -n xcc-dns delete pod

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: generate
generate: controller-gen go-generate
	$(CONTROLLER_GEN) \
		$(CRD_OPTIONS) \
		rbac:roleName=manager-role \
		webhook \
		paths="./apis/..." \
		output:crd:artifacts:config=manifests/crds/
	$(CONTROLLER_GEN) \
		object:headerFile="hack/boilerplate.go.txt" \
		paths="./..."

.PHONY: go-generate
go-generate:
	go generate ./...

.PHONY: addlicense
addlicense:
	go run github.com/google/addlicense -f ./hack/license.txt $(shell find . -path ./hack/tools/vendor -prune -false -o -name *.go)
	go run github.com/google/addlicense -f ./hack/license.txt $(shell find . -path ./hack/tools/vendor -prune -false -o -name *.sh)
	go run github.com/google/addlicense -f ./hack/license.txt $(shell find . -path ./hack/tools/vendor -prune -false -o -name Dockerfile)

.PHONY: checklicense
checklicense:
	go run github.com/google/addlicense -check -f ./hack/license.txt $(shell find . -path ./hack/tools/vendor -prune -false -o -name *.go)
	go run github.com/google/addlicense -check -f ./hack/license.txt $(shell find . -path ./hack/tools/vendor -prune -false -o -name *.sh)
	go run github.com/google/addlicense -check -f ./hack/license.txt $(shell find . -path ./hack/tools/vendor -prune -false -o -name Dockerfile)

.PHONY: example-apply-gateway-dns
example-apply-gateway-dns:
	kubectl --kubeconfig ./management.kubeconfig apply -f ./manifests/example/dev-team-gateway-dns.yaml

.PHONY: example-deploy-nginx
example-deploy-nginx: example-apply-gateway-dns
	kubectl --kubeconfig ./cluster-a.kubeconfig apply -f ./manifests/example/nginx/certs.yaml
	kubectl --kubeconfig ./cluster-a.kubeconfig apply -f ./manifests/example/nginx/nginx.yaml
	kubectl --kubeconfig ./cluster-a.kubeconfig apply -f ./manifests/example/nginx/httpproxy.yaml

.PHONY: example-curl-nginx
example-curl-nginx:
	kubectl --kubeconfig ./cluster-b.kubeconfig run -it --rm --restart=Never \
		--image=curlimages/curl curl -- \
		curl -v -k "https://nginx.gateway.cluster-a.dev-team.clusters.xcc.test"

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(shell go env GOPATH)/bin/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
