package spec

// TablespaceDeclaration carries the authored MySQL tablespace-family
// statement — CREATE/ALTER/DROP TABLESPACE, CREATE UNDO TABLESPACE, and
// the NDB-only LOGFILE GROUP family — on statements the parser adapter
// recognized as the MySQL grammar family (T25/#104). It is internal
// evidence and is never serialized.
//
// File payloads stay opaque by contract: the declaration records how many
// data/undo file clauses the statement authored, never their paths. The
// audit must not read, stat, copy, delete, or echo SQL-supplied file
// paths — presence and count are the only facts it vouches.
type TablespaceDeclaration struct {
	// Kind is the object family: "general" (InnoDB/NDB data files),
	// "undo" (MySQL 8.0 UNDO TABLESPACE, InnoDB-only), or "logfile_group"
	// (the NDB-only family).
	Kind string
	// Name is the authored object identity.
	Name string
	// AlterAction names the ALTER member the statement carries:
	// "add_datafile", "drop_datafile", "add_undo_file", "rename", or
	// "encryption". Empty on CREATE/DROP.
	AlterAction string
	// Engine is the authored ENGINE [=] value, "" when absent. Absence is
	// a declaration fact — never an inferred default.
	Engine string
	// NewName carries the RENAME TO target on ALTER TABLESPACE.
	NewName string
	// IfExists / IfNotExists carry the authored guards on DROP/CREATE.
	IfExists    bool
	IfNotExists bool
	// Encryption is the authored ENCRYPTION [=] {'Y'|'N'} when present.
	Encryption *bool
	// FileCount is the number of ADD/DROP DATAFILE or UNDOFILE clauses
	// the statement authored. Paths are opaque — never stored.
	FileCount int
	// Options carries authored scalar members (FILE_BLOCK_SIZE,
	// EXTENT_SIZE, INITIAL_SIZE, MAX_SIZE, AUTOEXTEND_SIZE, NODEGROUP,
	// WAIT, COMMENT, ENGINE_ATTRIBUTE, UNDO_BUFFER_SIZE, REDO_BUFFER_SIZE)
	// in authored order — presence and value, never defaults.
	Options map[string]string
}

// TablespaceKind vocabulary for TablespaceDeclaration.Kind.
const (
	TablespaceKindGeneral      = "general"
	TablespaceKindUndo         = "undo"
	TablespaceKindLogfileGroup = "logfile_group"
)

// TablespaceState is the ordered-state projection of one MySQL tablespace
// as a statement adjudicates it: existence plus the occupancy premise
// DROP TABLESPACE consults — MySQL refuses to drop a tablespace that
// still has tables bound (the association fact).
type TablespaceState struct {
	Name   string         `json:"name,omitempty"`
	Status MetadataStatus `json:"status,omitempty"`
	Exists bool           `json:"exists"`
	// Kind is the proven object family when observed; "" stays unknown.
	Kind string `json:"kind,omitempty"`
	// Occupied answers "tables bound to this tablespace" — nil when the
	// association was never proven. The batch-derived transition can only
	// vouch what it publishes; provider facts bind the real occupancy.
	Occupied *bool `json:"-"`
}

// Clone returns a deep copy safe for statement-metadata publication.
func (s *TablespaceState) Clone() *TablespaceState {
	if s == nil {
		return nil
	}
	out := *s
	if s.Occupied != nil {
		v := *s.Occupied
		out.Occupied = &v
	}
	return &out
}

// Proven reports whether the state carries a proven existence answer.
func (s *TablespaceState) Proven() bool {
	if s == nil {
		return false
	}
	return s.Status == MetadataStatusConfirmed || s.Status == MetadataStatusNotFound
}
