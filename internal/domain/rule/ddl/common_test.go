// Package ddl verifies shared DDL helper behavior.
// input: synthetic alter-table statements and rule identifier constants
// output: test coverage for reusable alter helper boundaries
// pos: domain DDL common helper and rule-ID test coverage
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestRegisterAlterRuleIDsStayStable(t *testing.T) {
	got := []string{
		ruleIDAlterModifyColumnTargetTypeFamilyAllowlist,
		ruleIDAlterChangeColumnTargetTypeFamilyAllowlist,
		ruleIDAlterModifyColumnExplicitNullabilityChangeForbid,
		ruleIDAlterChangeColumnExplicitNullabilityChangeForbid,
		ruleIDAlterModifyColumnExplicitDefaultChangeForbid,
		ruleIDAlterChangeColumnExplicitDefaultChangeForbid,
		ruleIDAlterModifyColumnExplicitAutoIncrementChangeForbid,
		ruleIDAlterChangeColumnExplicitAutoIncrementChangeForbid,
		ruleIDAlterRenameIndexForbid,
		ruleIDAlterAddIndexColumnsMaxCount,
		ruleIDAlterAddIndexDuplicateForbid,
		ruleIDAlterAddIndexUniquePrefixRequire,
		ruleIDAlterAddIndexSecondaryPrefixRequire,
		ruleIDAlterAddIndexFulltextPrefixRequire,
	}
	want := []string{
		"ddl.alter.modify_column.target_type_family.allowlist",
		"ddl.alter.change_column.target_type_family.allowlist",
		"ddl.alter.modify_column.explicit_nullability_change.forbid",
		"ddl.alter.change_column.explicit_nullability_change.forbid",
		"ddl.alter.modify_column.explicit_default_change.forbid",
		"ddl.alter.change_column.explicit_default_change.forbid",
		"ddl.alter.modify_column.explicit_auto_increment_change.forbid",
		"ddl.alter.change_column.explicit_auto_increment_change.forbid",
		"ddl.alter.rename_index.forbid",
		"ddl.alter.add_index.columns.max_count",
		"ddl.alter.add_index.duplicate.forbid",
		"ddl.alter.add_index.unique.prefix.require",
		"ddl.alter.add_index.secondary.prefix.require",
		"ddl.alter.add_index.fulltext.prefix.require",
	}

	for i, wantID := range want {
		if got[i] != wantID {
			t.Fatalf("expected rule id %d to be %q, got %q", i, wantID, got[i])
		}
	}
}

