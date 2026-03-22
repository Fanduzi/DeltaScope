.PHONY: test-e2e-cli test-e2e-cli-mysql test-e2e-cli-tidb

test-e2e-cli: test-e2e-cli-mysql test-e2e-cli-tidb

test-e2e-cli-mysql:
	./scripts/test_cli_metadata_e2e.sh mysql

test-e2e-cli-tidb:
	./scripts/test_cli_metadata_e2e.sh tidb
