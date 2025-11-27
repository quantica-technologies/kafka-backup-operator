# Makefile for Kafka Backup Operator

# Image URL to use all building/pushing image targets
IMG ?= kafka-backup-operator:latest
WORKER_IMG ?= kafka-backup-worker:latest

# Helm chart variables
CHART_DIR = deployments/helm/kafka-backup-operator
CHART_NAME = kafka-backup-operator
HELM_PACKAGE_DIR = deployments/helm/packages

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet ## Run tests.
	go test ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

.PHONY: build-all
build-all: ## Build all binaries
	@echo "Building all binaries..."
	go build -o bin/backup ./cmd/backup
	go build -o bin/restore ./cmd/restore
	go build -o bin/worker ./cmd/worker
	go build -o bin/operator ./cmd/operator
	@echo "All binaries built successfully"

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go

.PHONY: docker-build
docker-build: test ## Build docker image with the manager.
	docker build -t ${IMG} -f build/package/Dockerfile.operator .
	docker build -t ${WORKER_IMG} -f build/package/Dockerfile.worker .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}
	docker push ${WORKER_IMG}

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Helm

.PHONY: helm-crds
helm-crds: manifests ## Copy CRDs to Helm chart
	@echo "📦 Copying CRDs to Helm chart..."
	@mkdir -p $(CHART_DIR)/crds
	@cp config/crd/bases/*.yaml $(CHART_DIR)/crds/
	@echo "✅ CRDs copied successfully"
	@ls -la $(CHART_DIR)/crds/

.PHONY: helm-lint
helm-lint: helm-crds ## Lint Helm chart
	@echo "🔍 Linting Helm chart..."
	@helm lint $(CHART_DIR)
	@echo "✅ Helm chart linting passed"

.PHONY: helm-template
helm-template: helm-crds ## Render Helm templates
	@echo "📄 Rendering Helm templates..."
	@helm template $(CHART_NAME) $(CHART_DIR) \
		--namespace kafka-backup-system \
		--debug

.PHONY: helm-template-prod
helm-template-prod: helm-crds ## Render Helm templates with production values
	@echo "📄 Rendering Helm templates (production)..."
	@helm template $(CHART_NAME) $(CHART_DIR) \
		--namespace kafka-backup-system \
		--values $(CHART_DIR)/values-production.yaml \
		--debug

.PHONY: helm-package
helm-package: helm-crds helm-lint ## Package Helm chart
	@echo "📦 Packaging Helm chart..."
	@mkdir -p $(HELM_PACKAGE_DIR)
	@helm package $(CHART_DIR) --destination $(HELM_PACKAGE_DIR)
	@echo "✅ Helm chart packaged"
	@ls -la $(HELM_PACKAGE_DIR)

.PHONY: helm-install-dev
helm-install-dev: helm-crds ## Install Helm chart with development values
	@echo "🚀 Installing Helm chart (development)..."
	@helm upgrade --install $(CHART_NAME) $(CHART_DIR) \
		--namespace kafka-backup-system \
		--create-namespace \
		--values $(CHART_DIR)/values-development.yaml \
		--set image.tag=latest \
		--set worker.image.tag=latest
	@echo "✅ Helm chart installed"

.PHONY: helm-install-prod
helm-install-prod: helm-crds ## Install Helm chart with production values
	@echo "🚀 Installing Helm chart (production)..."
	@helm upgrade --install $(CHART_NAME) $(CHART_DIR) \
		--namespace kafka-backup-system \
		--create-namespace \
		--values $(CHART_DIR)/values-production.yaml \
		--set image.repository=$(IMG) \
		--set worker.image.repository=$(WORKER_IMG)
	@echo "✅ Helm chart installed"

.PHONY: helm-upgrade
helm-upgrade: manifests helm-crds ## Upgrade CRDs and Helm chart
	@echo "🔄 Applying CRD updates..."
	@kubectl apply -f $(CHART_DIR)/crds/
	@echo "🔄 Upgrading Helm chart..."
	@helm upgrade $(CHART_NAME) $(CHART_DIR) \
		--namespace kafka-backup-system
	@echo "✅ Upgrade completed"

.PHONY: helm-uninstall
helm-uninstall: ## Uninstall Helm chart
	@echo "🗑️  Uninstalling Helm chart..."
	@helm uninstall $(CHART_NAME) --namespace kafka-backup-system || true
	@echo "✅ Helm chart uninstalled"

.PHONY: helm-delete-crds
helm-delete-crds: ## Delete CRDs (WARNING: deletes all custom resources)
	@echo "⚠️  WARNING: This will delete all CRDs and their resources!"
	@read -p "Are you sure? [y/N] " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		kubectl delete -f $(CHART_DIR)/crds/ || true; \
		echo "✅ CRDs deleted"; \
	else \
		echo "❌ Cancelled"; \
	fi

.PHONY: helm-clean
helm-clean: ## Clean Helm artifacts
	@echo "🧹 Cleaning Helm artifacts..."
	@rm -rf $(HELM_PACKAGE_DIR)
	@rm -f $(CHART_DIR)/crds/*.yaml
	@echo "✅ Cleaned"

.PHONY: helm-docs
helm-docs: ## Generate Helm chart documentation
	@echo "📚 Generating Helm documentation..."
	@docker run --rm -v "$(PWD)/$(CHART_DIR):/helm-docs" jnorwood/helm-docs:latest
	@echo "✅ Documentation generated"

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
KUSTOMIZE_VERSION ?= v5.0.0
CONTROLLER_TOOLS_VERSION ?= v0.14.0

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	test -s $(LOCALBIN)/kustomize || { curl -s $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); }

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

##@ Examples

.PHONY: deploy-examples
deploy-examples: ## Deploy example resources
	@echo "📋 Deploying examples..."
	@kubectl apply -f examples/
	@echo "✅ Examples deployed"

.PHONY: create-namespace
create-namespace: ## Create kafka-backup-system namespace
	@kubectl create namespace kafka-backup-system --dry-run=client -o yaml | kubectl apply -f -

##@ Complete workflows

.PHONY: dev-install
dev-install: manifests helm-crds helm-install-dev deploy-examples ## Complete development install
	@echo "🎉 Development environment ready!"

.PHONY: prod-install
prod-install: docker-build docker-push manifests helm-crds helm-package helm-install-prod ## Complete production install
	@echo "🎉 Production environment ready!"

.PHONY: full-upgrade
full-upgrade: manifests generate helm-crds docker-build docker-push helm-upgrade ## Complete upgrade workflow
	@echo "🎉 Upgrade completed!"

.PHONY: clean-all
clean-all: helm-uninstall helm-clean undeploy ## Clean everything
	@echo "🧹 Cleaning all artifacts..."
	@rm -rf bin/
	@rm -f cover.out
	@echo "✅ All clean"