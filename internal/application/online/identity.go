// Package online provides the shared online session factory for SDK, CLI, and HTTP.
// input: pinned *sql.Conn, server version string, expected dialect
// output: validated ServerIdentity, CapabilityTarget, and bounded sentinel errors
// pos: shared identity parsing and session lifecycle for online query access
// note: if this file changes, update this header and module README.md.
package online

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// ProductFamily identifies the database product family.
type ProductFamily string

const (
	ProductMySQL      ProductFamily = "mysql"
	ProductTiDB       ProductFamily = "tidb"
	ProductPostgreSQL ProductFamily = "postgresql"
)

// VersionSeries identifies a supported major/minor version series.
type VersionSeries string

const (
	SeriesMySQL57 VersionSeries = "mysql-5.7"
	SeriesMySQL80 VersionSeries = "mysql-8.0"
	SeriesMySQL84 VersionSeries = "mysql-8.4"
	SeriesTiDB85  VersionSeries = "tidb-8.5"
	SeriesPG17    VersionSeries = "postgresql-17"
)

// Bounded sentinel errors for identity parsing. These messages never contain
// version strings, hostnames, ports, DSNs, or credentials.
var (
	ErrIdentityUnavailable = errors.New("server identity unavailable")
	ErrIdentityUnknown     = errors.New("unsupported database product")
	ErrIdentityMalformed   = errors.New("malformed server version")
	ErrIdentityUnsupported = errors.New("unsupported database version series")
	ErrDialectMismatch     = errors.New("configured dialect disagrees with server identity")
)

// ServerIdentity represents the validated database server identity.
// It never appears in public results, errors, or logs.
type ServerIdentity struct {
	Product    ProductFamily
	Major      int
	Minor      int
	Patch      int
	Series     VersionSeries
	RawVersion string // internal only, never exposed
}