func TestRegisterAlterHelpersExposeSemanticPayloads(t *testing.T) {
	statement := alterStatement(
		spec.Alter{
			Action: "modify_column",
			Name:   "age",
			Column: &spec.AlterColumn{
				Definition: &spec.Column{
					Name:     "age",
					Type:     "bigint(20) unsigned",
					Unsigned: true,
				},
				Change: &spec.AlterColumnChange{
					TouchesNullability: true,
					TouchesDefault:     true,
				},
			},
		},
		spec.Alter{
			Action: "rename_column",
			Name:   "old_email",
			Column: &spec.AlterColumn{
				OldName: "old_email",
				Definition: &spec.Column{
					Name: "email",
				},
			},
		},
		spec.Alter{
			Action: "change_column",
			Name:   "legacy_id",
			Column: &spec.AlterColumn{
				OldName: "legacy_id",
				Definition: &spec.Column{
					Name:          "id",
					Type:          "bigint(20)",
					AutoIncrement: true,
				},
				Change: &spec.AlterColumnChange{
					TouchesNullability:   true,
					TouchesAutoIncrement: true,
				},
			},
		},
		spec.Alter{
			Action: "rename_index",
			Name:   "idx_old",
			Index: &spec.AlterIndex{
				OldName: "idx_old",
				Definition: &spec.Index{
					Name: "idx_new",
				},
			},
		},
		spec.Alter{
			Action: "add_constraint",
			Name:   "uniq_email",
			Index: &spec.AlterIndex{
				Definition: &spec.Index{
					Name:    "uniq_email",
					Kind:    spec.IndexKindUnique,
					Columns: []string{"email"},
				},
			},
		},
		spec.Alter{
			Action:  "table_option",
			Options: map[string]string{"engine": "InnoDB"},
		},
	)

	if !appliesToAlterActions(statement, "rename_column", "rename_index") {
		t.Fatalf("expected statement to apply to selected alter actions")
	}

	renameColumn := matchingAlterActions(statement, "rename_column")
	if len(renameColumn) != 1 {
		t.Fatalf("expected 1 rename_column alter, got %d", len(renameColumn))
	}
	oldName, newName, ok := alterRenameNames(renameColumn[0])
	if !ok || oldName != "old_email" || newName != "email" {
		t.Fatalf("expected rename column old_email->email, got ok=%t old=%q new=%q", ok, oldName, newName)
	}

	renameIndex := matchingAlterActions(statement, "rename_index")
	if len(renameIndex) != 1 {
		t.Fatalf("expected 1 rename_index alter, got %d", len(renameIndex))
	}
	oldName, newName, ok = alterRenameNames(renameIndex[0])
	if !ok || oldName != "idx_old" || newName != "idx_new" {
		t.Fatalf("expected rename index idx_old->idx_new, got ok=%t old=%q new=%q", ok, oldName, newName)
	}

	standaloneRenameIndex := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Alter:     []spec.Alter{{Action: "rename_index", Name: "idx_old", Options: map[string]string{"new_name": "idx_new"}}},
		},
	}
	matchedRename := matchingRenameActions(standaloneRenameIndex, "rename_index")
	if len(matchedRename) != 1 {
		t.Fatalf("expected 1 standalone rename match, got %d", len(matchedRename))
	}
	oldName, newName, ok = alterRenameNames(matchedRename[0])
	if !ok || oldName != "idx_old" || newName != "idx_new" {
		t.Fatalf("expected standalone rename index idx_old->idx_new, got ok=%t old=%q new=%q", ok, oldName, newName)
	}

	modifyColumn := matchingAlterActions(statement, "modify_column")
	if len(modifyColumn) != 1 {
		t.Fatalf("expected 1 modify_column alter, got %d", len(modifyColumn))
	}
	column, ok := alterColumnDefinition(modifyColumn[0])
	if !ok || column.Name != "age" {
		t.Fatalf("expected modify column definition for age, got ok=%t column=%+v", ok, column)
	}
	if got := columnTypeFamily(*column); got != "integer" {
		t.Fatalf("expected bigint column type family integer, got %q", got)
	}
	change, ok := alterColumnChange(modifyColumn[0])
	if !ok || !change.TouchesNullability || !change.TouchesDefault {
		t.Fatalf("expected explicit modify-column change facts, got ok=%t change=%+v", ok, change)
	}
	if !alterTouchesExplicitNullability(modifyColumn[0]) || !alterTouchesExplicitDefault(modifyColumn[0]) {
		t.Fatalf("expected explicit nullability/default helpers to be true for modify column")
	}
	if alterTouchesExplicitAutoIncrement(modifyColumn[0]) {
		t.Fatalf("expected explicit auto_increment helper to stay false for modify column")
	}
	if family, ok := alterTargetColumnTypeFamily(modifyColumn[0]); !ok || family != "integer" {
		t.Fatalf("expected target type family integer, got ok=%t family=%q", ok, family)
	}

	changeColumn := matchingAlterActions(statement, "change_column")
	if len(changeColumn) != 1 {
		t.Fatalf("expected 1 change_column alter, got %d", len(changeColumn))
	}
	if !alterRenamesColumn(changeColumn[0]) {
		t.Fatalf("expected change_column helper to detect rename from legacy_id to id")
	}
	if !alterTouchesExplicitAutoIncrement(changeColumn[0]) {
		t.Fatalf("expected explicit auto_increment helper to be true for change column")
	}

	addIndex := matchingAlterActions(statement, "add_constraint")
	if len(addIndex) != 1 {
		t.Fatalf("expected 1 add_constraint alter, got %d", len(addIndex))
	}
	index, ok := alterIndexDefinition(addIndex[0])
	if !ok || index.Name != "uniq_email" || index.Kind != spec.IndexKindUnique {
		t.Fatalf("expected unique index definition, got ok=%t index=%+v", ok, index)
	}
	projectedIndexes := alterAddedIndexesByKind(statement, spec.IndexKindUnique)
	if len(projectedIndexes) != 1 || projectedIndexes[0].Name != "uniq_email" {
		t.Fatalf("expected projected unique add-index slice, got %+v", projectedIndexes)
	}
	projectedStatement := projectedAlterIndexesStatement(statement, projectedIndexes)
	if len(projectedStatement.DDL.Indexes) != 1 || projectedStatement.DDL.Indexes[0].Name != "uniq_email" {
		t.Fatalf("expected projected alter-index statement to carry uniq_email, got %+v", projectedStatement.DDL.Indexes)
	}

	options := matchingAlterActions(statement, "table_option")
	if len(options) != 1 {
		t.Fatalf("expected 1 table_option alter, got %d", len(options))
	}
	value, ok := alterOptionValue(options[0], "engine")
	if !ok || value != "InnoDB" {
		t.Fatalf("expected engine option InnoDB, got ok=%t value=%q", ok, value)
	}
}

