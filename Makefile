.PHONY: test release-test-gates build build-cli build-server build-mcp build-linux build-cli-pg smoke-pg-cli smoke-pg-host-surfaces smoke-pg-cli-linux smoke-pg-cli-manylinux-baseline smoke-pg-cli-manylinux-baseline-arm64 package-host-release-archive verify-pg-host-release-archive verify-pg-linux-release-archive verify-pg-linux-release-archive-arm64 package-pg-linux-release-archive-arm64 package-pg-cli-release test-e2e-cli test-e2e-cli-mysql test-e2e-cli-tidb test-e2e-mcp-mysql test-e2e-mcp-tidb test-e2e-http-mysql test-e2e-http-tidb test-e2e-cli-postgresql test-e2e-http-postgresql test-e2e-mcp-postgresql

BUILD_DIR ?= bin
CGO_ENABLED ?= 0
PG_SMOKE_SQL ?= CREATE TABLE users (id serial primary key);
PG_GLIBC_BASELINE ?= GLIBC_2.17
PG_MANYLINUX_IMAGE ?= quay.io/pypa/manylinux2014_x86_64
PG_MANYLINUX_PLATFORM ?= linux/amd64
GO_VERSION ?= $(shell go env GOVERSION | sed 's/^go//')

test:
	go test ./...

release-test-gates:
	go test ./...
	CGO_ENABLED=1 go test -tags postgresql ./internal/application/audit ./internal/interfaces/cli ./internal/interfaces/http ./internal/interfaces/mcp ./pkg/deltascope
	npm test --prefix packages/deltascope-mcp

build: build-cli build-server build-mcp

build-cli:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build -tags postgresql -o $(BUILD_DIR)/deltascope ./cmd/deltascope

# Transitional Package 1.4 policy:
# - `deltascope` is the primary PG-capable CLI entrypoint for local/unified builds.
# - `deltascope-pg` remains as a compatibility alias while the public release/install story is still converging.
# - deltascope-server-pg and deltascope-mcp-pg stay out of the public release path here.
build-cli-pg: build-cli
	cp ./$(BUILD_DIR)/deltascope ./$(BUILD_DIR)/deltascope-pg

smoke-pg-cli: build-cli-pg
	./$(BUILD_DIR)/deltascope --version
	./$(BUILD_DIR)/deltascope capabilities
	printf '%s\n' '$(PG_SMOKE_SQL)' | ./$(BUILD_DIR)/deltascope audit --dialect postgresql --format json --fail-on none
	./$(BUILD_DIR)/deltascope-pg --version >/dev/null

# Host-native PG-capable smoke for the unified surfaces.
# This is the portable baseline used by non-Linux smoke lanes before release-matrix convergence.
smoke-pg-host-surfaces: build
	./$(BUILD_DIR)/deltascope --version
	./$(BUILD_DIR)/deltascope capabilities
	printf '%s\n' '$(PG_SMOKE_SQL)' | ./$(BUILD_DIR)/deltascope audit --dialect postgresql --format json --fail-on none
	./$(BUILD_DIR)/deltascope-server --version
	./$(BUILD_DIR)/deltascope-mcp --version

# Host-native archive packaging truth for future darwin/linux-arm release convergence.
package-host-release-archive: smoke-pg-host-surfaces
	rm -rf dist
	VERSION=$(VERSION) BUILD_DIR=$(BUILD_DIR) DIST_DIR=dist bash ./scripts/package_host_release_archive.sh

verify-pg-host-release-archive:
	$(MAKE) package-host-release-archive VERSION=v0.0.0-dev BUILD_DIR=$(BUILD_DIR)
	os="$$(uname -s | tr '[:upper:]' '[:lower:]')"; \
	arch="$$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"; \
	archive="$$(ls dist/deltascope_*_"$$os"_"$$arch".tar.gz | head -n 1)"; \
	checksum="$$(ls dist/deltascope_*_"$$os"_"$$arch"_checksums.txt | head -n 1)"; \
	archive_contents="$$(mktemp)"; \
	trap 'rm -f "$$archive_contents"' EXIT; \
	test -n "$$archive"; \
	test -n "$$checksum"; \
	test -f "$$archive"; \
	test -f "$$checksum"; \
	tar -tzf "$$archive" > "$$archive_contents"; \
	grep -q '^deltascope$$' "$$archive_contents"; \
	grep -q '^deltascope-server$$' "$$archive_contents"; \
	grep -q '^deltascope-mcp$$' "$$archive_contents"; \
	grep -q '^README.md$$' "$$archive_contents"; \
	grep -q '^README_ZH.md$$' "$$archive_contents"; \
	grep -q '^CHANGELOG.md$$' "$$archive_contents"; \
	grep -q '^SECURITY.md$$' "$$archive_contents"; \
	grep -q "  $$(basename "$$archive")$$" "$$checksum"

