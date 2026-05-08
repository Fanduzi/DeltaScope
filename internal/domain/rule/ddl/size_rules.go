// Package ddl defines Tier-1 DDL rules.
// input: create-table statements plus instance facts for charset, row format, and large-prefix behavior
// output: metadata-backed rough row-size and index-key-length findings
// pos: DDL size-estimation rules for audit completion
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type tableRowSizeRule struct {
	required bool
	level    rule.Level
}

type indexKeyLengthRule struct {
	required bool
	level    rule.Level
}

func newTableRowSizeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDTableRowSizeMaxBytesRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	return tableRowSizeRule{required: required, level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func newIndexKeyLengthRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDIndexKeyLengthMaxBytesRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	return indexKeyLengthRule{required: required, level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r tableRowSizeRule) ID() string { return ruleIDTableRowSizeMaxBytesRequire }

func (r indexKeyLengthRule) ID() string { return ruleIDIndexKeyLengthMaxBytesRequire }

func (r tableRowSizeRule) AppliesTo(statement spec.Statement) bool {
	return r.required && appliesToCreateTable(statement)
}

func (r indexKeyLengthRule) AppliesTo(statement spec.Statement) bool {
	return r.required && appliesToCreateTableIndexes(statement)
}

func (r tableRowSizeRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	instance, ok := instanceFacts(statement)
	if !ok {
		return nil, nil
	}

	engine := normalizedTableOption(statement, "engine")
	if engine != "" && !strings.EqualFold(engine, "InnoDB") {
		return nil, nil
	}

	charset := resolvedTableCharset(statement, instance)
	rowFormat := resolvedRowFormat(statement, instance)
	totalRowBytes := 0
	compactRowBytes := 0
	for _, column := range statement.DDL.Columns {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		columnBytes := estimatedColumnBytes(column, charset)
		if columnBytes <= 0 {
			continue
		}
		totalRowBytes += columnBytes
		if rowFormat == "COMPACT" || rowFormat == "REDUNDANT" {
			compactRowBytes += minInt(columnBytes, 768)
		}
	}

	findings := make([]rule.Finding, 0)
	if totalRowBytes > 65535 {
		findings = append(findings, rule.Finding{
			RuleID:     r.ID(),
			Level:      r.level,
			Message:    fmt.Sprintf("estimated row size exceeds 65535 bytes (%d)", totalRowBytes),
			Suggestion: "shrink wide columns, move large payloads out of the row, or review the table design explicitly",
			Metadata: map[string]any{
				"table":      statement.DDL.Table.Name,
				"charset":    charset,
				"row_format": rowFormat,
				"estimated":  totalRowBytes,
				"limit":      65535,
			},
		})
	}

	if rowFormat == "COMPACT" && compactRowBytes > 8126 {
		findings = append(findings, rule.Finding{
			RuleID:     r.ID(),
			Level:      r.level,
			Message:    fmt.Sprintf("estimated compact-row payload exceeds 8126 bytes (%d)", compactRowBytes),
			Suggestion: "use DYNAMIC row_format or shrink wide varchar/char columns before creating the table",
			Metadata: map[string]any{
				"table":      statement.DDL.Table.Name,
				"charset":    charset,
				"row_format": rowFormat,
				"estimated":  compactRowBytes,
				"limit":      8126,
			},
		})
	}
	if rowFormat == "REDUNDANT" && compactRowBytes > 8000 {
		findings = append(findings, rule.Finding{
			RuleID:     r.ID(),
			Level:      r.level,
			Message:    fmt.Sprintf("estimated redundant-row payload exceeds 8000 bytes (%d)", compactRowBytes),
			Suggestion: "switch to DYNAMIC/COMPACT row format or shrink wide varchar/char columns before creating the table",
			Metadata: map[string]any{
				"table":      statement.DDL.Table.Name,
				"charset":    charset,
				"row_format": rowFormat,
				"estimated":  compactRowBytes,
				"limit":      8000,
			},
		})
	}
	return findings, nil
}

func (r indexKeyLengthRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	instance, ok := instanceFacts(statement)
	if !ok {
		return nil, nil
	}

	charset := resolvedTableCharset(statement, instance)
	limit := estimatedIndexKeyLimit(statement, instance)
	findings := make([]rule.Finding, 0)
	for _, index := range statement.DDL.Indexes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		total := 0
		for _, columnName := range index.Columns {
			column, ok := ddlColumnByName(statement, columnName)
			if !ok {
				continue
			}
			total += estimatedColumnBytes(column, charset)
		}
		if total <= limit || total == 0 {
			continue
		}
		findings = append(findings, rule.Finding{
			RuleID:     r.ID(),
			Level:      r.level,
			Message:    fmt.Sprintf("index %q estimated key length exceeds %d bytes (%d)", index.Name, limit, total),
			Suggestion: "reduce indexed column lengths, add explicit prefix lengths, or review large-prefix compatibility explicitly",
			Metadata: map[string]any{
				"table":           statement.DDL.Table.Name,
				"index":           index.Name,
				"columns":         append([]string(nil), index.Columns...),
				"charset":         charset,
				"estimated_bytes": total,
				"limit":           limit,
			},
		})
	}
	return findings, nil
}

