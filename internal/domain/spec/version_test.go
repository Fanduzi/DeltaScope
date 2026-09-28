// Package spec verifies the shared T04-B/#83 version fact model.
// input: caller target_version strings and provider product/version banners
// output: strict [v]MAJOR.MINOR.PATCH parsing, TiDB compatibility-prefix canonicalization, validated series, and bounded input errors
// pos: version parser contract tests shared by every transport
// note: if this file changes, update this header and module README.md.
package spec

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestParseTargetVersionCanonicalizesValidInput(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                string
		raw                 string
		want                string
		major, minor, patch int
	}{
		{name: "plain", raw: "8.4.10", want: "8.4.10", major: 8, minor: 4, patch: 10},
		{name: "lower v prefix", raw: "v8.0.46", want: "8.0.46", major: 8, minor: 0, patch: 46},
		{name: "upper V prefix", raw: "V5.7.44", want: "5.7.44", major: 5, minor: 7, patch: 44},
		{name: "surrounding whitespace", raw: "  8.4.10\t", want: "8.4.10", major: 8, minor: 4, patch: 10},
		{name: "zero components", raw: "0.0.0", want: "0.0.0", major: 0, minor: 0, patch: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			identity, err := ParseTargetVersion(tc.raw)
			if err != nil {
				t.Fatalf("expected valid parse, got %v", err)
			}
			if identity.Version != tc.want || identity.Major != tc.major || identity.Minor != tc.minor || identity.Patch != tc.patch {
				t.Fatalf("identity = %#v, want version=%q", identity, tc.want)
			}
			if identity.Source != VersionSourceTarget {
				t.Fatalf("expected source=target, got %q", identity.Source)
			}
		})
	}
}

func TestParseTargetVersionRejectsMalformedInput(t *testing.T) {
	t.Parallel()
	cases := []string{
		"",
		"v",
		"8.4",
		"8.4.x",
		"8.4.10-extra",
		"8.0.11-TiDB-v8.5.0",
		"-8.4.10",
		"8.4.10.1",
		"eight.point.four.ten",
		"99999999999999999999999.0.0",
		"8.-4.10",
		"8.4.-10",
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			identity, err := ParseTargetVersion(raw)
			if err == nil {
				t.Fatalf("expected input error for %q, got %#v", raw, identity)
			}
			if !errors.Is(err, ErrInvalidTargetVersion) {
				t.Fatalf("expected ErrInvalidTargetVersion for %q, got %v", raw, err)
			}
		})
	}
}

func TestVersionInValidatedSeriesCoversMilestoneAnchors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		product      string
		major, minor int
		want         bool
	}{
		{VersionProductMySQL, 5, 7, true},
		{VersionProductMySQL, 8, 0, true},
		{VersionProductMySQL, 8, 4, true},
		{VersionProductMySQL, 8, 3, false},
		{VersionProductMySQL, 9, 0, false},
		{VersionProductTiDB, 8, 5, true},
		{VersionProductTiDB, 8, 4, false},
		{VersionProductTiDB, 9, 0, false},
		{"", 8, 4, false},
		{"postgresql", 16, 0, false},
	}
	for _, tc := range cases {
		got := VersionInValidatedSeries(tc.product, tc.major, tc.minor)
		if got != tc.want {
			t.Fatalf("series(%q,%d,%d) = %t, want %t", tc.product, tc.major, tc.minor, got, tc.want)
		}
	}
}

func TestParseObservedVersionCanonicalizesBanners(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		banner      string
		wantProduct string
		wantVersion string
		wantRange   bool
	}{
		{name: "mysql plain", banner: "8.4.10", wantProduct: "mysql", wantVersion: "8.4.10", wantRange: true},
		{name: "mysql suffix", banner: "8.0.46-log", wantProduct: "mysql", wantVersion: "8.0.46", wantRange: true},
		{name: "mysql community", banner: "5.7.44-community", wantProduct: "mysql", wantVersion: "5.7.44", wantRange: true},
		{name: "mysql unvalidated", banner: "9.0.0", wantProduct: "mysql", wantVersion: "9.0.0", wantRange: false},
		{name: "mysql banner never inherits a tidb product", banner: "8.0.46", wantProduct: "mysql", wantVersion: "8.0.46", wantRange: true},
		{
			name:        "tidb compatibility prefix keeps tidb version",
			banner:      "8.0.11-TiDB-v8.5.0",
			wantProduct: "tidb", wantVersion: "8.5.0", wantRange: true,
		},
		{
			name:        "tidb banner is tidb even under a mysql request",
			banner:      "8.0.11-TiDB-v8.5.0-log",
			wantProduct: "tidb", wantVersion: "8.5.0", wantRange: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			identity := ParseObservedVersion(tc.banner)
			if identity == nil {
				t.Fatalf("expected identity for banner %q", tc.banner)
			}
			if identity.Product != tc.wantProduct || identity.Version != tc.wantVersion {
				t.Fatalf("identity = %#v, want %s %s", identity, tc.wantProduct, tc.wantVersion)
			}
			if identity.Source != VersionSourceObserved {
				t.Fatalf("expected source=observed, got %q", identity.Source)
			}
			if identity.ValidatedRange != tc.wantRange {
				t.Fatalf("validated_range = %t, want %t", identity.ValidatedRange, tc.wantRange)
			}
		})
	}
}

