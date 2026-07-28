.PHONY: test sql-corpus-gates query-access-corpus-gates sql-corpus-report ddl-census-report ddl-parser-error-feasibility-report parser-upgrade-candidate-evidence-report ddl-coverage-catalog-test parser-error-unsupported-contract-test unsupported-diagnostics-evidence-test release-test-gates build build-cli build-server build-mcp build-linux smoke-pg-cli smoke-pg-host-surfaces smoke-pg-cli-linux smoke-pg-cli-manylinux-baseline smoke-pg-cli-manylinux-baseline-arm64 package-host-release-archive verify-pg-host-release-archive verify-pg-linux-release-archive verify-pg-linux-release-archive-cn verify-pg-linux-release-archive-arm64 package-pg-linux-release-archive-amd64 package-pg-linux-release-archive-arm64 test-e2e-cli test-e2e-cli-mysql test-e2e-cli-tidb test-e2e-mcp-mysql test-e2e-mcp-tidb test-e2e-http-mysql test-e2e-http-tidb test-e2e-cli-postgresql test-e2e-cli-postgresql-metadata-objects test-e2e-http-postgresql test-e2e-mcp-postgresql test-e2e-cli-tls test-e2e-cli-tls-regression pg-unit-test-gates pg-e2e-gates pg-confidence-gates docs-example-gates release-surface-gates release-version-surface-gates release-version-contract-gates release-local-version-smoke release-dialect-hygiene-gates release-gitlab-codequality-smoke release-source-location-smoke release-workflow-hygiene-gates release-contract-gates release-consistency-test release-recovery-preflight release-recovery-contract-test release-tag-annotation-test release-tag-annotation-gate lint lint-fix lint-landing decision-record-gate release-tag-candidate-gate

BUILD_DIR ?= bin
CGO_ENABLED ?= 0
PG_SMOKE_SQL ?= CREATE TABLE users (id serial primary key);
PG_GLIBC_BASELINE ?= GLIBC_2.17
PG_MANYLINUX_IMAGE ?= quay.io/pypa/manylinux2014_x86_64
PG_MANYLINUX_PLATFORM ?= linux/amd64
GO_VERSION ?= $(shell go env GOVERSION | sed 's/^go//')
CLI_VERSION_LDFLAGS = $(if $(VERSION),-ldflags "-X github.com/Fanduzi/DeltaScope/internal/interfaces/cli.Version=$(VERSION)")
MAIN_VERSION_LDFLAGS = $(if $(VERSION),-ldflags "-X main.Version=$(VERSION)")

test:
	go test ./...

sql-corpus-gates:
	go test ./internal/application/audit -run 'TestSQLCorpusExpectedFilesAreWellFormed|TestSQLCorpusCoversSupportedRuleDialects' -tags postgresql -count=1

query-access-corpus-gates:
	go test ./internal/application/queryaccess/ -run TestQueryAccessCorpus -count=1
	go test -tags postgresql ./internal/application/queryaccess/ -run TestQueryAccessCorpusPostgreSQL -count=1

sql-corpus-report:
	go test ./internal/application/audit -run TestSQLCorpusPrintSupportedRuleCoverageInventory -tags postgresql -count=1 -v

ddl-census-report:
	go test ./internal/application/audit -count=1 -run TestCrossDialectDDLCoverageCensus -v
	go test ./internal/application/audit -count=1 -tags postgresql -run TestPostgreSQLDDLConsolidatedCoverageCensus -v

ddl-parser-error-feasibility-report:
	go test ./internal/application/audit -count=1 -run TestDDLParserErrorFeasibilityCensus -v
	go test ./internal/application/audit -count=1 -tags postgresql -run TestDDLParserErrorFeasibilityPostgreSQLCensus -v

parser-upgrade-candidate-evidence-report:
	$(MAKE) ddl-parser-error-feasibility-report

ddl-coverage-catalog-test:
	go test ./internal/application/audit -count=1 -tags postgresql -run TestDDLCoverageCatalog

parser-error-unsupported-contract-test:
	go test ./internal/application/audit -count=1 -run TestDDLParserErrorUnsupportedContract
	go test ./internal/application/audit -count=1 -tags postgresql -run TestDDLParserErrorUnsupportedContract
	go test ./pkg/deltascope -count=1 -run ParserErrorUnsupportedContract
	go test ./internal/interfaces/cli -count=1 -run CLIparserErrorUnsupported
	go test ./internal/interfaces/http -count=1 -run HandlerAuditParserErrorUnsupportedContract
	go test ./internal/interfaces/mcp -count=1 -run MCPParserErrorUnsupportedContract