func instanceFacts(statement spec.Statement) (*spec.InstanceFacts, bool) {
	if statement.Metadata == nil || statement.Metadata.Instance == nil {
		return nil, false
	}
	return statement.Metadata.Instance, true
}

func resolvedTableCharset(statement spec.Statement, instance *spec.InstanceFacts) string {
	if value := strings.ToLower(strings.TrimSpace(normalizedTableOption(statement, "charset"))); value != "" {
		return value
	}
	if instance != nil {
		if value := strings.ToLower(strings.TrimSpace(instance.DefaultCharset)); value != "" {
			return value
		}
	}
	return "utf8mb4"
}

func resolvedRowFormat(statement spec.Statement, instance *spec.InstanceFacts) string {
	if value := strings.ToUpper(strings.TrimSpace(normalizedTableOption(statement, "row_format"))); value != "" {
		return value
	}
	if instance != nil {
		if value := strings.ToUpper(strings.TrimSpace(instance.InnoDBDefaultRowFormat)); value != "" {
			return value
		}
	}
	return "DYNAMIC"
}

func normalizedTableOption(statement spec.Statement, key string) string {
	if statement.DDL == nil {
		return ""
	}
	return strings.TrimSpace(statement.DDL.Options[key])
}

func ddlColumnByName(statement spec.Statement, name string) (spec.Column, bool) {
	for _, column := range statement.DDL.Columns {
		if strings.EqualFold(column.Name, name) {
			return column, true
		}
	}
	return spec.Column{}, false
}

func estimatedColumnBytes(column spec.Column, tableCharset string) int {
	columnCharset := strings.ToLower(strings.TrimSpace(column.Charset))
	if columnCharset == "" {
		columnCharset = strings.ToLower(strings.TrimSpace(tableCharset))
	}
	bytesPerChar := charsetBytes(columnCharset)
	base := baseType(column)
	length := column.Length

	switch base {
	case "char":
		return maxInt(length, 1) * bytesPerChar
	case "varchar":
		payload := maxInt(length, 1) * bytesPerChar
		if payload <= 255 {
			return payload + 1
		}
		return payload + 2
	case "tinyint":
		return 1
	case "smallint":
		return 2
	case "mediumint":
		return 3
	case "int", "integer":
		return 4
	case "bigint", "double":
		return 8
	case "float":
		return 4
	case "date":
		return 3
	case "timestamp":
		return 4
	case "datetime":
		return 8
	case "time":
		return 3
	case "year":
		return 1
	case "json", "text", "tinytext", "mediumtext", "longtext", "blob", "tinyblob", "mediumblob", "longblob":
		return 20
	default:
		if length > 0 {
			return length * maxInt(bytesPerChar, 1)
		}
		return 8
	}
}

func charsetBytes(charset string) int {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "utf8mb4", "utf16", "utf16le", "utf32", "gb18030":
		return 4
	case "utf8", "ujis", "eucjpms":
		return 3
	case "big5", "gbk", "gb2312", "ucs2", "sjis", "cp932", "euckr":
		return 2
	case "", "binary":
		return 1
	default:
		return 1
	}
}

func estimatedIndexKeyLimit(statement spec.Statement, instance *spec.InstanceFacts) int {
	if statement.Dialect == spec.DialectTiDB {
		return 3072
	}
	if instance == nil {
		return 767
	}
	if versionMajor(instance.Version) >= 8 || instance.InnoDBLargePrefixEnabled {
		return 3072
	}
	return 767
}

func versionMajor(version string) int {
	version = strings.TrimSpace(version)
	if version == "" {
		return 0
	}
	parts := strings.Split(version, ".")
	if len(parts) == 0 {
		return 0
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0
	}
	return major
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
