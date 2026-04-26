# Image URL to use all building/pushing image targets
TAG ?= local
IMG ?= controller:$(TAG)
JOB_IMG ?= magos-job:$(TAG)
UI_IMG ?= ui:$(TAG)
API_IMG ?= magos-api:$(TAG)
RUSTFS_S3_PORT ?= 9000

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker
MAGOS_LOGS_RETENTION ?= 10

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./types/..." output:crd:artifacts:config=charts/magos/crds

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet setup-envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

# TODO(user): To use a different vendor for e2e tests, modify the setup under 'tests/e2e'.
# The default setup assumes Kind is pre-installed and builds/loads the Manager Docker image locally.
# CertManager is installed by default; skip with:
# - CERT_MANAGER_INSTALL_SKIP=true
KIND_CLUSTER ?= magos-test-e2e

.PHONY: kind-cluster
kind-cluster: kind ## Create a Kind cluster named $(KIND_CLUSTER) if it does not exist.
	@case "$$($(KIND) get clusters)" in \
		*"$(KIND_CLUSTER)"*) \
			echo "Kind cluster '$(KIND_CLUSTER)' already exists. Skipping creation." ;; \
		*) \
			echo "Creating Kind cluster '$(KIND_CLUSTER)'..."; \
			$(KIND) create cluster --name $(KIND_CLUSTER) ;; \
	esac

.PHONY: test-e2e
test-e2e: kind-cluster manifests generate fmt vet ## Run the e2e tests. Expected an isolated environment using Kind.
	KIND=$(KIND) KIND_CLUSTER=$(KIND_CLUSTER) go test -tags=e2e ./test/e2e/ -v -ginkgo.v
	$(MAKE) cleanup-test-e2e

.PHONY: cleanup-test-e2e
cleanup-test-e2e: ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER)

.PHONY: test-chainsaw
test-chainsaw: chainsaw ## Run controller behavior chainsaw tests (no helm install required).
	$(CHAINSAW) test test/chainsaw/tests/workspace test/chainsaw/tests/rollout test/chainsaw/tests/project

.PHONY: test-chainsaw-chart
test-chainsaw-chart: chainsaw ## Run chart installation chainsaw tests (requires helm install of magos in magos-system).
	$(CHAINSAW) test test/chainsaw/tests/chart

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

.PHONY: lint-config
lint-config: golangci-lint ## Verify golangci-lint linter configuration
	$(GOLANGCI_LINT) config verify

.PHONY: deps
deps:
	go mod tidy
	cd api && go mod tidy
	cd ui && npm install

# TODO: currently all logs go to 1 stdout stream, consider using a tmux set-up or other solution?
.PHONY: run
run: deps manifests generate fmt vet install-rustfs ## Run all components in parallel.
	@$(KUBECTL) wait deployment/magos-rustfs --for=condition=available --timeout=60s
	@trap 'kill 0' EXIT; \
	export MAGOS_LOGS_ENABLED=true; \
	export MAGOS_LOGS_RETENTION=$(MAGOS_LOGS_RETENTION); \
	export MAGOS_LOGS_S3_ENDPOINT="http://127.0.0.1:$(RUSTFS_S3_PORT)"; \
	export MAGOS_LOGS_S3_ACCESS_KEY_ID="$$($(KUBECTL) get secret magos-rustfs -o jsonpath='{.data.accessKey}' | base64 -d)"; \
	export MAGOS_LOGS_S3_SECRET_ACCESS_KEY="$$($(KUBECTL) get secret magos-rustfs -o jsonpath='{.data.secretKey}' | base64 -d)"; \
	$(KUBECTL) port-forward svc/magos-rustfs $(RUSTFS_S3_PORT):9000 & \
	$(MAKE) -s run-controller ARGS="$(ARGS)" & \
	$(MAKE) -s run-api & \
	$(MAKE) -s run-ui & \
	wait

.PHONY: run-controller
ARGS ?= --enable-workspace-controller --enable-project-controller --enable-variableset-controller --enable-rollout-controller --enable-refwatcher-controller
run-controller: manifests generate fmt vet ## Run a controller from your host.
	MAGOS_JOB_IMAGE=magos-job:local go run ./cmd/main.go $(ARGS)

