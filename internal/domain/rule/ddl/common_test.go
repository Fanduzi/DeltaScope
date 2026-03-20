// Package ddl verifies shared DDL helper behavior.
// input: synthetic alter-table statements and rule identifier constants
// output: test coverage for reusable alter helper boundaries
// pos: domain DDL common helper and rule-ID test coverage
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestRegisterAlterRuleIDsStayStable(t *testing.T) {
	got := []string{
		ruleIDAlterModifyColumnCompatibleRequire,
		ruleIDAlterChangeColumnCompatibleRequire,
		ruleIDAlterRenameIndexForbid,
		ruleIDAlterAddIndexUniquePrefixRequire,
		ruleIDAlterAddIndexSecondaryPrefixRequire,
		ruleIDAlterAddIndexFulltextPrefixRequire,
	}
	want := []string{
		"ddl.alter.modify_column.compatible.require",
		"ddl.alter.change_column.compatible.require",
		"ddl.alter.rename_index.forbid",
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

	addIndex := matchingAlterActions(statement, "add_constraint")
	if len(addIndex) != 1 {
		t.Fatalf("expected 1 add_constraint alter, got %d", len(addIndex))
	}
	index, ok := alterIndexDefinition(addIndex[0])
	if !ok || index.Name != "uniq_email" || index.Kind != spec.IndexKindUnique {
		t.Fatalf("expected unique index definition, got ok=%t index=%+v", ok, index)
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