func TestAppliesToStandaloneDDLAction(t *testing.T) {
	dropIndexStatement := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationDropIndex,
			Alter:     []spec.Alter{{Action: "drop_index", Name: "idx_users_email"}},
		},
	}
	if !appliesToStandaloneDDLAction(dropIndexStatement, "drop_index") {
		t.Fatal("expected standalone drop_index path to apply")
	}
	if appliesToStandaloneDDLAction(dropIndexStatement, "drop_column") {
		t.Fatal("expected non-matching action to stay false")
	}
	matches := matchingStandaloneDDLActions(dropIndexStatement, "drop_index")
	if len(matches) != 1 {
		t.Fatalf("expected one standalone drop index match, got %d", len(matches))
	}

	renameIndexStatement := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Alter:     []spec.Alter{{Action: "rename_index", Name: "idx_users_email", Options: map[string]string{"new_name": "idx_users_email_new"}}},
		},
	}
	if !appliesToStandaloneDDLAction(renameIndexStatement, "rename_index") {
		t.Fatal("expected standalone rename_index path to apply")
	}
	matches = matchingStandaloneDDLActions(renameIndexStatement, "rename_index")
	if len(matches) != 1 {
		t.Fatalf("expected one standalone rename index match, got %d", len(matches))
	}

	alterTableStatement := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter:     []spec.Alter{{Action: "drop_index", Name: "idx_users_email"}},
		},
	}
	if appliesToStandaloneDDLAction(alterTableStatement, "drop_index") {
		t.Fatal("expected alter-table operation to stay false for standalone helper")
	}
}

func TestNamingConfigParsesStructuredRequirements(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		params      map[string]any
		want        namingRequirement
		wantErrText string
	}{
		{
			name: "prefix suffix and contains are trimmed",
			params: map[string]any{
				"prefix":   "  tbl_  ",
				"suffix":   "  _audit ",
				"contains": []any{" order ", "user", " "},
			},
			want: namingRequirement{
				prefix:   "tbl_",
				suffix:   "_audit",
				contains: []string{"order", "user"},
			},
		},
		{
			name: "empty values stay inactive",
			params: map[string]any{
				"prefix":   "   ",
				"suffix":   "",
				"contains": []string{" ", ""},
			},
			want: namingRequirement{},
		},
		{
			name: "prefix must be string",
			params: map[string]any{
				"prefix": 1,
			},
			wantErrText: `rule ddl.naming.test param "prefix" must be string`,
		},
		{
			name: "suffix must be string",
			params: map[string]any{
				"suffix": true,
			},
			wantErrText: `rule ddl.naming.test param "suffix" must be string`,
		},
		{
			name: "contains must be string list",
			params: map[string]any{
				"contains": "order",
			},
			wantErrText: `rule ddl.naming.test param "contains" must be a string list`,
		},
		{
			name: "contains rejects non string members",
			params: map[string]any{
				"contains": []any{"order", 2},
			},
			wantErrText: `rule ddl.naming.test param "contains" must contain only strings`,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := policy.RulePolicy{Params: tt.params}

			got, err := namingRequirementParam("ddl.naming.test", cfg)
			if tt.wantErrText != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrText)
				}
				if !strings.Contains(err.Error(), tt.wantErrText) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErrText, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if got.prefix != tt.want.prefix {
				t.Fatalf("expected prefix %q, got %q", tt.want.prefix, got.prefix)
			}
			if got.suffix != tt.want.suffix {
				t.Fatalf("expected suffix %q, got %q", tt.want.suffix, got.suffix)
			}
			if len(got.contains) != len(tt.want.contains) {
				t.Fatalf("expected contains %v, got %v", tt.want.contains, got.contains)
			}
			for i := range tt.want.contains {
				if got.contains[i] != tt.want.contains[i] {
					t.Fatalf("expected contains %v, got %v", tt.want.contains, got.contains)
				}
			}
		})
	}
}
