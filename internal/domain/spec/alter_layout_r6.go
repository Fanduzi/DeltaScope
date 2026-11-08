package spec

import (
	"strconv"
	"strings"
)

// alter_layout_r6.go implements the T23-R6 outcome adjudication for the
// remaining TiDB physical-layout ALTER actions: CACHE/NOCACHE,
// ATTRIBUTES (table and partition level), and SET TIFLASH REPLICA. The
// verdicts are the same AutoRandomVerdict vocabulary the auto_random
// files use — the rules emit refused findings and undecidable evidence
// gaps; the ordered-state publisher consumes the same verdicts so a
// proven refusal never publishes a successor shape (issue #102).

// Reason codes for the layout actions — stable vocabulary shared by the
// rule findings, the evidence gaps, and the state publisher.
const (
	LayoutReasonCacheAlreadyCached   = "cache_already_cached"
	LayoutReasonNoCacheNotCached     = "nocache_not_cached"
	LayoutReasonCacheSysTable        = "cache_system_table"
	LayoutReasonCacheTemporary       = "cache_temporary_table"
	LayoutReasonCachePartitioned     = "cache_partitioned_table"
	LayoutReasonCacheSizeUnproven    = "cache_table_size_unproven"
	LayoutReasonCachedTableGuard     = "cached_table_alter_guard"
	LayoutReasonAttributesInvalid    = "attributes_spec_invalid"
	LayoutReasonAttributesNonPart    = "attributes_on_nonpartitioned"
	LayoutReasonAttributesPartMiss   = "attributes_partition_not_found"
	LayoutReasonPartitionedUnproven  = "partitioned_state_unproven"
	LayoutReasonMembersUnproven      = "partition_members_unproven"
	LayoutReasonTiFlashSysTable      = "tiflash_system_table"
	LayoutReasonTiFlashTemporary     = "tiflash_temporary_table"
	LayoutReasonTiFlashViewOrSeq     = "tiflash_view_or_sequence"
	LayoutReasonTiFlashCharset       = "tiflash_unsupported_charset"
	LayoutReasonTiFlashSameDef       = "tiflash_same_definition"
	LayoutReasonTiFlashStoreUnproven = "tiflash_store_count_unproven"
	LayoutReasonTiFlashHypo          = "tiflash_hypothetical_session_only"
	LayoutReasonOldReplicaUnproven   = "tiflash_old_definition_unproven"
	LayoutReasonCacheStatusUnproven  = "cache_status_unproven"
	LayoutReasonCharsetUnproven      = "charset_support_unproven"
)

// AlterLayoutR6Facts carries the catalog/statement facts the R6 outcomes
// adjudicate against. Zero values mean unproven, never absent.
type AlterLayoutR6Facts struct {
	// SysDB is true when the target schema is a system/memory database.
	SysDB bool
	// Temporary is the proven session/global temporary marker.
	Temporary bool
	// Partitioned is the proven partitioning answer; PartitionedKnown
	// distinguishes proven false from unobserved.
	Partitioned      bool
	PartitionedKnown bool
	// Members is the proven partition-member name set (lower-cased).
	// MembersKnown=false means the member space is unobserved.
	Members      map[string]bool
	MembersKnown bool
	// CacheStatus is the proven TableCacheStatusType: "enable" or
	// "disable". CacheStatusKnown=false leaves it unobserved.
	CacheStatus      string
	CacheStatusKnown bool
	// ViewOrSequence marks the target as a view or sequence (WrongObject
	// refusal); false covers both base tables and unproven kinds — the
	// target-table existence rules own that adjudication.
	ViewOrSequence bool
	// CharsetUnsupported reports a proven column charset outside the
	// TiFlash-supported set; CharsetKnown=false keeps it unobserved.
	CharsetUnsupported bool
	CharsetKnown       bool
	// ReplicaCount/ReplicaLabels carry the proven catalog TiFlash replica
	// definition; ReplicaKnown=false leaves it unobserved.
	ReplicaCount  uint64
	ReplicaLabels []string
	ReplicaKnown  bool
}