unsupported-diagnostics-evidence-test:
	go test ./internal/application/audit -count=1 -run TestParserErrorDiagnosticEvidence
	go test ./internal/application/audit -count=1 -tags postgresql -run TestUnsupportedStatementDiagnosticEvidencePostgreSQL
	go test ./pkg/deltascope -count=1 -run TestUnsupportedDiagnosticsEvidenceSDKParserError
	go test ./internal/interfaces/cli -count=1 -run TestUnsupportedDiagnosticsEvidenceCLI
	go test ./internal/interfaces/http -count=1 -run TestUnsupportedDiagnosticsEvidenceHTTPParserError
	go test ./internal/interfaces/mcp -count=1 -run TestUnsupportedDiagnosticsEvidenceMCPParserError

release-test-gates:
	go test ./...
	$(MAKE) sql-corpus-gates
	CGO_ENABLED=1 go test -tags postgresql ./internal/application/audit ./internal/interfaces/cli ./internal/interfaces/http ./internal/interfaces/mcp ./pkg/deltascope
	$(MAKE) test-e2e-cli-tls
	npm test --prefix packages/deltascope-mcp

build: build-cli build-server build-mcp

build-cli:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build -tags postgresql $(CLI_VERSION_LDFLAGS) -o $(BUILD_DIR)/deltascope ./cmd/deltascope

smoke-pg-cli: build-cli
	$(BUILD_DIR)/deltascope --version
	$(BUILD_DIR)/deltascope capabilities
	printf '%s\n' '$(PG_SMOKE_SQL)' | $(BUILD_DIR)/deltascope audit --dialect postgresql --format json --fail-on none

# Host-native PG-capable smoke for the unified surfaces.
# This is the portable baseline used by non-Linux smoke lanes before release-matrix convergence.
smoke-pg-host-surfaces: build
	$(BUILD_DIR)/deltascope --version
	$(BUILD_DIR)/deltascope capabilities
	printf '%s\n' '$(PG_SMOKE_SQL)' | $(BUILD_DIR)/deltascope audit --dialect postgresql --format json --fail-on none
	$(BUILD_DIR)/deltascope-server --version
	$(BUILD_DIR)/deltascope-mcp --version

# Host-native archive packaging truth for future darwin/linux-arm release convergence.
package-host-release-archive: smoke-pg-host-surfaces
	rm -rf dist
	VERSION=$(VERSION) BUILD_DIR=$(BUILD_DIR) DIST_DIR=dist bash ./scripts/package_host_release_archive.sh
	os="$$(uname -s | tr '[:upper:]' '[:lower:]')"; \
	arch="$$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"; \
	archive="$$(ls dist/deltascope_*_"$$os"_"$$arch".tar.gz | head -n 1)"; \
	checksum="$$(ls dist/deltascope_*_"$$os"_"$$arch"_checksums.txt | head -n 1)"; \
	VERSION=$(VERSION) ARCHIVE="$$archive" CHECKSUM="$$checksum" bash ./scripts/verify_release_archive.sh

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

# Phase 7 Slice 3 adds a reusable manylinux/glibc baseline gate for the converged Linux PG-capable binaries.
# This verifies the main Linux binaries inside a controlled Linux container and fails if the glibc baseline drifts above the approved threshold.
smoke-pg-cli-manylinux-baseline:
	PG_GLIBC_BASELINE=$(PG_GLIBC_BASELINE) PG_MANYLINUX_IMAGE=$(PG_MANYLINUX_IMAGE) PG_MANYLINUX_PLATFORM=$(PG_MANYLINUX_PLATFORM) PG_TARGET_ARCH=amd64 PG_GO_TARBALL_ARCH=amd64 bash ./scripts/verify_pg_manylinux_baseline.sh

smoke-pg-cli-manylinux-baseline-arm64:
	PG_GLIBC_BASELINE=$(PG_GLIBC_BASELINE) PG_MANYLINUX_IMAGE=quay.io/pypa/manylinux2014_aarch64 PG_MANYLINUX_PLATFORM=linux/arm64 PG_TARGET_ARCH=arm64 PG_GO_TARBALL_ARCH=arm64 bash ./scripts/verify_pg_manylinux_baseline.sh

