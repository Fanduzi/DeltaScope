// Package httpapi exposes the HTTP adapter for DeltaScope.
package httpapi

import "net/http"

// NewServer builds the DeltaScope HTTP service.
func NewServer(addr, configPath, version string, opts ...HandlerOption) (*http.Server, error) {
	handler, err := NewHandler(configPath, version, opts...)
	if err != nil {
		return nil, err
	}
	return &http.Server{
		Addr:    addr,
		Handler: handler,
	}, nil
}