// AlterTableCacheOutcome adjudicates `ALTER TABLE t CACHE|NOCACHE`
// against upstream executor.go AlterTableCache/AlterTableNoCache:
//
//	CACHE:    already-cached no-op → sys/mem-DB refusal → temporary
//	          refusal → partitioned refusal → 64MB data-size premise
//	          (undecidable offline) → applied.
//	NOCACHE:  not-cached no-op → applied.
//
// A refused arm returns Refused with the upstream reason; the size arm is
// the single runtime premise and stays an evidence gap, never a guess.
func AlterTableCacheOutcome(action string, facts AlterLayoutR6Facts) (AutoRandomVerdict, string) {
	if action == "nocache" {
		if facts.CacheStatusKnown && facts.CacheStatus != "enable" {
			return AutoRandomOK, LayoutReasonNoCacheNotCached
		}
		if !facts.CacheStatusKnown {
			return AutoRandomUndecidable, LayoutReasonCacheStatusUnproven
		}
		return AutoRandomOK, ""
	}
	// cache
	if facts.CacheStatusKnown && facts.CacheStatus == "enable" {
		return AutoRandomOK, LayoutReasonCacheAlreadyCached
	}
	if facts.SysDB {
		return AutoRandomRefused, LayoutReasonCacheSysTable
	}
	if facts.Temporary {
		return AutoRandomRefused, LayoutReasonCacheTemporary
	}
	if facts.PartitionedKnown && facts.Partitioned {
		return AutoRandomRefused, LayoutReasonCachePartitioned
	}
	if !facts.PartitionedKnown {
		return AutoRandomUndecidable, LayoutReasonPartitionedUnproven
	}
	// checkCacheTableSize scans the live table data up to 64MB — a
	// proven-too-large table is the only refused arm left, and data size
	// is a runtime premise the offline audit never observes.
	return AutoRandomUndecidable, LayoutReasonCacheSizeUnproven
}

// CachedTableGuardRefused reports the upstream guard: once a table is
// cache-enabled, every ALTER spec other than a single CACHE/NOCACHE is
// refused (ErrOptOnCacheTable), and multi-spec forms are refused too.
// The guard needs a proven enable status — unknown cache state cannot
// refuse on this arm.
func CachedTableGuardRefused(action string, alterCount int, facts AlterLayoutR6Facts) bool {
	if !facts.CacheStatusKnown || facts.CacheStatus != "enable" {
		return false
	}
	if alterCount != 1 {
		return true
	}
	return action != "cache" && action != "nocache"
}

// ParseAttributesSpecValue emulates upstream label.Rule.ApplyAttributesSpec:
// the spec text is wrapped as a YAML flow sequence (`[value]`) and every
// item must parse as a `key=value` label. It returns false when the
// authored text cannot form a valid label list. Quoted items and commas
// inside quotes are honored so a user value containing SQL-shaped text
// is never keyword-scanned.
func ParseAttributesSpecValue(value string) bool {
	items, ok := splitFlowList(value)
	if !ok {
		return false
	}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			return false
		}
		eq := strings.Index(item, "=")
		if eq <= 0 {
			return false
		}
		if strings.TrimSpace(item[:eq]) == "" {
			return false
		}
	}
	return true
}

