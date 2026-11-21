package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgCreateStorageRule struct{ level rule.Level }

func newPGCreateStorageRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgCreateStorageRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgCreateStorageRule) ID() string { return ruleIDPGCreateStorageValid }

func (r pgCreateStorageRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationCreateTable && s.DDL.PGCreateStorage != nil
}

func assessPGCreateStorage(s spec.Statement) (reason, parameter, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "", "instance.version"
	}
	if spec.PGCreateStorageLookup(s) == nil {
		return "", "", "create_table.storage_namespace"
	}
	storage := s.DDL.PGCreateStorage
	state := s.Metadata.PGCreateStorage
	if state == nil || state.Target != storage.Target || state.Method != storage.Method || state.Tablespace != storage.Tablespace {
		return "", "", "create_table.storage_catalog"
	}
	switch {
	case !state.SchemaExists:
		return "create_storage_schema_absent", "", ""
	case storage.IfNotExists && state.TargetExists:
		return "", "", ""
	case storage.Tablespace != "" && !state.TablespaceExists:
		return "create_storage_tablespace_absent", "", ""
	case storage.Tablespace != "" && state.Global:
		return "create_storage_global_tablespace", "", ""
	}
	if len(storage.Options) != 0 {
		reason, parameter, fact = assessPGTableReloptionDeclarations(storage.Options, false)
		if reason != "" || fact != "" {
			return reason, parameter, fact
		}
	}
	if storage.Method != "" && state.MethodKind != "t" {
		if state.MethodKind == "" {
			return "create_storage_method_absent", "", ""
		}
		return "create_storage_method_not_table", "", ""
	}
	return "", "", ""
}

func (r pgCreateStorageRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, parameter, _ := assessPGCreateStorage(s)
	if reason == "" {
		return nil, nil
	}
	message := "PostgreSQL rejects this table storage option declaration"
	switch reason {
	case "create_storage_schema_absent":
		message = "The PostgreSQL table creation schema does not exist"
	case "create_storage_tablespace_absent":
		message = "The PostgreSQL creation tablespace does not exist"
	case "create_storage_global_tablespace":
		message = "PostgreSQL reserves this tablespace for shared relations"
	case "create_storage_method_absent":
		message = "The PostgreSQL access method does not exist"
	case "create_storage_method_not_table":
		message = "PostgreSQL requires a table access method for CREATE TABLE"
	}
	target := s.DDL.PGCreateStorage.Target.Table
	metadata := map[string]any{"schema": target.Schema, "table": target.Name, "reason_code": reason}
	if parameter != "" {
		metadata["parameter"] = parameter
	}
	return []rule.Finding{{Level: r.level, Message: message, Metadata: metadata}}, nil
}

func (r pgCreateStorageRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, _, fact := assessPGCreateStorage(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_create_storage", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
