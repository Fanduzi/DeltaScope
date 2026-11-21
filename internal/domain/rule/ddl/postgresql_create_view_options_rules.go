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
	if s.Dialect != spec.DialectPostgreSQL || s.Unsupported != nil || s.DDL == nil || s.DDL.Operation != spec.DDLOperationCreateView {
		return false
	}
	return len(s.DDL.PGViewOptions) != 0 || pgLiteralViewCheckOption(s.DDL)
}

func pgLiteralViewCheckOption(ddl *spec.DDL) bool {
	if ddl.PGCreateViewOutput == nil || ddl.PGCreateViewOutput.Columns == nil {
		return false
	}
	if ddl.Options["check_option"] == "true" {
		return true
	}
	for _, option := range ddl.PGViewOptions {
		if option.Namespace == "" && option.Name == "check_option" {
			return true
		}
	}
	return false
}

func assessPGCreateViewOptions(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if pgLiteralViewCheckOption(s.DDL) {
		return "view_not_automatically_updatable", ""
	}
	return assessPGViewOptionDeclarations(s.DDL.PGViewOptions, s.DDL.Options["check_option"] == "true", false)
}

func assessPGViewOptionDeclarations(options []spec.PGViewOption, checkOptionClause, alter bool) (reason, fact string) {
	seen := make(map[string]bool, len(options))
	for _, option := range options {
		if option.Name == "" {
			return "", "view.options"
		}
		if option.Namespace != "" && (!alter || option.Namespace != "toast") {
			return "view_option_namespace_unrecognized", ""
		}
	}
	for _, option := range options {
		if option.Namespace != "" {
			continue
		}
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
	if seen["check_option"] && checkOptionClause {
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
	case "view_not_automatically_updatable":
		message = "PostgreSQL requires an automatically updatable view for CHECK OPTION"
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
