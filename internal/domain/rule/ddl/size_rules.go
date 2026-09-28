// Package ddl defines Tier-1 DDL rules.
// input: create-table statements plus instance facts for charset, row format, large-prefix behavior, and the canonical version identity
// output: metadata-backed rough row-size and index-key-length findings plus bounded version/instance-fact evidence gaps
// pos: DDL size-estimation rules for audit completion, including the version-dependent key-length bound contract
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"
	"sort"
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

// Version-dependent key-length fact contract (T04-B/#83): the InnoDB index
// key bound depends on product, version, row format, the InnoDB page size,
// and — only on MySQL 5.7 DYNAMIC/COMPRESSED — the innodb_large_prefix
// instance variable.
//
//   - MySQL COMPACT/REDUNDANT row formats cap at 767 bytes in every validated
//     series, so that bound resolves without any version fact.
//   - MySQL DYNAMIC/COMPRESSED bounds scale with innodb_page_size: 4KB → 768,
//     8KB → 1536, 16KB and larger applicable pages → 3072.
//   - MySQL 5.7 DYNAMIC/COMPRESSED additionally needs innodb_large_prefix:
//     OFF caps every row format at 767 bytes; ON applies the page-size bound;
//     an unknown value can never be guessed from the version.
//   - TiDB 8.5's bound is the tidb-server max-index-length config (default
//     3072, configurable up to 12288). When the actual value is unknown the
//     candidates are the documented endpoints {3072, 12288}.
//   - Missing instance facts produce a candidate bound family: a statement is
//     complete only when the outcome is identical under every candidate —
//     safe under the smallest bound or violating even the largest. Anything
//     in between reports a missing_instance_fact gap naming the absent facts.
//   - A missing MySQL version leaves candidate bound {3072}; only a total
//     exceeding it (>3072) is a proven violation and may still be reported
//     alongside the version evidence gap.
const (
	gapReasonMissingTargetVersion        = "missing_target_version"
	gapReasonTargetVersionOutOfRange     = "target_version_out_of_validated_range"
	gapReasonMissingInstanceFact         = "missing_instance_fact"
	factTargetVersion                    = "target.version"
	factTargetVersionValidatedRange      = "target.version.validated_range"
	factInstanceInnoDBDefaultRowFormat   = "instance.innodb_default_row_format"
	factInstanceInnoDBLargePrefixEnabled = "instance.innodb_large_prefix_enabled"
	factInstanceInnoDBPageSize           = "instance.innodb_page_size"
	factInstanceTiDBMaxIndexLength       = "instance.tidb_max_index_length"
)

const (
	indexKeyBoundMaxBytes     = 3072
	tidbMaxIndexLengthCeiling = 12288
)

// indexKeyLimitResolution is the rule's fact-resolution outcome for one
// statement: a single resolved bound, an unresolved candidate bound family
// (ascending) plus the missing facts behind it, or a suppressed check plus
// the evidence-gap identity of the missing version fact. gapReason is an
// unconditional (version-level) gap; missingFacts only downgrade the verdict
// when an index total actually falls between the strictest and loosest bound.
type indexKeyLimitResolution struct {
	bounds       []int
	suppressed   bool
	gapReason    string
	gapFacts     []string
	missingFacts []string
}

// mysqlPageBoundIndexKeyBytes maps an innodb_page_size value to the DYNAMIC/
// COMPRESSED index-prefix bound MySQL documents for it: 4KB → 768 bytes,
// 8KB → 1536 bytes, 16KB and larger applicable pages → 3072 bytes.
func mysqlPageBoundIndexKeyBytes(pageSize int) int {
	switch {
	case pageSize > 0 && pageSize <= 4<<10:
		return 768
	case pageSize <= 8<<10:
		return 1536
	default:
		return indexKeyBoundMaxBytes
	}
}