// splitFlowList splits the authored attribute text the way the upstream
// YAML unmarshal reads `[text]`: top-level commas outside single and
// double quotes. It returns ok=false on unbalanced quoting or bracket
// structure.
func splitFlowList(value string) ([]string, bool) {
	var items []string
	var cur strings.Builder
	var quote byte
	for i := 0; i < len(value); i++ {
		c := value[i]
		if quote != 0 {
			cur.WriteByte(c)
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
			cur.WriteByte(c)
		case '[':
			return nil, false
		case ']':
			return nil, false
		case ',':
			items = append(items, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if quote != 0 {
		return nil, false
	}
	items = append(items, cur.String())
	// strip optional surrounding quotes per item
	for i, item := range items {
		t := strings.TrimSpace(item)
		if len(t) >= 2 && (t[0] == '\'' && t[len(t)-1] == '\'') ||
			len(t) >= 2 && (t[0] == '"' && t[len(t)-1] == '"') {
			items[i] = t[1 : len(t)-1]
		} else {
			items[i] = t
		}
	}
	return items, true
}

// AlterAttributesOutcome adjudicates `ALTER TABLE t [PARTITION p]
// ATTRIBUTES='...'|DEFAULT`. Table level: an invalid spec refuses, else
// applies (the label rule write is the DDL job's own effect — accepted
// declaration is the modeled post-state, runtime PD propagation is never
// claimed). Partition level adds the non-partitioned refusal and the
// member-existence adjudication.
func AlterAttributesOutcome(decl *AlterAttributesDecl, partitions []string, facts AlterLayoutR6Facts) (AutoRandomVerdict, string) {
	partitionLevel := len(partitions) > 0
	if partitionLevel {
		if facts.PartitionedKnown && !facts.Partitioned {
			return AutoRandomRefused, LayoutReasonAttributesNonPart
		}
		if !facts.PartitionedKnown {
			return AutoRandomUndecidable, LayoutReasonPartitionedUnproven
		}
		if !facts.MembersKnown {
			return AutoRandomUndecidable, LayoutReasonMembersUnproven
		}
		for _, name := range partitions {
			if !facts.Members[name] {
				return AutoRandomRefused, LayoutReasonAttributesPartMiss
			}
		}
	}
	if decl == nil {
		return AutoRandomUndecidable, LayoutReasonMembersUnproven
	}
	if !decl.Default && !ParseAttributesSpecValue(decl.Value) {
		return AutoRandomRefused, LayoutReasonAttributesInvalid
	}
	return AutoRandomOK, ""
}

// AlterTiFlashReplicaOutcome adjudicates `SET TIFLASH REPLICA n
// [LOCATION LABELS ...]`:
//
//	sys/mem-DB refusal → temporary refusal → view/sequence refusal →
//	unsupported-charset refusal → Hypo session-only (never catalog) →
//	same-definition no-op → count=0 always applies (0 > stores is
//	impossible) → count>0 needs the live store-count premise.
//
// The store-count arm is the single runtime premise; the outcome never
// claims replica availability or sync progress.
func AlterTiFlashReplicaOutcome(decl *TiFlashReplicaDecl, facts AlterLayoutR6Facts) (AutoRandomVerdict, string) {
	if decl == nil {
		return AutoRandomUndecidable, LayoutReasonMembersUnproven
	}
	if facts.SysDB {
		return AutoRandomRefused, LayoutReasonTiFlashSysTable
	}
	if facts.Temporary {
		return AutoRandomRefused, LayoutReasonTiFlashTemporary
	}
	if facts.ViewOrSequence {
		return AutoRandomRefused, LayoutReasonTiFlashViewOrSeq
	}
	if facts.CharsetKnown && facts.CharsetUnsupported {
		return AutoRandomRefused, LayoutReasonTiFlashCharset
	}
	if !facts.CharsetKnown {
		return AutoRandomUndecidable, LayoutReasonCharsetUnproven
	}
	if decl.Hypo {
		return AutoRandomOK, LayoutReasonTiFlashHypo
	}
	if !facts.ReplicaKnown {
		if decl.Count == 0 {
			// Removing a replica when the old definition is unproven is
			// still OK — upstream applies it as a job either way.
			return AutoRandomOK, ""
		}
		return AutoRandomUndecidable, LayoutReasonOldReplicaUnproven
	}
	// A stored record never carries count=0 (upstream nils it on
	// removal), so the same-definition no-op only exists for positive
	// counts — a 0 decl reaches the job and applies either way.
	if decl.Count > 0 && facts.ReplicaCount == decl.Count && sameStringList(facts.ReplicaLabels, decl.Labels) {
		return AutoRandomOK, LayoutReasonTiFlashSameDef
	}
	if decl.Count == 0 {
		return AutoRandomOK, ""
	}
	return AutoRandomUndecidable, LayoutReasonTiFlashStoreUnproven
}

func sameStringList(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// IsMemOrSysSchema mirrors upstream util.IsMemOrSysDB — the system and
// memory databases whose objects refuse CACHE and TiFlash specs.
func IsMemOrSysSchema(schema string) bool {
	switch strings.ToLower(schema) {
	case "mysql", "sys", "information_schema", "performance_schema",
		"metrics_schema", "inspection_schema", "metric_schema":
		return true
	}
	return false
}

// TiFlashSupportedCharset mirrors upstream
// charset.TiFlashSupportedCharsets.
func TiFlashSupportedCharset(charset string) bool {
	switch strings.ToLower(charset) {
	case "utf8", "utf8mb4", "ascii", "latin1", "binary":
		return true
	}
	return false
}

// AlterLayoutR6FactsForSnapshot resolves the shared adjudication facts
// from a proven table shape. Every unknown stays unknown — a missing
// Option key is never a proven absence.
func AlterLayoutR6FactsForSnapshot(shape *TableSnapshot) AlterLayoutR6Facts {
	facts := AlterLayoutR6Facts{
		Temporary: shape.Temporary || shape.GlobalTemporary,
		ViewOrSequence: shape.RelationKind == "view" ||
			shape.RelationKind == "sequence" || shape.View != nil,
	}
	schema := shape.Schema
	if schema == "" && shape.Table != nil {
		schema = shape.Table.Schema
	}
	facts.SysDB = IsMemOrSysSchema(schema)
	if shape.Partitioned != nil {
		facts.PartitionedKnown = true
		facts.Partitioned = *shape.Partitioned
	}
	if facts.Partitioned && shape.PartitionNames != nil {
		facts.MembersKnown = true
		facts.Members = map[string]bool{}
		for _, name := range shape.PartitionNames {
			facts.Members[strings.ToLower(name)] = true
		}
	}
	if status, ok := shape.Options["cache_status"]; ok {
		facts.CacheStatusKnown = true
		facts.CacheStatus = status
	}
	if _, ok := shape.Options["tiflash_replica_count"]; ok {
		facts.ReplicaKnown = true
		if count, err := strconv.ParseUint(shape.Options["tiflash_replica_count"], 10, 64); err == nil {
			facts.ReplicaCount = count
		}
		if labels := shape.Options["tiflash_replica_labels"]; labels != "" {
			facts.ReplicaLabels = strings.Split(labels, ",")
		}
	}
	facts.CharsetKnown = len(shape.Columns) > 0
	for _, column := range shape.Columns {
		if column.Charset == "" {
			facts.CharsetKnown = false
			continue
		}
		if !TiFlashSupportedCharset(column.Charset) {
			facts.CharsetUnsupported = true
		}
	}
	return facts
}
