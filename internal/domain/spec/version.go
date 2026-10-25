// Package spec defines normalized statement specifications for rule evaluation.
package spec

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// VersionIdentity is the shared parser-neutral product/version fact attached to
// audited statements. Version is always the canonical MAJOR.MINOR.PATCH form;
// the raw provider banner is deliberately not part of this identity so it can
// never be projected into gaps, findings, or public results. The numeric
// components always serialize — a legitimate zero component (8.0.x, x.x.0)
// must never disappear from the public projection.
type VersionIdentity struct {
	Product        string `json:"product,omitempty"`
	Version        string `json:"version,omitempty"`
	Major          int    `json:"major"`
	Minor          int    `json:"minor"`
	Patch          int    `json:"patch"`
	Source         string `json:"source,omitempty"`
	ValidatedRange bool   `json:"validated_range"`
}

const (
	// VersionProductMySQL identifies a MySQL server identity.
	VersionProductMySQL = "mysql"
	// VersionProductTiDB identifies a TiDB server identity.
	VersionProductTiDB = "tidb"
	// VersionSourceTarget marks an identity derived from the caller-supplied target_version.
	VersionSourceTarget = "target"
	// VersionSourceObserved marks an identity derived from the live server banner.
	VersionSourceObserved = "observed"
)

// ErrInvalidTargetVersion rejects a target_version that is not strict
// [v]MAJOR.MINOR.PATCH syntax.
var ErrInvalidTargetVersion = errors.New("target_version must match [v]MAJOR.MINOR.PATCH")

var (
	targetVersionPattern  = regexp.MustCompile(`^[vV]?(\d+)\.(\d+)\.(\d+)$`)
	observedVersionPrefix = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)
	tidbVersionMarker     = regexp.MustCompile(`(?i)-TiDB-v(\d+)\.(\d+)\.(\d+)`)
)

// VersionProductForDialect maps an audit dialect to its server product identity.
// PostgreSQL is not a validated product for this version contract and reports no
// product so version-dependent MySQL-family rules never engage it.
func VersionProductForDialect(dialect Dialect) string {
	switch dialect {
	case DialectMySQL:
		return VersionProductMySQL
	case DialectTiDB:
		return VersionProductTiDB
	default:
		return ""
	}
}

// VersionInValidatedSeries reports whether (product, major, minor) belongs to
// the milestone-validated series: MySQL 5.7.x/8.0.x/8.4.x and TiDB 8.5.x.
func VersionInValidatedSeries(product string, major int, minor int) bool {
	switch strings.ToLower(strings.TrimSpace(product)) {
	case VersionProductMySQL:
		return (major == 5 && minor == 7) || (major == 8 && (minor == 0 || minor == 4))
	case VersionProductTiDB:
		return major == 8 && minor == 5
	default:
		return false
	}
}

// ParseTargetVersion parses the caller-supplied target_version. The grammar is
// exactly [v]MAJOR.MINOR.PATCH with optional surrounding whitespace; banners,
// compatibility strings, two-component versions, extra suffixes, signs, and
// overflowing components are rejected as input errors. Syntactically valid
// versions outside the validated series are NOT errors — they are returned
// with ValidatedRange=false so rules can report a bounded evidence gap.
func ParseTargetVersion(raw string) (VersionIdentity, error) {
	matches := targetVersionPattern.FindStringSubmatch(strings.TrimSpace(raw))
	if matches == nil {
		return VersionIdentity{}, fmt.Errorf("%w: %q", ErrInvalidTargetVersion, strings.TrimSpace(raw))
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return VersionIdentity{}, fmt.Errorf("%w: %q", ErrInvalidTargetVersion, strings.TrimSpace(raw))
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return VersionIdentity{}, fmt.Errorf("%w: %q", ErrInvalidTargetVersion, strings.TrimSpace(raw))
	}
	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return VersionIdentity{}, fmt.Errorf("%w: %q", ErrInvalidTargetVersion, strings.TrimSpace(raw))
	}
	return VersionIdentity{
		Version: fmt.Sprintf("%d.%d.%d", major, minor, patch),
		Major:   major,
		Minor:   minor,
		Patch:   patch,
		Source:  VersionSourceTarget,
	}, nil
}