// resolveIndexKeyLimit derives the key-length candidate bounds from the
// statement's version identity and instance facts — it never fabricates a
// fact. Version-level gaps (missing/out-of-range) are emitted unconditionally;
// instance-fact gaps are emitted only when at least one index lands between
// the smallest and largest candidate bounds, i.e. the missing fact would
// actually change the verdict.
func resolveIndexKeyLimit(statement spec.Statement, instance *spec.InstanceFacts) indexKeyLimitResolution {
	version := statementVersionIdentity(statement)
	if statement.Dialect == spec.DialectTiDB {
		if version == nil || version.Version == "" {
			return indexKeyLimitResolution{
				suppressed: true,
				gapReason:  gapReasonMissingTargetVersion,
				gapFacts:   []string{factTargetVersion},
			}
		}
		if !version.ValidatedRange {
			return indexKeyLimitResolution{
				suppressed: true,
				gapReason:  gapReasonTargetVersionOutOfRange,
				gapFacts:   []string{factTargetVersionValidatedRange},
			}
		}
		if instance != nil && instance.TiDBMaxIndexLengthKnown {
			return indexKeyLimitResolution{bounds: []int{instance.TiDBMaxIndexLengthBytes}}
		}
		return indexKeyLimitResolution{
			bounds:       []int{indexKeyBoundMaxBytes, tidbMaxIndexLengthCeiling},
			missingFacts: []string{factInstanceTiDBMaxIndexLength},
		}
	}

	rowFormat, rowFormatKnown := mysqlIndexRowFormat(statement, instance)
	if rowFormatKnown && (rowFormat == "COMPACT" || rowFormat == "REDUNDANT") {
		return indexKeyLimitResolution{bounds: []int{767}}
	}
	if version == nil || version.Version == "" {
		return indexKeyLimitResolution{
			bounds:    []int{indexKeyBoundMaxBytes},
			gapReason: gapReasonMissingTargetVersion,
			gapFacts:  []string{factTargetVersion},
		}
	}
	if !version.ValidatedRange {
		return indexKeyLimitResolution{
			suppressed: true,
			gapReason:  gapReasonTargetVersionOutOfRange,
			gapFacts:   []string{factTargetVersionValidatedRange},
		}
	}
	isMySQL57 := version.Major == 5 && version.Minor == 7
	largePrefixKnown := instance != nil && instance.InnoDBLargePrefixKnown
	// On MySQL 5.7 an OFF innodb_large_prefix caps every row format at 767
	// bytes, so the bound resolves without touching the row-format fact.
	if isMySQL57 && largePrefixKnown && !instance.InnoDBLargePrefixEnabled {
		return indexKeyLimitResolution{bounds: []int{767}}
	}

	var missing []string
	// The DYNAMIC/COMPRESSED candidate family starts with the page-size bound.
	dynamicBounds := []int{}
	pageSizeKnown := instance != nil && instance.InnoDBPageSizeKnown
	if pageSizeKnown {
		dynamicBounds = append(dynamicBounds, mysqlPageBoundIndexKeyBytes(instance.InnoDBPageSizeBytes))
	} else {
		missing = append(missing, factInstanceInnoDBPageSize)
		dynamicBounds = append(dynamicBounds, mysqlPageBoundIndexKeyBytes(4<<10), mysqlPageBoundIndexKeyBytes(8<<10), indexKeyBoundMaxBytes)
	}
	if isMySQL57 && !largePrefixKnown {
		missing = append(missing, factInstanceInnoDBLargePrefixEnabled)
		// LP unknown → OFF (767) remains a candidate alongside the page bounds.
		dynamicBounds = append([]int{767}, dynamicBounds...)
	}
	if rowFormatKnown {
		switch rowFormat {
		case "DYNAMIC", "COMPRESSED":
		default:
			// A known row format outside the documented InnoDB set can prove
			// nothing — keep it as a missing fact rather than guessing.
			missing = append(missing, factInstanceInnoDBDefaultRowFormat)
		}
	} else {
		// Row format unknown → the 767 COMPACT/REDUNDANT bound stays a
		// candidate alongside the DYNAMIC/COMPRESSED family.
		missing = append(missing, factInstanceInnoDBDefaultRowFormat)
		dynamicBounds = append([]int{767}, dynamicBounds...)
	}
	return indexKeyLimitResolution{
		bounds:       dedupeSortedBounds(dynamicBounds),
		missingFacts: missing,
	}
}

// dedupeSortedBounds collapses duplicate candidate bounds and returns them in
// ascending order so bounds[0] is the strictest and bounds[len-1] the loosest.
func dedupeSortedBounds(bounds []int) []int {
	sort.Ints(bounds)
	out := bounds[:0]
	for i, bound := range bounds {
		if i > 0 && bound == bounds[i-1] {
			continue
		}
		out = append(out, bound)
	}
	return out
}