# Release validation closure: verify the actual Linux amd64 PG GoReleaser archive inside a Linux container.
# This keeps Linux CGO truth on the Linux/container path and avoids pretending a Darwin host can validate it.
verify-pg-linux-release-archive:
	set -eu; \
	version="$(VERSION)"; \
	if [ -z "$$version" ]; then version="v0.0.0-dev"; fi; \
	$(MAKE) package-pg-linux-release-archive-amd64 VERSION="$$version"

# Local convenience wrapper for constrained-network hosts.
# Uses a domestic Go proxy first and disables checksum DB lookups that often stall or EOF locally.
verify-pg-linux-release-archive-cn:
	set -eu; \
	version="$(VERSION)"; \
	if [ -z "$$version" ]; then version="v0.0.0-dev"; fi; \
	GOPROXY="$${GOPROXY:-https://goproxy.cn,direct}" \
	GOSUMDB="$${GOSUMDB:-off}" \
	$(MAKE) verify-pg-linux-release-archive VERSION="$$version"

verify-pg-linux-release-archive-arm64:
	set -eu; \
	version="$(VERSION)"; \
	if [ -z "$$version" ]; then version="v0.0.0-dev"; fi; \
	$(MAKE) package-pg-linux-release-archive-arm64 VERSION="$$version"

package-pg-linux-release-archive-amd64:
	set -eu; \
	host_worktree="$$(pwd)"; \
	docker_env_args=""; \
	for env_name in HTTP_PROXY HTTPS_PROXY ALL_PROXY NO_PROXY http_proxy https_proxy all_proxy no_proxy GOPROXY GOSUMDB GONOSUMDB GONOPROXY GOPRIVATE; do \
		eval "env_value=\$${$$env_name-}"; \
		if [ -n "$$env_value" ]; then \
			docker_env_args="$$docker_env_args -e $$env_name=$$env_value"; \
		fi; \
	done; \
	rm -rf dist; \
	mkdir -p dist; \
	docker run --rm \
		--platform $(PG_MANYLINUX_PLATFORM) \
		--user "$$(id -u):$$(id -g)" \
		-v "$$host_worktree:/work" \
		-v "$$host_worktree/dist:/out" \
		-w /work \
		-e GO_VERSION="$(GO_VERSION)" \
		-e RELEASE_VERSION="$(VERSION)" \
		-e HOME=/tmp/deltascope-home \
		$$docker_env_args \
		$(PG_MANYLINUX_IMAGE) \
		bash -lc 'set -euo pipefail; mkdir -p "$$HOME" /tmp/gobin /tmp/release-src; GO_TARBALL="go$${GO_VERSION}.linux-amd64.tar.gz"; curl -fsSLo "/tmp/$${GO_TARBALL}" "https://go.dev/dl/$${GO_TARBALL}"; rm -rf /tmp/go; tar -C /tmp -xzf "/tmp/$${GO_TARBALL}"; export GOBIN=/tmp/gobin; export PATH="/tmp/go/bin:$$GOBIN:$$PATH"; go install github.com/goreleaser/goreleaser/v2@v2.12.7; tar --exclude=.git -C /work -cf - . | tar -C /tmp/release-src -xf -; cd /tmp/release-src; git init -q; git config user.name "release-bot"; git config user.email "release-bot@example.com"; git remote add origin https://github.com/Fanduzi/DeltaScope.git; git add .; git commit -qm "release snapshot"; git tag "$$RELEASE_VERSION"; goreleaser release --config .goreleaser.pg-smoke.yml --clean --skip=publish --skip=announce --skip=sign --skip=sbom; cp dist/deltascope_*_linux_amd64.tar.gz /out/; cp dist/deltascope_*_checksums.txt /out/'; \
	archive="$$(ls dist/deltascope_*_linux_amd64.tar.gz | head -n 1)"; \
	archive_base="$$(basename "$$archive")"; \
	prefix="$${archive_base%_linux_amd64.tar.gz}"; \
	generic_checksum="dist/$${prefix}_checksums.txt"; \
	platform_checksum="dist/$${prefix}_linux_amd64_checksums.txt"; \
	test -f "$$archive"; \
	test -f "$$generic_checksum"; \
	cp "$$generic_checksum" "$$platform_checksum"; \
	grep -q "  $${archive_base}$$" "$$platform_checksum"; \
	docker run --rm \
		--platform $(PG_MANYLINUX_PLATFORM) \
		--user "$$(id -u):$$(id -g)" \
		-v "$$host_worktree:/work" \
		-w /work \
		-e VERSION="$(VERSION)" \
		-e ARCHIVE="/work/$$archive" \
		-e CHECKSUM="/work/$$platform_checksum" \
		-e GLIBC_BASELINE="$(PG_GLIBC_BASELINE)" \
		$$docker_env_args \
		$(PG_MANYLINUX_IMAGE) \
		bash ./scripts/verify_release_archive.sh

