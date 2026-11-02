package spec

import (
	"strconv"
	"strings"
	"unicode"
)

// TTLDefinitionOutcome is the bounded result for one TTL definition after
// TiDB's checkTTLInfoValid prerequisites are evaluated.
type TTLDefinitionOutcome int

const (
	TTLDefinitionRefused TTLDefinitionOutcome = iota
	TTLDefinitionApplied
	TTLDefinitionUnproven
)

// TTLCreateOutcome evaluates a CREATE-side TTL declaration against the facts
// the statement/model can prove. LIKE uses the bound source snapshot; CTAS
// keeps select-produced columns unvouched. A nil fact collection leaves only
// the facts that need it unproven. effectiveSchema is the destination's
// resolved schema; it binds unqualified foreign-key parents for the
// self-reference check TiDB runs after TTLInfo lands on the TableInfo.
func TTLCreateOutcome(dialect Dialect, ddl *DDL, effectiveSchema string, facts *InstanceFacts, likeSource *TableSnapshot) TTLDefinitionOutcome {
	if dialect != DialectTiDB || ddl == nil {
		return TTLDefinitionUnproven
	}
	if ddl.TemporaryScope != "" {
		return TTLDefinitionRefused
	}
	declaration, ok := ttlDefinitionDeclaration(ddl.TTL)
	if !ok {
		return TTLDefinitionRefused
	}
	if ttlSelfForeignKey(ddl, effectiveSchema) {
		return TTLDefinitionRefused
	}
	shape := &TableSnapshot{Exists: true, ReferencedBy: []Table{}, TTLKnown: true}
	switch {
	case ddl.HasReferTable:
		if likeSource == nil {
			return TTLDefinitionUnproven
		}
		if likeSource.Temporary || likeSource.GlobalTemporary {
			return TTLDefinitionRefused
		}
		shape.Columns = likeSource.Columns
		shape.ColumnsTailUnknown = likeSource.ColumnsTailUnknown
		shape.PrimaryKey = likeSource.PrimaryKey
		shape.PrimaryKeyUnknown = likeSource.PrimaryKeyUnknown
		shape.PrimaryKeyIsHandle = likeSource.PrimaryKeyIsHandle
		shape.PrimaryKeyHandleUnknown = likeSource.PrimaryKeyHandleUnknown
		shape.PrimaryKeyClustered = likeSource.PrimaryKeyClustered
		shape.PrimaryKeyClusteredUnknown = likeSource.PrimaryKeyClusteredUnknown
	case ddl.HasSelect:
		shape.Columns = ddl.Columns
		shape.ColumnsTailUnknown = true
		shape.PrimaryKeyUnknown = len(ddl.Columns) > 0
	default:
		shape.Columns = ddl.Columns
		shape.PrimaryKey = ddl.PrimaryKey
		if ddl.PrimaryKey != nil {
			var mode *string
			var alterPK *bool
			if facts != nil {
				if facts.TiDBEnableClusteredIndex != "" {
					value := facts.TiDBEnableClusteredIndex
					mode = &value
				}
				if facts.TiDBAlterPrimaryKeyKnown {
					value := facts.TiDBAlterPrimaryKey
					alterPK = &value
				}
			}
			if handle := PrimaryKeyHandle(dialect, ddl.PrimaryKey, ddl.Columns, mode, alterPK); handle != nil {
				shape.PrimaryKeyIsHandle = *handle
			} else {
				shape.PrimaryKeyHandleUnknown = true
			}
			if clustered := PrimaryKeyClustered(dialect, ddl.PrimaryKey, ddl.Columns, mode, alterPK); clustered != nil {
				shape.PrimaryKeyClustered = *clustered
			} else {
				shape.PrimaryKeyClusteredUnknown = true
			}
		}
	}
	return TTLDefinitionLandingOutcome(shape, declaration)
}

// TTLForeignKeyParentOutcome applies TiDB's checkTableForeignKey parent
// premise: a proven TTL parent refuses a new inbound edge, while an
// unobserved TTL fact leaves the edge's landing unproven.
func TTLForeignKeyParentOutcome(dialect Dialect, parent *TableSnapshot) TTLDefinitionOutcome {
	if dialect != DialectTiDB {
		return TTLDefinitionApplied
	}
	if parent == nil || !parent.TTLKnown {
		return TTLDefinitionUnproven
	}
	if parent.TTL != nil {
		return TTLDefinitionRefused
	}
	return TTLDefinitionApplied
}

