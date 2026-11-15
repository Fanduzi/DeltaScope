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
import "strings"

type TablespaceDeclaration struct {
	// Kind is the object family: "general" (InnoDB/NDB data files),
	// "undo" (MySQL 8.0 UNDO TABLESPACE, InnoDB-only), or "logfile_group"
	// (the NDB-only family).
	Kind string
	// Name is the authored object identity.
	Name string
	// AlterAction names the ALTER member the statement carries:
	// "add_datafile", "drop_datafile", "add_undo_file", "rename",
	// "encryption", or "engine". Empty on CREATE/DROP.
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

// TablespaceEngineNDB reports whether the authored engine names the NDB
// family — NDB and its documented NDBCLUSTER alias are the same engine
// upstream, so both spellings route to the engine boundary instead of an
// InnoDB applicability refusal.
func TablespaceEngineNDB(engine string) bool {
	return strings.EqualFold(engine, "ndb") || strings.EqualFold(engine, "ndbcluster")
}

// Documented MySQL release floors for the gated tablespace capabilities —
// full MAJOR.MINOR.PATCH precision, never a series-level guess. The
// capability table comes from the 8.0 CREATE/ALTER TABLESPACE
// documentation and the 8.0.13/8.0.14 release notes (issue #104 T25-R3).
const (
	// TablespaceFloorUndoSQL is the floor for the UNDO TABLESPACE SQL
	// management surface — the UNDO keyword and CREATE/ALTER/DROP UNDO
	// were introduced in MySQL 8.0.14.
	TablespaceFloorUndoSQL = "8.0.14"
	// TablespaceFloorMemberless is the floor for a general CREATE
	// TABLESPACE without a file clause — InnoDB implicitly generates the
	// datafile name only from MySQL 8.0.14 (NDB always requires its
	// ADD DATAFILE member).
	TablespaceFloorMemberless = "8.0.14"
	// TablespaceFloorEncryption is the floor for the ENCRYPTION member on
	// general tablespaces, introduced in MySQL 8.0.13.
	TablespaceFloorEncryption = "8.0.13"
	// TablespaceFloorRename is the floor for ALTER TABLESPACE ...
	// RENAME TO — the rename member arrives in MySQL 8.0.3.
	TablespaceFloorRename = "8.0.3"
	// TablespaceFloorEngineAttribute is the floor for the
	// ENGINE_ATTRIBUTE option — the syntax member arrives in MySQL
	// 8.0.21 on CREATE and ALTER.
	TablespaceFloorEngineAttribute = "8.0.21"
	// TablespaceFloorAlterAutoextend is the floor for ALTER TABLESPACE
	// ... AUTOEXTEND_SIZE — the alter capability arrives in MySQL
	// 8.0.23. CREATE TABLESPACE ... AUTOEXTEND_SIZE carries NO floor:
	// MySQL 5.7 already accepts the option and ignores its effect (and
	// pre-8.0.23 keeps ignoring it) — the accepted-and-ignored arm must
	// never become a version refusal.
	TablespaceFloorAlterAutoextend = "8.0.23"
)

// TablespaceVersionOutcome is the three-way answer of a declaration's
// capability floor against the resolved version.
type TablespaceVersionOutcome int

const (
	// TablespaceVersionSupported means the proven version reaches every
	// floor the declaration requires (or nothing is gated).
	TablespaceVersionSupported TablespaceVersionOutcome = iota
	// TablespaceVersionRefused means the proven version sits below a
	// required floor — a determined native refusal.
	TablespaceVersionRefused
	// TablespaceVersionUnproven means the version fact never answered —
	// missing or outside the validated series — so the gate cannot
	// decide either way.
	TablespaceVersionUnproven
)

// TablespaceCapabilityFloor resolves the strictest documented MySQL floor
// the declaration requires, plus the surface label diagnostics cite. The
// file-carrying general create and the NDB file members (ADD/DROP
// DATAFILE/UNDOFILE) need no floor — they exist on the pre-8.0 grammar.
// gated=false means the declaration needs no capability floor.
func TablespaceCapabilityFloor(operation DDLOperation, decl *TablespaceDeclaration) (floor, feature string, gated bool) {
	if decl == nil {
		return "", "", false
	}
	floor = "0.0.0"
	raise := func(candidate, label string) {
		if tablespaceVersionFloorAbove(candidate, floor) {
			floor, feature = candidate, label
		}
		gated = true
	}
	if decl.Kind == TablespaceKindUndo {
		raise(TablespaceFloorUndoSQL, "undo tablespace")
	}
	if decl.Encryption != nil {
		raise(TablespaceFloorEncryption, "tablespace encryption")
	}
	if TablespaceMemberlessCreate(operation, decl) {
		raise(TablespaceFloorMemberless, "memberless create tablespace")
	}
	if operation == DDLOperationAlterTablespace && decl.AlterAction == "rename" {
		raise(TablespaceFloorRename, "tablespace rename")
	}
	// CREATE-side AUTOEXTEND_SIZE is accepted and ignored from MySQL 5.7
	// through the pre-8.0.23 line — it deliberately carries no floor and
	// must not collapse into a version refusal.
	if operation == DDLOperationAlterTablespace {
		if _, ok := decl.Options["autoextend_size"]; ok {
			raise(TablespaceFloorAlterAutoextend, "alter autoextend_size")
		}
	}
	if _, ok := decl.Options["engine_attribute"]; ok {
		raise(TablespaceFloorEngineAttribute, "engine_attribute")
	}
	return floor, feature, gated
}

// TablespaceRenameVerdict is the shared three-way adjudication of an
// ALTER TABLESPACE ... RENAME TO — the applicability rule and the
// ordered-state transition consume the same answer so a refusal, a
// migration, and an undecidable premise can never disagree.
type TablespaceRenameVerdict int

const (
	// TablespaceRenameMigratable means every premise proved out: the
	// source is a proven general object, the target is proven absent,
	// and the version proves the 8.0.3 rename member.
	TablespaceRenameMigratable TablespaceRenameVerdict = iota
	// TablespaceRenameRefused means a proven refusal holds the exact
	// pre-state — the reason names the refused arm.
	TablespaceRenameRefused
	// TablespaceRenameUndecidable means at least one premise never
	// answered — the possible worlds disagree, so neither the source
	// tombstone nor the target inheritance may be asserted.
	TablespaceRenameUndecidable
)

// TablespaceRenameOutcome adjudicates ALTER TABLESPACE name RENAME TO
// new_name against the two ordered states and the resolved version.
// source/target are the bound states (nil or unproven = never answered).
// Returns the verdict, the refusal reason for diagnostics ("" unless
// refused), and the premise facts still owed when undecidable.
func TablespaceRenameOutcome(source, target *TablespaceState, version *VersionIdentity) (verdict TablespaceRenameVerdict, reason string, owed []string) {
	switch tablespaceVersionOutcomeAtFloor(version, TablespaceFloorRename) {
	case TablespaceVersionRefused:
		// A proven pre-8.0.3 version refuses the rename member.
		return TablespaceRenameRefused, tablespaceRenameReasonVersion, nil
	case TablespaceVersionUnproven:
		owed = append(owed, "target.version")
	}
	if source == nil || !source.Proven() {
		owed = append(owed, "tablespace.existence")
	} else if !source.Exists {
		return TablespaceRenameRefused, tablespaceRenameReasonAbsent, nil
	} else if source.Kind == "" {
		// The undo-kind world refuses the general rename form — a
		// proven-exists object with an unproven family keeps the kind
		// fact owed rather than skipping that world.
		owed = append(owed, "tablespace.kind")
	} else if source.Kind != TablespaceKindGeneral {
		return TablespaceRenameRefused, tablespaceRenameReasonKind, nil
	}
	if target == nil || !target.Proven() {
		// Target existence decides between migration and the
		// collision-refusal world — unproven stays owed.
		owed = append(owed, "tablespace.rename_target")
	} else if target.Exists {
		return TablespaceRenameRefused, tablespaceRenameReasonCollision, nil
	}
	if len(owed) > 0 {
		return TablespaceRenameUndecidable, "", owed
	}
	return TablespaceRenameMigratable, "", nil
}

const (
	tablespaceRenameReasonVersion   = "version"
	tablespaceRenameReasonAbsent    = "absent"
	tablespaceRenameReasonKind      = "kind_mismatch"
	tablespaceRenameReasonCollision = "rename_target_present"
)

// TablespaceVersionOutcome adjudicates the declaration's capability floor
// against the resolved version — shared by the applicability rule and the
// ordered-state transition so a proven refusal, a proven pass, and an
// unproven version answer identically on both sides. A version outside
// the validated series answers Unproven, never a fabricated support.
func TablespaceVersionOutcomeFor(operation DDLOperation, decl *TablespaceDeclaration, version *VersionIdentity) TablespaceVersionOutcome {
	floor, _, gated := TablespaceCapabilityFloor(operation, decl)
	if !gated {
		return TablespaceVersionSupported
	}
	return tablespaceVersionOutcomeAtFloor(version, floor)
}

// tablespaceVersionOutcomeAtFloor is the single comparator every
// capability check shares: nil/unvalidated/zero version → Unproven,
// proven-below-floor → Refused, at-or-above → Supported. Missing
// components are never defaulted to a decidable value.
func tablespaceVersionOutcomeAtFloor(version *VersionIdentity, floor string) TablespaceVersionOutcome {
	if version == nil || !version.ValidatedRange || version.Major == 0 {
		return TablespaceVersionUnproven
	}
	major, minor, patch := tablespaceFloorTriple(floor)
	switch {
	case version.Major != major:
		if version.Major > major {
			return TablespaceVersionSupported
		}
		return TablespaceVersionRefused
	case version.Minor != minor:
		if version.Minor > minor {
			return TablespaceVersionSupported
		}
		return TablespaceVersionRefused
	case version.Patch >= patch:
		return TablespaceVersionSupported
	}
	return TablespaceVersionRefused
}

// tablespaceFloorTriple parses a pinned MAJOR.MINOR.PATCH floor constant.
func tablespaceFloorTriple(floor string) (major, minor, patch int) {
	parts := strings.SplitN(floor, ".", 3)
	if len(parts) != 3 {
		return 0, 0, 0
	}
	return atoiOrZero(parts[0]), atoiOrZero(parts[1]), atoiOrZero(parts[2])
}

// tablespaceVersionFloorAbove reports whether candidate sorts strictly
// above base as a MAJOR.MINOR.PATCH triple — both sides are pinned
// constants from the floor table.
func tablespaceVersionFloorAbove(candidate, base string) bool {
	ca, cb, cc := tablespaceFloorTriple(candidate)
	ba, bb, bc := tablespaceFloorTriple(base)
	if ca != ba {
		return ca > ba
	}
	if cb != bb {
		return cb > bb
	}
	return cc > bc
}

// TablespaceMemberlessCreate reports a general CREATE TABLESPACE with no
// file clause — InnoDB permits the omission only from MySQL 8.0.14 (the
// server generates the datafile name), and NDB always requires its
// ADD DATAFILE member. The capability floor decides the version arm; the
// authored-engine arm stays an evidence question.
func TablespaceMemberlessCreate(operation DDLOperation, decl *TablespaceDeclaration) bool {
	if decl == nil {
		return false
	}
	return operation == DDLOperationCreateTablespace &&
		decl.Kind == TablespaceKindGeneral && decl.FileCount == 0
}

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