// ValidateTargetVersion is the shared syntax preflight for caller-supplied
// target_version values. Transports call it before any connection lookup or
// open so a malformed value is an input error (ErrInvalidTargetVersion), never
// a connection failure. An empty value is valid — it means the caller supplied
// no target. Out-of-range syntax is NOT rejected here: a syntactically valid
// version outside the validated series still enters the audit and produces a
// bounded evidence gap.
func ValidateTargetVersion(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	_, err := ParseTargetVersion(raw)
	return err
}

// ParseObservedVersion canonicalizes a provider-reported version banner into a
// VersionIdentity whose product is derived from the banner itself — never from
// the caller-requested dialect. A TiDB compatibility banner such as
// "8.0.11-TiDB-v8.5.0" resolves to product=tidb version=8.5.0 — the MySQL
// compatibility prefix is never reported as the real version. Any other
// banner from a MySQL-protocol provider is a MySQL server and takes its
// leading MAJOR.MINOR.PATCH triplet (common server suffixes such as "-log"
// are tolerated). Returns nil when no version fact can be established;
// callers must treat that as missing evidence, never fabricate one.
func ParseObservedVersion(banner string) *VersionIdentity {
	raw := strings.TrimSpace(banner)
	if raw == "" {
		return nil
	}
	if matches := tidbVersionMarker.FindStringSubmatch(raw); matches != nil {
		return &VersionIdentity{
			Product:        VersionProductTiDB,
			Version:        matches[1] + "." + matches[2] + "." + matches[3],
			Major:          atoiOrZero(matches[1]),
			Minor:          atoiOrZero(matches[2]),
			Patch:          atoiOrZero(matches[3]),
			Source:         VersionSourceObserved,
			ValidatedRange: VersionInValidatedSeries(VersionProductTiDB, atoiOrZero(matches[1]), atoiOrZero(matches[2])),
		}
	}
	matches := observedVersionPrefix.FindStringSubmatch(raw)
	if matches == nil {
		return nil
	}
	major, minor, patch := atoiOrZero(matches[1]), atoiOrZero(matches[2]), atoiOrZero(matches[3])
	return &VersionIdentity{
		Product:        VersionProductMySQL,
		Version:        fmt.Sprintf("%d.%d.%d", major, minor, patch),
		Major:          major,
		Minor:          minor,
		Patch:          patch,
		Source:         VersionSourceObserved,
		ValidatedRange: VersionInValidatedSeries(VersionProductMySQL, major, minor),
	}
}

// WithTargetSource returns a copy of the identity stamped as the validated
// caller-supplied target for the given product.
func (v VersionIdentity) WithTargetSource(product string) VersionIdentity {
	v.Product = product
	v.Source = VersionSourceTarget
	v.ValidatedRange = VersionInValidatedSeries(product, v.Major, v.Minor)
	return v
}

func atoiOrZero(raw string) int {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return value
}

const (
	// RenameColumnMinimumSupportedVersion is the MySQL release that added
	// RENAME COLUMN. It is the syntax introduction in 8.0.3, not the later
	// INSTANT algorithm change.
	RenameColumnMinimumSupportedVersion = "8.0.3"

	renameColumnGapMissingVersion = "missing_target_version"
	renameColumnGapOutOfRange     = "target_version_out_of_validated_range"
	renameColumnFactVersion       = "target.version"
	renameColumnFactValidated     = "target.version.validated_range"
)

// RenameColumnVersionSupport is the shared RENAME COLUMN applicability result.
// Supported and Incompatible are mutually exclusive. A gap leaves both false:
// the fact is missing or outside the validated series, so the check must not
// invent a finding. A known incompatible version is a determined negative of
// this check, not an unimplemented semantic and not a parser failure.
type RenameColumnVersionSupport struct {
	Supported    bool
	Incompatible bool
	GapReason    string
	GapFacts     []string
}

