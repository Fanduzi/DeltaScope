package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGTableNamespaces(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		action, name := spec.PGTableNamespaceTarget(s)
		partitionRename := action == "rename_table" && spec.PGPartitionedTableBound(s)
		if name == "" || !spec.PGOrdinaryTableBound(s) && !partitionRename || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		table := s.DDL.Table
		lookup := spec.ObjectLookupRequest{
			Schema: table.Schema, Name: name, Type: "table_namespace",
			Qualifiers: map[string]string{"table": table.Name, "action": action},
		}
		if partitionRename {
			lookup.Qualifiers["kind"] = "p"
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, &lookup)
		if err != nil {
			return err
		}
		if object == nil || object.PGTableNamespace == nil {
			continue
		}
		facts := *object.PGTableNamespace
		if facts.Schema == table.Schema && facts.Table == table.Name && facts.Destination == name && facts.Action == action {
			s.Metadata.PGTableNamespace = &facts
		}
	}
	return nil
}
