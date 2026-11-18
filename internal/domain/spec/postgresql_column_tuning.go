package spec

// PGColumnOptionValueKind distinguishes modeled numeric literals from opaque
// values. Quoted strings and expressions are never retained as payloads.
type PGColumnOptionValueKind string

const (
	PGColumnOptionNumeric   PGColumnOptionValueKind = "numeric"
	PGColumnOptionQuoted    PGColumnOptionValueKind = "quoted"
	PGColumnOptionAbsent    PGColumnOptionValueKind = "absent"
	PGColumnOptionUnmodeled PGColumnOptionValueKind = "unmodeled"
)

// PGColumnOption preserves a column option's exact key, namespace and order.
// Numeric is non-nil only for a proven finite numeric AST literal.
type PGColumnOption struct {
	Name      string
	Namespace string
	ValueKind PGColumnOptionValueKind
	Numeric   *float64
}

// PGColumnTuning separates authored DEFAULT from a numeric statistics target
// and retains ordered options rather than a lossy count/last-wins map.
type PGColumnTuning struct {
	StatisticsDefault bool
	StatisticsValue   *int64
	Options           []PGColumnOption
	Unmodeled         bool
}
