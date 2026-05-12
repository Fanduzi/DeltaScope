//go:build postgresql

package postgresql

import (
	"context"
	"fmt"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

type replicationLifecycleASTFact struct {
	Name           string
	SQL            string
	TopKind        string
	ObjectName     string
	NameNodeShape  string
	ForAllTables   bool
	IfExists       bool
	Cascade        bool
	DropRemoveType string
	AlterAction    string
	AlterActionRaw int32
	SubKind        string
	SubKindRaw     int32
	HasConninfo    bool
}

type replicationLifecycleBaselineFact struct {
	Name               string
	Kind               spec.Kind
	Classifies         bool
	Unsupported        bool
	UnsupportedFeature string
	UnsupportedReason  string
	DDLOperation       string
	DDLObjectName      string
	DDLObjectType      string
	DDLOptions         map[string]string
}

var pgReplicationLifecycleCensusCases = []struct {
	Name string
	SQL  string
}{
	{Name: "create_publication_for_all_tables", SQL: "CREATE PUBLICATION pub_all FOR ALL TABLES"},
	{Name: "alter_publication_add_table", SQL: "ALTER PUBLICATION pub_all ADD TABLE users"},
	{Name: "alter_publication_drop_table", SQL: "ALTER PUBLICATION pub_all DROP TABLE users"},
	{Name: "drop_publication", SQL: "DROP PUBLICATION pub_all"},
	{Name: "drop_publication_if_exists", SQL: "DROP PUBLICATION IF EXISTS pub_all"},
	{Name: "create_subscription", SQL: "CREATE SUBSCRIPTION sub CONNECTION 'postgres://example' PUBLICATION pub_all"},
	{Name: "alter_subscription_disable", SQL: "ALTER SUBSCRIPTION sub DISABLE"},
	{Name: "alter_subscription_enable", SQL: "ALTER SUBSCRIPTION sub ENABLE"},
	{Name: "drop_subscription", SQL: "DROP SUBSCRIPTION sub"},
	{Name: "drop_subscription_if_exists", SQL: "DROP SUBSCRIPTION IF EXISTS sub"},
}

func TestReplicationLifecycleASTCensus(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== PostgreSQL Replication Lifecycle AST Census ===")
	t.Logf("%-40s | %-30s | %s", "Case", "Node Kind", "AST Facts")
	t.Log(string(make([]byte, 160)))

	for _, tc := range pgReplicationLifecycleCensusCases {
		tree, err := pg_query.Parse(tc.SQL)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.Name, err)
		}
		if len(tree.Stmts) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", tc.Name, len(tree.Stmts))
		}

		node := tree.Stmts[0].Stmt
		fact := inspectReplicationLifecycleAST(t, tc.Name, tc.SQL, node)
		assertReplicationLifecycleASTFacts(t, fact)
	}
}

