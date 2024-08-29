.DEFAULT_GOAL := all

# GO_BUILD_ARGS should be set when running 'go build' or 'go install'.
VERSION_PKG = "$(shell go list -m)/internal/version"
export GIT_VERSION = $(shell git describe --dirty --tags --always)
export GIT_COMMIT = $(shell git rev-parse HEAD)
BUILD_DIR = $(PWD)/bin
GO_BUILD_ARGS = \
  -gcflags "all=-trimpath=$(shell dirname $(shell pwd))" \
  -asmflags "all=-trimpath=$(shell dirname $(shell pwd))" \
  -ldflags " \
    -s \
    -w \
    -X '$(VERSION_PKG).GitVersion=$(GIT_VERSION)' \
    -X '$(VERSION_PKG).GitCommit=$(GIT_COMMIT)' \
  " \

# Always use Go modules
export GO111MODULE = on

# Setup project-local paths and build settings
SHELL=/bin/bash
export PATH := $(BUILD_DIR):$(PATH)

# bingo manages consistent tooling versions for things like kind, kustomize, etc.
include .bingo/Variables.mk

##@ Development

.PHONY: all
all: test lint build

# Run tests
.PHONY: test
# Use envtest based on the version of kubernetes/client-go configured in the go.mod file.
# If this version of envtest is not available yet, submit a PR similar to
# https://github.com/kubernetes-sigs/kubebuilder/pull/2287 targeting the kubebuilder
# "tools-releases" branch. Make sure to look up the appropriate etcd version in the
# kubernetes release notes for the minor version you're building tools for.
CLIENT_GO_VERSION = $(shell go list -m k8s.io/client-go | cut -d" " -f2 | sed 's/^v0\.\([[:digit:]]\{1,\}\)\.[[:digit:]]\{1,\}$$/1.\1.x/')
TESTPKG ?= ./...
test: build $(SETUP_ENVTEST)
	eval $$($(SETUP_ENVTEST) use -p env $(CLIENT_GO_VERSION)) && go test -race -covermode atomic -coverprofile cover.out $(TESTPKG)

.PHONY: test-sanity
test-sanity: fix lint ## Test repo formatting, linting, etc.
	go vet ./...
	git diff --exit-code # diff again to ensure other checks don't change repo

# Build manager binary
.PHONY: build
build:
	mkdir -p $(BUILD_DIR) && go build $(GO_BUILD_ARGS) -o $(BUILD_DIR) ./

# Run various checks against code
.PHONY: lint
lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run


.PHONY: fix
fix: $(GOLANGCI_LINT) ## Fixup files in the repo.
	go mod tidy
	go fmt ./...
	$(GOLANGCI_LINT) run --fix

.PHONY: bingo-upgrade
bingo-upgrade: $(BINGO) ## Upgrade tools
	@for pkg in $$($(BINGO) list | awk '{ print $$1 }' | tail -n +3); do \
		echo "Upgrading $$pkg to latest..."; \
		$(BINGO) get "$$pkg@latest"; \
	done

.PHONY: release
release: GORELEASER_ARGS ?= --snapshot --clean --skip-sign
release: $(GORELEASER)
	$(GORELEASER) $(GORELEASER_ARGS)

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
