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
	// NameOverLimit marks an authored name exceeding the documented
	// 64-character server-name bound, counted in characters (runes of
	// the unquoted text) — never bytes. MySQL truncates/refuses at the
	// engine layer; the fact stays declarative here.
	NameOverLimit bool
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
