// Package auditmeta prepares metadata-aware audit requests for multiple adapters.
// input: parsed SQL statements and normalized statement specs from the audit application layer
// output: target-table facts used for schema inference before metadata-aware audit execution
// pos: shared SQL target inference helper for metadata-aware adapters
// note: if this file changes, update this header and module README.md.
package auditmeta

import (
	"strings"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type schemaTarget struct {
	Schema           string
	Name             string
	RequiresExisting bool
}

func collectTargetTables(sqlText string, dialect spec.Dialect) ([]schemaTarget, error) {
	parsed, err := appaudit.Parse(sqlText, dialect)
	if err != nil {
		return nil, err
	}
	statements, err := appaudit.Extract(parsed)
	if err != nil {
		return nil, err
	}

	targetsByName := make(map[string]schemaTarget)
	order := make([]string, 0)
	for _, statement := range statements {
		schema, name, requiresExisting := statementTarget(statement)
		if name == "" {
			continue
		}
		key := strings.ToLower(schema) + "." + strings.ToLower(name)
		existing, ok := targetsByName[key]
		if ok {
			existing.RequiresExisting = existing.RequiresExisting || requiresExisting
			targetsByName[key] = existing
			continue
		}
		targetsByName[key] = schemaTarget{Schema: schema, Name: name, RequiresExisting: requiresExisting}
		order = append(order, key)
	}

	targets := make([]schemaTarget, 0, len(order))
	for _, key := range order {
		targets = append(targets, targetsByName[key])
	}
	return targets, nil
}

func statementTarget(statement spec.Statement) (string, string, bool) {
	if statement.DDL != nil && statement.DDL.Table != nil {
		switch statement.DDL.Operation {
		case spec.DDLOperationCreateTable, spec.DDLOperationAlterTable, spec.DDLOperationDropTable, spec.DDLOperationTruncateTable:
			// approved table-backed metadata targets
		default:
			return "", "", false
		}
		schema := strings.TrimSpace(statement.DDL.Table.Schema)
		name := strings.TrimSpace(statement.DDL.Table.Name)
		if name == "" {
			return "", "", false
		}
		return schema, name, statement.DDL.Operation != spec.DDLOperationCreateTable
	}
	if statement.DML != nil && len(statement.DML.Tables) > 0 {
		schema := strings.TrimSpace(statement.DML.Tables[0].Schema)
		name := strings.TrimSpace(statement.DML.Tables[0].Name)
		if name == "" {
			return "", "", false
		}
		return schema, name, true
	}
	return "", "", false
}
