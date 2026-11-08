// Package tidb verifies the T22/#101 extracted resource-group declaration
// facts the lifecycle rules consume — authored values preserved verbatim at
// the parser seam, guards faithful, order kept, unrecognized members
// bounded rather than silently dropped.
package tidbparser

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
)

func parseExtractT22(t *testing.T, sql string) spec.Statement {
	t.Helper()
	parser := New()
	result, err := parser.Parse(context.Background(), sql)
	if err != nil {
		t.Fatalf("parse sql: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 ast statement, got %d", len(result.Statements))
	}
	wrapped := WrapStatements(result.Statements, result.Warnings)
	statement, err := wrapped[0].Extractor.Extract(spec.DialectTiDB, wrapped[0].RawSQL)
	if err != nil {
		t.Fatalf("extract statement: %v", err)
	}
	return statement
}

// The frozen first path: CREATE 100 then ALTER 200 must keep each authored
// RU rate distinct at the parser seam — never collapsed, never defaulted.
func TestT22FirstPathRUPerSecFidelity(t *testing.T) {
	create := parseExtractT22(t, "CREATE RESOURCE GROUP golden_rg RU_PER_SEC = 100;")
	alter := parseExtractT22(t, "ALTER RESOURCE GROUP golden_rg RU_PER_SEC = 200;")

	if create.DDL.Operation != spec.DDLOperationCreateResourceGroup ||
		alter.DDL.Operation != spec.DDLOperationAlterResourceGroup {
		t.Fatalf("operations = %q/%q", create.DDL.Operation, alter.DDL.Operation)
	}
	cDecl, aDecl := create.DDL.ResourceGroup, alter.DDL.ResourceGroup
	if cDecl == nil || aDecl == nil {
		t.Fatalf("declarations = %v/%v, want typed resource-group declarations", cDecl, aDecl)
	}
	if cDecl.Name != "golden_rg" || aDecl.Name != "golden_rg" {
		t.Fatalf("names = %q/%q", cDecl.Name, aDecl.Name)
	}
	cRU, cOK := cDecl.DeclaredRUPerSec()
	aRU, aOK := aDecl.DeclaredRUPerSec()
	if !cOK || cRU == nil || *cRU != 100 {
		t.Fatalf("create RU_PER_SEC = %v (ok=%t), want authored 100", cRU, cOK)
	}
	if !aOK || aRU == nil || *aRU != 200 {
		t.Fatalf("alter RU_PER_SEC = %v (ok=%t), want authored 200", aRU, aOK)
	}
}

// Every modeled member keeps its authored shape: UNLIMITED stays a token,
// PRIORITY keeps the name, bare BURSTABLE projects the 8.5 enabled flag,
// QUERY_LIMIT keeps typed members, guards land on the right field.
func TestT22DeclarationMemberFidelity(t *testing.T) {
	stmt := parseExtractT22(t,
		"CREATE RESOURCE GROUP IF NOT EXISTS rg RU_PER_SEC=UNLIMITED PRIORITY=HIGH BURSTABLE QUERY_LIMIT=(EXEC_ELAPSED='30s' PROCESSED_KEYS=1000 ACTION=KILL);")
	decl := stmt.DDL.ResourceGroup
	if decl == nil || !decl.IfNotExists {
		t.Fatalf("declaration = %+v, want IF NOT EXISTS bound", decl)
	}
	ru, ok := decl.DeclaredRUPerSec()
	if !ok || ru != nil {
		t.Fatalf("RU = %v (ok=%t), want declared UNLIMITED token", ru, ok)
	}
	if got := decl.DeclaredPriority(); got != "HIGH" {
		t.Fatalf("priority = %q, want HIGH", got)
	}
	burstable, ok := decl.DeclaredBurstable()
	if !ok || burstable != spec.ResourceGroupBurstableEnabled {
		t.Fatalf("burstable = %q (ok=%t), want bare form = enabled (TiDB 8.5 BoolValue)", burstable, ok)
	}
	limit := decl.DeclaredRunaway()
	if limit == nil || limit.Reset {
		t.Fatalf("query_limit = %+v, want populated clause", limit)
	}
	if len(limit.Members) != 3 || limit.Action() != "kill" {
		t.Fatalf("query_limit members = %+v action=%q, want 3 members + KILL", limit.Members, limit.Action())
	}
}

// Guards stay dialect-faithful: IF EXISTS on ALTER/DROP binds to the
// declaration's own field, never the create arm.
func TestT22GuardFidelity(t *testing.T) {
	alter := parseExtractT22(t, "ALTER RESOURCE GROUP IF EXISTS rg PRIORITY=LOW;")
	drop := parseExtractT22(t, "DROP RESOURCE GROUP IF EXISTS rg;")
	if !alter.DDL.ResourceGroup.IfExists || alter.DDL.ResourceGroup.IfNotExists {
		t.Fatalf("alter guards = %+v", alter.DDL.ResourceGroup)
	}
	if !drop.DDL.ResourceGroup.IfExists || len(drop.DDL.ResourceGroup.Options) != 0 {
		t.Fatalf("drop declaration = %+v", drop.DDL.ResourceGroup)
	}
}

