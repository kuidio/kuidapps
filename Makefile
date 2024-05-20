# Copyright 2024 Nokia
# Licensed under the Apache License 2.0
# SPDX-License-Identifier: Apache-2.0

VERSION ?= latest
REGISTRY ?= europe-docker.pkg.dev/srlinux/eu.gcr.io
PROJECT ?= kuidapps
IMG ?= $(REGISTRY)/${PROJECT}:$(VERSION)

REPO = github.com/kuidio/kuidapps
USERID := 10000

## Tool Binaries
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
CONTROLLER_TOOLS_VERSION ?= v0.15.0
KFORM ?= $(LOCALBIN)/kform
KFORM_VERSION ?= v0.0.5

# go versions
TARGET_GO_VERSION := go1.21.4
GO_FALLBACK := go
# We prefer $TARGET_GO_VERSION if it is not available we go with whatever go we find ($GO_FALLBACK)
GO_BIN := $(shell if [ "$$(which $(TARGET_GO_VERSION))" != "" ]; then echo $$(which $(TARGET_GO_VERSION)); else echo $$(which $(GO_FALLBACK)); fi)

.PHONY: codegen fix fmt vet lint test tidy

all: codegen fmt vet lint test tidy

docker:
	GOOS=linux GOARCH=arm64 go build -o install/bin/apiserver
	docker build install --tag apiserver-caas:v0.0.0 --ssh default=$(SSH_AUTH_SOCK)

.PHONY:
docker-build: ## Build docker image with the manager.
	ssh-add ./keys/id_rsa 2>/dev/null; true
	docker build --build-arg USERID=$(USERID) . -t ${IMG} --ssh default=$(SSH_AUTH_SOCK)

.PHONY: docker-push
docker-push:  docker-build ## Push docker image with the manager.
	docker push ${IMG}

install: docker
	kustomize build install | kubectl apply -f -

reinstall: docker
	kustomize build install | kubectl apply -f -
	kubectl delete pods -n kuid-system --all

apiserver-logs:
	kubectl logs -l apiserver=true --container apiserver -n kuid-system -f --tail 1000

.PHONY: generate
generate: controller-gen 
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./apis/..."

.PHONY: manifests
manifests: controller-gen generate ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	mkdir -p artifacts
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./apis/..." output:crd:artifacts:config=artifacts


.PHONY: artifacts
artifacts: kform manifests
	mkdir -p artifacts/out
	if [ ! -e "artifacts/.kform/kform-inventory.yaml" ]; then \
        $(KFORM) init artifacts; \
    fi
	$(KFORM) apply artifacts -i artifacts/in/configmap-input-vars.yaml -o artifacts/out/artifacts.yaml

.PHONY:
fix:
	go fix ./...

.PHONY:
fmt:
	test -z $(go fmt ./tools/...)

.PHONY:
tidy:
	go mod tidy

.PHONY:
lint:
	(which golangci-lint || go get github.com/golangci/golangci-lint/cmd/golangci-lint)
	$(GOBIN)/golangci-lint run ./...

.PHONY:
test:
	go test -cover ./...

vet:
	go vet ./...

local-run:
	apiserver-boot run local --run=etcd,apiserver

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: kform
kform: $(KFORM) ## Download kform locally if necessary.
$(KFORM): $(LOCALBIN)
	test -s $(LOCALBIN)/kform || GOBIN=$(LOCALBIN) go install github.com/kform-dev/kform/cmd/kform@$(KFORM_VERSION)