func inspectReplicationLifecycleAST(t *testing.T, name, sql string, node *pg_query.Node) replicationLifecycleASTFact {
	t.Helper()
	fact := replicationLifecycleASTFact{Name: name, SQL: sql}

	switch n := node.GetNode().(type) {
	case *pg_query.Node_CreatePublicationStmt:
		stmt := n.CreatePublicationStmt
		fact.TopKind = "CreatePublicationStmt"
		fact.ObjectName = stmt.GetPubname()
		fact.NameNodeShape = "string_field"
		fact.ForAllTables = stmt.GetForAllTables()

		t.Logf("%-40s | %-30s | pubname=%q foralltables=%v shape=%s",
			name, fact.TopKind, fact.ObjectName, fact.ForAllTables, fact.NameNodeShape)

	case *pg_query.Node_AlterPublicationStmt:
		stmt := n.AlterPublicationStmt
		fact.TopKind = "AlterPublicationStmt"
		fact.ObjectName = stmt.GetPubname()
		fact.NameNodeShape = "string_field"
		fact.AlterActionRaw = int32(stmt.GetAction())
		fact.AlterAction = stmt.GetAction().String()

		t.Logf("%-40s | %-30s | pubname=%q action=%s action_raw=%d shape=%s",
			name, fact.TopKind, fact.ObjectName, fact.AlterAction,
			fact.AlterActionRaw, fact.NameNodeShape)

	case *pg_query.Node_DropStmt:
		stmt := n.DropStmt
		fact.TopKind = "DropStmt"
		fact.DropRemoveType = stmt.GetRemoveType().String()
		fact.IfExists = stmt.GetMissingOk()
		fact.Cascade = stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE
		for _, obj := range stmt.GetObjects() {
			if s := obj.GetString_(); s != nil {
				fact.ObjectName = s.GetSval()
				fact.NameNodeShape = "String"
				break
			}
		}

		t.Logf("%-40s | %-30s | name=%q remove_type=%s if_exists=%v cascade=%v shape=%s",
			name, fact.TopKind, fact.ObjectName, fact.DropRemoveType,
			fact.IfExists, fact.Cascade, fact.NameNodeShape)

	case *pg_query.Node_CreateSubscriptionStmt:
		stmt := n.CreateSubscriptionStmt
		fact.TopKind = "CreateSubscriptionStmt"
		fact.ObjectName = stmt.GetSubname()
		fact.NameNodeShape = "string_field"
		fact.HasConninfo = stmt.GetConninfo() != ""

		t.Logf("%-40s | %-30s | subname=%q has_conninfo=%v shape=%s",
			name, fact.TopKind, fact.ObjectName, fact.HasConninfo, fact.NameNodeShape)

	case *pg_query.Node_AlterSubscriptionStmt:
		stmt := n.AlterSubscriptionStmt
		fact.TopKind = "AlterSubscriptionStmt"
		fact.ObjectName = stmt.GetSubname()
		fact.NameNodeShape = "string_field"
		fact.SubKind = stmt.GetKind().String()
		fact.SubKindRaw = int32(stmt.GetKind())

		t.Logf("%-40s | %-30s | subname=%q kind=%s kind_raw=%d shape=%s",
			name, fact.TopKind, fact.ObjectName, fact.SubKind,
			fact.SubKindRaw, fact.NameNodeShape)

	case *pg_query.Node_DropSubscriptionStmt:
		stmt := n.DropSubscriptionStmt
		fact.TopKind = "DropSubscriptionStmt"
		fact.ObjectName = stmt.GetSubname()
		fact.NameNodeShape = "string_field"
		fact.IfExists = stmt.GetMissingOk()

		t.Logf("%-40s | %-30s | subname=%q missing_ok=%v shape=%s",
			name, fact.TopKind, fact.ObjectName, fact.IfExists, fact.NameNodeShape)

	default:
		t.Fatalf("%s: unexpected node type %T", name, node.GetNode())
	}

	return fact
}

func assertReplicationLifecycleASTFacts(t *testing.T, fact replicationLifecycleASTFact) {
	t.Helper()

	if fact.TopKind == "" {
		t.Errorf("%s: expected non-empty top kind", fact.Name)
	}
	if fact.ObjectName == "" {
		t.Errorf("%s: expected non-empty object name", fact.Name)
	}
	if fact.NameNodeShape == "" {
		t.Errorf("%s: expected non-empty name shape", fact.Name)
	}
}

func TestReplicationLifecycleDeltaScopeBaseline(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== PostgreSQL Replication Lifecycle DeltaScope Baseline ===")
	t.Logf("%-40s | %-10s | %-5s | %-12s | %s",
		"Case", "Kind", "DDL?", "Unsupported?", "Detail")
	t.Log(string(make([]byte, 160)))

	for _, tc := range pgReplicationLifecycleCensusCases {
		p := New()
		result, parseErr := p.Parse(context.Background(), tc.SQL)
		if parseErr != nil {
			t.Logf("%-40s | %-10s | %-5s | %-12s | parse error: %v",
				tc.Name, "ERROR", "-", "-", parseErr)
			t.Errorf("%s: unexpected parse error: %v", tc.Name, parseErr)
			continue
		}
		if len(result.Statements) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", tc.Name, len(result.Statements))
		}

		es := result.Statements[0]
		classifies := es.Kind == spec.KindDDL

		baseline := replicationLifecycleBaselineFact{
			Name:       tc.Name,
			Kind:       es.Kind,
			Classifies: classifies,
		}

		stmt, extractErr := es.Extractor.Extract(spec.DialectPostgreSQL, es.RawSQL)
		if extractErr != nil {
			t.Logf("%-40s | %-10s | %-5v | %-12s | extract error: %v",
				tc.Name, baseline.Kind, classifies, "-", extractErr)
			t.Errorf("%s: unexpected extract error: %v", tc.Name, extractErr)
			continue
		}

		baseline.Unsupported = stmt.Unsupported != nil
		detail := ""
		if stmt.Unsupported != nil {
			baseline.UnsupportedFeature = stmt.Unsupported.Feature
			baseline.UnsupportedReason = stmt.Unsupported.Reason
			detail = fmt.Sprintf("%s: %s", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
		} else if stmt.DDL != nil {
			baseline.DDLOperation = string(stmt.DDL.Operation)
			baseline.DDLObjectName = stmt.DDL.ObjectName
			baseline.DDLObjectType = stmt.DDL.ObjectType
			if len(stmt.DDL.Options) > 0 {
				baseline.DDLOptions = stmt.DDL.Options
			}
			detail = fmt.Sprintf("op=%s obj=%q type=%q opts=%v",
				stmt.DDL.Operation, stmt.DDL.ObjectName, stmt.DDL.ObjectType, stmt.DDL.Options)
		}

		unsupported := "no"
		if baseline.Unsupported {
			unsupported = "yes"
		}
		classStr := "no"
		if baseline.Classifies {
			classStr = "yes"
		}

		t.Logf("%-40s | %-10s | %-5s | %-12s | %s",
			tc.Name, baseline.Kind, classStr, unsupported, detail)

		assertReplicationLifecycleBaseline(t, baseline)
	}
}

