package spec

import (
	"strconv"
	"strings"
)

// SpatialReferenceDeclaration carries the authored MySQL spatial
// reference system statement — CREATE [OR REPLACE] [IF NOT EXISTS]
// SPATIAL REFERENCE SYSTEM and DROP SPATIAL REFERENCE SYSTEM [IF EXISTS]
// — on statements the parser adapter recognized as the MySQL grammar
// family (T26/#105). It is internal evidence and is never serialized.
//
// DEFINITION payload privacy: the WKT/WKT2 source text is a free-form
// string literal. The declaration records presence, byte length, and
// literal CR/LF presence only — never the payload text; legality is a
// semantic check this stage does not claim. NAME and ORGANIZATION
// identities are kept verbatim because the promised semantic checks
// (name conflict, organization consistency) consume them; the raw
// input still holds every payload for a later definition check.
// PublicSQL renders the fixed-shape redacted form.
type SpatialReferenceDeclaration struct {
	// Kind is the statement family: "create", "create_or_replace", or
	// "drop".
	Kind string
	// SRID is the authored numeric identifier. Recognition guarantees a
	// valid unsigned 32-bit value — negative and overflowing forms are
	// never claimed by the recognizer.
	SRID uint32
	// IfNotExists / IfExists carry the authored guards.
	IfNotExists bool
	IfExists    bool
	// Attrs carries the authored SRS attributes — presence, identities,
	// and payload lengths; never DEFINITION or DESCRIPTION text.
	Attrs SRSAttributes
	// DuplicateAttrs names attribute keywords authored more than once —
	// a declaration fact for later adjudication, not a verdict.
	DuplicateAttrs []string
}

// SRSAttributes records the authored srs_attribute members. Name and
// OrgName are the unquoted authored spellings — doubled-quote escapes
// resolved, nothing more; backslash escapes and charset introducers stay
// unresolved, so they are not complete MySQL string decodes.
type SRSAttributes struct {
	// HasName / Name carry the authored NAME identity (unquoted
	// spelling, doubled quotes resolved).
	HasName bool
	Name    string
	// HasDefinition marks an authored DEFINITION clause;
	// DefinitionByteLen is the byte length of the unquoted text and
	// DefHasNewline records literal CR/LF presence in that text — a
	// lexical fact only, not the definition-legality verdict (the
	// documented no-newline rule is one member of that check, and the
	// decode step owed before semantic consumers is registered with
	// the contract). The WKT payload itself is never stored.
	HasDefinition     bool
	DefinitionByteLen int
	DefHasNewline     bool
	// HasOrganization / OrgName carry the authored ORGANIZATION
	// identity; HasIdentifiedBy / OrgID carry the IDENTIFIED BY number —
	// recognition requires the complete pair.
	HasOrganization bool
	OrgName         string
	HasIdentifiedBy bool
	OrgID           uint32
	// HasDescription marks an authored DESCRIPTION clause;
	// DescriptionByteLen is the byte length of the unquoted text. The
	// description text is never stored.
	HasDescription     bool
	DescriptionByteLen int
}

// SpatialReferenceDeclaration kind vocabulary.
const (
	SRSKindCreate          = "create"
	SRSKindCreateOrReplace = "create_or_replace"
	SRSKindDrop            = "drop"
)

// SpatialReferenceSystemState is the projection of the MySQL SRS fact
// reads — the bounded ST_SPATIAL_REFERENCE_SYSTEMS identity read plus
// the optional NAME / ORGANIZATION conflict witness reads and the
// ST_GEOMETRY_COLUMNS dependency witness set, resolved through the
// provider object seam (T26/#105 provider round). It is internal
// evidence and is never serialized.
//
// The existence contract is per-fact, never guessed:
//   - Read=true + Exists=true: a catalog row proved the SRID present.
//   - Read=true + Exists=false: the query ran and returned zero rows —
//     unknown, never proven-absent: catalog visibility is
//     privilege-filtered and no approved completeness contract exists.
//   - Read=false: the read was never issued — UnreadReason names why
//     (identity unproven, version-inapplicable). A failed read is an
//     error propagated by the resolver, never a state value.
type SpatialReferenceSystemState struct {
	// SRID is the authored numeric identity — the provable catalog key.
	SRID uint32 `json:"-"`
	// Status is the snapshot vocabulary: confirmed on a proven row,
	// otherwise unavailable (never not_found — no approved completeness
	// contract exists for the SRS catalog).
	Status MetadataStatus `json:"-"`
	// Exists is the proven-presence answer.
	Exists bool `json:"-"`
	// Read reports whether the existence read was actually issued.
	Read bool `json:"-"`
	// UnreadReason names why the existence read stayed unissued when
	// Read is false — a stable token from the MetadataUnread
	// vocabulary.
	UnreadReason string `json:"-"`
	// CatalogName is the SRS_NAME witness from the matched existence
	// row — the name fact is evidence only; it is never read through
	// DEFINITION or DESCRIPTION columns.
	CatalogName string `json:"-"`
	// NameConflict carries the SRS_NAME match read — nil means the
	// read never happened (the name identity was not provable or never
	// requested); a non-nil fact is the returned SRID witness set.
	NameConflict *SRSConflictFact `json:"-"`
	// OrgConflict carries the ORGANIZATION + ORGANIZATION_COORDSYS_ID
	// pair read — nil means the read never happened (the pair was not
	// provable, only partially authored, or never requested); a
	// non-nil fact is the returned SRID witness set.
	OrgConflict *SRSConflictFact `json:"-"`
	// Dependencies carries the ST_GEOMETRY_COLUMNS witness set for this
	// SRID — nil means the read never happened; a non-nil fact's
	// Columns are the visible cross-schema witnesses, never a
	// proven-complete set.
	Dependencies *SRSDependencyFact `json:"-"`
}