// TTLDefinitionLandingOutcome applies TiDB's checkTTLInfoValid order:
// temporary table, interval expression, common-handle primary key, inbound
// foreign keys, then the time column. Unknown prerequisites withhold the
// result; proven violations refuse it.
func TTLDefinitionLandingOutcome(shape *TableSnapshot, declaration TTLDeclaration) TTLDefinitionOutcome {
	if shape == nil {
		return TTLDefinitionUnproven
	}
	if shape.Temporary || shape.GlobalTemporary {
		return TTLDefinitionRefused
	}
	if outcome := ttlIntervalOutcome(declaration); outcome != TTLDefinitionApplied {
		return outcome
	}
	if outcome := ttlPrimaryKeyOutcome(shape); outcome != TTLDefinitionApplied {
		return outcome
	}
	if len(shape.ReferencedBy) > 0 {
		return TTLDefinitionRefused
	}
	// nil means the reverse-edge set was never read; only a non-nil
	// empty/non-empty slice is the catalog's proven answer.
	if shape.ReferencedByUnknown || shape.ReferencedBy == nil {
		return TTLDefinitionUnproven
	}
	return ttlTimeColumnOutcome(shape, declaration.TimeColumn)
}

func ttlDefinitionDeclaration(declarations []TTLDeclaration) (TTLDeclaration, bool) {
	var declaration TTLDeclaration
	count := 0
	for _, member := range declarations {
		if member.Member == "ttl" {
			declaration = member
			count++
		}
	}
	return declaration, count == 1
}

func ttlSelfForeignKey(ddl *DDL, effectiveSchema string) bool {
	if ddl == nil || ddl.Table == nil {
		return false
	}
	for _, member := range ddl.Constraints {
		if member.Type != "foreign_key" || !strings.EqualFold(member.ReferencedTable, ddl.Table.Name) {
			continue
		}
		schema := strings.TrimSpace(member.ReferencedSchema)
		if schema == "" {
			schema = effectiveSchema
		}
		if strings.EqualFold(schema, effectiveSchema) {
			return true
		}
	}
	return false
}

func ttlPrimaryKeyOutcome(shape *TableSnapshot) TTLDefinitionOutcome {
	if shape == nil {
		return TTLDefinitionUnproven
	}
	if shape.PrimaryKeyClusteredUnknown || shape.PrimaryKeyHandleUnknown {
		return TTLDefinitionUnproven
	}
	if !shape.PrimaryKeyClustered || shape.PrimaryKeyIsHandle {
		return TTLDefinitionApplied
	}
	if shape.PrimaryKey == nil || shape.PrimaryKeyUnknown {
		return TTLDefinitionUnproven
	}
	for _, member := range shape.PrimaryKey.Columns {
		column := shape.FindColumn(member)
		if column == nil {
			return TTLDefinitionUnproven
		}
		switch ttlBaseColumnType(column.Type) {
		case "float", "double", "real":
			return TTLDefinitionRefused
		case "":
			return TTLDefinitionUnproven
		}
	}
	return TTLDefinitionApplied
}

func ttlBaseColumnType(declared string) string {
	base := strings.ToLower(strings.TrimSpace(declared))
	if idx := strings.IndexByte(base, '('); idx >= 0 {
		base = strings.TrimSpace(base[:idx])
	}
	if idx := strings.IndexByte(base, ' '); idx >= 0 {
		base = strings.TrimSpace(base[:idx])
	}
	return base
}

func ttlIntervalOutcome(declaration TTLDeclaration) TTLDefinitionOutcome {
	expr := strings.TrimSpace(declaration.IntervalExpr)
	unit := strings.ToLower(strings.TrimSpace(declaration.IntervalUnit))
	if expr == "" || unit == "" {
		return TTLDefinitionUnproven
	}
	if quote := strings.IndexByte(expr, '\''); quote >= 0 {
		value, ok := ttlQuotedLiteral(expr[quote:])
		if !ok {
			return TTLDefinitionUnproven
		}
		if ttlIntervalLiteralValid(unit, value) {
			return TTLDefinitionApplied
		}
		return TTLDefinitionRefused
	}
	if ttlIntervalNumericLiteral(expr) && ttlIntervalSingleUnit(unit) {
		return TTLDefinitionApplied
	}
	return TTLDefinitionUnproven
}

