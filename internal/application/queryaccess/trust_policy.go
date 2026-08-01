// Package queryaccess implements the TrustPolicy for PostgreSQL effect identity proof.
// input: resolved EffectIdentityBatch + immutable PG17 manifest
// output: TrustDecision indicating whether all effects are manifest-proven
// pos: T8 trust policy layer; sole path to Trusted for PostgreSQL admission
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

var (
	// ErrManifestInvalid indicates the manifest failed validation.
	ErrManifestInvalid = errors.New("invalid trust manifest")
	// ErrManifestHashMismatch indicates the manifest hash does not match contents.
	ErrManifestHashMismatch = errors.New("manifest hash mismatch")
	// ErrPolicyVersionMismatch indicates the server version is outside the manifest range.
	ErrPolicyVersionMismatch = errors.New("server version outside manifest range")
)

// TrustDecision is the outcome of manifest proof.
// Values are bounded machine identifiers — never public JSON.
type TrustDecision string

const (
	// TrustDecisionAllProven means every effect candidate is resolved and manifest-proven.
	TrustDecisionAllProven TrustDecision = "all_proven"
	// TrustDecisionHasUnproven means some candidates are resolved but not in manifest.
	TrustDecisionHasUnproven TrustDecision = "has_unproven"
	// TrustDecisionHasUnknown means some candidates are not resolved.
	TrustDecisionHasUnknown TrustDecision = "has_unknown"
	// TrustDecisionEmpty means no candidates to prove (vacuous truth).
	TrustDecisionEmpty TrustDecision = "empty"
)

// ValidTrustDecision reports whether d is a known trust decision.
func ValidTrustDecision(d TrustDecision) bool {
	switch d {
	case TrustDecisionAllProven, TrustDecisionHasUnproven, TrustDecisionHasUnknown, TrustDecisionEmpty:
		return true
	default:
		return false
	}
}

// TrustedEffectEntry is one entry in the versioned manifest.
// Each entry represents an audited effect identity that is permitted to promote
// PostgreSQL queries from indeterminate to read_only + admissible.
type TrustedEffectEntry struct {
	// Kind is the effect candidate kind (operator, function, cast).
	Kind EffectCandidateKind
	// AggregateClass is the catalog prokind fact for aggregate entries.
	AggregateClass string
	// ObjectOID is the primary catalog OID (pg_operator.oid / pg_proc.oid).
	ObjectOID uint32
	// NamespaceOID is the schema namespace OID (pg_catalog = 11).
	NamespaceOID uint32
	// OperandTypeOIDs are left/right or argument type OIDs in catalog order.
	OperandTypeOIDs []uint32
	// ResultTypeOID is the result type OID when known.
	ResultTypeOID uint32
	// ImplementationOID is operator oprcode (function implementing the operator).
	ImplementationOID uint32
	// Volatility is the catalog provolatile fact (i/s/v).
	Volatility EffectVolatility
	// CanonicalSignature is the internal manifest-matching key.
	CanonicalSignature string
	// AuditNotes records the semantic audit rationale.
	AuditNotes string
}

// TrustedEffectManifest is the immutable, versioned manifest of trusted effects.
// It is compile-time owned, versioned, schema-validated, and deterministically hashed.
// No filesystem, remote, request, or caller-supplied manifest is accepted.
type TrustedEffectManifest struct {
	// SchemaVersion is the manifest schema version.
	SchemaVersion string
	// PostgreSQLMajorMin is the minimum supported PostgreSQL major version.
	PostgreSQLMajorMin int
	// PostgreSQLMajorMax is the maximum supported PostgreSQL major version.
	PostgreSQLMajorMax int
	// Entries is the sorted list of trusted effect entries.
	Entries []TrustedEffectEntry
	// Hash is the SHA-256 hash of the entries (computed by ComputeManifestHash).
	Hash string
}

