// Package main starts the deltascope executable.
// input: shell process startup and the internal CLI adapter package
// output: process-level invocation of the DeltaScope CLI
// pos: executable entrypoint for the deltascope command
// note: if this file changes, update this header and module README.md.
package main

import "github.com/Fanduzi/DeltaScope/internal/interfaces/cli"

func main() {
	cli.Run()
}
