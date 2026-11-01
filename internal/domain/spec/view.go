package spec

import "strings"

type ViewDeclaration struct {
	OrReplace         bool
	Columns           []string
	Algorithm         string
	AlgorithmExplicit bool
	AlgorithmOffset   int `json:"-"`
	Security          string
	SecurityExplicit  bool
	SecurityOffset    int `json:"-"`
	CheckOption       string
	CheckExplicit     bool
	CheckBare         bool
	CheckOffset       int `json:"-"`
	DefinerKind       string
	DefinerOffset     int    `json:"-"`
	DefinerPrincipal  string `json:"-"`
	Definition        *ViewDefinition
	TargetOffsets     []int `json:"-"`
}

type ViewDefinition struct {
	SQL              string `json:"-"`
	Sources          []Table
	SourceAliases    []string
	SourceOffsets    []int `json:"-"`
	SourcesKnown     bool
	Bindings         []ViewColumnBinding
	References       []ViewColumnBinding `json:"-"`
	ShapeKnown       bool
	UnlabeledComplex bool       `json:"-"`
	NonMergeable     bool       `json:"-"`
	NonUpdatable     bool       `json:"-"`
	HasCTE           bool       `json:"-"`
	Query            *ViewQuery `json:"-"`
	QueryLimit       bool       `json:"-"`
}

type ViewQuery struct {
	Sources       []ViewQuerySource
	Bindings      []ViewColumnBinding
	References    []ViewColumnBinding
	With          []*ViewQuery
	Subqueries    []*ViewQuery
	SetOperands   []*ViewQuery
	ColumnAliases []string
	Unsupported   bool
	Limited       bool
	NonMergeable  bool
	NonUpdatable  bool
	Unlabeled     bool
}

type ViewQuerySource struct {
	Table  *Table
	Alias  string
	Query  *ViewQuery
	Offset int
	Index  int
}

type ViewColumnBinding struct {
	Schema          string
	Table           string
	Column          string
	Output          string
	Offset          int  `json:"-"`
	Star            bool `json:"-"`
	Expression      bool `json:"-"`
	OutputReference bool `json:"-"`
}

type ViewDependencyProjection struct {
	Root       int
	Definition *ViewDefinition
	Sources    []TargetBinding
}

type ViewSnapshot struct {
	Definition       *ViewDefinition
	Algorithm        string
	Security         string
	CheckOption      string
	Updatable        *bool
	DefinerKind      string
	DefinerPrincipal string `json:"-"`
}

type ViewMetadata struct {
	TargetIdentity      Table
	Target              *TableSnapshot
	Targets             []TargetBinding `json:"-"`
	Sources             []TargetBinding
	Dependencies        []TargetBinding
	DependencyRoots     []int
	DependencyChecks    []ViewDependencyProjection `json:"-"`
	DependencyCycle     bool
	DependencyCycleRoot int
	DependencyUnproven  bool
	DependencyLimit     bool
}

type ViewLifecycleAssessment struct {
	TargetInvalid      bool
	SourceInvalid      bool
	SourceRootIndex    int
	SourceBindingIndex int
	SourceOffset       int
	SourceFailures     []ViewSourceFailure
	OutputNames        []string
	OutputNamesKnown   bool
	TargetRequired     []string
	SourceRequired     []string
	Security           string
	SecurityUnknown    bool
	ReviewPrincipal    bool
	DependencyLimit    bool
	AlgorithmWarn      bool
	CheckInvalid       bool
	CheckRequired      []string
	DropTargets        []ViewDropTarget
}

type ViewSourceFailure struct {
	Root         int
	BindingIndex int
	Offset       int
}

type ViewDropFate uint8

const (
	ViewDropKeep ViewDropFate = iota
	ViewDropRemove
	ViewDropUnknown
	ViewDropNotView
)

type ViewDropTarget struct {
	Binding TargetBinding
	Invalid bool
	Fate    ViewDropFate
}

func appendViewFact(facts []string, fact string) []string {
	for _, existing := range facts {
		if existing == fact {
			return facts
		}
	}
	return append(facts, fact)
}

