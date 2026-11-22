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
// position inside the parent CREATE's RawSQL. Exactly one member payload
// may be set in addition to the embedded Statement — SQL members carry
// the parsed statement; structured members keep their own model (T28).
type RoutineBodyStatement struct {
	Statement `json:"-"`
	// Offset is the byte offset of this body statement's first
	// non-whitespace byte inside the parent statement's RawSQL.
	Offset int `json:"-"`
	// Compound is a parsed control-flow member (IF/CASE/WHILE/LOOP/
	// REPEAT/nested BEGIN) whose inner members were extracted through
	// the same machinery — nil for flat members.
	Compound *RoutineCompound `json:"-"`
	// Declare is a parsed DECLARE member — variable, condition,
	// handler, or cursor declarations. Handler and cursor payloads keep
	// their executable member for normal rule evaluation.
	Declare *RoutineDeclare `json:"-"`
	// Set is a parsed SET assignment member — recorded so variable
	// bindings can resolve statically known PREPARE sources.
	Set *RoutineSet `json:"-"`
	// Prepare is a parsed PREPARE name FROM source member — the source
	// keeps its authored shape; evaluation resolves what is statically
	// known and bounds the rest (T28).
	Prepare *RoutinePrepare `json:"-"`
	// Flow marks a modeled flow-control member with no SQL effect —
	// LEAVE/ITERATE, SIGNAL/RESIGNAL, OPEN/FETCH/CLOSE, GET DIAGNOSTICS.
	// It carries the leading verb, lowercased.
	Flow string `json:"-"`
}

// RoutineCompound is one parsed control-flow construct inside a routine
// body. Branches holds IF/CASE arm bodies in source order (including
// ELSE); Body holds the single member list of WHILE/LOOP/REPEAT/nested
// BEGIN constructs. Inner members are extracted recursively — depth and
// member-count bounds stay explicit (T28).
type RoutineCompound struct {
	// Kind is the normalized construct keyword: if|case|while|loop|
	// repeat|begin.
	Kind string `json:"kind"`
	// Label is the authored begin/loop label when present.
	Label string `json:"label,omitempty"`
	// Branches carries ordered arm bodies for IF (THEN/ELSEIF…/ELSE)
	// and CASE (WHEN…/ELSE) constructs.
	Branches []*RoutineBody `json:"-"`
	// HasElse marks an authored ELSE arm — branched constructs without
	// one keep an implicit no-op path for binding merges.
	HasElse bool `json:"-"`
	// Body carries the member list of WHILE/LOOP/REPEAT/nested BEGIN.
	Body *RoutineBody `json:"-"`
}

// RoutineDeclare models one DECLARE member. Variable declarations bind
// declared names; condition declarations are definitional facts;
// handler and cursor declarations keep their executable member so it
// audits through the normal machinery (T28).
type RoutineDeclare struct {
	// Kind is variable|condition|handler|cursor.
	Kind string `json:"kind"`
	// Names carries the declared identifier(s), lowercased.
	Names []string `json:"names,omitempty"`
	// VarType is the declared data-type text for variable declarations,
	// normalized to a lowercase token list (e.g. "int", "decimal(10,2)").
	VarType string `json:"var_type,omitempty"`
	// Member is the handler's executable statement or the cursor's
	// SELECT — audited through the same member machinery.
	Member *RoutineBodyStatement `json:"-"`
}

// RoutineAssignment is one SET target = expression binding. Literal
// records the resolved text only when the right-hand side is statically
// a string literal (or a run of adjacent literals); HasLiteral marks
// that proof — any other expression shape stays dynamic.
type RoutineAssignment struct {
	// Target is the assigned variable name lowercased: "@q", "q".
	// System variables (@@…) and non-variable targets never enter here.
	Target string `json:"target"`
	// Literal is the unquoted literal content when HasLiteral is true.
	Literal string `json:"literal,omitempty"`
	// HasLiteral marks a statically proven string-literal source.
	HasLiteral bool `json:"has_literal,omitempty"`
}

// RoutineSet models a SET member's variable assignments.
type RoutineSet struct {
	Assignments []RoutineAssignment `json:"assignments,omitempty"`
	// Unmodeled marks assignment members whose target was not a bindable
	// variable (@@system vars and unusual shapes) — kept visible as a
	// bounded effect rather than silently dropped (T28).
	Unmodeled bool `json:"unmodeled,omitempty"`
}

// RoutinePrepareSource describes the authored FROM expression of a
// PREPARE member: a string literal, a variable reference, or any other
// expression shape. Only literal sources (direct or resolved through
// SET bindings) can ever yield statically auditable SQL.
type RoutinePrepareSource struct {
	// Kind is literal|var|expr.
	Kind string `json:"kind"`
	// Literal is the unquoted literal content when Kind=literal.
	Literal string `json:"literal,omitempty"`
	// Var is the referenced variable name when Kind=var.
	Var string `json:"var,omitempty"`
}

// RoutinePrepare models PREPARE name FROM source inside a routine body.
// SourceOffset is the byte offset of the source expression inside the
// parent RawSQL so resolved-literal audits still point at authored SQL.
type RoutinePrepare struct {
	Name         string               `json:"name"`
	Source       RoutinePrepareSource `json:"source"`
	SourceOffset int                  `json:"-"`
}

// RoutineParam is one declared routine parameter — direction keywords
// for procedures, bare type signatures for functions. Names feed local
// variable shadowing analysis and PREPARE-source resolution (T28).
type RoutineParam struct {
	// Direction is in|out|inout — empty for function parameters.
	Direction string `json:"direction,omitempty"`
	Name      string `json:"name"`
	// VarType is the declared type text normalized to a lowercase token
	// list (e.g. "int", "varchar(32)").
	VarType string `json:"var_type,omitempty"`
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
	CatalogTarget         Table       `json:"-"`
	IfExists, IfNotExists bool        `json:"-"`
	Kind                  RoutineKind `json:"kind"`
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
	// ParamsPresent marks an authored parameter list.
	ParamsPresent bool `json:"params_present,omitempty"`
	// Params carries the modeled parameter declarations in authored
	// order (names + normalized type text); a non-empty list makes
	// ParamsPresent true (T28).
	Params []RoutineParam `json:"params,omitempty"`
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
