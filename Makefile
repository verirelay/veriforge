.PHONY: build mock-init forge run-qemu test lint check clean

DIST_DIR := dist
CACHE_DIR := .veriforge_cache
VERIFORGE := $(DIST_DIR)/veriforge
MOCK_INIT := $(DIST_DIR)/mock-init
INITRD := $(DIST_DIR)/initrd.img
VMLINUX := $(DIST_DIR)/vmlinuz
BUILD_REPORT := $(DIST_DIR)/build-report.json
VERIRELAY_REPOSITORY ?=
VERIRELAY_COMMIT ?=
VCPU_TYPE ?= EPYC-v4
KERNEL_APPEND ?=

build:
	mkdir -p $(DIST_DIR)
	go build -o $(VERIFORGE) ./cmd/veriforge

mock-init:
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(MOCK_INIT) ./examples/mock-init

forge: build mock-init
	@test -n "$(VERIRELAY_REPOSITORY)" || (echo "VERIRELAY_REPOSITORY is required" >&2; exit 1)
	@test -n "$(VERIRELAY_COMMIT)" || (echo "VERIRELAY_COMMIT is required" >&2; exit 1)
	@test -n "$(KERNEL_APPEND)" || (echo "KERNEL_APPEND is required" >&2; exit 1)
	$(VERIFORGE) \
		-init $(MOCK_INIT) \
		-verirelay-repository $(VERIRELAY_REPOSITORY) \
		-verirelay-commit $(VERIRELAY_COMMIT) \
		-vcpu-type $(VCPU_TYPE) \
		-kernel-append "$(KERNEL_APPEND)" \
		-out-initrd $(INITRD) \
		-out-vmlinuz $(VMLINUX) \
		-out-report $(BUILD_REPORT) \
		-cache $(CACHE_DIR)

run-qemu:
	./scripts/run-qemu.sh

test:
	go test ./...

lint:
	golangci-lint run

check: test build mock-init lint

clean:
	rm -rf $(DIST_DIR)