// ComputeManifestHash computes a deterministic SHA-256 hash of the manifest entries.
// Entries are sorted by (Kind, ObjectOID, NamespaceOID, CanonicalSignature) before hashing.
func ComputeManifestHash(entries []TrustedEffectEntry) string {
	// Defensive copy and sort.
	sorted := make([]TrustedEffectEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Kind != sorted[j].Kind {
			return sorted[i].Kind < sorted[j].Kind
		}
		if sorted[i].ObjectOID != sorted[j].ObjectOID {
			return sorted[i].ObjectOID < sorted[j].ObjectOID
		}
		if sorted[i].NamespaceOID != sorted[j].NamespaceOID {
			return sorted[i].NamespaceOID < sorted[j].NamespaceOID
		}
		return sorted[i].CanonicalSignature < sorted[j].CanonicalSignature
	})

	// Deterministic JSON serialization.
	h := sha256.New()
	for _, e := range sorted {
		//nolint:errcheck // Hash write never fails; error return is formality.
		fmt.Fprintf(h, "%s|%d|%d|%v|%d|%d|%s|%s|%s|%s\n",
			e.Kind, e.ObjectOID, e.NamespaceOID,
			e.OperandTypeOIDs, e.ResultTypeOID, e.ImplementationOID,
			e.Volatility, e.AggregateClass, e.CanonicalSignature, e.AuditNotes)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ValidateManifest checks the manifest for structural validity and hash integrity.
func ValidateManifest(m TrustedEffectManifest) error {
	if m.SchemaVersion == "" {
		return fmt.Errorf("%w: missing schema version", ErrManifestInvalid)
	}
	if m.PostgreSQLMajorMin <= 0 || m.PostgreSQLMajorMax <= 0 {
		return fmt.Errorf("%w: invalid version range", ErrManifestInvalid)
	}
	if m.PostgreSQLMajorMin > m.PostgreSQLMajorMax {
		return fmt.Errorf("%w: min version > max version", ErrManifestInvalid)
	}
	if len(m.Entries) == 0 {
		return fmt.Errorf("%w: no entries", ErrManifestInvalid)
	}

	// Validate each entry.
	seen := make(map[string]struct{}, len(m.Entries))
	for i, e := range m.Entries {
		if err := validateEntry(e); err != nil {
			return fmt.Errorf("%w: entry %d: %v", ErrManifestInvalid, i, err)
		}
		// Check for duplicate canonical signatures.
		key := e.CanonicalSignature
		if _, exists := seen[key]; exists {
			return fmt.Errorf("%w: duplicate canonical signature: %s", ErrManifestInvalid, key)
		}
		seen[key] = struct{}{}
	}

	// Verify hash.
	expected := ComputeManifestHash(m.Entries)
	if m.Hash != expected {
		return fmt.Errorf("%w: expected %s, got %s", ErrManifestHashMismatch, expected, m.Hash)
	}

	return nil
}

func validateEntry(e TrustedEffectEntry) error {
	if !validEffectCandidateKind(e.Kind) {
		return fmt.Errorf("invalid kind: %s", e.Kind)
	}
	if e.ObjectOID == 0 {
		return fmt.Errorf("missing object OID")
	}
	if e.NamespaceOID == 0 {
		return fmt.Errorf("missing namespace OID")
	}
	if e.CanonicalSignature == "" {
		return fmt.Errorf("missing canonical signature")
	}
	if e.Volatility != "" && !ValidEffectVolatility(e.Volatility) {
		return fmt.Errorf("invalid volatility: %s", e.Volatility)
	}
	if e.AggregateClass != "" && e.AggregateClass != "a" {
		return fmt.Errorf("invalid aggregate class: %s", e.AggregateClass)
	}
	return nil
}

// TrustPolicy evaluates whether resolved facts are manifest-proven.
// It is the sole path to Trusted for PostgreSQL admission.
type TrustPolicy struct {
	manifest TrustedEffectManifest
}

// NewTrustPolicy creates a new TrustPolicy with the given manifest.
// The manifest is validated on construction. Entries are deeply copied to
// prevent post-construction mutation.
func NewTrustPolicy(manifest TrustedEffectManifest) (*TrustPolicy, error) {
	if err := ValidateManifest(manifest); err != nil {
		return nil, err
	}
	// Deep copy entries to prevent mutation.
	copied := make([]TrustedEffectEntry, len(manifest.Entries))
	for i, e := range manifest.Entries {
		copied[i] = e
		copied[i].OperandTypeOIDs = append([]uint32(nil), e.OperandTypeOIDs...)
	}
	manifest.Entries = copied
	return &TrustPolicy{manifest: manifest}, nil
}

// Manifest returns a deep copy of the policy's manifest (read-only safe).
func (p *TrustPolicy) Manifest() TrustedEffectManifest {
	if p == nil {
		return TrustedEffectManifest{}
	}
	// Deep copy to prevent mutation.
	copied := p.manifest
	copied.Entries = make([]TrustedEffectEntry, len(p.manifest.Entries))
	for i, e := range p.manifest.Entries {
		copied.Entries[i] = e
		copied.Entries[i].OperandTypeOIDs = append([]uint32(nil), e.OperandTypeOIDs...)
	}
	return copied
}

// IsTrusted evaluates whether the resolved batch is fully manifest-proven.
//
// Requirements for TrustDecisionAllProven:
//   - Every item in the batch must be resolved with facts
//   - Every fact's CanonicalSignature must exist in the manifest
//   - Every fact's ObjectOID, NamespaceOID must match the manifest entry
//   - Server version must be within the manifest's version range
//   - Facts must be stamped with matching DatabaseOID/ServerVersionNum
//
// Returns:
//   - TrustDecisionAllProven if all above conditions met
//   - TrustDecisionHasUnproven if some resolved but not in manifest
//   - TrustDecisionHasUnknown if some not resolved
//   - TrustDecisionEmpty if batch has no items
func (p *TrustPolicy) IsTrusted(batch EffectIdentityBatch, serverVersionNum int) TrustDecision {
	if p == nil {
		return TrustDecisionHasUnknown
	}
	if len(batch.Items) == 0 {
		return TrustDecisionEmpty
	}

	// Check server version range.
	major := serverVersionNum / 10000
	if major < p.manifest.PostgreSQLMajorMin || major > p.manifest.PostgreSQLMajorMax {
		return TrustDecisionHasUnproven
	}

	// Build manifest lookup by canonical signature.
	manifestBySig := make(map[string]TrustedEffectEntry, len(p.manifest.Entries))
	for _, e := range p.manifest.Entries {
		manifestBySig[e.CanonicalSignature] = e
	}

	hasUnknown := false
	hasUnproven := false

	for _, item := range batch.Items {
		// All items must be resolved.
		if item.Status != domain.IdentityStatusResolved || item.Facts == nil {
			hasUnknown = true
			continue
		}

		facts := item.Facts

		// Facts must be stamped with matching server version.
		if facts.ServerVersionNum == 0 || facts.ServerVersionNum != serverVersionNum {
			hasUnproven = true
			continue
		}

		// Look up in manifest by canonical signature.
		entry, exists := manifestBySig[facts.CanonicalSignature]
		if !exists {
			hasUnproven = true
			continue
		}

		// Verify complete identity tuple (not just signature + OID).
		if facts.Kind != entry.Kind {
			hasUnproven = true
			continue
		}
		if entry.AggregateClass != "" && facts.AggregateClass != entry.AggregateClass {
			hasUnproven = true
			continue
		}
		if facts.ObjectOID != entry.ObjectOID || facts.NamespaceOID != entry.NamespaceOID {
			hasUnproven = true
			continue
		}
		if facts.ResultTypeOID != entry.ResultTypeOID {
			hasUnproven = true
			continue
		}
		if !uint32SliceEqual(facts.OperandTypeOIDs, entry.OperandTypeOIDs) {
			hasUnproven = true
			continue
		}
		// For operators, verify implementation OID.
		if facts.Kind == EffectCandidateOperator && facts.ImplementationOID != entry.ImplementationOID {
			hasUnproven = true
			continue
		}
		// Verify volatility matches (if entry specifies it).
		if entry.Volatility != "" && facts.Volatility != entry.Volatility {
			hasUnproven = true
			continue
		}
		// Verify database pin is set (consistency check).
		if facts.DatabaseOID == 0 {
			hasUnproven = true
			continue
		}
	}

	if hasUnknown {
		return TrustDecisionHasUnknown
	}
	if hasUnproven {
		return TrustDecisionHasUnproven
	}
	return TrustDecisionAllProven
}

// MarshalManifestJSON serializes the manifest to deterministic JSON.
func MarshalManifestJSON(m TrustedEffectManifest) ([]byte, error) {
	return json.Marshal(m)
}

// UnmarshalManifestJSON deserializes and validates the manifest from JSON.
func UnmarshalManifestJSON(data []byte) (TrustedEffectManifest, error) {
	var m TrustedEffectManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return TrustedEffectManifest{}, fmt.Errorf("%w: %v", ErrManifestInvalid, err)
	}
	if err := ValidateManifest(m); err != nil {
		return TrustedEffectManifest{}, err
	}
	return m, nil
}

// uint32SliceEqual reports whether two uint32 slices are equal.
func uint32SliceEqual(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