func TestParseObservedVersionReturnsNilWithoutFact(t *testing.T) {
	t.Parallel()
	for _, banner := range []string{"", "   ", "not-a-version", "8.0"} {
		if identity := ParseObservedVersion(banner); identity != nil {
			t.Fatalf("expected nil identity for banner %q, got %#v", banner, identity)
		}
	}
}

// TestVersionIdentitySerializesZeroComponents proves the public JSON
// projection never drops legitimate zero-valued numeric components:
// MySQL 8.0.46 keeps minor=0 and TiDB 8.5.0 keeps patch=0.
func TestVersionIdentitySerializesZeroComponents(t *testing.T) {
	t.Parallel()
	mysql8046, err := ParseTargetVersion("8.0.46")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	raw, err := json.Marshal(mysql8046.WithTargetSource(VersionProductMySQL))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	minor, present := decoded["minor"]
	if !present || minor != float64(0) {
		t.Fatalf("8.0.46 must serialize minor=0, got %v (present=%t)", minor, present)
	}

	tidb850, err := ParseTargetVersion("8.5.0")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	raw, err = json.Marshal(tidb850.WithTargetSource(VersionProductTiDB))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded = map[string]any{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	patch, present := decoded["patch"]
	if !present || patch != float64(0) {
		t.Fatalf("8.5.0 must serialize patch=0, got %v (present=%t)", patch, present)
	}
}

// TestValidateTargetVersionIsSharedPreflight locks the preflight contract:
// blank is valid (no target supplied), malformed values are input errors, and
// syntactically valid out-of-range versions pass preflight so the audit can
// report a bounded evidence gap instead of a preflight rejection.
func TestValidateTargetVersionIsSharedPreflight(t *testing.T) {
	t.Parallel()
	if err := ValidateTargetVersion(""); err != nil {
		t.Fatalf("empty target must be valid, got %v", err)
	}
	if err := ValidateTargetVersion("  "); err != nil {
		t.Fatalf("blank target must be valid, got %v", err)
	}
	for _, raw := range []string{"8.4", "8.4.x", "8.4.10-extra", "8.0.11-TiDB-v8.5.0", "v"} {
		if err := ValidateTargetVersion(raw); !errors.Is(err, ErrInvalidTargetVersion) {
			t.Fatalf("expected ErrInvalidTargetVersion for %q, got %v", raw, err)
		}
	}
	for _, raw := range []string{"8.4.10", "v8.0.46", "8.3.0", "9.0.0"} {
		if err := ValidateTargetVersion(raw); err != nil {
			t.Fatalf("out-of-range or valid target %q must pass preflight, got %v", raw, err)
		}
	}
}

func TestWithTargetSourceStampsProductAndRange(t *testing.T) {
	t.Parallel()
	parsed, err := ParseTargetVersion("8.4.10")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mysql := parsed.WithTargetSource(VersionProductMySQL)
	if mysql.Product != "mysql" || mysql.Source != VersionSourceTarget || !mysql.ValidatedRange {
		t.Fatalf("expected mysql/target/validated, got %#v", mysql)
	}
	oor, err := ParseTargetVersion("8.3.0")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if stamped := oor.WithTargetSource(VersionProductMySQL); stamped.ValidatedRange {
		t.Fatalf("expected 8.3.0 out of validated range, got %#v", stamped)
	}
	tidb, err := ParseTargetVersion("8.5.0")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if stamped := tidb.WithTargetSource(VersionProductTiDB); !stamped.ValidatedRange || stamped.Product != "tidb" {
		t.Fatalf("expected tidb/target/validated, got %#v", stamped)
	}
}

func TestVersionProductForDialect(t *testing.T) {
	t.Parallel()
	if got := VersionProductForDialect(DialectMySQL); got != "mysql" {
		t.Fatalf("mysql dialect product = %q", got)
	}
	if got := VersionProductForDialect(DialectTiDB); got != "tidb" {
		t.Fatalf("tidb dialect product = %q", got)
	}
	if got := VersionProductForDialect(DialectPostgreSQL); got != "" {
		t.Fatalf("postgresql must report no version product, got %q", got)
	}
}
