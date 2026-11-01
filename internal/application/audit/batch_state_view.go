package audit

import (
	"context"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type viewStateReadError struct{ cause error }

func (viewStateReadError) Error() string   { return "view metadata read failed" }
func (e viewStateReadError) Unwrap() error { return e.cause }

func (s *batchState) applyDropView(statement spec.Statement) error {
	assessment := spec.AssessViewLifecycle(statement)
	for _, target := range assessment.DropTargets {
		key := s.keyFor(s.schema, target.Binding.Table)
		switch target.Fate {
		case spec.ViewDropRemove:
			s.entries[key] = &batchTableEntry{state: tableAbsent, displaySchema: key.schema, displayTable: key.table}
		case spec.ViewDropUnknown:
			s.entries[key] = &batchTableEntry{state: tableUnknown, notView: target.Binding.NotView, displaySchema: key.schema, displayTable: key.table}
		case spec.ViewDropNotView:
			s.entries[key] = &batchTableEntry{state: tableUnknown, notView: true, displaySchema: key.schema, displayTable: key.table}
		}
	}
	return nil
}

func (s *batchState) projectViewMetadata(ctx context.Context, statement spec.Statement, target *spec.TableSnapshot, targets []spec.TargetBinding, sources []*spec.TableSnapshot) (*spec.ViewMetadata, error) {
	key := s.keyFor(s.schema, *statement.DDL.Table)
	metadata := &spec.ViewMetadata{
		TargetIdentity:      spec.Table{Schema: key.schema, Name: key.table},
		Target:              target,
		TargetSchemaAbsent:  s.schemaStateForTarget(key.schema) == schemaAbsent,
		DependencyCycleRoot: -1,
	}
	for _, target := range targets {
		targetKey := s.keyFor(s.schema, target.Table)
		target.Table = spec.Table{Schema: targetKey.schema, Name: targetKey.table}
		metadata.Targets = append(metadata.Targets, target)
	}
	definition := statement.DDL.View.Definition
	if definition == nil {
		return metadata, nil
	}
	if len(definition.Sources) > 256 || definition.QueryLimit {
		metadata.DependencyLimit = true
		return metadata, nil
	}
	for i, source := range definition.Sources {
		sourceKey := s.keyFor(s.schema, source)
		binding := spec.TargetBinding{Table: spec.Table{Schema: sourceKey.schema, Name: sourceKey.table}}
		if i < len(sources) {
			binding.Snapshot = sources[i]
		}
		metadata.Sources = append(metadata.Sources, binding)
	}
	seen := map[batchTableKey]struct{}{}
	path := map[batchTableKey]bool{key: true}
	edges := 0
	var visit func(spec.TargetBinding, int, int) error
	visit = func(binding spec.TargetBinding, depth, root int) error {
		current := s.keyFor(s.schema, binding.Table)
		if path[current] {
			if !metadata.DependencyCycle {
				metadata.DependencyCycleRoot = root
			}
			metadata.DependencyCycle = true
			return nil
		}
		if _, exists := seen[current]; exists {
			return nil
		}
		if depth > 32 || len(seen) >= 256 {
			metadata.DependencyLimit = true
			return nil
		}
		seen[current] = struct{}{}
		snapshot := binding.Snapshot
		if snapshot == nil || !snapshot.Exists || snapshot.RelationKind != "view" ||
			snapshot.View == nil || snapshot.View.Definition == nil ||
			!snapshot.View.Definition.SourcesKnown {
			return nil
		}
		path[current] = true
		defer delete(path, current)
		selected := spec.SelectViewDefinition(snapshot.View.Definition, s.dialect)
		if selected.QueryLimit || len(selected.Sources) > 256 {
			metadata.DependencyLimit = true
			return nil
		}
		check := spec.ViewDependencyProjection{Root: root, Definition: selected}
		for _, source := range selected.Sources {
			if strings.TrimSpace(source.Schema) == "" {
				metadata.DependencyUnproven = true
				check.Sources = append(check.Sources, spec.TargetBinding{Table: source})
				continue
			}
			if edges >= 1024 {
				metadata.DependencyLimit = true
				break
			}
			edges++
			ns, entry, sourceKey, err := s.bindSource(ctx, source)
			if err != nil {
				return err
			}
			dependency := s.projectBoundSnapshot(sourceKey, ns, entry)
			dependency.Table = spec.Table{Schema: sourceKey.schema, Name: sourceKey.table}
			metadata.Dependencies = append(metadata.Dependencies, dependency)
			metadata.DependencyRoots = append(metadata.DependencyRoots, root)
			check.Sources = append(check.Sources, dependency)
			if err := visit(dependency, depth+1, root); err != nil {
				return err
			}
			if metadata.DependencyLimit {
				break
			}
		}
		metadata.DependencyChecks = append(metadata.DependencyChecks, check)
		return nil
	}
	for i, source := range metadata.Sources {
		if metadata.DependencyLimit {
			break
		}
		if err := visit(source, 1, i); err != nil {
			return nil, err
		}
	}
	return metadata, nil
}

func (s *batchState) applyView(statement spec.Statement) error {
	ddl := statement.DDL
	if ddl == nil || ddl.View == nil || ddl.Table == nil || statement.Metadata == nil || statement.Metadata.View == nil {
		return nil
	}
	assessment := spec.AssessViewLifecycle(statement)
	if assessment.TargetInvalid || assessment.SourceInvalid || assessment.CheckInvalid {
		return nil
	}
	key := s.keyFor(s.schema, *ddl.Table)
	if len(assessment.TargetRequired) != 0 || len(assessment.SourceRequired) != 0 ||
		len(assessment.CheckRequired) != 0 || assessment.DependencyLimit || len(ddl.UnextractedOptions) != 0 {
		prior := statement.Metadata.View.Target
		if ddl.Operation == spec.DDLOperationAlterView && prior != nil && prior.Exists && prior.RelationKind == "view" {
			shape := cloneTableSnapshot(prior)
			shape.Columns = nil
			if shape.View != nil {
				shape.View.Definition = nil
				shape.View.Updatable = nil
				if ddl.View.AlgorithmExplicit && ddl.View.Algorithm != "undefined" {
					shape.View.Algorithm = ""
				}
				if ddl.View.DefinerKind != "" {
					shape.View.DefinerKind = ""
					shape.View.DefinerPrincipal = ""
				}
				if ddl.View.SecurityExplicit && shape.View.Security != ddl.View.Security {
					shape.View.Security = ""
				}
				check := "none"
				if ddl.View.CheckExplicit {
					check = ddl.View.CheckOption
				}
				if shape.View.CheckOption != check {
					shape.View.CheckOption = ""
				}
			}
			s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: prior.Schema, displayTable: ddl.Table.Name}
		} else {
			s.invalidateKey(key)
		}
		return nil
	}
	var shape *spec.TableSnapshot
	if ddl.Operation == spec.DDLOperationAlterView {
		shape = cloneTableSnapshot(statement.Metadata.View.Target)
	} else {
		shape = &spec.TableSnapshot{
			Schema: key.schema, Table: &spec.Table{Schema: key.schema, Name: key.table},
			Exists: true, RelationKind: "view",
		}
	}
	if shape.View == nil {
		shape.View = &spec.ViewSnapshot{}
	}
	definition := spec.FreezeViewDefinition(ddl.View.Definition, statement.Metadata.View.Sources, statement.Dialect)
	for i := range definition.Sources {
		definition.Sources[i] = statement.Metadata.View.Sources[i].Table
	}
	spec.BindViewDefinitionSources(definition, statement.Metadata.View.Sources)
	shape.View.Definition = definition
	shape.View.Security = assessment.Security
	if ddl.Operation == spec.DDLOperationCreateView {
		shape.View.Algorithm = "undefined"
		shape.View.DefinerKind = "current_user"
		shape.View.DefinerPrincipal = ""
	}
	if statement.Dialect == spec.DialectMySQL {
		shape.View.Algorithm = assessment.AlgorithmFinal
	} else if ddl.View.AlgorithmExplicit && (ddl.Operation == spec.DDLOperationCreateView || ddl.View.Algorithm != "undefined") {
		shape.View.Algorithm = ddl.View.Algorithm
	}
	if ddl.View.DefinerKind != "" {
		shape.View.DefinerKind = ddl.View.DefinerKind
		shape.View.DefinerPrincipal = ddl.View.DefinerPrincipal
	}
	shape.View.CheckOption = "none"
	if statement.Dialect == spec.DialectTiDB {
		shape.View.CheckOption = "cascaded"
	}
	if ddl.View.CheckExplicit {
		shape.View.CheckOption = ddl.View.CheckOption
	}
	shape.View.Updatable = nil
	if statement.Dialect == spec.DialectMySQL && definition.ShapeKnown {
		updatable := !definition.NonUpdatable
		proven := true
		for _, source := range statement.Metadata.View.Sources {
			snapshot := source.Snapshot
			if snapshot == nil || !snapshot.Exists || snapshot.RelationKind != "view" {
				continue
			}
			if snapshot.View == nil || snapshot.View.Updatable == nil {
				proven = false
				continue
			}
			updatable = updatable && *snapshot.View.Updatable
		}
		if proven {
			shape.View.Updatable = &updatable
		}
	}
	shape.Columns = nil
	if definition.ShapeKnown {
		if definition.Query != nil {
			definition.Bindings = append([]spec.ViewColumnBinding(nil), definition.Query.Bindings...)
		} else {
			var source *spec.TableSnapshot
			if len(statement.Metadata.View.Sources) == 1 {
				source = statement.Metadata.View.Sources[0].Snapshot
			}
			var frozen []spec.ViewColumnBinding
			for _, binding := range definition.Bindings {
				if !binding.Star {
					frozen = append(frozen, binding)
					continue
				}
				if source == nil {
					continue
				}
				for _, column := range source.Columns {
					if strings.EqualFold(column.Visibility, "invisible") {
						continue
					}
					frozen = append(frozen, spec.ViewColumnBinding{
						Schema: binding.Schema, Table: binding.Table, Column: column.Name,
						Output: column.Name, Offset: binding.Offset,
					})
				}
			}
			definition.Bindings = frozen
		}
		if assessment.OutputNamesKnown {
			shape.Columns = make([]spec.Column, 0, len(assessment.OutputNames))
			for i, name := range assessment.OutputNames {
				shape.Columns = append(shape.Columns, spec.Column{Name: name})
				if i < len(definition.Bindings) {
					definition.Bindings[i].Output = name
				}
			}
		}
	}
	s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: key.schema, displayTable: key.table}
	return nil
}
