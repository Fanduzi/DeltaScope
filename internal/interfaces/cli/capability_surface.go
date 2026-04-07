//go:build !postgresql

package cli

func supportedDialects() []string {
	return []string{"mysql", "tidb"}
}

func rootCommandShort() string {
	return unifiedRootCommandShort()
}

func dialectFlagDescription() string {
	return dialectFlagPrefix() + " (postgresql requires a PG-capable build)"
}

func capabilityBuildNote() string {
	return "postgresql requires a PG-capable build"
}
