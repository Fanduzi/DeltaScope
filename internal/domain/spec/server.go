package spec

import "strings"

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
	// Name is the authored server identity, verbatim. Server names are
	// object identities (like table names), not credentials.
	Name string
	// IfExists carries the authored guard on DROP SERVER.
	IfExists bool
	// Wrapper is the authored FOREIGN DATA WRAPPER identity, verbatim.
	Wrapper string
	// Options records each authored OPTIONS member in source order —
	// key plus value kind only. Literal values never enter the model.
	Options []ServerOption
	// DuplicateKeys names option keys authored more than once — a
	// declaration fact for later adjudication, not a verdict.
	DuplicateKeys []string
	// NameOverLimit marks an authored name exceeding the documented
	// 64-character server-name bound (MySQL truncates/refuses at the
	// engine layer; the fact stays declarative here).
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

// PublicSQL renders the fixed-shape redacted form for serialized output:
// the keyword skeleton, the object identity, and an opaque options
// marker. Connection coordinates, credentials, and the raw input text
// never appear. The shape is stable so downstream consumers can
// recognize it without re-deriving which family produced it.
func (d *ServerDeclaration) PublicSQL() string {
	var b strings.Builder
	switch d.Kind {
	case ServerKindCreate:
		b.WriteString("CREATE SERVER ")
		b.WriteString(d.Name)
		if d.Wrapper != "" {
			b.WriteString(" FOREIGN DATA WRAPPER ")
			b.WriteString(d.Wrapper)
		}
		b.WriteString(" OPTIONS (<redacted>)")
	case ServerKindAlter:
		b.WriteString("ALTER SERVER ")
		b.WriteString(d.Name)
		b.WriteString(" OPTIONS (<redacted>)")
	case ServerKindDrop:
		b.WriteString("DROP SERVER ")
		if d.IfExists {
			b.WriteString("IF EXISTS ")
		}
		b.WriteString(d.Name)
	default:
		return "<redacted server statement>"
	}
	return b.String()
}