# Phase 7 Slice 2 keeps the Linux PG smoke lane aligned with the local CLI smoke path.
# This validates an Ubuntu/Linux CGO build environment only; it is not a manylinux/glibc release guarantee.
smoke-pg-cli-linux: smoke-pg-cli
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/deltascope ./cmd/deltascope

# Phase 7 Slice 3 adds a reusable manylinux/glibc baseline gate for the only public PG v1 artifact.
# This verifies `deltascope-pg` inside a controlled Linux container and fails if the glibc baseline drifts above the approved threshold.
smoke-pg-cli-manylinux-baseline:
	PG_GLIBC_BASELINE=$(PG_GLIBC_BASELINE) PG_MANYLINUX_IMAGE=$(PG_MANYLINUX_IMAGE) PG_MANYLINUX_PLATFORM=$(PG_MANYLINUX_PLATFORM) PG_TARGET_ARCH=amd64 PG_GO_TARBALL_ARCH=amd64 PG_TRANSITIONAL_ALIAS=1 bash ./scripts/verify_pg_manylinux_baseline.sh

smoke-pg-cli-manylinux-baseline-arm64:
	PG_GLIBC_BASELINE=$(PG_GLIBC_BASELINE) PG_MANYLINUX_IMAGE=quay.io/pypa/manylinux2014_aarch64 PG_MANYLINUX_PLATFORM=linux/arm64 PG_TARGET_ARCH=arm64 PG_GO_TARBALL_ARCH=arm64 PG_TRANSITIONAL_ALIAS=0 bash ./scripts/verify_pg_manylinux_baseline.sh

# Release validation closure: verify the actual Linux amd64 PG GoReleaser archive inside a Linux container.
# This keeps Linux CGO truth on the Linux/container path and avoids pretending a Darwin host can validate it.
verify-pg-linux-release-archive:
	set -eu; \
	rm -rf dist; \
	docker run --rm \
		--platform $(PG_MANYLINUX_PLATFORM) \
		--user "$$(id -u):$$(id -g)" \
		-v "$$(pwd):/work" \
		-w /work \
		-e GO_VERSION="$(GO_VERSION)" \
		-e HOME=/tmp/deltascope-home \
		$(PG_MANYLINUX_IMAGE) \
		bash -lc 'set -euo pipefail; mkdir -p "$$HOME" /tmp/gobin; GO_TARBALL="go$${GO_VERSION}.linux-amd64.tar.gz"; curl -fsSLo "/tmp/$${GO_TARBALL}" "https://go.dev/dl/$${GO_TARBALL}"; rm -rf /tmp/go; tar -C /tmp -xzf "/tmp/$${GO_TARBALL}"; export GOBIN=/tmp/gobin; export PATH="/tmp/go/bin:$$GOBIN:$$PATH"; go install github.com/goreleaser/goreleaser/v2@v2.12.7; goreleaser release --config .goreleaser.pg-smoke.yml --clean --snapshot --skip=publish --skip=announce --skip=sign --skip=sbom'; \
	archive="$$(ls dist/deltascope_*_linux_amd64.tar.gz | head -n 1)"; \
	checksum="$$(ls dist/deltascope_*_checksums.txt | head -n 1)"; \
	archive_contents="$$(mktemp)"; \
	trap 'rm -f "$$archive_contents"' EXIT; \
	test -n "$$archive"; \
	test -n "$$checksum"; \
	test -f "$$archive"; \
	test -f "$$checksum"; \
	tar -tzf "$$archive" > "$$archive_contents"; \
	grep -q '^deltascope$$' "$$archive_contents"; \
	grep -q '^deltascope-server$$' "$$archive_contents"; \
	grep -q '^deltascope-mcp$$' "$$archive_contents"; \
	grep -q '^README.md$$' "$$archive_contents"; \
	grep -q '^README_ZH.md$$' "$$archive_contents"; \
	grep -q '^CHANGELOG.md$$' "$$archive_contents"; \
	grep -q '^SECURITY.md$$' "$$archive_contents"; \
	grep -q "  $$(basename "$$archive")$$" "$$checksum"

