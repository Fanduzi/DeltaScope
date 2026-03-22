.PHONY: test build build-cli build-server test-e2e-cli test-e2e-cli-mysql test-e2e-cli-tidb

BUILD_DIR ?= .build/bin

test:
	go test ./...

build: build-cli build-server

build-cli:
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/deltascope ./cmd/deltascope

build-server:
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/deltascope-server ./cmd/deltascope-server

test-e2e-cli: test-e2e-cli-mysql test-e2e-cli-tidb

test-e2e-cli-mysql: build-cli
	./scripts/test_cli_metadata_e2e.sh mysql

test-e2e-cli-tidb: build-cli
	./scripts/test_cli_metadata_e2e.sh tidb