func sameViewIdentity(a, b Table) bool {
	return strings.EqualFold(a.Schema, b.Schema) && strings.EqualFold(a.Name, b.Name)
}

func AssessViewLifecycle(statement Statement) ViewLifecycleAssessment {
	assessment := ViewLifecycleAssessment{SourceRootIndex: -1, SourceBindingIndex: -1, SourceOffset: -1}
	if statement.DDL == nil || statement.DDL.View == nil {
		return assessment
	}
	ddl := statement.DDL
	assessment.ReviewPrincipal = ddl.View.DefinerKind == "named"
	var metadata *ViewMetadata
	if statement.Metadata != nil {
		metadata = statement.Metadata.View
	}
	if metadata == nil {
		assessment.TargetRequired = appendViewFact(assessment.TargetRequired, "table.exists")
		assessment.SourceRequired = appendViewFact(assessment.SourceRequired, "table.exists")
		assessment.SecurityUnknown = true
		return assessment
	}
	if ddl.Operation == DDLOperationDropView {
		return assessDropView(statement, metadata)
	}
	switch {
	case len(metadata.Targets) > 0 && metadata.Targets[0].NotView && ddl.Operation == DDLOperationAlterView:
		assessment.TargetInvalid = true
	case metadata.Target == nil:
		assessment.TargetRequired = appendViewFact(assessment.TargetRequired, "table.exists")
	case ddl.Operation == DDLOperationAlterView && !metadata.Target.Exists:
		assessment.TargetInvalid = true
	case ddl.Operation == DDLOperationCreateView && metadata.Target.Exists && !ddl.View.OrReplace:
		assessment.TargetInvalid = true
	case metadata.Target.Exists && metadata.Target.RelationKind == "":
		assessment.TargetRequired = appendViewFact(assessment.TargetRequired, "relation.kind")
	case ddl.Operation == DDLOperationAlterView && metadata.Target.RelationKind != "view":
		assessment.TargetInvalid = true
	case ddl.Operation == DDLOperationCreateView && metadata.Target.Exists &&
		(!ddl.View.OrReplace || metadata.Target.RelationKind != "view"):
		assessment.TargetInvalid = true
	}
	if ddl.View.SecurityExplicit {
		assessment.Security = ddl.View.Security
	} else if ddl.Operation == DDLOperationCreateView && (statement.Dialect == DialectMySQL || statement.Dialect == DialectTiDB) {
		assessment.Security = "definer"
	} else if metadata.Target != nil && metadata.Target.View != nil {
		assessment.Security = metadata.Target.View.Security
	}
	assessment.SecurityUnknown = assessment.Security == "" &&
		(ddl.Operation != DDLOperationAlterView || !assessment.TargetInvalid || ddl.View.SecurityExplicit)
	definition := ddl.View.Definition
	if statement.Dialect == DialectMySQL {
		assessment.AlgorithmWarn = ddl.View.AlgorithmExplicit &&
			(ddl.View.Algorithm == "temptable" || ddl.View.Algorithm == "merge" && definition != nil && definition.NonMergeable)
		if ddl.View.CheckExplicit {
			effectiveAlgorithm := ddl.View.Algorithm
			if ddl.Operation == DDLOperationAlterView && (!ddl.View.AlgorithmExplicit || effectiveAlgorithm == "undefined") &&
				metadata.Target != nil && metadata.Target.View != nil {
				effectiveAlgorithm = metadata.Target.View.Algorithm
			}
			if effectiveAlgorithm == "temptable" || definition != nil && definition.NonUpdatable {
				assessment.CheckInvalid = true
			} else if definition != nil && definition.ShapeKnown && len(metadata.Sources) == 1 {
				source := metadata.Sources[0].Snapshot
				if source == nil || source.Exists && source.RelationKind == "" {
					assessment.CheckRequired = appendViewFact(assessment.CheckRequired, "view.updatable")
				} else if source.Exists && source.RelationKind == "view" &&
					(source.View == nil || source.View.Updatable == nil) {
					assessment.CheckRequired = appendViewFact(assessment.CheckRequired, "view.updatable")
				} else if source.View != nil && source.View.Updatable != nil && !*source.View.Updatable {
					assessment.CheckInvalid = true
				}
			}
			if ddl.Operation == DDLOperationAlterView && !assessment.TargetInvalid &&
				(!ddl.View.AlgorithmExplicit || ddl.View.Algorithm == "undefined") &&
				effectiveAlgorithm == "" && !assessment.CheckInvalid {
				assessment.CheckRequired = appendViewFact(assessment.CheckRequired, "view.algorithm")
			}
		}
	}
	assessment.DependencyLimit = metadata.DependencyLimit
	if metadata.DependencyLimit && definition != nil && (len(definition.Sources) > 256 || definition.QueryLimit) {
		return assessment
	}
	shapeUnsupported := false
	for _, option := range ddl.UnextractedOptions {
		if option == "view_query_shape" {
			shapeUnsupported = true
			break
		}
	}
	if definition == nil || !definition.SourcesKnown {
		if !shapeUnsupported {
			assessment.SourceRequired = appendViewFact(assessment.SourceRequired, "view.dependencies")
		}
	} else if len(metadata.Sources) != len(definition.Sources) {
		assessment.SourceRequired = appendViewFact(assessment.SourceRequired, "view.dependencies")
	}
	markSourceOffset := func(root, binding, offset int) {
		if !assessment.SourceInvalid {
			assessment.SourceRootIndex = root
			assessment.SourceBindingIndex = binding
			assessment.SourceOffset = offset
		}
		assessment.SourceInvalid = true
		for _, existing := range assessment.SourceFailures {
			if existing.Root == root {
				return
			}
		}
		assessment.SourceFailures = append(assessment.SourceFailures, ViewSourceFailure{Root: root, BindingIndex: binding, Offset: offset})
	}
	markSourceInvalid := func(root, binding int) {
		markSourceOffset(root, binding, -1)
	}
	if definition != nil && definition.HasCTE && statement.Dialect == DialectMySQL &&
		(statement.Metadata == nil || statement.Metadata.Version == nil || statement.Metadata.Version.Major == 0) {
		assessment.SourceRequired = appendViewFact(assessment.SourceRequired, "database.version")
	}
	if metadata.DependencyCycle {
		markSourceInvalid(metadata.DependencyCycleRoot, -1)
	}
	if metadata.DependencyUnproven {
		assessment.SourceRequired = appendViewFact(assessment.SourceRequired, "view.dependencies")
	}
	all := make([]TargetBinding, 0, len(metadata.Sources)+len(metadata.Dependencies))
	all = append(all, metadata.Sources...)
	all = append(all, metadata.Dependencies...)
	for i, source := range all {
		root := i
		if i >= len(metadata.Sources) {
			dependency := i - len(metadata.Sources)
			root = -1
			if dependency < len(metadata.DependencyRoots) {
				root = metadata.DependencyRoots[dependency]
			}
		}
		if sameViewIdentity(source.Table, metadata.TargetIdentity) {
			markSourceInvalid(root, -1)
			continue
		}
		switch {
		case source.Snapshot == nil:
			assessment.SourceRequired = appendViewFact(assessment.SourceRequired, "table.exists")
		case !source.Snapshot.Exists || source.Snapshot.Temporary:
			markSourceInvalid(root, -1)
		case source.Snapshot.RelationKind == "":
			assessment.SourceRequired = appendViewFact(assessment.SourceRequired, "relation.kind")
		case source.Snapshot.RelationKind == "view" && (source.Snapshot.View == nil || source.Snapshot.View.Definition == nil):
			assessment.SourceRequired = appendViewFact(assessment.SourceRequired, "view.definition")
		case source.Snapshot.RelationKind == "view" && !source.Snapshot.View.Definition.SourcesKnown:
			assessment.SourceRequired = appendViewFact(assessment.SourceRequired, "view.dependencies")
		}
	}
	if definition != nil && !definition.ShapeKnown && !shapeUnsupported &&
		(len(metadata.Sources) != 1 || metadata.Sources[0].Snapshot == nil ||
			(metadata.Sources[0].Snapshot.Exists && !metadata.Sources[0].Snapshot.Temporary)) {
		assessment.SourceRequired = appendViewFact(assessment.SourceRequired, "view.columns")
	}
	assessBindings := func(definition *ViewDefinition, sources []TargetBinding) (int, []string) {
		if definition == nil || !definition.ShapeKnown || len(sources) != 1 {
			return -1, nil
		}
		identity := sources[0].Table
		qualifier := identity.Name
		if len(definition.SourceAliases) > 0 && definition.SourceAliases[0] != "" {
			qualifier = definition.SourceAliases[0]
		}
		bindings := make([]ViewColumnBinding, 0, len(definition.Bindings)+len(definition.References))
		bindings = append(bindings, definition.Bindings...)
		bindings = append(bindings, definition.References...)
		invalidQualifier := make([]bool, len(bindings))
		invalid := -1
		var required []string
		for i, binding := range bindings {
			if binding.Expression {
				continue
			}
			if binding.Table != "" && !strings.EqualFold(binding.Table, qualifier) ||
				binding.Schema != "" && !strings.EqualFold(binding.Schema, identity.Schema) {
				invalidQualifier[i] = true
				if invalid < 0 {
					invalid = i
				}
			}
		}
		source := sources[0].Snapshot
		if source != nil && source.Exists && !source.Temporary {
			if source.Columns == nil {
				for i, invalid := range invalidQualifier {
					if !invalid && !bindings[i].Expression {
						required = appendViewFact(required, "view.columns")
						break
					}
				}
			} else {
				for i, binding := range bindings {
					if invalidQualifier[i] {
						continue
					}
					if binding.Expression {
						continue
					}
					if binding.Star {
						if source.ColumnsTailUnknown {
							required = appendViewFact(required, "view.columns")
						}
						continue
					}
					if !source.HasColumn(binding.Column) {
						if source.ColumnsTailUnknown {
							required = appendViewFact(required, "view.columns")
						} else if invalid < 0 {
							invalid = i
						}
					}
				}
			}
		}
		return invalid, required
	}
	if definition != nil && definition.Query != nil && definition.ShapeKnown && !metadata.DependencyLimit {
		queryAssessment := assessViewQuery(definition.Query, metadata.Sources, statement.Dialect)
		for _, failure := range queryAssessment.failures {
			markSourceOffset(failure.Root, -1, failure.Offset)
		}
		for _, fact := range queryAssessment.required {
			assessment.SourceRequired = appendViewFact(assessment.SourceRequired, fact)
		}
		assessment.DependencyLimit = assessment.DependencyLimit || queryAssessment.limited
		outputs, countKnown := queryAssessment.outputs, queryAssessment.countKnown
		if countKnown && len(ddl.View.Columns) > 0 {
			if len(ddl.View.Columns) != len(outputs) {
				markSourceInvalid(-1, -1)
			} else {
				outputs = append([]string(nil), ddl.View.Columns...)
			}
		}
		if countKnown {
			assessment.OutputNamesKnown = true
			seen := make(map[string]struct{}, len(outputs))
			for _, output := range outputs {
				if output == "" {
					assessment.OutputNamesKnown = false
					continue
				}
				key := strings.ToLower(output)
				if _, exists := seen[key]; exists {
					markSourceInvalid(-1, -1)
				}
				seen[key] = struct{}{}
			}
			assessment.OutputNames = outputs
		}
	} else if definition != nil && definition.Query == nil {
		invalid, required := assessBindings(definition, metadata.Sources)
		if invalid >= 0 {
			markSourceInvalid(0, invalid)
		}
		for _, fact := range required {
			assessment.SourceRequired = appendViewFact(assessment.SourceRequired, fact)
		}
	}
	if definition != nil && definition.Query == nil && definition.ShapeKnown {
		countKnown := true
		var outputs []string
		for _, binding := range definition.Bindings {
			if !binding.Star {
				outputs = append(outputs, binding.Output)
				continue
			}
			if len(metadata.Sources) != 1 || metadata.Sources[0].Snapshot == nil ||
				metadata.Sources[0].Snapshot.Columns == nil || metadata.Sources[0].Snapshot.ColumnsTailUnknown {
				countKnown = false
				break
			}
			for _, column := range metadata.Sources[0].Snapshot.Columns {
				if !strings.EqualFold(column.Visibility, "invisible") {
					outputs = append(outputs, column.Name)
				}
			}
		}
		if countKnown && len(ddl.View.Columns) > 0 {
			if len(ddl.View.Columns) != len(outputs) {
				markSourceInvalid(-1, -1)
			} else {
				outputs = append([]string(nil), ddl.View.Columns...)
			}
		}
		if countKnown {
			assessment.OutputNamesKnown = true
			seen := make(map[string]struct{}, len(outputs))
			for _, output := range outputs {
				if output == "" {
					assessment.OutputNamesKnown = false
					continue
				}
				key := strings.ToLower(output)
				if _, exists := seen[key]; exists {
					markSourceInvalid(-1, -1)
				}
				seen[key] = struct{}{}
			}
			assessment.OutputNames = outputs
		}
	}
	for _, dependency := range metadata.DependencyChecks {
		if dependency.Definition != nil && dependency.Definition.Query != nil {
			queryAssessment := assessViewQuery(dependency.Definition.Query, dependency.Sources, statement.Dialect)
			if len(queryAssessment.failures) > 0 {
				markSourceInvalid(dependency.Root, -1)
			}
			for _, fact := range queryAssessment.required {
				assessment.SourceRequired = appendViewFact(assessment.SourceRequired, fact)
			}
			assessment.DependencyLimit = assessment.DependencyLimit || queryAssessment.limited
			continue
		}
		invalid, required := assessBindings(dependency.Definition, dependency.Sources)
		if invalid >= 0 {
			markSourceInvalid(dependency.Root, -1)
		}
		for _, fact := range required {
			assessment.SourceRequired = appendViewFact(assessment.SourceRequired, fact)
		}
	}
	return assessment
}