// SRSConflictFact is the bounded witness answer of one conflict read —
// the SRIDs the catalog returned for the conflict predicate. Matched
// identities are raw witnesses: a match on the statement's own SRID and
// a match on another SRID both appear, and consumers distinguish them.
// Complete is always false until an approved completeness contract
// exists for the catalog's visibility — a non-nil empty set is "read,
// zero rows, unknown", never a proven "no conflict".
type SRSConflictFact struct {
	// MatchedSRIDs lists every catalog SRID the conflict predicate
	// returned — witness evidence, sorted in read order.
	MatchedSRIDs []uint32 `json:"-"`
	// Complete marks an approved complete-visibility read. Always false
	// this round — no approved completeness contract exists, so a
	// zero-row answer is unproven, never "no conflict".
	Complete bool `json:"-"`
}

// SRSDependentColumn is one cross-schema dependency witness — a geometry
// column whose declared SRID is the statement's target identity.
type SRSDependentColumn struct {
	Schema string `json:"-"`
	Table  string `json:"-"`
	Column string `json:"-"`
	// SRID echoes the column's declared spatial reference system — the
	// catalog's own answer, kept beside the identity it was read under.
	SRID uint32 `json:"-"`
}

// SRSDependencyFact is the bounded ST_GEOMETRY_COLUMNS witness set for
// one SRID. The catalog view is privilege-filtered (a view over COLUMNS),
// so the visible set is evidence only: positive rows prove dependency
// presence; an empty Columns with Complete=false is "read, zero rows,
// unproven" — never an instance-wide "no dependencies" claim.
type SRSDependencyFact struct {
	// Columns lists the visible dependency witnesses.
	Columns []SRSDependentColumn `json:"-"`
	// Complete marks an approved complete-visibility read. Always false
	// this round — restricted visibility can never collapse into
	// instance-wide absence.
	Complete bool `json:"-"`
}

// SRIDName renders the numeric identifier as the object's name slot —
// the SRID is the catalog identity for spatial reference systems.
func (d *SpatialReferenceDeclaration) SRIDName() string {
	return strconv.FormatUint(uint64(d.SRID), 10)
}

// PublicSQL renders the fixed-shape redacted form for serialized output:
// the keyword skeleton, guards, and the numeric SRID — attribute
// payloads never appear. The shape is stable so downstream consumers can
// recognize it without re-deriving which family produced it.
func (d *SpatialReferenceDeclaration) PublicSQL() string {
	var b strings.Builder
	srid := strconv.FormatUint(uint64(d.SRID), 10)
	switch d.Kind {
	case SRSKindCreate:
		b.WriteString("CREATE SPATIAL REFERENCE SYSTEM ")
		if d.IfNotExists {
			b.WriteString("IF NOT EXISTS ")
		}
		b.WriteString(srid)
	case SRSKindCreateOrReplace:
		b.WriteString("CREATE OR REPLACE SPATIAL REFERENCE SYSTEM ")
		b.WriteString(srid)
	case SRSKindDrop:
		b.WriteString("DROP SPATIAL REFERENCE SYSTEM ")
		if d.IfExists {
			b.WriteString("IF EXISTS ")
		}
		b.WriteString(srid)
	default:
		return "<redacted spatial reference system statement>"
	}
	if d.Kind != SRSKindDrop && (d.Attrs.HasName || d.Attrs.HasDefinition || d.Attrs.HasOrganization || d.Attrs.HasDescription) {
		b.WriteString(" <attributes redacted>")
	}
	return b.String()
}