// BACKGROUND is outside the T22 modeled surface: it stays a recognized
// member kind carrying bounded evidence, never a silently dropped option.
func TestT22BackgroundStaysBounded(t *testing.T) {
	stmt := parseExtractT22(t, "CREATE RESOURCE GROUP rg RU_PER_SEC=100 BACKGROUND=(TASK_TYPES='br');")
	decl := stmt.DDL.ResourceGroup
	found := false
	for _, option := range decl.Options {
		if option.Kind == spec.ResourceGroupOptionBackground {
			found = true
			if option.Background == nil || option.Background.Members != 1 || option.Background.Reset {
				t.Fatalf("background = %+v, want one bounded member", option.Background)
			}
		}
	}
	if !found {
		t.Fatalf("options = %+v, want a bounded background member", decl.Options)
	}
}

// The fixed TiDB 8.5 grammar only has bare BURSTABLE and BURSTABLE = bool —
// both are the burst-enabled/disabled boolean forms. The locked parser's
// newer keyword forms (MODERATED/UNLIMITED/OFF) are v9 vocabulary the AST
// accepts but the target does not: they extract as unsupported_form, an
// unmodeled member the coverage boundary reports instead of silently
// treating later-version syntax as 8.5 truth.
func TestT22BurstableExplicitFormsUnsupported(t *testing.T) {
	for _, sql := range []string{
		"CREATE RESOURCE GROUP rg RU_PER_SEC=100 BURSTABLE=MODERATED;",
		"CREATE RESOURCE GROUP rg RU_PER_SEC=100 BURSTABLE=UNLIMITED;",
		"CREATE RESOURCE GROUP rg RU_PER_SEC=100 BURSTABLE=OFF;",
	} {
		decl := parseExtractT22(t, sql).DDL.ResourceGroup
		burstable, ok := decl.DeclaredBurstable()
		if !ok || burstable != spec.ResourceGroupBurstableUnsupportedForm {
			t.Fatalf("burstable = %q (ok=%t) for %q, want unsupported_form", burstable, ok, sql)
		}
		found := false
		for _, kind := range decl.UnmodeledOptionKinds() {
			if kind == spec.ResourceGroupOptionBurstable {
				found = true
			}
		}
		if !found {
			t.Fatalf("unmodeled kinds = %v for %q, want the burstable aspect bounded", decl.UnmodeledOptionKinds(), sql)
		}
	}
}

// The legal v8.5 `BURSTABLE [EqOpt] Boolean` arm rides the supplement
// path: the locked grammar cannot represent it, so a same-width blank +
// typed rebind recovers the authored truth. Every Boolean spelling the
// fixed grammar accepts must extract enabled/disabled — never a parse
// error, never a guessed flag.
func TestT22BurstableBooleanArmExtracts(t *testing.T) {
	for _, tc := range []struct {
		sql  string
		want spec.ResourceGroupBurstable
	}{
		{"CREATE RESOURCE GROUP rg RU_PER_SEC=100 BURSTABLE=TRUE;", spec.ResourceGroupBurstableEnabled},
		{"CREATE RESOURCE GROUP rg RU_PER_SEC=100 BURSTABLE TRUE;", spec.ResourceGroupBurstableEnabled},
		{"CREATE RESOURCE GROUP rg RU_PER_SEC=100 BURSTABLE = TRUE;", spec.ResourceGroupBurstableEnabled},
		{"CREATE RESOURCE GROUP rg RU_PER_SEC=100 BURSTABLE=1;", spec.ResourceGroupBurstableEnabled},
		{"CREATE RESOURCE GROUP rg RU_PER_SEC=100 BURSTABLE=42;", spec.ResourceGroupBurstableEnabled},
		{"CREATE RESOURCE GROUP rg RU_PER_SEC=100 BURSTABLE=FALSE;", spec.ResourceGroupBurstableDisabled},
		{"CREATE RESOURCE GROUP rg RU_PER_SEC=100 BURSTABLE FALSE;", spec.ResourceGroupBurstableDisabled},
		{"CREATE RESOURCE GROUP rg RU_PER_SEC=100 BURSTABLE=0;", spec.ResourceGroupBurstableDisabled},
		{"ALTER RESOURCE GROUP rg BURSTABLE=FALSE;", spec.ResourceGroupBurstableDisabled},
		{"ALTER RESOURCE GROUP rg BURSTABLE=1;", spec.ResourceGroupBurstableEnabled},
	} {
		stmt := parseExtractT22(t, tc.sql)
		burstable, ok := stmt.DDL.ResourceGroup.DeclaredBurstable()
		if !ok || burstable != tc.want {
			t.Fatalf("burstable = %q (ok=%t) for %q, want %q", burstable, ok, tc.sql, tc.want)
		}
		for _, kind := range stmt.DDL.ResourceGroup.UnmodeledOptionKinds() {
			if kind == spec.ResourceGroupOptionBurstable {
				t.Fatalf("burstable marked unmodeled for %q — legal 8.5 arm is modeled", tc.sql)
			}
		}
	}
}

