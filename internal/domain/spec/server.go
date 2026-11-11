package spec

// ServerDeclaration carries the authored MySQL SERVER-family statement —
// CREATE SERVER, ALTER SERVER, and DROP SERVER — on statements the parser
// adapter recognized as the MySQL grammar family (T26/#105). It is
// internal evidence and is never serialized.
//
// Connection-option values are opaque by contract: OPTIONS carries
// credentials and connection coordinates (HOST/DATABASE/USER/PASSWORD/
// SOCKET/OWNER/PORT), so the declaration records option keys, value
// kinds, and counts only — never the literal payloads. PublicSQL renders
// the fixed-shape redacted form for every serialized output field.
type ServerDeclaration struct {
	// Kind is the statement family: "create", "alter", or "drop".
	Kind string
	// Name is the authored server identity — the unquoted spelling with
	// doubled-quote escapes resolved (not a complete MySQL string
	// decode: backslash escapes and charset introducers stay
	// unresolved). Server names are object identities, not credentials;
	// they stay internal and never appear in PublicSQL.
	Name string
	// IfExists carries the authored guard on DROP SERVER.
	IfExists bool
	// Wrapper is the authored FOREIGN DATA WRAPPER identity (unquoted
	// spelling, doubled quotes resolved) — internal only.
	Wrapper string
	// Options records each authored OPTIONS member in source order —
	// key plus value kind only. Literal values never enter the model.
	Options []ServerOption
	// DuplicateKeys names option keys authored more than once — a
	// declaration fact for later adjudication, not a verdict.
	DuplicateKeys []string
	// NameSpellingRuneCount is the rune count of the unquoted authored
	// spelling — a lexical measurement only. It is not a verdict on the
	// documented 64-character server-name bound: the decoded length
	// needs a full MySQL string decode (backslash escapes, charset
	// introducers) that stays a registered future step, so `'...\4'`
	// spells 65 runes while its decoded name is 64. Consumers must not
	// read the spelling count as a proven name-length violation.
	NameSpellingRuneCount int
	// NameIdent marks whether Name is a complete decoded catalog
	// identity: identifier token forms (a bare word or a backquoted
	// identifier — doubled-backtick unescape is the entire identifier
	// decode) carry the catalog name itself, while a string-literal
	// spelling resolves only doubled quotes and stays unprovable under
	// SQL-mode and charset-introducer variance (T26/#105 provider
	// round). A false value means the authored spelling exists but is
	// not a provable catalog identity — provider reads keyed on the
	// name must not fire for it.
	NameIdent bool
}

// ServerState is the projection of the MySQL SERVER existence fact —
// the bounded mysql.servers identity read resolved through the provider
// object seam (T26/#105 provider round). It is internal evidence and is
// never serialized.
//
// The read contract is per-fact, never guessed:
//   - Read=true + Exists=true: a catalog row proved the identity present.
//   - Read=true + Exists=false: the query ran and returned zero rows —
//     unknown, never proven-absent: mysql.servers visibility depends on
//     account grants and no approved completeness contract exists.
//   - Read=false: the read was never issued — UnreadReason names why
//     (identity unproven, inapplicable surface). A failed read is an
//     error propagated by the resolver, never a state value.
type ServerState struct {
	// Name echoes the authored identity the statement named — the
	// proven catalog spelling when a row confirmed it, else the
	// authored value (a spelling when NameIdent is false). Internal
	// only: server names never reach serialized output.
	Name string `json:"-"`
	// Status is the snapshot vocabulary: confirmed on a proven row,
	// otherwise unavailable (never not_found — no approved
	// completeness contract exists for mysql.servers).
	Status MetadataStatus `json:"-"`
	// Exists is the proven-presence answer.
	Exists bool `json:"-"`
	// Read reports whether the catalog read was actually issued.
	Read bool `json:"-"`
	// UnreadReason names why the read stayed unissued when Read is
	// false — a stable token from the MetadataUnread vocabulary.
	UnreadReason string `json:"-"`
	// OrderedAbsent is the same-adjudication ordered channel's proven
	// absent answer: set only by a proven drop or the approved
	// convergent arm inside this batch — never by the catalog (a
	// zero-row catalog answer stays unknown) and never when the entry
	// carries a barrier or taint. OrderedAbsent=true implies
	// Exists=false, Status is never not_found, and Read is never set
	// by the ordered channel (T26-SERVER-DROP-EXECUTION-01; contract:
	// docs/verification/t26-semantics-01 D.4).
	OrderedAbsent bool `json:"-"`
}

// ServerOption records one OPTIONS member — key and value kind only.
type ServerOption struct {
	// Key is the lowercased option keyword: host, database, user,
	// password, socket, owner, or port.
	Key string
	// ValueKind is "char" for a string literal or "numeric" for an
	// unsigned literal — never the literal itself.
	ValueKind string
}

// ServerDeclaration kind vocabulary.
const (
	ServerKindCreate = "create"
	ServerKindAlter  = "alter"
	ServerKindDrop   = "drop"
)

// PublicSQL renders the fixed-shape redacted skeleton for serialized
// output — verb plus IF EXISTS guard only. The server name, wrapper,
// option keys/values, and every other authored fragment stay internal:
// serialized output carries no identity or connection text at all. The
// shape is stable so downstream consumers can recognize the family
// without re-deriving which statement produced it.
func (d *ServerDeclaration) PublicSQL() string {
	switch d.Kind {
	case ServerKindCreate:
		return "CREATE SERVER <redacted>"
	case ServerKindAlter:
		return "ALTER SERVER <redacted>"
	case ServerKindDrop:
		if d.IfExists {
			return "DROP SERVER IF EXISTS <redacted>"
		}
		return "DROP SERVER <redacted>"
	default:
		return "<redacted server statement>"
	}
}
