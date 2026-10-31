package audit

import (
	"context"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// Schema lifecycle ordered-state (T17, issue #96): schemaEntries tracks one
// entry per schema identity the batch touched, with the same three-state
// knowledge classes tables carry. A batch-created schema is provably
// present for later statements even though the provider cannot see it; a
// batch-dropped schema is provably absent and every tracked table identity
// under it dies with it; anything else stays unvouched rather than
// inheriting a stale provider answer or a fabricated default.
type schemaKnowledge int

const (
	schemaNeverTouched schemaKnowledge = iota
	schemaPresent
	schemaAbsent
	schemaUnknown
)

// batchSchemaEntry is one schema's ordered fact: existence plus the proven
// default attributes (charset, collation, encryption, placement_policy).
// An absent attrs key means the attribute was never proven — authored
// declarations land verbatim, engine derivations land through the frozen
// derivation table, and everything else stays withheld.
type batchSchemaEntry struct {
	state schemaKnowledge
	attrs map[string]string
	// batchCreated marks a schema the batch itself published: the batch is
	// the only writer under it, so a never-touched table identity there is
	// provably absent without a provider read.
	batchCreated bool
}

// schemaTargetName resolves the schema identity a schema-scoped statement
// binds. A named object uses its own name; a name-less ALTER DATABASE
// (AlterDefaultSchema) binds the request's trusted schema — empty when the
// request carries none, which the callers treat as unvouched rather than a
// connection-derived guess.
func schemaTargetName(ddl *spec.DDL, requestSchema string) string {
	if ddl == nil {
		return ""
	}
	switch ddl.Operation {
	case spec.DDLOperationCreateSchema, spec.DDLOperationDropSchema:
		return strings.ToLower(strings.TrimSpace(ddl.ObjectName))
	case spec.DDLOperationAlterSchema:
		if ddl.AlterDefaultSchema {
			return strings.ToLower(strings.TrimSpace(requestSchema))
		}
		return strings.ToLower(strings.TrimSpace(ddl.ObjectName))
	}
	return ""
}

// resolveSchema returns the schema entry for name, consulting the provider's
// ObjectResolver exactly once per schema per request. Provider answers land
// as pre-state entries: confirmed → present + observed attributes, not-found
// → absent, anything else → unknown. Entries the batch itself published are
// answered from the batch, never re-asked.
func (s *batchState) resolveSchema(ctx context.Context, name string) (*batchSchemaEntry, error) {
	if entry, ok := s.schemaEntries[name]; ok {
		return entry, nil
	}
	entry := &batchSchemaEntry{state: schemaUnknown}
	if s.contaminated || s.provider == nil {
		s.schemaEntries[name] = entry
		return entry, nil
	}
	var snapshot *spec.ObjectSnapshot
	var err error
	if s.schemaResolver != nil {
		snapshot, err = s.schemaResolver(ctx, name)
	} else {
		resolver, ok := s.provider.(ObjectResolver)
		if !ok {
			s.schemaEntries[name] = entry
			return entry, nil
		}
		snapshot, err = resolver.ResolveObject(ctx, s.dialect, spec.ObjectLookupRequest{
			Schema: s.schema,
			Type:   "schema",
			Name:   name,
		})
	}
	if err != nil {
		return nil, err
	}
	switch {
	case snapshot == nil:
	case snapshot.IsConfirmed() && snapshot.Exists:
		entry.state = schemaPresent
		entry.attrs = map[string]string{}
		for k, v := range snapshot.Attributes {
			entry.attrs[k] = v
		}
	case snapshot.IsNotFound():
		entry.state = schemaAbsent
	default:
	}
	s.schemaEntries[name] = entry
	return entry, nil
}

// schemaAttrOutcome is the three-way result of option adjudication:
// publish the computed attributes, withhold them (the statement's landing
// is undecidable), or refuse (a version-proven parse/option rejection —
// the statement never executes, so the pre-state stands verbatim).
type schemaAttrOutcome int

const (
	schemaAttrsApply schemaAttrOutcome = iota
	schemaAttrsWithhold
	schemaAttrsRefused
)

// schemaPostAttrs computes the attribute set a successful CREATE or ALTER
// publishes. Declared options land verbatim; the engine's deterministic
// derivations land alongside (charset→dialect-default collation,
// collation→its charset); absent declarations keep the prior entry's proven
// values on ALTER or derive the server defaults on CREATE. Version-gated
// options adjudicate by the resolved version: ENCRYPTION before 8.0.16 is a
// proven parse refusal (5.7 answers 1064), an unproven version withholds,
// TiDB's accepted-ignored form publishes nothing, and a TiDB placement
// clause's binding premise is unvouchable today.
func schemaPostAttrs(dialect spec.Dialect, version *spec.VersionIdentity, prior *batchSchemaEntry, declared map[string]string, creating bool) (map[string]string, schemaAttrOutcome) {
	attrs := map[string]string{}
	if prior != nil {
		for k, v := range prior.attrs {
			attrs[k] = v
		}
	}
	charset, hasCharset := declared["charset"]
	collate, hasCollate := declared["collate"]
	switch {
	case hasCharset && hasCollate:
		attrs["charset"] = charset
		attrs["collation"] = collate
	case hasCharset:
		attrs["charset"] = charset
		if derived := defaultCollationForCharset(dialect, charset); derived != "" {
			attrs["collation"] = derived
		} else {
			delete(attrs, "collation")
		}
	case hasCollate:
		attrs["collation"] = collate
		if derived := charsetOfCollation(collate); derived != "" {
			attrs["charset"] = derived
		} else {
			delete(attrs, "charset")
		}
	case creating:
		if defCharset, defCollate := schemaServerDefaults(dialect, version); defCharset != "" {
			attrs["charset"] = defCharset
			attrs["collation"] = defCollate
		}
	}
	if enc, ok := declared["encryption"]; ok {
		switch dialect {
		case spec.DialectTiDB:
			// TiDB accepts DEFAULT ENCRYPTION and ignores it — no schema
			// attribute ever lands (probed rc0 + warning 1064, no SCHEMATA
			// column).
			delete(attrs, "encryption")
		case spec.DialectMySQL:
			if version == nil {
				// Version unproven: the option's applicability is
				// undecidable → the whole outcome stays withheld.
				return nil, schemaAttrsWithhold
			}
			if version.Major < 8 || (version.Major == 8 && version.Minor == 0 && version.Patch < 16) {
				// ENCRYPTION before 8.0.16 is a proven parse refusal
				// (5.7 answers 1064) — the statement never executes.
				return nil, schemaAttrsRefused
			}
			attrs["encryption"] = schemaEncryptionValue(enc)
		}
	}
	if _, ok := declared["placement_policy"]; ok {
		if dialect == spec.DialectTiDB {
			// Binding premise — the named placement policy's existence —
			// is unvouchable today (8239 refusal is the engine's answer to
			// a missing policy, and no provider lookup exists). The whole
			// outcome stays withheld rather than fabricating either side.
			return nil, schemaAttrsWithhold
		}
		// MySQL refuses the clause outright (1064 vendor boundary) — the
		// statement never executes.
		return nil, schemaAttrsRefused
	}
	return attrs, schemaAttrsApply
}

// schemaEncryptionValue normalizes a declared ENCRYPTION clause to the
// SCHEMATA vocabulary: 'Y'/'N' input lands as 'yes'/'no' attributes.
func schemaEncryptionValue(declared string) string {
	switch strings.ToLower(strings.TrimSpace(declared)) {
	case "y", "yes", "1":
		return "yes"
	case "n", "no", "0":
		return "no"
	}
	return strings.ToLower(strings.TrimSpace(declared))
}

// schemaServerDefaults is the frozen per-anchor derivation of a CREATE
// DATABASE that declares neither charset nor collation — the server's own
// defaults, version-known (probed):
// MySQL 5.7 latin1/latin1_swedish_ci, MySQL 8.x utf8mb4/utf8mb4_0900_ai_ci,
// TiDB utf8mb4/utf8mb4_bin. An unproven version withholds both facts rather
// than guessing the newest defaults.
func schemaServerDefaults(dialect spec.Dialect, version *spec.VersionIdentity) (string, string) {
	if dialect == spec.DialectTiDB {
		return "utf8mb4", "utf8mb4_bin"
	}
	if dialect != spec.DialectMySQL || version == nil {
		return "", ""
	}
	if version.Major < 8 {
		return "latin1", "latin1_swedish_ci"
	}
	return "utf8mb4", "utf8mb4_0900_ai_ci"
}

// defaultCollationForCharset is the frozen per-anchor derivation of the
// collation an engine assigns when only CHARACTER SET is declared (probed:
// latin1→latin1_swedish_ci on MySQL, latin1→latin1_bin on TiDB; utf8mb4
// follows the server default). Unknown charsets withhold rather than guess.
func defaultCollationForCharset(dialect spec.Dialect, charset string) string {
	switch strings.ToLower(charset) {
	case "utf8mb4":
		if dialect == spec.DialectTiDB {
			return "utf8mb4_bin"
		}
		return "utf8mb4_0900_ai_ci"
	case "utf8":
		if dialect == spec.DialectTiDB {
			return "utf8_bin"
		}
		return "utf8_general_ci"
	case "latin1":
		if dialect == spec.DialectTiDB {
			return "latin1_bin"
		}
		return "latin1_swedish_ci"
	case "ascii":
		return "ascii_general_ci"
	case "gbk":
		return "gbk_chinese_ci"
	case "utf8mb3":
		if dialect == spec.DialectTiDB {
			return "utf8mb3_bin"
		}
		return "utf8mb3_general_ci"
	case "binary":
		return "binary"
	case "big5":
		return "big5_chinese_ci"
	case "ucs2":
		return "ucs2_general_ci"
	case "utf16":
		return "utf16_general_ci"
	case "utf32":
		return "utf32_general_ci"
	case "gb18030":
		return "gb18030_chinese_ci"
	}
	return ""
}

// applyCreateSchema models CREATE DATABASE/SCHEMA: refusal on a proven
// duplicate (1007, pre-state kept), no-op on IF NOT EXISTS over present,
// withheld outcome over unvouched existence, otherwise publish present with
// declared/derived attributes.
func (s *batchState) applyCreateSchema(ctx context.Context, statement spec.Statement) error {
	name := schemaTargetName(statement.DDL, s.schema)
	if name == "" {
		s.contaminated = true
		return nil
	}
	entry, err := s.resolveSchema(ctx, name)
	if err != nil {
		return err
	}
	declared := schemaDeclaredOptions(statement.DDL)
	// A MySQL-side placement clause never executes (1064): proven refusal.
	if _, ok := declared["placement_policy"]; ok && s.dialect != spec.DialectTiDB {
		return nil
	}
	switch entry.state {
	case schemaPresent:
		// IF NOT EXISTS over present is a proven no-op (note 1007); a bare
		// create over present is a proven refusal. Both keep the pre-state.
		return nil
	case schemaAbsent:
		attrs, outcome := schemaPostAttrs(s.dialect, s.resolvedVersion, nil, declared, true)
		switch outcome {
		case schemaAttrsRefused:
			// Proven option refusal: the statement never executes, so the
			// absent pre-state stands — no tombstone, no withheld shape.
			return nil
		case schemaAttrsWithhold:
			entry.state = schemaUnknown
			entry.attrs = nil
			return nil
		}
		entry.state = schemaPresent
		entry.attrs = attrs
		entry.batchCreated = true
		return nil
	default:
		// CREATE ... IF NOT EXISTS over an unvouched schema proves presence
		// either way: the no-op arm keeps a pre-existing schema, the create
		// arm publishes a new one. Declared attributes stay withheld — on
		// the no-op arm the engine ignored them, and which arm ran is
		// undecidable. batchCreated stays false: a pre-existing schema may
		// hold tables the batch never saw.
		if statement.DDL.Options["if_not_exists"] == "true" {
			entry.state = schemaPresent
			entry.attrs = nil
		}
		return nil
	}
}

// target (3503/1049/err-1 by anchor — every form refuses, pre-state kept),
// withheld declared attributes over unvouched premises (placement binding,
// unknown-version encryption), declared-attribute publication otherwise.
// Undeclared attributes keep their proven values; a charset-only change
// resets collation through the derivation table.
func (s *batchState) applyAlterSchema(ctx context.Context, statement spec.Statement) error {
	name := schemaTargetName(statement.DDL, s.schema)
	if name == "" {
		// A name-less ALTER with no trusted request schema cannot be bound —
		// its scope is unresolvable, so the batch contaminates rather than
		// guessing a connection default.
		s.contaminated = true
		return nil
	}
	entry, err := s.resolveSchema(ctx, name)
	if err != nil {
		return err
	}
	declared := schemaDeclaredOptions(statement.DDL)
	if _, ok := declared["placement_policy"]; ok && s.dialect != spec.DialectTiDB {
		return nil
	}
	switch entry.state {
	case schemaAbsent:
		return nil
	case schemaPresent:
		attrs, outcome := schemaPostAttrs(s.dialect, s.resolvedVersion, entry, declared, false)
		switch outcome {
		case schemaAttrsRefused:
			// Proven option refusal keeps the whole pre-state verbatim.
			return nil
		case schemaAttrsWithhold:
			// Refused-or-succeeded is undecidable: existence stays proven
			// (both arms preserve it) but every declared attribute the
			// statement could have changed is withheld.
			for k := range declared {
				delete(entry.attrs, k)
			}
			if _, ok := declared["charset"]; ok {
				delete(entry.attrs, "collation")
			}
			return nil
		}
		entry.attrs = attrs
		return nil
	default:
		return nil
	}
}

// applyDropSchema models DROP DATABASE/SCHEMA: refusal on proven-absent
// without IF EXISTS (1008), no-op on IF EXISTS over absent, withheld outcome
// over unvouched existence, otherwise proven drop — the schema tombstones to
// absent and every tracked table identity under it (persistent and
// temporary) becomes proven-absent with it.
func (s *batchState) applyDropSchema(ctx context.Context, statement spec.Statement) error {
	name := schemaTargetName(statement.DDL, s.schema)
	if name == "" {
		s.contaminated = true
		return nil
	}
	entry, err := s.resolveSchema(ctx, name)
	if err != nil {
		return err
	}
	switch entry.state {
	case schemaPresent:
		entry.state = schemaAbsent
		entry.attrs = nil
		for key := range s.entries {
			if key.schema == name {
				s.entries[key] = &batchTableEntry{state: tableAbsent}
			}
		}
		for key := range s.tempEntries {
			if key.schema == name {
				s.tempEntries[key] = &batchTableEntry{state: tableAbsent}
			}
		}
		return nil
	case schemaAbsent:
		return nil
	default:
		if statement.DDL.Options["if_exists"] == "true" {
			// DROP ... IF EXISTS proves absence either way: the drop arm
			// removes a pre-existing schema, the no-op arm confirms there
			// was none. Every tracked table identity under it is absent in
			// both arms — a dropped schema's tables die with it, an absent
			// schema never held them.
			entry.state = schemaAbsent
			entry.attrs = nil
			for key := range s.entries {
				if key.schema == name {
					s.entries[key] = &batchTableEntry{state: tableAbsent}
				}
			}
			for key := range s.tempEntries {
				if key.schema == name {
					s.tempEntries[key] = &batchTableEntry{state: tableAbsent}
				}
			}
			return nil
		}
		// Undecidable drop: the schema and every identity under it stay
		// unvouched — never marked deleted.
		entry.state = schemaUnknown
		for key := range s.entries {
			if key.schema == name {
				s.entries[key] = &batchTableEntry{state: tableUnknown}
			}
		}
		for key := range s.tempEntries {
			if key.schema == name {
				s.tempEntries[key] = &batchTableEntry{state: tableUnknown}
			}
		}
		return nil
	}
}

// schemaDeclaredOptions reads the authored schema-level options from the
// normalized DDL — the same map the extractor fills, minus the clause flags
// (if_exists/if_not_exists are premises, not attributes).
func schemaDeclaredOptions(ddl *spec.DDL) map[string]string {
	declared := map[string]string{}
	for k, v := range ddl.Options {
		switch k {
		case "charset", "collate", "encryption", "placement_policy":
			declared[k] = v
		}
	}
	return declared
}

// projectSchemaTarget renders the statement-visible pre-state snapshot for a
// schema-scoped statement: proven attributes on present, an exists=false
// snapshot on absent, nil when the schema is unvouched or the statement is
// not schema-scoped.
func (s *batchState) projectSchemaTarget(ctx context.Context, statement spec.Statement) (*spec.ObjectSnapshot, error) {
	if statement.DDL == nil {
		return nil, nil
	}
	switch statement.DDL.Operation {
	case spec.DDLOperationCreateSchema, spec.DDLOperationAlterSchema, spec.DDLOperationDropSchema:
	default:
		return nil, nil
	}
	name := schemaTargetName(statement.DDL, s.schema)
	if name == "" {
		return nil, nil
	}
	entry, err := s.resolveSchema(ctx, name)
	if err != nil {
		return nil, err
	}
	snapshot := &spec.ObjectSnapshot{Type: "schema", Name: name, Schema: name}
	switch entry.state {
	case schemaPresent:
		snapshot.Status = spec.MetadataStatusConfirmed
		snapshot.Exists = true
		snapshot.Attributes = map[string]string{}
		for k, v := range entry.attrs {
			snapshot.Attributes[k] = v
		}
		return snapshot, nil
	case schemaAbsent:
		snapshot.Status = spec.MetadataStatusNotFound
		return snapshot, nil
	default:
		return nil, nil
	}
}

// schemaStateForTarget reports the batch's current knowledge of the schema
// an unqualified-or-qualified table identity resolves under — the ordered
// successor of invalidatedSchemas. Never-touched schemas answer
// schemaNeverTouched so provider reads proceed unchanged.
func (s *batchState) schemaStateForTarget(schema string) schemaKnowledge {
	entry, ok := s.schemaEntries[strings.ToLower(schema)]
	if !ok {
		return schemaNeverTouched
	}
	return entry.state
}