// The supplement never invents syntax: non-Boolean operands keep the
// base grammar's own parse error, and a duplicate BURSTABLE arm stays a
// duplicated-option rejection exactly as v8.5's CheckAppend decrees.
func TestT22BurstableNonBooleanKeepsParseError(t *testing.T) {
	parser := New()
	for _, sql := range []string{
		"CREATE RESOURCE GROUP rg BURSTABLE='yes';",
		"CREATE RESOURCE GROUP rg BURSTABLE=-1;",
		"CREATE RESOURCE GROUP rg BURSTABLE=1.5;",
		"CREATE RESOURCE GROUP rg BURSTABLE=abc;",
		"CREATE RESOURCE GROUP rg BURSTABLE=TRUE BURSTABLE=FALSE;",
		"CREATE RESOURCE GROUP rg BURSTABLE BURSTABLE;",
	} {
		if _, err := parser.Parse(context.Background(), sql); err == nil {
			t.Fatalf("%q parsed — the supplement must not rescue non-Boolean or duplicate arms", sql)
		}
	}
}

// RawSQL and positions stay verbatim: the same-width rewrite is a
// parse-input detail, never visible in the recovered statement text.
func TestT22BurstableBooleanPreservesRawSQL(t *testing.T) {
	sql := "CREATE TABLE t (x int); CREATE RESOURCE GROUP rg RU_PER_SEC=100 BURSTABLE=FALSE; ALTER RESOURCE GROUP rg BURSTABLE=1;"
	parser := New()
	result, err := parser.Parse(context.Background(), sql)
	if err != nil {
		t.Fatalf("parse sql: %v", err)
	}
	if len(result.Statements) != 3 {
		t.Fatalf("statements = %d, want 3", len(result.Statements))
	}
	wrapped := WrapStatements(result.Statements, result.Warnings)
	if got := strings.TrimSpace(wrapped[1].RawSQL); got != "CREATE RESOURCE GROUP rg RU_PER_SEC=100 BURSTABLE=FALSE;" {
		t.Fatalf("raw sql = %q, want verbatim authored text", wrapped[1].RawSQL)
	}
	if got := strings.TrimSpace(wrapped[2].RawSQL); got != "ALTER RESOURCE GROUP rg BURSTABLE=1;" {
		t.Fatalf("raw sql = %q, want verbatim authored text", wrapped[2].RawSQL)
	}
	alter, err := wrapped[2].Extractor.Extract(spec.DialectTiDB, wrapped[2].RawSQL)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if burstable, _ := alter.DDL.ResourceGroup.DeclaredBurstable(); burstable != spec.ResourceGroupBurstableEnabled {
		t.Fatalf("alter burstable = %q, want enabled", burstable)
	}
}

// The MySQL dialect route keeps an explicit vendor boundary — the parsed
// TiDB RU form never becomes a silent audited create (MySQL's own CPU
// resource-group family is a separate surface, T24).
func TestT22MySQLKeepsVendorBoundary(t *testing.T) {
	parser := New()
	for _, sql := range []string{
		"CREATE RESOURCE GROUP rg RU_PER_SEC=100;",
		"ALTER RESOURCE GROUP rg RU_PER_SEC=200;",
	} {
		result, err := parser.Parse(context.Background(), sql)
		if err != nil {
			t.Fatalf("parse sql: %v", err)
		}
		wrapped := WrapStatements(result.Statements, result.Warnings)
		statement, err := wrapped[0].Extractor.Extract(spec.DialectMySQL, wrapped[0].RawSQL)
		if err != nil {
			t.Fatalf("extract statement: %v", err)
		}
		if statement.Unsupported == nil ||
			statement.Unsupported.Reason != spec.UnsupportedVendorBoundaryReason ||
			statement.Unsupported.Metadata["boundary"] != "vendor" {
			t.Fatalf("unsupported = %+v, want vendor boundary marker for %q", statement.Unsupported, sql)
		}
	}
}

// The AST arm behind the UNLIMITED token is verified once so the extractor
// contract does not rely on the grammar's numeric arm alone.
func TestT22UnlimitedASTArm(t *testing.T) {
	parser := New()
	result, err := parser.Parse(context.Background(), "CREATE RESOURCE GROUP rg RU_PER_SEC=UNLIMITED;")
	if err != nil {
		t.Fatalf("parse sql: %v", err)
	}
	create, ok := result.Statements[0].(*ast.CreateResourceGroupStmt)
	if !ok {
		t.Fatalf("statement is %T", result.Statements[0])
	}
	if len(create.ResourceGroupOptionList) != 1 || create.ResourceGroupOptionList[0].Tp != ast.ResourceRURate {
		t.Fatalf("options = %#v, want one RU_PER_SEC option", create.ResourceGroupOptionList)
	}
}
