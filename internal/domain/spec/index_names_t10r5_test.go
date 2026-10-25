// Package spec verifies the T10-R5 auto-naming boundary: MySQL 8.0.46's
// make_unique_key_name truncates the base to NAME_CHAR_LEN-3 (61 bytes)
// before appending `_N` inside the suffix branch, so a 62/63/64-byte base
// colliding with an earlier claim stores as 61-byte base + "_2" — while
// TiDB v8.5.0 appends the suffix to the full base and fails the statement
// (1059) once the derived name crosses the 64-byte identifier limit
// (probed on the R5 anchors).
package spec

import (
	"strings"
	"testing"
)

func t10r5Base(n int) string { return strings.Repeat("c", n) }

// MySQL truncates the suffix-branch base to 61 bytes before appending —
// a collision on a 62/63/64-byte base stores the same 63-byte name. The
// 61-byte control shows no truncation happens below the limit, and the
// 63-byte no-collision control keeps the verbatim column name.
func TestT10R5MySQLSuffixTruncationBoundary(t *testing.T) {
	t.Parallel()
	collide := func(base string) []Index {
		return []Index{
			{Name: base, Kind: IndexKindSecondary, Columns: []string{"a"}, KeyParts: []IndexKeyPart{t10r4Col("a")}},
			{Kind: IndexKindSecondary, Columns: []string{base}, KeyParts: []IndexKeyPart{t10r4Col(base)}},
		}
	}
	truncated := t10r5Base(61) + "_2"
	cases := []struct {
		name      string
		indexes   []Index
		wantNames []string
		wantProv  bool
	}{
		{
			name:      "61-byte base keeps the full base under the suffix (63-byte stored name)",
			indexes:   collide(t10r5Base(61)),
			wantNames: []string{t10r5Base(61), t10r5Base(61) + "_2"},
			wantProv:  true,
		},
		{
			name:      "62-byte base truncates before suffixing",
			indexes:   collide(t10r5Base(62)),
			wantNames: []string{t10r5Base(62), truncated},
			wantProv:  true,
		},
		{
			name:      "63-byte base truncates before suffixing",
			indexes:   collide(t10r5Base(63)),
			wantNames: []string{t10r5Base(63), truncated},
			wantProv:  true,
		},
		{
			name:      "64-byte base truncates before suffixing",
			indexes:   collide(t10r5Base(64)),
			wantNames: []string{t10r5Base(64), truncated},
			wantProv:  true,
		},
		{
			name: "63-byte base without collision keeps the verbatim name",
			indexes: []Index{
				{Kind: IndexKindSecondary, Columns: []string{t10r5Base(63)}, KeyParts: []IndexKeyPart{t10r4Col(t10r5Base(63))}},
			},
			wantNames: []string{t10r5Base(63)},
			wantProv:  true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resolved, provable := StoredIndexNames(DialectMySQL, tc.indexes)
			if provable != tc.wantProv {
				t.Fatalf("provable = %v, want %v", provable, tc.wantProv)
			}
			if !tc.wantProv {
				return
			}
			for i, want := range tc.wantNames {
				if resolved[i].Name != want {
					t.Fatalf("resolved[%d].Name = %q (len %d), want %q (len %d)",
						i, resolved[i].Name, len(resolved[i].Name), want, len(want))
				}
			}
		})
	}
}

// TiDB derives differently — verified on v8.5.0: the suffix appends to the
// full base, so a 62-byte base stores base+"_2" (64 bytes) and a 63/64-byte
// base fails the whole CREATE natively (1059), keeping the shape unproven.
func TestT10R5TiDBSuffixLengthBoundary(t *testing.T) {
	t.Parallel()
	collide := func(base string) []Index {
		return []Index{
			{Name: base, Kind: IndexKindSecondary, Columns: []string{"a"}, KeyParts: []IndexKeyPart{t10r4Col("a")}},
			{Kind: IndexKindSecondary, Columns: []string{base}, KeyParts: []IndexKeyPart{t10r4Col(base)}},
		}
	}
	cases := []struct {
		name      string
		indexes   []Index
		wantNames []string
		wantProv  bool
	}{
		{
			name:      "62-byte base appends without truncation (64-byte stored name)",
			indexes:   collide(t10r5Base(62)),
			wantNames: []string{t10r5Base(62), t10r5Base(62) + "_2"},
			wantProv:  true,
		},
		{
			name:     "63-byte base crosses the limit natively — the shape is unproven",
			indexes:  collide(t10r5Base(63)),
			wantProv: false,
		},
		{
			name:     "64-byte base crosses the limit natively — the shape is unproven",
			indexes:  collide(t10r5Base(64)),
			wantProv: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resolved, provable := StoredIndexNames(DialectTiDB, tc.indexes)
			if provable != tc.wantProv {
				t.Fatalf("provable = %v, want %v", provable, tc.wantProv)
			}
			if !tc.wantProv {
				return
			}
			for i, want := range tc.wantNames {
				if resolved[i].Name != want {
					t.Fatalf("resolved[%d].Name = %q, want %q", i, resolved[i].Name, want)
				}
			}
		})
	}
}
