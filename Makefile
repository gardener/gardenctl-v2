# SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

export LD_FLAGS=$(shell ./hack/get-build-ld-flags.sh)

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-13s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: test
test: fmt lint ## Run tests.
	@./hack/test-integration.sh

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: lint
lint: ## Run golangci-lint against code.
	@./hack/golangci-lint.sh

##@ Build

.PHONY: build
build: build-darwin-amd64 build-darwin-arm64 build-linux-amd64 build-windows-amd64 ## Build gardenctl binary for darwin, linux and windows.

.PHONY: build-linux-amd64
build-linux-amd64: ## Build gardenctl binary for linux.
	@./hack/build-linux-amd64.sh

.PHONY: build-darwin-amd64
build-darwin-amd64: ## Build gardenctl binary for darwin on Intel processors.
	@./hack/build-darwin-amd64.sh

.PHONY: build-darwin-arm64
build-darwin-arm64: ## Build gardenctl binary for darwin on Apple Silicon processors.
	@./hack/build-darwin-arm64.sh

.PHONY: build-windows-amd64
build-windows-amd64: ## Build gardenctl binary for windows.
	@./hack/build-windows-amd64.sh
