package spec

// event.go — typed MySQL event declaration (T30/#109). Event bodies
// reuse the T28 RoutineBody member model and walker; schedule,
// completion and enabled-state attributes stay modeled facts so the
// audit can report scheduled-execution risk without predicting runtime
// outcomes.

// EventScheduleKind distinguishes one-shot from repeating schedules.
type EventScheduleKind string

const (
	EventScheduleAt    EventScheduleKind = "at"
	EventScheduleEvery EventScheduleKind = "every"
)

// EventSchedule records the authored ON SCHEDULE clause verbatim: the
// audit models its shape and repetition, never evaluates clock time.
type EventSchedule struct {
	Kind   EventScheduleKind `json:"-"`
	Every  string            `json:"-"` // EVERY <interval> <unit> authored text
	At     string            `json:"-"` // AT <timestamp expr> authored text
	Starts string            `json:"-"` // STARTS <timestamp expr> authored text
	Ends   string            `json:"-"` // ENDS <timestamp expr> authored text
}

// Repeating reports whether the schedule repeats (EVERY) versus a
// single AT firing.
func (s *EventSchedule) Repeating() bool { return s != nil && s.Kind == EventScheduleEvery }

// EventStatus is the authored enablement clause.
type EventStatus string

const (
	EventStatusEnabled        EventStatus = "enable"
	EventStatusDisabled       EventStatus = "disable"
	EventStatusDisableOnSlave EventStatus = "disable_on_slave"
)

// EventKind is the lifecycle operation the declaration carries.
type EventKind string

const (
	EventKindCreate EventKind = "create"
	EventKindAlter  EventKind = "alter"
	EventKindDrop   EventKind = "drop"
)

// EventDeclaration is the normalized form the audit consumes. Body
// stays nonserialized like RoutineDeclaration bodies; Definer keeps the
// clause shape only (never the account payload).
type EventDeclaration struct {
	CatalogName, CatalogRenameTo Table          `json:"-"`
	Kind                         EventKind      `json:"-"`
	Name                         Table          `json:"-"`
	Definer                      string         `json:"-"` // clause shape only, never the account
	Schedule                     *EventSchedule `json:"-"`
	OnCompletion                 *bool          `json:"-"` // nil=unauthored, true=PRESERVE, false=NOT PRESERVE
	Status                       EventStatus    `json:"-"`
	StatusSet                    bool           `json:"-"` // ENABLE/DISABLE/DISABLE ON SLAVE authored
	CommentDeclared              bool           `json:"-"` // COMMENT clause authored; text never stored
	RenameTo                     *Table         `json:"-"` // ALTER EVENT ... RENAME TO
	IfExists                     bool           `json:"-"` // DROP EVENT IF EXISTS
	IfNotExists                  bool           `json:"-"` // CREATE EVENT IF NOT EXISTS
	Body                         *RoutineBody   `json:"-"`
}

// EffectiveEnabled reports whether the event is scheduled to execute
// somewhere: CREATE defaults to enabled, ALTER changes state only when
// authored, and DISABLE ON SLAVE still executes on the primary — only
// a full DISABLE stops the schedule.
func (d *EventDeclaration) EffectiveEnabled() bool {
	switch {
	case d.StatusSet:
		return d.Status != EventStatusDisabled
	case d.Kind == EventKindCreate:
		return true
	default:
		return false
	}
}