// mysqlIndexRowFormat resolves the effective InnoDB row format when a fact
// exists — the statement option first, then the instance default — and reports
// whether the value came from real evidence instead of a guess.
func mysqlIndexRowFormat(statement spec.Statement, instance *spec.InstanceFacts) (string, bool) {
	if value := strings.ToUpper(strings.TrimSpace(normalizedTableOption(statement, "row_format"))); value != "" {
		return value, true
	}
	if instance != nil {
		if value := strings.ToUpper(strings.TrimSpace(instance.InnoDBDefaultRowFormat)); value != "" {
			return value, true
		}
	}
	return "", false
}

// statementVersionIdentity returns the shared version fact enrichment attached,
// or nil when no verified target/observed version exists.
func statementVersionIdentity(statement spec.Statement) *spec.VersionIdentity {
	if statement.Metadata == nil {
		return nil
	}
	return statement.Metadata.Version
}

// indexEstimatedTotals returns each secondary index's estimated byte length
// under the resolved charset — shared by Evaluate and the ambiguity check so
// the two paths can never diverge.
func indexEstimatedTotals(statement spec.Statement, charset string) map[string]int {
	totals := make(map[string]int, len(statement.DDL.Indexes))
	for _, index := range statement.DDL.Indexes {
		total := 0
		for _, columnName := range index.Columns {
			column, ok := ddlColumnByName(statement, columnName)
			if !ok {
				continue
			}
			total += estimatedColumnBytes(column, charset)
		}
		totals[index.Name] = total
	}
	return totals
}

// indexKeyBoundAmbiguous reports whether any index total lands strictly inside
// the candidate family — above the strictest bound but not beyond the loosest —
// meaning the missing facts would change the verdict.
func indexKeyBoundAmbiguous(totals map[string]int, bounds []int) bool {
	if len(bounds) < 2 {
		return false
	}
	loosest := bounds[len(bounds)-1]
	for _, total := range totals {
		if total > bounds[0] && total <= loosest {
			return true
		}
	}
	return false
}

// EvidenceGaps reports which declared facts the version-dependent bound needed
// but could not obtain. Version-level gaps are unconditional; instance-fact
// gaps fire only when a missing fact would actually change the verdict — a
// statement already proven safe under the strictest candidate bound or
// violating even the loosest one stays complete. It never suppresses
// Evaluate.
func (r indexKeyLengthRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) || !versionDependentKeyLengthDialect(statement.Dialect) {
		return nil
	}
	instance, _ := instanceFacts(statement)
	resolution := resolveIndexKeyLimit(statement, instance)
	if resolution.gapReason != "" {
		return []rule.EvidenceGap{{
			ReasonCode:    resolution.gapReason,
			RequiredFacts: resolution.gapFacts,
		}}
	}
	if len(resolution.missingFacts) > 0 {
		charset := resolvedTableCharset(statement, instance)
		if indexKeyBoundAmbiguous(indexEstimatedTotals(statement, charset), resolution.bounds) {
			return []rule.EvidenceGap{{
				ReasonCode:    gapReasonMissingInstanceFact,
				RequiredFacts: append([]string(nil), resolution.missingFacts...),
			}}
		}
	}
	return nil
}

func versionDependentKeyLengthDialect(dialect spec.Dialect) bool {
	return dialect == spec.DialectMySQL || dialect == spec.DialectTiDB
}

func (r indexKeyLengthRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	instance, _ := instanceFacts(statement)
	if !versionDependentKeyLengthDialect(statement.Dialect) {
		if instance == nil {
			return nil, nil
		}
		return r.evaluateWithBounds(ctx, statement, resolvedTableCharset(statement, instance), []int{estimatedIndexKeyLimit(statement, instance)})
	}

	charset := resolvedTableCharset(statement, instance)
	resolution := resolveIndexKeyLimit(statement, instance)
	if resolution.suppressed {
		return nil, nil
	}
	return r.evaluateWithBounds(ctx, statement, charset, resolution.bounds)
}

// evaluateWithBounds emits findings for indexes whose estimated byte length
// exceeds every candidate bound — the reported limit is the loosest candidate,
// the largest bound the violation is proven against. Totals inside the
// candidate family are ambiguous and stay silent here; EvidenceGaps reports
// the missing facts that would resolve them.
func (r indexKeyLengthRule) evaluateWithBounds(ctx context.Context, statement spec.Statement, charset string, bounds []int) ([]rule.Finding, error) {
	if len(bounds) == 0 {
		return nil, nil
	}
	limit := bounds[len(bounds)-1]
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
		if total == 0 || total <= limit {
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