type viewTargetKind uint8

const (
	viewTargetUnknown viewTargetKind = iota
	viewTargetMissing
	viewTargetView
	viewTargetTable
	viewTargetUnknownKind
	viewTargetNotView
)

func classifyViewTarget(binding TargetBinding) viewTargetKind {
	if binding.Snapshot == nil {
		if binding.NotView {
			return viewTargetNotView
		}
		return viewTargetUnknown
	}
	if !binding.Snapshot.Exists {
		return viewTargetMissing
	}
	switch binding.Snapshot.RelationKind {
	case "view":
		return viewTargetView
	case "":
		return viewTargetUnknownKind
	default:
		return viewTargetTable
	}
}

func assessDropView(statement Statement, metadata *ViewMetadata) ViewLifecycleAssessment {
	assessment := ViewLifecycleAssessment{SourceRootIndex: -1, SourceBindingIndex: -1}
	targets := metadata.Targets
	if len(targets) == 0 && statement.DDL.Table != nil {
		targets = []TargetBinding{{Table: metadata.TargetIdentity, Snapshot: metadata.Target}}
	}
	ifExists := statement.DDL.Options["if_exists"] == "true"
	kinds := make([]viewTargetKind, len(targets))
	duplicates := make([]int, len(targets))
	seen := make(map[Table]int, len(targets))
	knownView, potentialBad, duplicateMySQL := false, false, false
	for i, binding := range targets {
		kinds[i] = classifyViewTarget(binding)
		duplicates[i] = -1
		key := Table{Schema: strings.ToLower(binding.Table.Schema), Name: strings.ToLower(binding.Table.Name)}
		if previous, exists := seen[key]; exists {
			duplicates[i] = previous
			if statement.Dialect == DialectMySQL {
				duplicateMySQL = true
			}
		} else {
			seen[key] = i
		}
		knownView = knownView || kinds[i] == viewTargetView
		potentialBad = potentialBad || kinds[i] != viewTargetView
	}
	unknownVersion := statement.Dialect == DialectMySQL &&
		(statement.Metadata == nil || statement.Metadata.Version == nil || statement.Metadata.Version.Major == 0)
	atomic := statement.Dialect == DialectMySQL && !unknownVersion && statement.Metadata.Version.Major >= 8
	if statement.Dialect == DialectMySQL && unknownVersion && !ifExists && len(targets) > 1 &&
		knownView && potentialBad && !duplicateMySQL {
		assessment.TargetRequired = appendViewFact(assessment.TargetRequired, "database.version")
	}
	tidbStopped, tidbMayStop := false, false
	for i, binding := range targets {
		kind := kinds[i]
		if statement.Dialect == DialectTiDB && duplicates[i] >= 0 {
			switch assessment.DropTargets[duplicates[i]].Fate {
			case ViewDropRemove:
				kind = viewTargetMissing
			case ViewDropNotView:
				kind = viewTargetNotView
			}
		}
		target := ViewDropTarget{Binding: binding, Fate: ViewDropKeep}
		switch {
		case duplicates[i] >= 0 && (statement.Dialect == DialectMySQL ||
			statement.Dialect == DialectTiDB && !ifExists):
			target.Invalid = true
		case kind == viewTargetMissing:
			target.Invalid = !ifExists
		case kind == viewTargetTable:
			target.Invalid = !ifExists || statement.Dialect == DialectTiDB
		case kind == viewTargetNotView && !ifExists:
			target.Invalid = true
		case kind == viewTargetUnknown:
			if !ifExists || statement.Dialect == DialectTiDB {
				assessment.TargetRequired = appendViewFact(assessment.TargetRequired, "table.exists")
			}
		case kind == viewTargetUnknownKind || kind == viewTargetNotView:
			if !ifExists || statement.Dialect == DialectTiDB {
				fact := "relation.kind"
				if kind == viewTargetNotView {
					fact = "table.exists"
				}
				assessment.TargetRequired = appendViewFact(assessment.TargetRequired, fact)
			}
		}
		assessment.TargetInvalid = assessment.TargetInvalid || target.Invalid
		switch statement.Dialect {
		case DialectTiDB:
			switch {
			case tidbStopped:
				target.Fate = ViewDropKeep
			case tidbMayStop:
				if kind == viewTargetView || kind == viewTargetUnknown || kind == viewTargetUnknownKind {
					target.Fate = ViewDropUnknown
				}
				if kind == viewTargetTable {
					tidbStopped = true
					tidbMayStop = false
				}
			default:
				switch kind {
				case viewTargetView:
					target.Fate = ViewDropRemove
				case viewTargetTable:
					tidbStopped = true
				case viewTargetUnknown, viewTargetUnknownKind, viewTargetNotView:
					target.Fate = ViewDropNotView
					tidbMayStop = true
				}
			}
		case DialectMySQL:
			switch {
			case duplicateMySQL:
				target.Fate = ViewDropKeep
			case ifExists:
				switch kind {
				case viewTargetView:
					target.Fate = ViewDropRemove
				case viewTargetUnknown, viewTargetUnknownKind:
					target.Fate = ViewDropNotView
				}
			case !unknownVersion && !atomic:
				switch kind {
				case viewTargetView:
					target.Fate = ViewDropRemove
				case viewTargetUnknown, viewTargetUnknownKind:
					target.Fate = ViewDropNotView
				}
			case potentialBad:
				if atomic && hasKnownInvalidDropTarget(kinds) {
					target.Fate = ViewDropKeep
				} else if kind == viewTargetView || kind == viewTargetUnknown || kind == viewTargetUnknownKind {
					target.Fate = ViewDropUnknown
				}
			case kind == viewTargetView:
				target.Fate = ViewDropRemove
			}
		}
		assessment.DropTargets = append(assessment.DropTargets, target)
	}
	return assessment
}

