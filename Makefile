.PHONY: test build build-cli build-server build-mcp build-linux build-cli-pg smoke-pg-cli smoke-pg-cli-linux smoke-pg-cli-manylinux-baseline package-pg-cli-release test-e2e-cli test-e2e-cli-mysql test-e2e-cli-tidb test-e2e-mcp-mysql test-e2e-mcp-tidb test-e2e-http-mysql test-e2e-http-tidb

BUILD_DIR ?= bin
CGO_ENABLED ?= 0
PG_SMOKE_SQL ?= CREATE TABLE users (id serial primary key);
PG_GLIBC_BASELINE ?= GLIBC_2.17
PG_MANYLINUX_IMAGE ?= quay.io/pypa/manylinux2014_x86_64
PG_MANYLINUX_PLATFORM ?= linux/amd64

test:
	go test ./...

build: build-cli build-server build-mcp

build-cli:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) go build -o $(BUILD_DIR)/deltascope ./cmd/deltascope

# Phase 7 Slice 1 keeps the public PG artifact boundary locked to the CLI only.
# deltascope-server-pg and deltascope-mcp-pg stay out of the public release path here.
build-cli-pg:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build -tags postgresql -o $(BUILD_DIR)/deltascope-pg ./cmd/deltascope

smoke-pg-cli: build-cli-pg
	./$(BUILD_DIR)/deltascope-pg --version
	./$(BUILD_DIR)/deltascope-pg capabilities
	printf '%s\n' '$(PG_SMOKE_SQL)' | ./$(BUILD_DIR)/deltascope-pg audit --dialect postgresql --format json --fail-on none

# Phase 7 Slice 2 keeps the Linux PG smoke lane aligned with the local CLI smoke path.
# This validates an Ubuntu/Linux CGO build environment only; it is not a manylinux/glibc release guarantee.
smoke-pg-cli-linux: smoke-pg-cli
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/deltascope ./cmd/deltascope

# Phase 7 Slice 3 adds a reusable manylinux/glibc baseline gate for the only public PG v1 artifact.
# This verifies `deltascope-pg` inside a controlled Linux container and fails if the glibc baseline drifts above the approved threshold.
smoke-pg-cli-manylinux-baseline:
	PG_GLIBC_BASELINE=$(PG_GLIBC_BASELINE) PG_MANYLINUX_IMAGE=$(PG_MANYLINUX_IMAGE) PG_MANYLINUX_PLATFORM=$(PG_MANYLINUX_PLATFORM) bash ./scripts/verify_pg_manylinux_baseline.sh

# Phase 7 Slice 4 packages only the approved public PG v1 artifact after the manylinux/glibc gate passes.
# `deltascope-server-pg` and `deltascope-mcp-pg` are intentionally excluded from this public release path.
package-pg-cli-release: smoke-pg-cli-manylinux-baseline
	VERSION=$(VERSION) BUILD_DIR=$(BUILD_DIR) DIST_DIR=dist bash ./scripts/package_pg_cli_release.sh

build-server:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) go build -o $(BUILD_DIR)/deltascope-server ./cmd/deltascope-server

build-mcp:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) go build -o $(BUILD_DIR)/deltascope-mcp ./cmd/deltascope-mcp

build-linux:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/deltascope-linux-amd64 ./cmd/deltascope
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/deltascope-server-linux-amd64 ./cmd/deltascope-server
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/deltascope-mcp-linux-amd64 ./cmd/deltascope-mcp

test-e2e-cli: test-e2e-cli-mysql test-e2e-cli-tidb

test-e2e-cli-mysql: build-cli
	./scripts/test_cli_metadata_e2e.sh mysql

test-e2e-cli-tidb: build-cli
	./scripts/test_cli_metadata_e2e.sh tidb

test-e2e-mcp-mysql:
	./scripts/test_mcp_metadata_e2e.sh mysql

test-e2e-mcp-tidb:
	./scripts/test_mcp_metadata_e2e.sh tidb

test-e2e-http-mysql:
	./scripts/test_http_metadata_e2e.sh mysql

test-e2e-http-tidb:
	./scripts/test_http_metadata_e2e.sh tidb