.PHONY: run-api
run-api: ## Run the API server from your host.
	cd ./api/cmd && go run ./api/main.go

.PHONY: run-ui
run-ui: ## Run the react UI from your host, requires to have npm installed.
	cd ./ui && npm run dev


##@ Code Generation
##
## Full pipeline:  CRD types ──► manifests + deepcopy   (controller-gen)
##                 CRD types ──► clientset / informers   (kube_codegen)
##                 Go handlers ──► OpenAPI spec           (swag)
##                 OpenAPI spec ──► TypeScript types       (openapi-typescript)
.PHONY: generate
generate: generate-controller generate-api-client generate-swagger generate-ui-types ## Run full code generation pipeline (deepcopy, clients, OpenAPI, TS types).

.PHONY: generate-controller
generate-controller: controller-gen ## Generate deepcopy, conversion and defaulter functions.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./types/..."

.PHONY: generate-api-client
generate-api-client: client-gen lister-gen informer-gen ## Generate typed Kubernetes clientset, informers and listers.
	cd hack/tools && go mod download
	CODE_GENERATOR_VERSION=$(CODE_GENERATOR_VERSION) LOCALBIN=$(LOCALBIN) hack/update-codegen.sh

.PHONY: generate-swagger
generate-swagger: swag ## Generate OpenAPI spec (swagger.json) from handler annotations.
	cd api && $(SWAG) fmt -g cmd/api/main.go -d ./cmd/api/,./internal/
	cd api && $(SWAG) init \
		-g main.go \
		--dir ./cmd/api/,./internal/api/,./internal/service/ \
		--output internal/api/docs \
		--outputTypes json \
		--v3.1 \
		--parseDependency \
		--parseInternal

.PHONY: generate-ui-types
generate-ui-types: ## Generate TypeScript API types from swagger.json (OpenAPI spec).
	cd ui && npm run generate

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker images for all components.
	$(CONTAINER_TOOL) build -t ${IMG} .
	$(CONTAINER_TOOL) build -t ${UI_IMG} -f ui/Dockerfile ui/
	$(CONTAINER_TOOL) build -t ${JOB_IMG} -f cmd/job/Dockerfile .
	$(CONTAINER_TOOL) build -t ${API_IMG} -f api/Dockerfile .

.PHONY: docker-push
docker-push: ## Push docker images for all components.
	$(CONTAINER_TOOL) push ${IMG}
	$(CONTAINER_TOOL) push ${UI_IMG}
	$(CONTAINER_TOOL) push ${JOB_IMG}
	$(CONTAINER_TOOL) push ${API_IMG}

.PHONY: kind-load
kind-load: kind ## load locally built docker image(s) into kind cluster.
	$(KIND) load docker-image ${IMG} --name $(KIND_CLUSTER)
	$(KIND) load docker-image ${UI_IMG} --name $(KIND_CLUSTER)
	$(KIND) load docker-image ${JOB_IMG} --name $(KIND_CLUSTER)
	$(KIND) load docker-image ${API_IMG} --name $(KIND_CLUSTER)

# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name magos-builder
	$(CONTAINER_TOOL) buildx use magos-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm magos-builder
	rm Dockerfile.cross

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests install-validatingpolicy-crd install-job-rbac install-rustfs ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUBECTL) apply -f charts/magos/crds/

.PHONY: install-rustfs
install-rustfs: ## Install RustFS into the cluster for local development (default namespace).
	$(KUBECTL) apply -f hack/local-rustfs.yaml

.PHONY: install-job-rbac
install-job-rbac: ## Install the magos-job ServiceAccount and RBAC for local development (default namespace).
	$(KUBECTL) apply -f hack/local-job-rbac.yaml

