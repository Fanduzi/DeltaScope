package spec

// PGViewOption stores the exact declaration name and its bounded value assessment.
type PGViewOption struct {
	Name, Namespace string
	ValueKnown      bool
	ValueValid      bool
}
