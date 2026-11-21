package spec

// PGCreateColumns records direct column-name and qualified type premises.
type PGCreateColumns struct {
	Target      PGDestructiveTableTarget
	IfNotExists bool
	Refusal     string
	Types       []PGColumnTypeTarget
}
