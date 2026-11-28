package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGReplicaIdentityIndexes(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		name := spec.PGReplicaIdentityIndexTarget(s)
		partitionRoot := spec.PGPartitionedTableBound(s)
		if name == "" || !spec.PGOrdinaryTableBound(s) && !partitionRoot || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		table := s.DDL.Table
		lookup := spec.ObjectLookupRequest{
			Schema: table.Schema, Name: name, Type: "replica_identity_index",
			Qualifiers: map[string]string{"table": table.Name},
		}
		if partitionRoot {
			lookup.Qualifiers["kind"] = "p"
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, &lookup)
		if err != nil {
			return err
		}
		if object == nil || object.PGReplicaIdentity == nil {
			continue
		}
		facts := *object.PGReplicaIdentity
		if facts.Schema == table.Schema && facts.Table == table.Name && facts.Name == name {
			s.Metadata.PGReplicaIdentity = &facts
		}
	}
	return nil
}
