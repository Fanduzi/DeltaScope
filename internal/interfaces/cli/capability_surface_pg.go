//go:build postgresql

package cli

func supportedDialects() []string {
	return productDialects()
}

func rootCommandShort() string {
	return unifiedRootCommandShort()
}

func dialectFlagDescription() string {
	return dialectFlagPrefix()
}

func capabilityBuildNote() string {
	return ""
}
