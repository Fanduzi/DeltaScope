#!/bin/bash
# Custom entrypoint for PostgreSQL TLS E2E.
# Copies TLS files and fixes permissions before starting PostgreSQL.

set -e

# Copy TLS files to a location writable by postgres user
cp /etc/tls/server-cert.pem /var/lib/postgresql/server-cert.pem
cp /etc/tls/server-key.pem /var/lib/postgresql/server-key.pem
cp /etc/tls/ca-cert.pem /var/lib/postgresql/ca-cert.pem

# Fix permissions (postgres user is UID 999 in the official image)
chown 999:999 /var/lib/postgresql/server-cert.pem
chown 999:999 /var/lib/postgresql/server-key.pem
chown 999:999 /var/lib/postgresql/ca-cert.pem
chmod 600 /var/lib/postgresql/server-key.pem
chmod 644 /var/lib/postgresql/server-cert.pem
chmod 644 /var/lib/postgresql/ca-cert.pem

# Call the original entrypoint
exec docker-entrypoint.sh "$@"