// mysqlVersionPattern matches MySQL/TiDB version strings like 5.7.44, 8.0.46, 8.0.11-TiDB-v8.5.7.
var mysqlVersionPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:.*)$`)

// tidbVersionPattern matches the TiDB component in version strings like 8.0.11-TiDB-v8.5.7.
var tidbVersionPattern = regexp.MustCompile(`TiDB[_-]v?(\d+)\.(\d+)\.(\d+)`)

// pgVersionPattern matches PostgreSQL version strings like "PostgreSQL 17.4".
var pgVersionPattern = regexp.MustCompile(`^PostgreSQL\s+(\d+)\.(\d+)(?:\.(\d+))?(?:.*)$`)

// ParseServerIdentity parses a VERSION() string and validates against supported series.
// Returns bounded sentinel errors for unknown/unsupported/malformed identity.
// The raw version string is stored internally but never exposed in errors.
func ParseServerIdentity(rawVersion string, expectedDialect string) (*ServerIdentity, error) {
	if strings.TrimSpace(rawVersion) == "" {
		return nil, ErrIdentityMalformed
	}

	// Try PostgreSQL first.
	if id, err := parsePostgreSQLIdentity(rawVersion); err == nil {
		if expectedDialect != "" && expectedDialect != "postgresql" {
			return nil, ErrDialectMismatch
		}
		return id, nil
	} else if err != ErrIdentityUnknown {
		// PostgreSQL matched but failed validation.
		if expectedDialect != "" && expectedDialect != "postgresql" {
			return nil, ErrDialectMismatch
		}
		return nil, err
	}

	// Try TiDB (must check before MySQL because TiDB version embeds MySQL base version).
	tidbID, tidbErr := parseTiDBIdentity(rawVersion)
	if tidbErr == nil {
		if expectedDialect != "" && expectedDialect != "tidb" {
			return nil, ErrDialectMismatch
		}
		return tidbID, nil
	}
	if tidbErr != ErrIdentityUnknown {
		if expectedDialect != "" && expectedDialect != "tidb" {
			return nil, ErrDialectMismatch
		}
		return nil, tidbErr
	}

	// Try MySQL (no TiDB marker in the string).
	if id, err := parseMySQLIdentity(rawVersion); err == nil {
		if expectedDialect != "" && expectedDialect != "mysql" {
			return nil, ErrDialectMismatch
		}
		return id, nil
	} else if err != ErrIdentityUnknown {
		if expectedDialect != "" && expectedDialect != "mysql" {
			return nil, ErrDialectMismatch
		}
		return nil, err
	}

	// Nothing matched.
	return nil, ErrIdentityUnknown
}

// parsePostgreSQLIdentity parses a PostgreSQL version string.
func parsePostgreSQLIdentity(raw string) (*ServerIdentity, error) {
	matches := pgVersionPattern.FindStringSubmatch(raw)
	if matches == nil {
		return nil, ErrIdentityUnknown
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch := 0
	if matches[3] != "" {
		patch, _ = strconv.Atoi(matches[3])
	}

	series, ok := pgSeriesForVersion(major, minor)
	if !ok {
		return nil, ErrIdentityUnsupported
	}

	return &ServerIdentity{
		Product:    ProductPostgreSQL,
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Series:     series,
		RawVersion: raw,
	}, nil
}

// parseTiDBIdentity parses a TiDB version string embedded in MySQL-compatible output.
func parseTiDBIdentity(raw string) (*ServerIdentity, error) {
	// TiDB versions contain "TiDB" in the string and embed a TiDB-specific version.
	matches := tidbVersionPattern.FindStringSubmatch(raw)
	if matches == nil {
		return nil, ErrIdentityUnknown
	}

	tidbMajor, _ := strconv.Atoi(matches[1])
	tidbMinor, _ := strconv.Atoi(matches[2])
	tidbPatch, _ := strconv.Atoi(matches[3])

	series, ok := tidbSeriesForVersion(tidbMajor, tidbMinor)
	if !ok {
		return nil, ErrIdentityUnsupported
	}

	return &ServerIdentity{
		Product:    ProductTiDB,
		Major:      tidbMajor,
		Minor:      tidbMinor,
		Patch:      tidbPatch,
		Series:     series,
		RawVersion: raw,
	}, nil
}

// parseMySQLIdentity parses a MySQL-compatible version string.
func parseMySQLIdentity(raw string) (*ServerIdentity, error) {
	// Reject known forks that don't match MySQL compatibility.
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "mariadb") {
		return nil, ErrIdentityUnknown
	}

	matches := mysqlVersionPattern.FindStringSubmatch(raw)
	if matches == nil {
		return nil, ErrIdentityMalformed
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	series, ok := mysqlSeriesForVersion(major, minor)
	if !ok {
		return nil, ErrIdentityUnsupported
	}

	return &ServerIdentity{
		Product:    ProductMySQL,
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Series:     series,
		RawVersion: raw,
	}, nil
}

// pgSeriesForVersion maps PostgreSQL major.minor to a supported series.
func pgSeriesForVersion(major, minor int) (VersionSeries, bool) {
	switch major {
	case 17:
		return SeriesPG17, true
	default:
		return "", false
	}
}

// mysqlSeriesForVersion maps MySQL major.minor to a supported series.
func mysqlSeriesForVersion(major, minor int) (VersionSeries, bool) {
	switch {
	case major == 5 && minor == 7:
		return SeriesMySQL57, true
	case major == 8 && minor == 0:
		return SeriesMySQL80, true
	case major == 8 && minor == 4:
		return SeriesMySQL84, true
	default:
		return "", false
	}
}

// tidbSeriesForVersion maps TiDB major.minor to a supported series.
func tidbSeriesForVersion(major, minor int) (VersionSeries, bool) {
	switch {
	case major == 8 && minor == 5:
		return SeriesTiDB85, true
	default:
		return "", false
	}
}

// CapabilityTarget represents the internal analysis capability derived from identity.
type CapabilityTarget string

const (
	TargetMySQL57 CapabilityTarget = "mysql-5.7"
	TargetMySQL80 CapabilityTarget = "mysql-8.0"
	TargetMySQL84 CapabilityTarget = "mysql-8.4"
	TargetTiDB85  CapabilityTarget = "tidb-8.5"
	TargetPG17    CapabilityTarget = "postgresql-17"
)

// DeriveCapabilityTarget maps a validated ServerIdentity to its internal capability target.
func DeriveCapabilityTarget(id *ServerIdentity) CapabilityTarget {
	if id == nil {
		return ""
	}
	switch id.Series {
	case SeriesMySQL57:
		return TargetMySQL57
	case SeriesMySQL80:
		return TargetMySQL80
	case SeriesMySQL84:
		return TargetMySQL84
	case SeriesTiDB85:
		return TargetTiDB85
	case SeriesPG17:
		return TargetPG17
	default:
		return ""
	}
}
