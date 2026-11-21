package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgLanguageDeclarationRule struct{ level rule.Level }

func newPGLanguageDeclarationRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgLanguageDeclarationRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgLanguageDeclarationRule) ID() string { return ruleIDPGLanguageDeclarationValid }

func (r pgLanguageDeclarationRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationCreateLanguage && s.DDL.PGLanguage != nil
}

func assessPGLanguage(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	d := s.DDL.PGLanguage
	state := s.Metadata.PGLanguage
	if state == nil || state.Name != d.Name || len(state.Routines) != len(d.Routines) || len(d.Routines) == 0 {
		return "", "language.catalog"
	}
	for i, routine := range d.Routines {
		observed := state.Routines[i]
		if observed.Target != routine || routine.Schema == "" || routine.Name == "" || routine.Schema == "pg_temp" {
			return "", "language.callback_identity"
		}
		if routine.Catalog != "" {
			if state.Database == "" {
				return "", "language.database_identity"
			}
			if routine.Catalog != state.Database {
				return "language_callback_cross_database", ""
			}
		}
		if !observed.Exists {
			return "language_callback_absent", ""
		}
		if routine.Kind == "handler" && !observed.HandlerReturn {
			return "language_handler_return_type", ""
		}
	}
	if state.Exists && !d.Replace {
		return "language_exists", ""
	}
	return "", ""
}

func (r pgLanguageDeclarationRule) Evaluate(_ context.Context, s spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGLanguage(s)
	messages := map[string]string{
		"language_callback_cross_database": "PostgreSQL disallows a language callback in another database",
		"language_callback_absent":         "The PostgreSQL language callback does not have the required signature",
		"language_handler_return_type":     "The PostgreSQL language handler must return language_handler",
		"language_exists":                  "The PostgreSQL language already exists",
	}
	if message := messages[reason]; message != "" {
		return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
			"name": s.DDL.PGLanguage.Name, "reason_code": reason,
		}}}, nil
	}
	return nil, nil
}

func (r pgLanguageDeclarationRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGLanguage(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_language_declaration", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
