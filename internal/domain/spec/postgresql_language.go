package spec

// PGLanguageDeclaration binds an explicit handler-form CREATE LANGUAGE.
type PGLanguageDeclaration struct {
	Name             string
	Replace, Trusted bool
	Routines         []PGLanguageRoutine
}

// PGLanguageRoutine holds the exact authored function identity and its role.
type PGLanguageRoutine struct {
	Kind, Catalog, Schema, Name string
}

// PGLanguageRoutineState records local function-signature facts.
type PGLanguageRoutineState struct {
	Target                PGLanguageRoutine
	Exists, HandlerReturn bool
}

// PGLanguageState holds the current database's declaration prerequisites.
type PGLanguageState struct {
	Name, Database string
	Exists         bool
	Routines       []PGLanguageRoutineState
}

// PGLanguageLookup requests exact existence and function-signature facts.
func PGLanguageLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil || s.DDL.PGLanguage == nil {
		return nil
	}
	d := s.DDL.PGLanguage
	if d.Name == "" {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_language_declaration", Name: d.Name, PGLanguageRoutines: d.Routines}
}
