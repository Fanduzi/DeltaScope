package spec

// trigger.go — typed MySQL trigger declaration (T29/#108). Trigger
// bodies reuse the T28 RoutineBody member model and walker; OLD/NEW
// pseudo-column usage is recorded lexically so the audit binds it to
// the trigger target and event instead of treating it as a table.

// TriggerTiming is the activation point relative to the event.
type TriggerTiming string

const (
	TriggerTimingBefore TriggerTiming = "before"
	TriggerTimingAfter  TriggerTiming = "after"
)

// TriggerEvent is the DML event that activates the trigger.
type TriggerEvent string

const (
	TriggerEventInsert TriggerEvent = "insert"
	TriggerEventUpdate TriggerEvent = "update"
	TriggerEventDelete TriggerEvent = "delete"
)

// AllowsOld reports whether OLD.<col> pseudo-columns can mean anything
// under the event: DELETE and UPDATE carry an old row image, INSERT
// does not.
func (e TriggerEvent) AllowsOld() bool { return e != TriggerEventInsert }

// AllowsNew reports whether NEW.<col> pseudo-columns can mean anything
// under the event: INSERT and UPDATE carry a new row image, DELETE
// does not.
func (e TriggerEvent) AllowsNew() bool { return e != TriggerEventDelete }

// TriggerOrderKind is the authored ordering direction.
type TriggerOrderKind string

const (
	TriggerOrderFollows  TriggerOrderKind = "follows"
	TriggerOrderPrecedes TriggerOrderKind = "precedes"
)

// TriggerOrder records the optional MySQL 5.7.2+ execution ordering
// clause relative to another trigger on the same table.
type TriggerOrder struct {
	Kind TriggerOrderKind `json:"-"`
	Name string           `json:"-"` // sibling trigger name
}

// TriggerKind is the lifecycle operation the declaration carries.
type TriggerKind string

const (
	TriggerKindCreate TriggerKind = "create"
	TriggerKindDrop   TriggerKind = "drop"
)

// TriggerDeclaration is the normalized form the audit consumes.
// Body stays nonserialized like RoutineDeclaration bodies; Definer
// keeps the clause shape only (never the account payload).
type TriggerDeclaration struct {
	Kind       TriggerKind   `json:"-"`
	Name       Table         `json:"-"`
	Definer    string        `json:"-"` // clause shape only, never the account
	Timing     TriggerTiming `json:"-"`
	Event      TriggerEvent  `json:"-"`
	Target     Table         `json:"-"` // table the trigger binds to
	Order      *TriggerOrder `json:"-"`
	IfExists   bool          `json:"-"` // DROP TRIGGER IF EXISTS
	UsesOld    bool          `json:"-"` // body references OLD.<col>
	UsesNew    bool          `json:"-"` // body references NEW.<col>
	OldColumns []string      `json:"-"` // sorted unique OLD.<col> column dependencies
	NewColumns []string      `json:"-"` // sorted unique NEW.<col> column dependencies
	Body       *RoutineBody  `json:"-"`
}
