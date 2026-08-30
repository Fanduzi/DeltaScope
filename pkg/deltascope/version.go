// Package deltascope exposes the stable public audit API.
// input: build metadata consumers and public version/logo queries
// output: shared default version and ASCII logo values for CLIs and services
// pos: public package metadata alongside the stable audit entrypoint
// note: if this file changes, update this header and module README.md.
package deltascope

const (
	// DefaultVersion is the repository's current default semantic version.
	DefaultVersion = "v0.500.0"

	// Logo is the canonical ASCII DeltaScope banner used by human-facing commands.
	Logo = "    ____       ____        _____                     \n" +
		"   / __ \\___  / / /_____ _/ ___/_________  ____  ___ \n" +
		"  / / / / _ \\/ / __/ __ `/\\__ \\/ ___/ __ \\/ __ \\/ _ \\\n" +
		" / /_/ /  __/ / /_/ /_/ /___/ / /__/ /_/ / /_/ /  __/\n" +
		"/_____/\\___/_/\\__/\\__,_//____/\\___/\\____/ .___/\\___/ \n" +
		"                                       /_/           "
)
