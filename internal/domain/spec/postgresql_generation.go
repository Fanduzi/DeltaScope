package spec

// PGColumnGeneration preserves authored PostgreSQL generation declarations.
// Legacy GeneratedWhen/IdentityOptions projections cannot prove uniqueness or
// preserve 64-bit sequence options; new declaration checks use these facts.
// Expressions, account privileges and runtime sequence state are not carried.
type PGColumnGeneration struct {
	IdentityCount int
	StoredCount   int
	Mode          string // a (ALWAYS) or d (BY DEFAULT); only meaningful for one declaration
	ExplicitNull  bool
	// TypeWidth is 16/32/64 for a proven pg_catalog integer scalar, -1
	// for a proven other type, and 0 when name resolution is unproven.
	TypeWidth int
	Options   []SequenceOption // authored order, including duplicates
	Unmodeled bool
}
