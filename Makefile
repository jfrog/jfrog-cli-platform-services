SHELL := /bin/bash
.DEFAULT_GOAL = build
GOCMD = go
export PROJECT_DIR ?= $(CURDIR)
BINARY_CLI = bin
WORKSPACE_ROOT = $(shell cd "${PROJECT_DIR}" && pwd)
TOOLS_DIR := $(CURDIR)/.tools
SCRIPTS_DIR = ${PROJECT_DIR}/.github/scripts
TARGET_DIR = ${PROJECT_DIR}/target
LINKERFLAGS = -s -w
COMPILERFLAGS = all=-trimpath=$(WORKSPACE_ROOT)
GOOS = $(shell go env GOOS)
GOARCH = $(shell go env GOARCH)
GO_SOURCES = $(eval GO_SOURCES := $$(shell find . -type f -name "*.go" | grep -v ".*_mock\.go"))$(GO_SOURCES)

########## FORMAT ##########

format: GOFUMPT GOIMPORTS
	@${GOFUMPT} -w $(GO_SOURCES)
	@${GOIMPORTS} -w -local jfrog.com $(GO_SOURCES)

GOFUMPT = ${TOOLS_DIR}/gofumpt
GOFUMPT_VERSION = 0.5.0

GOFUMPT:
	${GOFUMPT} --version 2>/dev/null | grep ${GOFUMPT_VERSION} || GOBIN=${TOOLS_DIR} $(GOCMD) install mvdan.cc/gofumpt@v${GOFUMPT_VERSION}

GOIMPORTS = ${TOOLS_DIR}/goimports
GOIMPORTS_VERSION = 0.16.1

GOIMPORTS:
	GOBIN=${TOOLS_DIR} $(GOCMD) install golang.org/x/tools/cmd/goimports@v${GOIMPORTS_VERSION}

########## ANALYSE ##########

GOLANGCI_LINT         = ${TOOLS_DIR}/golangci-lint
GOLANGCI_LINT_VERSION = 1.62.2

verify: GOLANGCI_LINT
	echo $(GO_SOURCES)
	$(GOLANGCI_LINT) run --out-format tab --config "${WORKSPACE_ROOT}/.golangci.yml"

GOLANGCI_LINT:
	${GOLANGCI_LINT} --version 2>/dev/null | grep ${GOLANGCI_LINT_VERSION} || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ${TOOLS_DIR} v${GOLANGCI_LINT_VERSION}

########## BUILD ##########
prereq::
	${GOCMD} install github.com/jstemmer/go-junit-report@v1.0.0

build::
	go env GOOS GOARCH
	go build -ldflags="${LINKERFLAGS}" -gcflags ${COMPILERFLAGS} -o ${BINARY_CLI}/worker-cli-plugin main.go


build-install:: build
	mkdir -p "${HOME}/.jfrog/plugins/worker/bin"
	mv ${BINARY_CLI}/worker-cli-plugin "${HOME}/.jfrog/plugins/worker/bin/worker"
	chmod +x "${HOME}/.jfrog/plugins/worker/bin/worker"

########## TEST ##########

.PHONY: clean-mock
clean-mock:
	@echo Cleaning generated mock files
	find . -path "*/mocks/*.go" -delete

.PHONY: generate-mock
generate-mock: clean-mock
	@echo Generating test mocks
	go generate ./...

test-prereq: prereq generate-mock
	mkdir -p target/reports

test: PACKAGES=./...
test: TEST_ARGS=-short
test: test-prereq do-run-tests

itest: PACKAGES=./test/...
itest: TAGS=-tags=itest
itest: TEST_ARGS=-count=1 -p=1
itest:: test-prereq do-run-tests

do-run-tests::
	$(SCRIPTS_DIR)/gotest.sh $$(go list $(TAGS) $(PACKAGES) | grep -v "^.*/mocks$$") -timeout 30m -coverpkg=github.com/jfrog/jfrog-cli-platform-services/... -coverprofile=$(TARGET_DIR)/reports/coverage.out $(TEST_ARGS) $(TAGS)

.PHONY: $(MAKECMDGOALS)
