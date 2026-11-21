package spec

// PGBaseTypeDeclaration preserves the destination and ordered options for base type commands.
type PGBaseTypeDeclaration struct {
	Target  PGDestructiveTableTarget
	Action  string
	Options []PGBaseTypeOption
}

// PGBaseTypeOption retains the option identity and bounded storage argument.
type PGBaseTypeOption struct {
	Name       string
	Storage    string
	ValueKnown bool
}