func ttlQuotedLiteral(expr string) (string, bool) {
	if len(expr) < 2 || expr[0] != '\'' || expr[len(expr)-1] != '\'' {
		return "", false
	}
	var value strings.Builder
	for i := 1; i < len(expr)-1; i++ {
		if expr[i] == '\'' && i+1 < len(expr)-1 && expr[i+1] == '\'' {
			value.WriteByte('\'')
			i++
			continue
		}
		if expr[i] == '\\' && i+1 < len(expr)-1 {
			i++
		}
		value.WriteByte(expr[i])
	}
	return value.String(), true
}

func ttlIntervalLiteralValid(unit, value string) bool {
	if ttlIntervalSingleUnit(unit) {
		return ttlIntervalNumericLiteral(value)
	}
	parts, ok := ttlIntervalUnitParts(unit)
	if !ok {
		return false
	}
	if parts[0] == ' ' {
		fields := strings.Fields(value)
		if len(fields) != 2 {
			return false
		}
		return ttlIntervalDigits(fields[0]) && ttlIntervalParts(fields[1], parts[1:])
	}
	return ttlIntervalParts(value, parts)
}

func ttlIntervalParts(value string, parts []byte) bool {
	for _, separator := range parts {
		if separator == 0 {
			return ttlIntervalDigits(value)
		}
		idx := strings.IndexByte(value, separator)
		if idx <= 0 || !ttlIntervalDigits(value[:idx]) {
			return false
		}
		value = value[idx+1:]
	}
	return false
}

func ttlIntervalUnitParts(unit string) ([]byte, bool) {
	switch unit {
	case "second_microsecond":
		return []byte{'.', 0}, true
	case "minute_microsecond":
		return []byte{':', '.', 0}, true
	case "minute_second":
		return []byte{':', 0}, true
	case "hour_microsecond":
		return []byte{':', ':', '.', 0}, true
	case "hour_second":
		return []byte{':', ':', 0}, true
	case "hour_minute":
		return []byte{':', 0}, true
	case "day_microsecond":
		return []byte{' ', ':', ':', '.', 0}, true
	case "day_second":
		return []byte{' ', ':', ':', 0}, true
	case "day_minute":
		return []byte{' ', ':', 0}, true
	case "day_hour":
		return []byte{' ', 0}, true
	case "year_month":
		return []byte{'-', 0}, true
	default:
		return nil, false
	}
}

func ttlIntervalSingleUnit(unit string) bool {
	switch unit {
	case "microsecond", "second", "minute", "hour", "day", "week", "month", "quarter", "year":
		return true
	default:
		return false
	}
}

func ttlIntervalNumericLiteral(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	_, err := strconv.ParseFloat(value, 64)
	return err == nil
}

func ttlIntervalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, c := range value {
		if !unicode.IsDigit(c) {
			return false
		}
	}
	return true
}

func ttlTimeColumnOutcome(shape *TableSnapshot, timeColumn string) TTLDefinitionOutcome {
	if shape == nil || shape.Columns == nil {
		return TTLDefinitionUnproven
	}
	column := shape.FindColumn(timeColumn)
	if column == nil {
		if shape.ColumnsTailUnknown {
			return TTLDefinitionUnproven
		}
		return TTLDefinitionRefused
	}
	if !TTLTimeColumnType(column.Type) {
		return TTLDefinitionRefused
	}
	if shape.ColumnsTailUnknown {
		return TTLDefinitionUnproven
	}
	return TTLDefinitionApplied
}

// TTLTimeColumnType reports whether the normalized base type is one TiDB
// accepts as a TTL time column.
func TTLTimeColumnType(declared string) bool {
	switch strings.ToLower(strings.TrimSpace(declared)) {
	case "date", "datetime", "timestamp":
		return true
	default:
		return false
	}
}
