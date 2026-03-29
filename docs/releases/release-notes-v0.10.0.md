# DeltaScope v0.10.0 Release Notes

Release date: 2026-03-29

## Overview

DeltaScope `v0.10.0` introduces a hardened HTTP service surface with authentication, middleware guardrails, rate limiting, and Prometheus metrics.

## New Features

### HTTP adapter migrated to Gin with middleware chain

- HTTP adapter now runs on Gin while preserving the existing `/healthz`, `/version`, and `/v1/audit` contracts.
- Added baseline middleware chain: request ID, panic recovery, request timeout context, auth, rate limit, and structured access logs.

### API key authentication for HTTP audit endpoints

- Added optional `X-API-Key` protection.
- Added explicit error semantics:
  - `401 auth_required` when key is missing
  - `403 auth_invalid` when key is invalid

### Rate limiting and operational metrics

- Added optional per-key throttling (`api-key` or `ip`) with `429 rate_limited`.
- Added `/metrics` endpoint with Prometheus metrics:
  - `deltascope_http_requests_total`
  - `deltascope_http_request_duration_seconds`

### Proxy-aware IP limiting hardening

- Added `-trusted-proxies` to explicitly define trusted proxy CIDRs for client IP extraction.
- Default behavior trusts no proxies to reduce spoofed forwarded-header risk.

## Bug Fixes

- Added stale-entry cleanup for in-memory rate limiter buckets to reduce long-run memory growth from high-cardinality keys.
- Removed Gin global mode side effect from library-level handler construction; release mode is now set in server entrypoint.

## Breaking Changes

None.

## Upgrade Notes

Use HTTP auth/limit flags as needed:

```bash
deltascope-server \
  -listen 127.0.0.1:8083 \
  -auth-enabled \
  -auth-keys 'k1,k2' \
  -rate-limit-enabled \
  -rate-limit-rps 10 \
  -rate-limit-burst 20 \
  -rate-limit-key api-key
```

If you use `-rate-limit-key ip` behind a reverse proxy, set trusted proxy CIDRs:

```bash
deltascope-server -rate-limit-key ip -trusted-proxies '10.0.0.0/8,192.168.0.0/16'
```