package-pg-linux-release-archive-arm64:
	set -eu; \
	host_worktree="$$(pwd)"; \
	docker_env_args=""; \
	for env_name in HTTP_PROXY HTTPS_PROXY ALL_PROXY NO_PROXY http_proxy https_proxy all_proxy no_proxy GOPROXY GOSUMDB GONOSUMDB GONOPROXY GOPRIVATE; do \
		eval "env_value=\$${$$env_name-}"; \
		if [ -n "$$env_value" ]; then \
			docker_env_args="$$docker_env_args -e $$env_name=$$env_value"; \
		fi; \
	done; \
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
		$$docker_env_args \
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
	grep -q "  $${archive_base}$$" "$$platform_checksum"; \
	docker run --rm \
		--platform linux/arm64 \
		--user "$$(id -u):$$(id -g)" \
		-v "$$host_worktree:/work" \
		-w /work \
		-e VERSION="$(VERSION)" \
		-e ARCHIVE="/work/$$archive" \
		-e CHECKSUM="/work/$$platform_checksum" \
		-e GLIBC_BASELINE="$(PG_GLIBC_BASELINE)" \
		$$docker_env_args \
		quay.io/pypa/manylinux2014_aarch64 \
		bash ./scripts/verify_release_archive.sh

build-server:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build -tags postgresql $(MAIN_VERSION_LDFLAGS) -o $(BUILD_DIR)/deltascope-server ./cmd/deltascope-server

build-mcp:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build -tags postgresql $(MAIN_VERSION_LDFLAGS) -o $(BUILD_DIR)/deltascope-mcp ./cmd/deltascope-mcp

build-linux:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 go build $(CLI_VERSION_LDFLAGS) -o $(BUILD_DIR)/deltascope-linux-amd64 ./cmd/deltascope
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 go build $(MAIN_VERSION_LDFLAGS) -o $(BUILD_DIR)/deltascope-server-linux-amd64 ./cmd/deltascope-server
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 go build $(MAIN_VERSION_LDFLAGS) -o $(BUILD_DIR)/deltascope-mcp-linux-amd64 ./cmd/deltascope-mcp

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

test-e2e-cli-postgresql-metadata-objects: build-cli
	./scripts/test_cli_metadata_e2e_postgresql_objects.sh

test-e2e-http-postgresql:
	./scripts/test_http_metadata_e2e_postgresql.sh

test-e2e-mcp-postgresql:
	./scripts/test_mcp_metadata_e2e_postgresql.sh

# test-e2e-http-tls: run TLS-enabled HTTP audit E2E tests.
test-e2e-http-tls:
	./scripts/test_http_tls_e2e.sh

# test-e2e-cli-tls: run TLS-enabled CLI audit and query-access E2E tests.
# Required mode — fails closed when Docker is unavailable.
test-e2e-cli-tls: build-cli
	DELTASCOPE_CLI_BIN="$(BUILD_DIR)/deltascope" DELTASCOPE_CLI_TLS_E2E_REQUIRED=1 ./scripts/test_cli_tls_e2e.sh

# test-e2e-cli-tls-regression: verify CLI TLS fixture lifecycle (dynamic ports, cleanup, Docker policy).
test-e2e-cli-tls-regression:
	./scripts/test_cli_tls_e2e_regression.sh

# Canonical PostgreSQL confidence gates (v0.22.0).
# These targets compose existing commands into reusable confidence entry-points.

# pg-unit-test-gates: run all PostgreSQL-tagged unit tests (no Docker required).
pg-unit-test-gates:
	CGO_ENABLED=1 go test -tags postgresql -count=1 ./internal/infrastructure/parser/postgresql ./internal/application/audit ./internal/application/auditmeta ./internal/interfaces/cli ./internal/interfaces/http ./internal/interfaces/mcp ./pkg/deltascope

