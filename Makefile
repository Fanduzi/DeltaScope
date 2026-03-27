.PHONY: test build build-cli build-server build-mcp build-linux test-e2e-cli test-e2e-cli-mysql test-e2e-cli-tidb test-e2e-mcp-mysql test-e2e-mcp-tidb

BUILD_DIR ?= bin

test:
	go test ./...

build: build-cli build-server build-mcp

build-cli:
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/deltascope ./cmd/deltascope

build-server:
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/deltascope-server ./cmd/deltascope-server

build-mcp:
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/deltascope-mcp ./cmd/deltascope-mcp

build-linux:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/deltascope-linux-amd64 ./cmd/deltascope
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/deltascope-server-linux-amd64 ./cmd/deltascope-server
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/deltascope-mcp-linux-amd64 ./cmd/deltascope-mcp

test-e2e-cli: test-e2e-cli-mysql test-e2e-cli-tidb

test-e2e-cli-mysql: build-cli
	./scripts/test_cli_metadata_e2e.sh mysql

test-e2e-cli-tidb: build-cli
	./scripts/test_cli_metadata_e2e.sh tidb

test-e2e-mcp-mysql:
	./scripts/test_mcp_metadata_e2e.sh mysql

test-e2e-mcp-tidb:
	./scripts/test_mcp_metadata_e2e.sh tidb
