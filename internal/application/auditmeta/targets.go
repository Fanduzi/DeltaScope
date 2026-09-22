// Package auditmeta prepares metadata-aware audit requests for multiple adapters.
// input: parsed and partially parsed SQL statements plus normalized statement specs from the audit application layer
// output: Mutation Target and DDL TableTargets facts from valid statements used for schema inference before metadata-aware audit execution
// pos: shared SQL target inference helper for metadata-aware adapters
// note: if this file changes, update this header and module README.md.
package auditmeta

import (
	"context"
	"strings"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type schemaTarget struct {
	Schema           string
	Name             string
	RequiresExisting bool
}

func collectTargetTables(ctx context.Context, sqlText string, dialect spec.Dialect) ([]schemaTarget, error) {
	parsed, err := appaudit.Parse(ctx, sqlText, dialect)
	if err != nil && len(parsed.Statements) == 0 {
		return nil, err
	}
	statements, err := appaudit.Extract(ctx, parsed)
	if err != nil {
		return nil, err
	}

	type targetKey struct {
		schema string
		name   string
	}
	targetsByName := make(map[targetKey]schemaTarget)
	order := make([]targetKey, 0)
	for _, statement := range statements {
		for _, target := range statementTargets(statement) {
			if target.Name == "" {
				continue
			}
			key := targetKey{schema: strings.ToLower(target.Schema), name: strings.ToLower(target.Name)}
			existing, ok := targetsByName[key]
			if ok {
				existing.RequiresExisting = existing.RequiresExisting || target.RequiresExisting
				targetsByName[key] = existing
				continue
			}
			targetsByName[key] = target
			order = append(order, key)
		}
	}

	targets := make([]schemaTarget, 0, len(order))
	for _, key := range order {
		targets = append(targets, targetsByName[key])
	}
	return targets, nil
}

func statementTargets(statement spec.Statement) []schemaTarget {
	if statement.DDL != nil {
		switch statement.DDL.Operation {
		case spec.DDLOperationCreateTable, spec.DDLOperationAlterTable, spec.DDLOperationDropTable, spec.DDLOperationTruncateTable:
			// approved table-backed metadata targets
		default:
			return nil
		}
		requiresExisting := statement.DDL.Operation != spec.DDLOperationCreateTable
		tables := statement.DDL.TableTargets()
		if statement.DDL.Operation == spec.DDLOperationAlterTable && len(tables) > 1 {
			// ALTER targets after the first are rename destinations. They are
			// new names, not existing objects: an unqualified destination
			// resolves to the session schema being inferred (no independent
			// signal) and a qualified one points at the future location, so
			// only the altered table participates in session-schema inference.
			tables = tables[:1]
		}
		targets := make([]schemaTarget, 0, len(tables))
		for _, table := range tables {
			name := strings.TrimSpace(table.Name)
			if name == "" {
				continue
			}
			targets = append(targets, schemaTarget{
				Schema:           strings.TrimSpace(table.Schema),
				Name:             name,
				RequiresExisting: requiresExisting,
			})
		}
		return targets
	}
	if statement.DML != nil {
		targets := statement.DML.MutationTargetTables()
		if len(targets) == 0 {
			return nil
		}
		name := strings.TrimSpace(targets[0].Name)
		if name == "" {
			return nil
		}
		return []schemaTarget{{
			Schema:           strings.TrimSpace(targets[0].Schema),
			Name:             name,
			RequiresExisting: true,
		}}
	}
	return nil
}
