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

// TriggerOrder records the optional MySQL 8.0.23+ execution ordering
// clause relative to another trigger on the same table.
type TriggerOrder struct {
	Kind string `json:"-"` // "follows" | "precedes"
	Name string `json:"-"` // sibling trigger name
}

// TriggerDeclaration is the normalized form the audit consumes.
// Body stays nonserialized like RoutineDeclaration bodies; Definer
// keeps the clause shape only (never the account payload).
type TriggerDeclaration struct {
	Kind     string        `json:"-"` // "create" | "drop"
	Name     Table         `json:"-"`
	Definer  string        `json:"-"` // clause shape only, never the account
	Timing   TriggerTiming `json:"-"`
	Event    TriggerEvent  `json:"-"`
	Target   Table         `json:"-"` // table the trigger binds to
	Order    *TriggerOrder `json:"-"`
	IfExists bool          `json:"-"` // DROP TRIGGER IF EXISTS
	UsesOld  bool          `json:"-"` // body references OLD.<col>
	UsesNew  bool          `json:"-"` // body references NEW.<col>
	Body     *RoutineBody  `json:"-"`
}

// OldNewValid reports whether every recorded OLD/NEW use is legal for
// the declared event: INSERT may read NEW only, DELETE reads OLD only,
// UPDATE may read both. Illegal uses surface as bounded
// trigger.body.old_new evidence — never silently allowed.
func (d *TriggerDeclaration) OldNewValid() bool {
	switch d.Event {
	case TriggerEventInsert:
		return !d.UsesOld
	case TriggerEventDelete:
		return !d.UsesNew
	}
	return true
}