func assertReplicationLifecycleBaseline(t *testing.T, fact replicationLifecycleBaselineFact) {
	t.Helper()

	if !fact.Classifies {
		t.Errorf("%s: expected KindDDL classification", fact.Name)
		return
	}
	if fact.Unsupported {
		t.Errorf("%s: expected normalized, got unsupported %s: %s", fact.Name, fact.UnsupportedFeature, fact.UnsupportedReason)
		return
	}

	switch fact.Name {
	case "create_publication_for_all_tables":
		if fact.DDLOperation != "create_publication" {
			t.Errorf("%s: expected create_publication, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLObjectName != "pub_all" {
			t.Errorf("%s: expected pub_all, got %q", fact.Name, fact.DDLObjectName)
		}
		if fact.DDLOptions["all_tables"] != "true" {
			t.Errorf("%s: expected all_tables=true, got %q", fact.Name, fact.DDLOptions["all_tables"])
		}
	case "alter_publication_add_table":
		if fact.DDLOperation != "alter_publication" {
			t.Errorf("%s: expected alter_publication, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["action"] != "add_table" {
			t.Errorf("%s: expected action=add_table, got %q", fact.Name, fact.DDLOptions["action"])
		}
	case "alter_publication_drop_table":
		if fact.DDLOperation != "alter_publication" {
			t.Errorf("%s: expected alter_publication, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["action"] != "drop_table" {
			t.Errorf("%s: expected action=drop_table, got %q", fact.Name, fact.DDLOptions["action"])
		}
	case "drop_publication", "drop_publication_if_exists":
		if fact.DDLOperation != "drop_publication" {
			t.Errorf("%s: expected drop_publication, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLObjectName != "pub_all" {
			t.Errorf("%s: expected pub_all, got %q", fact.Name, fact.DDLObjectName)
		}
	case "create_subscription":
		if fact.DDLOperation != "create_subscription" {
			t.Errorf("%s: expected create_subscription, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLObjectName != "sub" {
			t.Errorf("%s: expected sub, got %q", fact.Name, fact.DDLObjectName)
		}
		// Connection string value must NOT appear in options.
		if fact.DDLOptions["has_connection"] != "true" {
			t.Errorf("%s: expected has_connection=true, got %q", fact.Name, fact.DDLOptions["has_connection"])
		}
	case "alter_subscription_disable":
		if fact.DDLOperation != "alter_subscription" {
			t.Errorf("%s: expected alter_subscription, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["action"] != "disable" {
			t.Errorf("%s: expected action=disable, got %q", fact.Name, fact.DDLOptions["action"])
		}
	case "alter_subscription_enable":
		if fact.DDLOperation != "alter_subscription" {
			t.Errorf("%s: expected alter_subscription, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["action"] != "enable" {
			t.Errorf("%s: expected action=enable, got %q", fact.Name, fact.DDLOptions["action"])
		}
	case "drop_subscription", "drop_subscription_if_exists":
		if fact.DDLOperation != "drop_subscription" {
			t.Errorf("%s: expected drop_subscription, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLObjectName != "sub" {
			t.Errorf("%s: expected sub, got %q", fact.Name, fact.DDLObjectName)
		}
	}
}