func hasKnownInvalidDropTarget(kinds []viewTargetKind) bool {
	for _, kind := range kinds {
		if kind == viewTargetMissing || kind == viewTargetTable || kind == viewTargetNotView {
			return true
		}
	}
	return false
}

func CloneViewDefinition(in *ViewDefinition) *ViewDefinition {
	if in == nil {
		return nil
	}
	out := *in
	if in.Sources != nil {
		out.Sources = append([]Table{}, in.Sources...)
	}
	if in.SourceAliases != nil {
		out.SourceAliases = append([]string{}, in.SourceAliases...)
	}
	if in.SourceOffsets != nil {
		out.SourceOffsets = append([]int{}, in.SourceOffsets...)
	}
	if in.Bindings != nil {
		out.Bindings = append([]ViewColumnBinding{}, in.Bindings...)
	}
	if in.References != nil {
		out.References = append([]ViewColumnBinding{}, in.References...)
	}
	out.Query = cloneViewQuery(in.Query, make(map[*ViewQuery]*ViewQuery))
	return &out
}

func cloneViewQuery(in *ViewQuery, copies map[*ViewQuery]*ViewQuery) *ViewQuery {
	if in == nil {
		return nil
	}
	if prior := copies[in]; prior != nil {
		return prior
	}
	out := *in
	copies[in] = &out
	out.Bindings = append([]ViewColumnBinding(nil), in.Bindings...)
	out.References = append([]ViewColumnBinding(nil), in.References...)
	out.ColumnAliases = append([]string(nil), in.ColumnAliases...)
	out.Sources = append([]ViewQuerySource(nil), in.Sources...)
	for i := range out.Sources {
		if in.Sources[i].Table != nil {
			table := *in.Sources[i].Table
			out.Sources[i].Table = &table
		}
		out.Sources[i].Query = cloneViewQuery(in.Sources[i].Query, copies)
	}
	cloneList := func(in []*ViewQuery) []*ViewQuery {
		out := make([]*ViewQuery, len(in))
		for i, query := range in {
			out[i] = cloneViewQuery(query, copies)
		}
		return out
	}
	out.With = cloneList(in.With)
	out.Subqueries = cloneList(in.Subqueries)
	out.SetOperands = cloneList(in.SetOperands)
	return &out
}

