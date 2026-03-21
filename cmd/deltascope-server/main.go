// Package main starts the DeltaScope HTTP service.
// input: process flags for listen address, optional config path, and version printing
// output: a long-running JSON HTTP server process over the offline audit engine
// pos: HTTP service entrypoint above the internal HTTP adapter
// note: if this file changes, update this header and module README.md.
package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"

	httpapi "github.com/Fanduzi/DeltaScope/internal/interfaces/http"
	publicapi "github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

// Version is the build version printed by the HTTP service entrypoint.
var Version = publicapi.DefaultVersion

func main() {
	listen := flag.String("listen", "127.0.0.1:8083", "HTTP listen address")
	configPath := flag.String("config", "", "path to YAML policy config")
	showVersion := flag.Bool("version", false, "print the DeltaScope server build version")
	flag.Parse()

	if *showVersion {
		fmt.Println(Version)
		return
	}

	server, err := httpapi.NewServer(*listen, *configPath, Version)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "build server: %v\n", err)
		os.Exit(2)
	}

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		_, _ = fmt.Fprintf(os.Stderr, "serve http: %v\n", err)
		os.Exit(3)
	}
}
