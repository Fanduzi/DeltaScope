package spec

// routine.go — stored routine (PROCEDURE/FUNCTION) declarations for the
// MySQL family (issue #106 T27). The declaration carries the definitional
// header plus the statically bounded body; body statements reuse the
// shared spec.Statement model so rule evaluation stays identical to
// top-level SQL. The body is never executed, never applied to ordered
// schema state, and never duplicated into serialized output — findings
// reference offsets inside the parent statement's RawSQL.

// RoutineKind distinguishes stored procedures from stored functions.
type RoutineKind string

const (
	RoutineKindProcedure RoutineKind = "procedure"
	RoutineKindFunction  RoutineKind = "function"
)

// RoutineCharacteristic is one modeled routine option. Only bounded key
// shapes enter the model; free-text payloads (COMMENT) and anything not
// in the modeled set stay unextracted evidence, never strings stored here.
type RoutineCharacteristic struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
	// Offset is the byte offset of the characteristic's first token
	// inside the parent statement's RawSQL; negative when unknown.
	Offset int `json:"-"`
}

// RoutineBodyStatement binds one extracted body statement to its source
// position inside the parent CREATE's RawSQL.
type RoutineBodyStatement struct {
	Statement `json:"-"`
	// Offset is the byte offset of this body statement's first
	// non-whitespace byte inside the parent statement's RawSQL.
	Offset int `json:"-"`
}

// RoutineBody is the parsed compound (BEGIN…END) or single-statement
// body of a routine definition.
type RoutineBody struct {
	// Compound reports BEGIN…END framing; false means a bare
	// single-statement body (CREATE PROCEDURE p() DELETE …).
	Compound bool `json:"compound,omitempty"`
	// Offset is the byte offset of the body region's first token inside
	// the parent statement's RawSQL — the BEGIN keyword for compound
	// bodies, the statement text otherwise.
	Offset int `json:"-"`
	// Statements carries each statically parsed body member in source
	// order. Dynamic forms (EXECUTE/prepare-built text) never appear
	// here — they are boundaries, not static members.
	Statements []RoutineBodyStatement `json:"-"`
}

// RoutineDeclaration is the modeled CREATE PROCEDURE/FUNCTION
// definition. Fields record definitional facts only — the model never
// predicts runtime branches or applies body effects to outer state.
type RoutineDeclaration struct {
	Kind RoutineKind `json:"kind"`
	// Name is the routine identity exactly as authored (schema.qualifier
	// preserved in Schema when written).
	Name Table `json:"name"`
	// Definer records the authored DEFINER clause spelling shape:
	// "user@host" for a named account, "current_user" for the function
	// form; empty when the clause is absent (the runtime then resolves
	// the creator — unproven, not guessed).
	Definer string `json:"definer,omitempty"`
	// DefinerOffset is the byte offset of the DEFINER value's first
	// token inside the parent RawSQL; negative when absent/unknown.
	DefinerOffset int `json:"-"`
	// ParamsPresent marks an authored parameter list. The individual
	// parameter declarations are parsed for boundary correctness but not
	// yet modeled — presence keeps the fact auditable as a gap.
	ParamsPresent bool `json:"params_present,omitempty"`
	// ReturnsType is the authored RETURNS type spelling on FUNCTION —
	// normalized lowercase base type, length/attribute payload omitted.
	ReturnsType string `json:"returns_type,omitempty"`
	// Characteristics carries the modeled routine options
	// (deterministic, sql security, contains-sql family, language).
	Characteristics []RoutineCharacteristic `json:"characteristics,omitempty"`
	// Body is the statically bounded body; nil for routines whose
	// definition carries no parseable body (UDF SONAME forms).
	Body *RoutineBody `json:"-"`
}