# pg-e2e-gates: run all three Docker-backed PostgreSQL E2E suites.
pg-e2e-gates:
	$(MAKE) test-e2e-cli-postgresql
	$(MAKE) test-e2e-http-postgresql
	$(MAKE) test-e2e-mcp-postgresql

# pg-confidence-gates: run all PostgreSQL-tagged unit tests plus Docker E2E.
pg-confidence-gates:
	$(MAKE) pg-unit-test-gates
	$(MAKE) pg-e2e-gates

# docs-example-gates: static drift check for current public docs and CI examples.
# VERSION is forwarded to the checker; when unset the version-pin check is skipped.
docs-example-gates:
	@VERSION="$(VERSION)" python3 ./scripts/verify_docs_examples.py

# release-surface-gates: verify reusable package/release invariants that should block release.
# Includes docs-example-gates (public docs/examples drift) plus the package/version/README invariants.
# VERSION may be passed explicitly (for tag release checks) and otherwise defaults to the current MCP launcher package version.
release-surface-gates: docs-example-gates
	@set -eu; \
	version="$(VERSION)"; \
	if [ -z "$$version" ]; then \
		version="v$$(node -p 'require("./packages/deltascope-mcp/package.json").version')"; \
	fi; \
	version_no_v="$${version#v}"; \
	test "$$(node -p 'require("./packages/deltascope-mcp/package.json").version')" = "$$version_no_v"; \
	grep -q "DefaultVersion = \"$$version\"" pkg/deltascope/version.go; \
	if grep -Eq 'Pack \(v[0-9]+\.[0-9]+\.[0-9]+\)|Previous milestone|上一里程碑' README.md README_ZH.md; then \
		echo "README files must not contain release-history milestone sections; use release notes instead" >&2; \
		exit 1; \
	fi; \
	(cd packages/deltascope-mcp && npm pack --dry-run)

# release-version-surface-gates: verify versioned docs surfaces for the current release.
# This stays separate from the release-blocking package contract so docs wording can evolve independently.
release-version-surface-gates:
	@VERSION="$(VERSION)" bash ./scripts/verify_release_version_surfaces.sh
	@VERSION="$(VERSION)" python3 ./scripts/verify_release_consistency.py

release-version-contract-gates: release-version-surface-gates

# release-local-version-smoke: build all binaries with VERSION ldflags and verify version output.
release-local-version-smoke:
	@test -n "$(VERSION)" || (echo "VERSION is required" >&2; exit 1)
	$(MAKE) build VERSION="$(VERSION)"
	$(BUILD_DIR)/deltascope --version | grep -q "$(VERSION)"
	$(BUILD_DIR)/deltascope --version | grep -q "postgresql"
	$(BUILD_DIR)/deltascope-server --version | grep -q "^$(VERSION)$$"
	$(BUILD_DIR)/deltascope-mcp -version | grep -q "^$(VERSION)$$"

# release-dialect-hygiene-gates: verify default-policy dialect isolation across PG, MySQL, and TiDB.
release-dialect-hygiene-gates: build-cli
	DELTASCOPE_BIN="$(BUILD_DIR)/deltascope" bash ./scripts/verify_release_dialect_hygiene.sh

# release-gitlab-codequality-smoke: validate GitLab Code Quality JSON output contract against a built CLI binary.
release-gitlab-codequality-smoke: build-cli
	DELTASCOPE_BIN="$(BUILD_DIR)/deltascope" bash ./scripts/verify_gitlab_codequality_output.sh

# release-source-location-smoke: validate source location fidelity across GitHub Actions, SARIF, GitLab Code Quality, and TiDB outputs.
release-source-location-smoke: build-cli
	DELTASCOPE_BIN="$(BUILD_DIR)/deltascope" bash ./scripts/verify_source_location_fidelity.sh

# release-workflow-hygiene-gates: validate release workflow avoids noisy tolerated Homebrew cleanup.
release-workflow-hygiene-gates:
	bash ./scripts/verify_release_workflow_hygiene.sh

