// Package connresolve turns caller connection input into a ready-to-open configuration.
// input: dialect, catalog hints, omitted-port flags, and raw connection/TLS/authentication errors
// output: MySQL/TiDB catalog binding, PostgreSQL omitted-port default, and Connection Failure Class
// pos: application Transport Connection Resolution path that stops before open
// note: if this file changes, update this header and module README.md.
package connresolve
