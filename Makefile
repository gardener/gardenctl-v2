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

# Run tests
.PHONY: test
test: generate lint
	go test ./... -coverprofile cover.out

# Run golangci-lint against code
.PHONY: lint
lint:
	@./hack/golangci-lint.sh

.PHONY: generate
generate:
	go generate ./...

.PHONY: build
build: build-darwin build-linux

.PHONY: build-linux
build-linux:
	@./hack/build-linux-amd64.sh

.PHONY: build-darwin
build-darwin:
	@./hack/build-darwin-amd64.sh