# release-gofmt-gate: gofmt must be clean on all tracked Go sources.
# Mirrors the gofmt check the CI Lint workflow enforces (golangci-lint gofmt), so
# formatting drift is caught locally before tagging instead of turning main red.
# Checks tracked files only, so untracked scratch dirs never block a release.
release-gofmt-gate:
	@unformatted=$$(git ls-files '*.go' | xargs gofmt -l 2>/dev/null); \
	if [ -n "$$unformatted" ]; then \
		echo "release-gofmt-gate: gofmt violations in tracked Go files:" >&2; \
		printf '%s\n' "$$unformatted" | sed 's/^/  /' >&2; \
		echo "Fix with: gofmt -w \$$(git ls-files '*.go')  (or: make lint-fix)" >&2; \
		exit 1; \
	fi
	@echo "release-gofmt-gate: gofmt clean"

# release-contract-gates: unified pre-release gate composing all version, surface, binary, launcher, dialect, output format, source location, and gofmt checks.
release-contract-gates: release-surface-gates release-version-surface-gates release-local-version-smoke release-dialect-hygiene-gates release-gitlab-codequality-smoke release-source-location-smoke release-workflow-hygiene-gates release-gofmt-gate
	npm test --prefix packages/deltascope-mcp

# Static analysis
lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

# Validate inline JS syntax in landing page HTML files.
# Uses new Function() to parse without executing. Catches mismatched quotes,
# unterminated strings, and other SyntaxErrors before deploy.
lint-landing:
	@for f in docs/landing/*.html; do \
		node -e "\
		const fs = require('fs'); \
		const html = fs.readFileSync('$$f', 'utf8'); \
		const blocks = html.match(/<script[^>]*>([\\s\\S]*?)<\\/script>/gi) || []; \
		let errors = 0; \
		blocks.forEach((block, i) => { \
			const js = block.replace(/<script[^>]*>/i, '').replace(/<\\/script>/i, '').trim(); \
			if (!js || block.includes('application/ld+json')) return; \
			try { new Function(js); } \
			catch (e) { console.error('$$f script block ' + (i+1) + ': ' + e.message); errors++; } \
		}); \
		if (errors) process.exit(1); \
		"; \
	done
	@echo "landing page JS syntax OK"

release-consistency-test:
	python3 ./scripts/test_verify_release_consistency.py

# Release recovery preflight: verify GitHub Release assets and npm package state.
# Read-only — never publishes, uploads, or deletes.
release-recovery-preflight:
	@test -n "$(VERSION)" || (echo "VERSION is required (e.g. VERSION=v0.230.0)" >&2; exit 1)
	VERSION="$(VERSION)" python3 ./scripts/verify_release_assets.py
	VERSION="$(VERSION)" bash ./scripts/verify_npm_package_state.sh

RELEASE_RECOVERY_CONTRACT_VERSION ?= v0.240.0

# Release recovery contract test: preflight + static dry-run contract verification.
# Does not dispatch any workflow.
release-recovery-contract-test:
	VERSION="$(RELEASE_RECOVERY_CONTRACT_VERSION)" $(MAKE) release-recovery-preflight
	@grep -q "dry_run" .github/workflows/release-recover.yml
	@grep -q "Homebrew cask would be updated" .github/workflows/release-recover.yml
	@grep -q "npm package would be published" .github/workflows/release-recover.yml
	@grep -q "!inputs.dry_run" .github/workflows/release-recover.yml
	@grep -q 'GH_TOKEN:.*secrets\.GITHUB_TOKEN' .github/workflows/release-recover.yml
	@echo "release-recovery-contract-test: dry-run contract OK, preflight auth wiring OK"

# Heuristic gate: if changed paths + diff keywords suggest a decision record
# is needed but no docs/decisions/*.md is present, fail.
decision-record-gate:
	./scripts/check_decision_record.sh main...HEAD

# Release tag annotation gates.
# release-tag-annotation-test runs unit tests (no tag required).
# release-tag-annotation-gate verifies an existing tag is annotated (run post-tag only).
release-tag-annotation-test:
	python3 scripts/test_verify_release_tag_annotation.py

release-tag-annotation-gate:
	@test -n "$(VERSION)" || (echo "VERSION is required (e.g. VERSION=v0.251.0)" >&2; exit 1)
	VERSION="$(VERSION)" python3 scripts/verify_release_tag_annotation.py

# release-tag-candidate-gate: verify tag target matches approved release candidate.
# Set RELEASE_CANDIDATE_SHA to enforce exact match; omit to skip that check.
release-tag-candidate-gate:
	@test -n "$(VERSION)" || (echo "VERSION is required (e.g. VERSION=v0.460.0)" >&2; exit 1)
	bash scripts/verify_release_tag_candidate.sh "$(VERSION)"

