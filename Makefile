# SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

REPO_ROOT      := $(shell git rev-parse --show-toplevel)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

export LD_FLAGS=$(shell ./hack/get-build-ld-flags.sh)

#########################################
# Tools                                 #
#########################################

TOOLS_DIR := hack/tools
include hack/tools.mk

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

.PHONY: tidy
tidy: ## Clean up go.mod and go.sum by removing unused dependencies.
	go mod tidy

.PHONY: clean
clean: ## Remove generated files and clean up directories.
	@hack/clean.sh ./internal/... ./pkg/...

.PHONY: gen-markdown
gen-markdown: ## Generate markdown help files
	go run ./internal/gen/markdown.go

.PHONY: generate
generate: gen-markdown $(MOCKGEN) fmt  ## Run go generate
	@hack/generate.sh ./pkg/... ./internal/...

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: check-generate
check-generate: ## Verify if code generation is up-to-date by running generate and checking for changes.
	@hack/check-generate.sh $(REPO_ROOT)

.PHONY: lint
lint: ## Run golangci-lint against code.
	@./hack/golangci-lint.sh

.PHONY: sast
sast: $(GOSEC) ## Run gosec against code
	@./hack/sast.sh

.PHONY: sast-report
sast-report: $(GOSEC) ## Run gosec against code and export report to SARIF.
	@./hack/sast.sh --gosec-report true

.PHONY: test
test: fmt lint check-markdown go-test sast ## Run tests.

.PHONY: check-markdown
check-markdown: ## Check that the generated markdown is up-to-date
	@./hack/check-markdown.sh

.PHONY: go-test
go-test: ## Run go tests.
	@./hack/test-integration.sh

.PHONY: verify ## Run basic verification including linting, tests, static analysis and check if the generated markdown is up-to-date.
verify: lint go-test sast check-markdown

.PHONY: verify-extended ## Run extended verification including code generation check, linting, tests, and detailed static analysis report.
verify-extended: check-generate check-markdown lint go-test sast-report

##@ Build

.PHONY: build
build: build-darwin-amd64 build-darwin-arm64 build-linux-amd64 build-linux-arm64 build-windows-amd64 ## Build gardenctl binary for darwin, linux and windows.

.PHONY: build-linux-amd64
build-linux-amd64: ## Build gardenctl binary for Linux on Intel processors.
	@./hack/build-linux-amd64.sh

.PHONY: build-linux-arm64
build-linux-arm64: ## Build gardenctl binary for Linux on ARM processors.
	@./hack/build-linux-arm64.sh

.PHONY: build-darwin-amd64
build-darwin-amd64: ## Build gardenctl binary for darwin on Intel processors.
	@./hack/build-darwin-amd64.sh

.PHONY: build-darwin-arm64
build-darwin-arm64: ## Build gardenctl binary for darwin on Apple Silicon processors.
	@./hack/build-darwin-arm64.sh

.PHONY: build-windows-amd64
build-windows-amd64: ## Build gardenctl binary for Windows on Intel processors.
	@./hack/build-windows-amd64.sh