func SelectViewDefinition(in *ViewDefinition, dialect Dialect) *ViewDefinition {
	out := CloneViewDefinition(in)
	if out == nil || out.Query == nil {
		return out
	}
	out.Sources = nil
	out.SourceAliases = nil
	out.SourceOffsets = nil
	out.QueryLimit = false
	visited := make(map[*ViewQuery]bool)
	var visit func(*ViewQuery)
	visit = func(query *ViewQuery) {
		if query == nil || visited[query] || len(out.Sources) > 256 {
			return
		}
		visited[query] = true
		if query.Limited {
			out.QueryLimit = true
		}
		if query.Unsupported {
			out.ShapeKnown = false
		}
		if dialect == DialectTiDB {
			for _, body := range query.With {
				visit(body)
			}
		}
		for i := range query.Sources {
			if len(out.Sources) > 256 {
				break
			}
			source := &query.Sources[i]
			if source.Table != nil {
				source.Index = len(out.Sources)
				out.Sources = append(out.Sources, *source.Table)
				out.SourceAliases = append(out.SourceAliases, source.Alias)
				out.SourceOffsets = append(out.SourceOffsets, source.Offset)
			}
			visit(source.Query)
		}
		for _, operand := range query.SetOperands {
			visit(operand)
		}
		for _, subquery := range query.Subqueries {
			visit(subquery)
		}
	}
	visit(out.Query)
	return out
}

func BindViewDefinitionSources(definition *ViewDefinition, sources []TargetBinding) {
	if definition == nil || definition.Query == nil {
		return
	}
	seen := make(map[*ViewQuery]bool)
	var visit func(*ViewQuery)
	visit = func(query *ViewQuery) {
		if query == nil || seen[query] {
			return
		}
		seen[query] = true
		for i := range query.Sources {
			source := &query.Sources[i]
			if source.Table != nil && source.Index >= 0 && source.Index < len(sources) {
				table := sources[source.Index].Table
				source.Table = &table
			}
			visit(source.Query)
		}
		for _, child := range query.With {
			visit(child)
		}
		for _, child := range query.SetOperands {
			visit(child)
		}
		for _, child := range query.Subqueries {
			visit(child)
		}
	}
	visit(definition.Query)
}

func CloneViewSnapshot(in *ViewSnapshot) *ViewSnapshot {
	if in == nil {
		return nil
	}
	out := *in
	out.Definition = CloneViewDefinition(in.Definition)
	if in.Updatable != nil {
		value := *in.Updatable
		out.Updatable = &value
	}
	return &out
}