// RenameColumnVersionSupportFor classifies whether one already-parsed version
// identity can use RENAME COLUMN. MySQL 8.0.3+ inside 8.0, MySQL 8.4, and
// TiDB 8.5 are supported. MySQL 5.7 and MySQL 8.0.0–8.0.2 are incompatible.
// Nil, empty, or out-of-series identities are gaps. CHANGE COLUMN must not
// consult this result.
func RenameColumnVersionSupportFor(version *VersionIdentity) RenameColumnVersionSupport {
	if version == nil || strings.TrimSpace(version.Product) == "" || strings.TrimSpace(version.Version) == "" {
		return RenameColumnVersionSupport{
			GapReason: renameColumnGapMissingVersion,
			GapFacts:  []string{renameColumnFactVersion},
		}
	}
	if !version.ValidatedRange {
		return RenameColumnVersionSupport{
			GapReason: renameColumnGapOutOfRange,
			GapFacts:  []string{renameColumnFactValidated},
		}
	}
	switch strings.ToLower(strings.TrimSpace(version.Product)) {
	case VersionProductMySQL:
		if version.Major == 8 && version.Minor == 4 {
			return RenameColumnVersionSupport{Supported: true}
		}
		if version.Major == 8 && version.Minor == 0 && version.Patch >= 3 {
			return RenameColumnVersionSupport{Supported: true}
		}
		if (version.Major == 5 && version.Minor == 7) || (version.Major == 8 && version.Minor == 0 && version.Patch < 3) {
			return RenameColumnVersionSupport{Incompatible: true}
		}
	case VersionProductTiDB:
		if version.Major == 8 && version.Minor == 5 {
			return RenameColumnVersionSupport{Supported: true}
		}
	}
	return RenameColumnVersionSupport{
		GapReason: renameColumnGapOutOfRange,
		GapFacts:  []string{renameColumnFactValidated},
	}
}

// CHECK constraint version boundaries (documented; anchors probe ≥8.0.46 so
// sub-anchor versions inherit the documented boundary only, never probe
// evidence). MySQL 8.0.16 first enforces CHECK constraints; 8.0.19 adds the
// ALTER CHECK / DROP CHECK syntax. TiDB's boundary is the
// tidb_enable_check_constraint runtime switch, not a version — the syntax
// parses on both arms while the flag decides whether anything is stored.
const (
	CheckMinEnforcementMySQL = "8.0.16"
	CheckMinAlterSyntaxMySQL = "8.0.19"
)

// CheckArm is the resolved CHECK capability of one product arm:
// parse-and-ignore, store-and-enforce, or undecidable because the governing
// fact (version, runtime flag) was never observed.
type CheckArm int

const (
	// CheckArmUnknown means the governing fact was unobserved — the arm's
	// CHECK semantics cannot be answered either way.
	CheckArmUnknown CheckArm = iota
	// CheckArmIgnored means the arm parses CHECK syntax but never stores or
	// enforces it: MySQL before 8.0.16, or TiDB under an observed
	// tidb_enable_check_constraint=OFF.
	CheckArmIgnored
	// CheckArmEnforced means CHECK members land and enforce: MySQL
	// ≥8.0.16, or TiDB under an observed tidb_enable_check_constraint=ON.
	CheckArmEnforced
)

// ResolveCheckArm classifies the statement's arm from the resolved version
// and instance facts. A missing or unvalidated version answers Unknown on
// MySQL (5.7 and 8.x cannot be told apart); TiDB answers only from the
// observed flag — its Known bit keeps "probed OFF" distinct from "never
// probed".
func ResolveCheckArm(dialect Dialect, version *VersionIdentity, instance *InstanceFacts) CheckArm {
	switch dialect {
	case DialectMySQL:
		if version == nil || !version.ValidatedRange || version.Major == 0 {
			return CheckArmUnknown
		}
		if version.Major < 8 || (version.Major == 8 && version.Minor == 0 && version.Patch < 16) {
			return CheckArmIgnored
		}
		return CheckArmEnforced
	case DialectTiDB:
		if instance == nil || !instance.TiDBEnableCheckConstraintKnown {
			return CheckArmUnknown
		}
		if instance.TiDBEnableCheckConstraint {
			return CheckArmEnforced
		}
		return CheckArmIgnored
	default:
		return CheckArmUnknown
	}
}

// CheckAlterSyntaxSupported reports whether the ALTER|DROP CHECK syntax
// executes on the arm — MySQL ≥8.0.19 per the documented boundary. TiDB
// parses the syntax on both flag arms, so the flag, not this gate, decides
// the effect. A nil/unvalidated version reports false; callers that must
// distinguish "refused" from "unobserved" check the version fact first.
func CheckAlterSyntaxSupported(dialect Dialect, version *VersionIdentity) bool {
	switch dialect {
	case DialectMySQL:
		if version == nil || !version.ValidatedRange || version.Major == 0 {
			return false
		}
		return version.Major > 8 || (version.Major == 8 && (version.Minor > 0 || version.Patch >= 19))
	case DialectTiDB:
		return true
	default:
		return false
	}
}
