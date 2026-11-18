package spec

// PGTableReloptionValue describes a bounded AST value without retaining
// arbitrary table option text. Integer is used only for fillfactor.
type PGTableReloptionValue uint8

const (
	PGTableReloptionUnknown PGTableReloptionValue = iota
	PGTableReloptionInteger
	PGTableReloptionBoolean
	PGTableReloptionInvalidBoolean
)

// PGTableReloption retains one authored member in list order.
// HasValue records the AST argument independently of its interpretation.
type PGTableReloption struct {
	Name, Namespace string
	HasValue        bool
	Value           PGTableReloptionValue
	Integer         int32
}
