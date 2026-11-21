package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgWrapperChangeRule struct{ level rule.Level }

func newPGWrapperChangeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgWrapperChangeRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgWrapperChangeRule) ID() string { return ruleIDPGWrapperChangeValid }

func (r pgWrapperChangeRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil &&
		s.DDL != nil && s.DDL.Operation == spec.DDLOperationAlterForeignDataWrapper && s.DDL.PGWrapperChange != nil
}

func assessPGWrapperChange(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	decl := s.DDL.PGWrapperChange
	state := s.Metadata.PGWrapperChange
	if state == nil || state.Name != decl.Name || state.RenameTo != decl.RenameTo || state.Owner != decl.Owner {
		return "", "foreign_data_wrapper.catalog"
	}
	if decl.Owner.Kind != "" && !state.OwnerExists {
		if decl.Owner.Kind == "named" || decl.Owner.Kind == "public" {
			return "wrapper_owner_absent", ""
		}
		return "", "foreign_data_wrapper.owner_identity"
	}
	switch {
	case !state.Exists:
		return "wrapper_source_absent", ""
	case decl.RenameTo != "" && state.DestinationOccupied:
		return "wrapper_destination_occupied", ""
	case decl.Owner.Kind != "" && !state.OwnerSuperuser:
		return "wrapper_owner_not_superuser", ""
	}
	if len(decl.Functions) != len(state.Functions) {
		return "", "foreign_data_wrapper.routines"
	}
	validator := state.HasValidator
	seen := make(map[string]bool, 2)
	for i, fn := range decl.Functions {
		if seen[fn.Kind] {
			return "wrapper_function_duplicate", ""
		}
		seen[fn.Kind] = true
		if fn.Kind == "validator" {
			validator = !fn.Clear
		}
		if fn.Clear {
			continue
		}
		target := spec.PGDestructiveTableTarget{Table: fn.Target}
		observed := state.Functions[i]
		if !target.Qualified() || target.Table.Schema == "pg_toast_temp" || observed.Target != fn.Target || observed.Kind != fn.Kind {
			return "", "foreign_data_wrapper.routine_identity"
		}
		if !observed.Exists {
			return "wrapper_function_absent", ""
		}
		if fn.Kind == "handler" && !observed.Handler {
			return "wrapper_handler_result_invalid", ""
		}
	}
	if reason, fact = assessPGForeignOptions(decl.Options, state.Options); reason != "" || fact != "" {
		return reason, fact
	}
	if len(decl.Options) != 0 && validator {
		return "", "foreign_data_wrapper.validator"
	}
	return "", ""
}

func assessPGForeignOptions(options []spec.PGForeignOption, existing []string) (reason, fact string) {
	if len(options) == 0 {
		return "", ""
	}
	if existing == nil {
		return "", "foreign_options.directory"
	}
	names := make(map[string]bool, len(existing)+len(options))
	for _, name := range existing {
		names[name] = true
	}
	for _, option := range options {
		switch option.Action {
		case "add":
			if names[option.Name] {
				return "foreign_option_exists", ""
			}
			names[option.Name] = true
		case "set", "drop":
			if !names[option.Name] {
				return "foreign_option_absent", ""
			}
			if option.Action == "drop" {
				delete(names, option.Name)
			}
		default:
			return "", "foreign_options.action"
		}
	}
	return "", ""
}

func (r pgWrapperChangeRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGWrapperChange(s)
	messages := map[string]string{
		"wrapper_owner_absent":           "The requested PostgreSQL wrapper owner does not exist",
		"wrapper_owner_not_superuser":    "PostgreSQL requires a superuser as foreign-data wrapper owner",
		"wrapper_source_absent":          "The PostgreSQL foreign-data wrapper does not exist",
		"wrapper_destination_occupied":   "A PostgreSQL foreign-data wrapper occupies the destination name",
		"wrapper_function_duplicate":     "A PostgreSQL wrapper routine option was specified more than once",
		"wrapper_function_absent":        "The PostgreSQL wrapper routine signature does not exist",
		"wrapper_handler_result_invalid": "The PostgreSQL wrapper handler must return fdw_handler",
		"foreign_option_exists":          "The PostgreSQL foreign option already exists",
		"foreign_option_absent":          "The PostgreSQL foreign option does not exist",
	}
	message := messages[reason]
	if message == "" {
		return nil, nil
	}
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"wrapper": s.DDL.PGWrapperChange.Name, "reason_code": reason,
	}}}, nil
}

func (r pgWrapperChangeRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGWrapperChange(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_wrapper_change", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
