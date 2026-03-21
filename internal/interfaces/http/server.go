// Package httpapi exposes the HTTP adapter for DeltaScope.
// input: listen addresses plus service-level config/version wiring
// output: ready-to-run HTTP server instances for the JSON audit service
// pos: long-running net/http server assembly for the HTTP interface milestone
// note: if this file changes, update this header and module README.md.
package httpapi

import "net/http"

// NewServer builds the DeltaScope HTTP service.
func NewServer(addr, configPath, version string) (*http.Server, error) {
	handler, err := NewHandler(configPath, version)
	if err != nil {
		return nil, err
	}
	return &http.Server{
		Addr:    addr,
		Handler: handler,
	}, nil
}