.PHONY: install-validatingpolicy-crd
install-validatingpolicy-crd: ## Install the Kyverno ValidatingPolicy CRD if not already present (skips when Kyverno is already installed).
	@$(KUBECTL) get crd validatingpolicies.policies.kyverno.io >/dev/null 2>&1 && \
		echo "ValidatingPolicy CRD already installed, skipping" || \
		helm template magos charts/magos/ --set policy.kyverno.installCRD=true \
		  --show-only templates/kyverno-validatingpolicy-crd.yaml | $(KUBECTL) apply -f -

.PHONY: uninstall-validatingpolicy-crd
uninstall-validatingpolicy-crd: ## Remove the Kyverno ValidatingPolicy CRD (skips when Kyverno is installed, as it owns the CRD).
	@$(KUBECTL) get pods --all-namespaces -l app.kubernetes.io/part-of=kyverno --no-headers 2>/dev/null | grep -q . && \
		echo "Kyverno is running, skipping CRD removal to avoid breaking it" || \
		helm template magos charts/magos/ --set policy.kyverno.installCRD=true \
		  --show-only templates/kyverno-validatingpolicy-crd.yaml | $(KUBECTL) delete --ignore-not-found -f -

.PHONY: uninstall
uninstall: uninstall-validatingpolicy-crd ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f charts/magos/crds/

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KIND ?= $(LOCALBIN)/kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
CHAINSAW ?= $(LOCALBIN)/chainsaw
SWAG ?= $(LOCALBIN)/swag
CLIENT_GEN ?= $(LOCALBIN)/client-gen
LISTER_GEN ?= $(LOCALBIN)/lister-gen
INFORMER_GEN ?= $(LOCALBIN)/informer-gen

## Tool Versions
KUSTOMIZE_VERSION ?= v5.7.1
CONTROLLER_TOOLS_VERSION ?= v0.19.0
#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell go list -m -f "{{ .Version }}" sigs.k8s.io/controller-runtime | awk -F'[v.]' '{printf "release-%d.%d", $$2, $$3}')
#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell go list -m -f "{{ .Version }}" k8s.io/api | awk -F'[v.]' '{printf "1.%d", $$3}')
GOLANGCI_LINT_VERSION ?= v2.4.0
KIND_VERSION ?= v0.31.0
CHAINSAW_VERSION ?= 93b1e3d8620313bb08dc314981bc972af7dd356a
# see https://github.com/swaggo/swag/issues/1898
# we are using the release-candidate version because otherwise openapi 3.0 and 3.1 are not supported
# while for the client (react-fetch) we need openapi 3.1 support
SWAG_VERSION ?= v2.0.0-rc5
CODE_GENERATOR_VERSION ?= v0.35.3

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@$(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

.PHONY: chainsaw
chainsaw: $(CHAINSAW) ## Download chainsaw locally if necessary.
$(CHAINSAW): $(LOCALBIN)
	$(call go-install-tool,$(CHAINSAW),github.com/kyverno/chainsaw,$(CHAINSAW_VERSION))

.PHONY: kind
kind: $(KIND) ## Download kind locally if necessary.
$(KIND): $(LOCALBIN)
	$(call go-install-tool,$(KIND),sigs.k8s.io/kind,$(KIND_VERSION))

.PHONY: swag
swag: $(SWAG) ## Download swag locally if necessary.
$(SWAG): $(LOCALBIN)
	$(call go-install-tool,$(SWAG),github.com/swaggo/swag/v2/cmd/swag,$(SWAG_VERSION))

.PHONY: client-gen
client-gen: $(CLIENT_GEN) ## Download client-gen locally if necessary.
$(CLIENT_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CLIENT_GEN),k8s.io/code-generator/cmd/client-gen,$(CODE_GENERATOR_VERSION))

.PHONY: lister-gen
lister-gen: $(LISTER_GEN) ## Download lister-gen locally if necessary.
$(LISTER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(LISTER_GEN),k8s.io/code-generator/cmd/lister-gen,$(CODE_GENERATOR_VERSION))

.PHONY: informer-gen
informer-gen: $(INFORMER_GEN) ## Download informer-gen locally if necessary.
$(INFORMER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(INFORMER_GEN),k8s.io/code-generator/cmd/informer-gen,$(CODE_GENERATOR_VERSION))


# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $$(realpath $(1)-$(3)) $(1)
endef
