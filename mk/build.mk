BUILD_INFO_GIT_TAG ?= $(shell git describe --tags 2>/dev/null || echo unknown)
BUILD_INFO_GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
BUILD_INFO_BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ" || echo unknown)
BUILD_INFO_VERSION ?= $(shell prefix=$$(echo $(BUILD_INFO_GIT_TAG) | cut -c 1); if [ "$${prefix}" = "v" ]; then echo $(BUILD_INFO_GIT_TAG) | cut -c 2- ; else echo $(BUILD_INFO_GIT_TAG) ; fi)

build_info_fields := \
	version=$(BUILD_INFO_VERSION) \
	gitTag=$(BUILD_INFO_GIT_TAG) \
	gitCommit=$(BUILD_INFO_GIT_COMMIT) \
	buildDate=$(BUILD_INFO_BUILD_DATE)
build_info_ld_flags := $(foreach entry,$(build_info_fields), -X github.com/kumahq/kuma/pkg/version.$(entry))

LD_FLAGS := -ldflags="-s -w $(build_info_ld_flags) $(EXTRA_LD_FLAGS)"
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
GOFLAGS :=

TOP := $(shell pwd)
BUILD_DIR ?= $(TOP)/build
BUILD_ARTIFACTS_DIR ?= $(BUILD_DIR)/artifacts-${GOOS}-${GOARCH}
BUILD_KUMACTL_DIR := ${BUILD_ARTIFACTS_DIR}/kumactl
export PATH := $(BUILD_KUMACTL_DIR):$(PATH)

GO_BUILD := GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=0 go build -v $(GOFLAGS) $(LD_FLAGS)
GO_BUILD_COREDNS := GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=0 go build -v

COREDNS_GIT_REPOSITORY ?= https://github.com/coredns/coredns.git
COREDNS_VERSION ?= v1.8.3
COREDNS_TMP_DIRECTORY ?= $(BUILD_DIR)/coredns
COREDNS_PLUGIN_CFG_PATH ?= $(TOP)/tools/builds/coredns/templates/plugin.cfg

.PHONY: build
build: build/kuma-cp build/kuma-dp build/kumactl build/kuma-prometheus-sd build/coredns ## Dev: Build all binaries

.PHONY: build/kuma-cp
build/kuma-cp: ## Dev: Build `Control Plane` binary
	$(GO_BUILD) -o ${BUILD_ARTIFACTS_DIR}/kuma-cp/kuma-cp ./app/kuma-cp

.PHONY: build/kuma-dp
build/kuma-dp: ## Dev: Build `kuma-dp` binary
	$(GO_BUILD) -o ${BUILD_ARTIFACTS_DIR}/kuma-dp/kuma-dp ./app/kuma-dp

.PHONY: build/kumactl
build/kumactl: ## Dev: Build `kumactl` binary
	$(GO_BUILD) -o $(BUILD_ARTIFACTS_DIR)/kumactl/kumactl ./app/kumactl

.PHONY: build/coredns
build/coredns:
	rm -rf "$(COREDNS_TMP_DIRECTORY)"
	git clone --branch $(COREDNS_VERSION) --depth 1 $(COREDNS_GIT_REPOSITORY) $(COREDNS_TMP_DIRECTORY)
	cp $(COREDNS_PLUGIN_CFG_PATH) $(COREDNS_TMP_DIRECTORY)
	cd $(COREDNS_TMP_DIRECTORY) && \
		GOOS= GOARCH= go generate coredns.go && \
		go get github.com/coredns/alternate && \
		$(GO_BUILD_COREDNS) -ldflags="-s -w -X github.com/coredns/coredns/coremain.GitCommit=$(shell git describe --dirty --always)" -o $(BUILD_ARTIFACTS_DIR)/coredns/coredns
	rm -rf "$(COREDNS_TMP_DIRECTORY)"

.PHONY: build/kuma-prometheus-sd
build/kuma-prometheus-sd: ## Dev: Build `kuma-prometheus-sd` binary
	$(GO_BUILD) -o ${BUILD_ARTIFACTS_DIR}/kuma-prometheus-sd/kuma-prometheus-sd ./app/kuma-prometheus-sd

.PHONY: build/kuma-cp/linux-amd64
build/kuma-cp/linux-amd64:
	GOOS=linux GOARCH=amd64 $(MAKE) build/kuma-cp

.PHONY: build/kuma-dp/linux-amd64
build/kuma-dp/linux-amd64:
	GOOS=linux GOARCH=amd64 $(MAKE) build/kuma-dp

.PHONY: build/kumactl/linux-amd64
build/kumactl/linux-amd64:
	GOOS=linux GOARCH=amd64 $(MAKE) build/kumactl

.PHONY: build/kuma-prometheus-sd/linux-amd64
build/kuma-prometheus-sd/linux-amd64:
	GOOS=linux GOARCH=amd64 $(MAKE) build/kuma-prometheus-sd

.PHONY: build/coredns/linux-amd64
build/coredns/linux-amd64:
	GOOS=linux GOARCH=amd64 $(MAKE) build/coredns

.PHONY: clean
clean: clean/build ## Dev: Clean

.PHONY: clean/build
clean/build: ## Dev: Remove build/ dir
	rm -rf "$(BUILD_DIR)"
