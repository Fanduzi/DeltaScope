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
