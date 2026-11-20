package spec

// PGDestructiveReferences binds a RESTRICT dependency observation to the exact target list.
// BlockedPosition is one-based. Zero means the observed set has no outside reference.
type PGDestructiveReferences struct {
	Targets                []PGDestructiveTableTarget
	BlockedPosition        int
	IneligibleOnlyPosition int // One-based position of the first partitioned ONLY target outside the earlier selection.
}

// PGTruncateReferenceTargets returns qualified relations for TRUNCATE observations.
// CASCADE only needs a read when an earlier member could already cover a partitioned ONLY target.
func PGTruncateReferenceTargets(s Statement) []PGDestructiveTableTarget {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationTruncateTable || s.DDL.PGDestructiveTable == nil ||
		s.Metadata == nil || !PG17Observed(s.Metadata.Instance) {
		return nil
	}
	targets := s.DDL.PGDestructiveTable.Targets
	if len(targets) != len(s.Metadata.PGDestructiveTableRelations) {
		return nil
	}
	var priorRecursive, conditionalOnly bool
	for i, target := range targets {
		observed := s.Metadata.PGDestructiveTableRelations[i]
		kind := observed.Relation.Kind
		if !target.Qualified() || target.Table.Name != observed.Relation.Name || target.Table.Schema != observed.Schema || (kind != "r" && kind != "p") {
			return nil
		}
		if kind == "p" && target.Only {
			if !priorRecursive {
				return nil
			}
			conditionalOnly = true
		}
		priorRecursive = priorRecursive || !target.Only
	}
	if s.DDL.PGDestructiveTable.Cascade && !conditionalOnly {
		return nil
	}
	return targets
}

// Matches compares only the exact identities relevant to the catalog observation.
func (f PGDestructiveReferences) Matches(targets []PGDestructiveTableTarget) bool {
	if len(targets) == 0 || len(f.Targets) != len(targets) || f.BlockedPosition < 0 || f.BlockedPosition > len(targets) ||
		f.IneligibleOnlyPosition < 0 || f.IneligibleOnlyPosition > len(targets) {
		return false
	}
	for i, target := range targets {
		got := f.Targets[i]
		if got.Table.Schema != target.Table.Schema || got.Table.Name != target.Table.Name || got.Only != target.Only || got.CatalogQualified != target.CatalogQualified {
			return false
		}
	}
	return true
}

// PGDropReferenceTargets returns a fully bound, qualified DROP RESTRICT list.
// The provider verifies table shape, persistence and external references.
func PGDropReferenceTargets(s Statement) []PGDestructiveTableTarget {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationDropTable || s.DDL.PGDestructiveTable == nil || s.DDL.PGDestructiveTable.Cascade ||
		s.Metadata == nil || !PG17Observed(s.Metadata.Instance) {
		return nil
	}
	declared := s.DDL.PGDestructiveTable
	if len(declared.Targets) != len(s.Metadata.PGDestructiveTableRelations) {
		return nil
	}
	for i, target := range declared.Targets {
		observed := s.Metadata.PGDestructiveTableRelations[i]
		if !target.Qualified() || target.Table.Name != observed.Relation.Name || target.Table.Schema != observed.Schema {
			return nil
		}
		if observed.Relation.Kind != "r" && (observed.Relation.Kind != "" || !declared.IfExists) {
			return nil
		}
	}
	return declared.Targets
}
