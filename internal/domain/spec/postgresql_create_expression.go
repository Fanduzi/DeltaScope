package spec

// PGCreateExpressionRefusal records a rejection proved by a CREATE TABLE expression AST.
type PGCreateExpressionRefusal struct {
	Target      PGDestructiveTableTarget
	IfNotExists bool
	Unbound     bool
	Reason      string
}