verify-pg-linux-release-archive-arm64:
	set -eu; \
	rm -rf dist; \
	docker run --rm \
		--platform linux/arm64 \
		--user "$$(id -u):$$(id -g)" \
		-v "$$(pwd):/work" \
		-w /work \
		-e GO_VERSION="$(GO_VERSION)" \
		-e HOME=/tmp/deltascope-home \
		quay.io/pypa/manylinux2014_aarch64 \
		bash -lc 'set -euo pipefail; mkdir -p "$$HOME" /tmp/gobin; GO_TARBALL="go$${GO_VERSION}.linux-arm64.tar.gz"; curl -fsSLo "/tmp/$${GO_TARBALL}" "https://go.dev/dl/$${GO_TARBALL}"; rm -rf /tmp/go; tar -C /tmp -xzf "/tmp/$${GO_TARBALL}"; export GOBIN=/tmp/gobin; export PATH="/tmp/go/bin:$$GOBIN:$$PATH"; go install github.com/goreleaser/goreleaser/v2@v2.12.7; goreleaser release --config .goreleaser.pg-smoke-arm64.yml --clean --snapshot --skip=publish --skip=announce --skip=sign --skip=sbom'; \
	archive="$$(ls dist/deltascope_*_linux_arm64.tar.gz | head -n 1)"; \
	checksum="$$(ls dist/deltascope_*_checksums.txt | head -n 1)"; \
	archive_contents="$$(mktemp)"; \
	trap 'rm -f "$$archive_contents"' EXIT; \
	test -n "$$archive"; \
	test -n "$$checksum"; \
	test -f "$$archive"; \
	test -f "$$checksum"; \
	tar -tzf "$$archive" > "$$archive_contents"; \
	grep -q '^deltascope$$' "$$archive_contents"; \
	grep -q '^deltascope-server$$' "$$archive_contents"; \
	grep -q '^deltascope-mcp$$' "$$archive_contents"; \
	grep -q '^README.md$$' "$$archive_contents"; \
	grep -q '^README_ZH.md$$' "$$archive_contents"; \
	grep -q '^CHANGELOG.md$$' "$$archive_contents"; \
	grep -q '^SECURITY.md$$' "$$archive_contents"; \
	grep -q "  $$(basename "$$archive")$$" "$$checksum"

package-pg-linux-release-archive-arm64:
	set -eu; \
	host_worktree="$$(pwd)"; \
	rm -rf dist; \
	mkdir -p dist; \
	docker run --rm \
		--platform linux/arm64 \
		--user "$$(id -u):$$(id -g)" \
		-v "$$host_worktree:/work" \
		-v "$$host_worktree/dist:/out" \
		-w /work \
		-e GO_VERSION="$(GO_VERSION)" \
		-e RELEASE_VERSION="$(VERSION)" \
		-e HOME=/tmp/deltascope-home \
		quay.io/pypa/manylinux2014_aarch64 \
		bash -lc 'set -euo pipefail; mkdir -p "$$HOME" /tmp/gobin /tmp/release-src; GO_TARBALL="go$${GO_VERSION}.linux-arm64.tar.gz"; curl -fsSLo "/tmp/$${GO_TARBALL}" "https://go.dev/dl/$${GO_TARBALL}"; rm -rf /tmp/go; tar -C /tmp -xzf "/tmp/$${GO_TARBALL}"; export GOBIN=/tmp/gobin; export PATH="/tmp/go/bin:$$GOBIN:$$PATH"; go install github.com/goreleaser/goreleaser/v2@v2.12.7; tar --exclude=.git -C /work -cf - . | tar -C /tmp/release-src -xf -; cd /tmp/release-src; git init -q; git config user.name "release-bot"; git config user.email "release-bot@example.com"; git remote add origin https://github.com/Fanduzi/DeltaScope.git; git add .; git commit -qm "release snapshot"; git tag "$$RELEASE_VERSION"; goreleaser release --config .goreleaser.pg-smoke-arm64.yml --clean --skip=publish --skip=announce --skip=sign --skip=sbom; cp dist/deltascope_*_linux_arm64.tar.gz /out/; cp dist/deltascope_*_checksums.txt /out/'; \
	archive="$$(ls dist/deltascope_*_linux_arm64.tar.gz | head -n 1)"; \
	archive_base="$$(basename "$$archive")"; \
	prefix="$${archive_base%_linux_arm64.tar.gz}"; \
	generic_checksum="dist/$${prefix}_checksums.txt"; \
	platform_checksum="dist/$${prefix}_linux_arm64_checksums.txt"; \
	test -f "$$archive"; \
	test -f "$$generic_checksum"; \
	mv "$$generic_checksum" "$$platform_checksum"; \
	grep -q "  $${archive_base}$$" "$$platform_checksum"

# Phase 7 Slice 4 packages only the approved public PG v1 artifact after the manylinux/glibc gate passes.
# `deltascope-server-pg` and `deltascope-mcp-pg` are intentionally excluded from this public release path.
package-pg-cli-release: smoke-pg-cli-manylinux-baseline
	VERSION=$(VERSION) BUILD_DIR=$(BUILD_DIR) DIST_DIR=dist bash ./scripts/package_pg_cli_release.sh

build-server:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build -tags postgresql -o $(BUILD_DIR)/deltascope-server ./cmd/deltascope-server

build-mcp:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build -tags postgresql -o $(BUILD_DIR)/deltascope-mcp ./cmd/deltascope-mcp

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

test-e2e-cli-postgresql: build-cli
	./scripts/test_cli_metadata_e2e_postgresql.sh

test-e2e-http-postgresql:
	./scripts/test_http_metadata_e2e_postgresql.sh

test-e2e-mcp-postgresql:
	./scripts/test_mcp_metadata_e2e_postgresql.sh
