package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgCreateViewOptionsRule struct{ level rule.Level }

func newPGCreateViewOptionsRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgCreateViewOptionsRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgCreateViewOptionsRule) ID() string { return ruleIDPGCreateViewOptionsValid }

func (r pgCreateViewOptionsRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationCreateView && len(s.DDL.PGViewOptions) != 0
}

func assessPGCreateViewOptions(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	seen := make(map[string]bool, len(s.DDL.PGViewOptions))
	for _, option := range s.DDL.PGViewOptions {
		if option.Name == "" {
			return "", "view.options"
		}
		if option.Namespace != "" {
			return "view_option_namespace_unrecognized", ""
		}
	}
	for _, option := range s.DDL.PGViewOptions {
		switch option.Name {
		case "security_barrier", "security_invoker", "check_option":
		default:
			return "view_option_unrecognized", ""
		}
		if seen[option.Name] {
			return "view_option_duplicate", ""
		}
		seen[option.Name] = true
		if !option.ValueKnown {
			return "", "view.option_value"
		}
		if !option.ValueValid {
			return "view_option_invalid_value", ""
		}
	}
	if seen["check_option"] && s.DDL.Options["check_option"] == "true" {
		return "view_check_option_duplicate", ""
	}
	return "", ""
}

func (r pgCreateViewOptionsRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGCreateViewOptions(s)
	if reason == "" {
		return nil, nil
	}
	message := "PostgreSQL rejects the declared CREATE VIEW options"
	switch reason {
	case "view_option_namespace_unrecognized":
		message = "PostgreSQL does not accept a namespace on this CREATE VIEW option"
	case "view_option_unrecognized":
		message = "PostgreSQL does not recognize a declared CREATE VIEW option"
	case "view_option_duplicate":
		message = "A CREATE VIEW option is declared more than once"
	case "view_option_invalid_value":
		message = "A CREATE VIEW option has an invalid value"
	case "view_check_option_duplicate":
		message = "CHECK OPTION appears in both the option list and the clause"
	}
	return []rule.Finding{{Level: r.level, Message: message,
		Metadata: map[string]any{"view": s.DDL.ObjectName, "reason_code": reason}}}, nil
}

func (r pgCreateViewOptionsRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGCreateViewOptions(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_view_options", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
