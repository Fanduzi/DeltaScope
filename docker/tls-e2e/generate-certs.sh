#!/usr/bin/env bash
# Generates self-signed CA and server certificates for TLS E2E testing.
# Creates trusted and untrusted CA chains to prove certificate validation.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERTS_DIR="${SCRIPT_DIR}/certs"

mkdir -p "${CERTS_DIR}/trusted" "${CERTS_DIR}/untrusted"

generate_ca() {
  local dir="$1"
  local name="$2"

  openssl genrsa -out "${dir}/${name}-ca-key.pem" 2048 2>/dev/null
  openssl req -new -x509 -days 3650 -key "${dir}/${name}-ca-key.pem" \
    -out "${dir}/${name}-ca-cert.pem" \
    -subj "/CN=${name}-ca" 2>/dev/null
}

generate_server_cert() {
  local dir="$1"
  local ca_name="$2"
  local server_name="$3"
  shift 3
  local sans="$*"

  # Create SAN extension
  local san_ext="[v3_req]\nsubjectAltName = @alt_names\n\n[alt_names]"
  local i=1
  for san in ${sans}; do
    san_ext="${san_ext}\nDNS.${i} = ${san}"
    i=$((i + 1))
  done

  openssl genrsa -out "${dir}/${server_name}-key.pem" 2048 2>/dev/null
  openssl req -new -key "${dir}/${server_name}-key.pem" \
    -out "${dir}/${server_name}-csr.pem" \
    -subj "/CN=${server_name}" 2>/dev/null

  # Create extension file
  echo -e "${san_ext}" > "${dir}/${server_name}-ext.cnf"

  openssl x509 -req -days 3650 \
    -in "${dir}/${server_name}-csr.pem" \
    -CA "${dir}/${ca_name}-ca-cert.pem" \
    -CAkey "${dir}/${ca_name}-ca-key.pem" \
    -CAcreateserial \
    -out "${dir}/${server_name}-cert.pem" \
    -extfile "${dir}/${server_name}-ext.cnf" \
    -extensions v3_req 2>/dev/null

  # Cleanup CSR and ext file
  rm -f "${dir}/${server_name}-csr.pem" "${dir}/${server_name}-ext.cnf"
}

# Generate trusted CA
generate_ca "${CERTS_DIR}/trusted" "trusted"

# Generate untrusted CA
generate_ca "${CERTS_DIR}/untrusted" "untrusted"

# Generate MySQL server cert (trusted CA, hostname: mysql-tls)
generate_server_cert "${CERTS_DIR}/trusted" "trusted" "mysql-tls" "mysql-tls" "localhost"

# Generate PostgreSQL server cert (trusted CA, hostname: postgresql-tls)
generate_server_cert "${CERTS_DIR}/trusted" "trusted" "postgresql-tls" "postgresql-tls" "localhost"

# Generate MySQL server cert with untrusted CA (for negative testing)
generate_server_cert "${CERTS_DIR}/untrusted" "untrusted" "mysql-tls-untrusted" "mysql-tls-untrusted" "localhost"

# Generate PostgreSQL server cert with untrusted CA (for negative testing)
generate_server_cert "${CERTS_DIR}/untrusted" "untrusted" "postgresql-tls-untrusted" "postgresql-tls-untrusted" "localhost"

# Set permissions for PostgreSQL (requires key to be owned by postgres user in container)
chmod 600 "${CERTS_DIR}/trusted/postgresql-tls-key.pem"
chmod 600 "${CERTS_DIR}/untrusted/postgresql-tls-untrusted-key.pem"

echo "TLS certificates generated in ${CERTS_DIR}"
echo "Trusted CA: ${CERTS_DIR}/trusted/trusted-ca-cert.pem"
echo "Untrusted CA: ${CERTS_DIR}/untrusted/untrusted-ca-cert.pem"
