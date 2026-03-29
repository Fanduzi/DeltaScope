// Package main starts the DeltaScope HTTP service.
// input: process flags for listen address, optional config path, and version printing
// output: a long-running JSON HTTP server process over the offline audit engine
// pos: HTTP service entrypoint above the internal HTTP adapter
// note: if this file changes, update this header and module README.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	httpapi "github.com/Fanduzi/DeltaScope/internal/interfaces/http"
	publicapi "github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

// Version is the build version printed by the HTTP service entrypoint.
var Version = publicapi.DefaultVersion

func main() {
	listen := flag.String("listen", "127.0.0.1:8083", "HTTP listen address")
	configPath := flag.String("config", "", "path to YAML policy config")
	showVersion := flag.Bool("version", false, "print the DeltaScope server build version")
	authEnabled := flag.Bool("auth-enabled", false, "enable X-API-Key authentication for protected routes")
	authKeys := flag.String("auth-keys", "", "comma-separated API keys for X-API-Key auth")
	authAllowPaths := flag.String("auth-allow-paths", "/healthz,/readyz,/version,/metrics", "comma-separated paths that bypass auth")
	rateLimitEnabled := flag.Bool("rate-limit-enabled", false, "enable rate limiting middleware")
	rateLimitRPS := flag.Float64("rate-limit-rps", 5, "rate limit requests per second")
	rateLimitBurst := flag.Int("rate-limit-burst", 10, "rate limit burst size")
	rateLimitKey := flag.String("rate-limit-key", "api-key", "rate limit key strategy: api-key or ip")
	rateLimitAllowPaths := flag.String("rate-limit-allow-paths", "/healthz,/readyz,/version,/metrics", "comma-separated paths that bypass rate limiting")
	metricsEnabled := flag.Bool("metrics-enabled", true, "enable Prometheus metrics endpoint at /metrics")
	trustedProxies := flag.String("trusted-proxies", "", "comma-separated trusted proxy CIDRs for client IP extraction; empty means trust no proxies")
	flag.Parse()

	if *showVersion {
		fmt.Println(Version)
		return
	}

	keys := parseCSV(*authKeys)
	allowPaths := parseCSV(*authAllowPaths)
	proxies := parseCSV(*trustedProxies)
	if *authEnabled && len(keys) == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "auth is enabled but no keys were provided; set --auth-keys")
		os.Exit(2)
	}
	gin.SetMode(gin.ReleaseMode)

	server, err := httpapi.NewServer(*listen, *configPath, Version, httpapi.WithAuthConfig(httpapi.AuthConfig{
		Enabled:    *authEnabled,
		Keys:       keys,
		AllowPaths: allowPaths,
	}), httpapi.WithMiddlewareConfig(httpapi.MiddlewareConfig{
		MetricsEnabled: metricsEnabled,
		RateLimit: httpapi.RateLimitConfig{
			Enabled:    *rateLimitEnabled,
			RPS:        *rateLimitRPS,
			Burst:      *rateLimitBurst,
			KeyBy:      *rateLimitKey,
			AllowPaths: parseCSV(*rateLimitAllowPaths),
		},
		TrustedProxies: proxies,
	}))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "build server: %v\n", err)
		os.Exit(2)
	}

	// Start serving in a background goroutine so we can listen for signals.
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.ListenAndServe()
	}()

	// Wait for SIGINT or SIGTERM, then perform graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			_, _ = fmt.Fprintf(os.Stderr, "serve http: %v\n", err)
			os.Exit(3)
		}
	case sig := <-quit:
		_, _ = fmt.Fprintf(os.Stderr, "received signal %s, shutting down\n", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "graceful shutdown: %v\n", err)
			os.Exit(3)
		}
	}
}

func parseCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}
