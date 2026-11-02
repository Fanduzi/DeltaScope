// Package audit models TiDB TTL transitions.
package audit

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type ttlCreateOutcome int

const (
	ttlCreateNone ttlCreateOutcome = iota
	ttlCreateApplied
	ttlCreateRefused
	ttlCreateUnproven
)

func createTTLStatementOutcome(dialect spec.Dialect, ddl *spec.DDL) ttlCreateOutcome {
	if ddl == nil || len(ddl.TTL) == 0 {
		return ttlCreateNone
	}
	if dialect == spec.DialectMySQL {
		hasFeatureComment := false
		for _, declaration := range ddl.TTL {
			switch declaration.Source {
			case spec.TTLClauseSourceNative:
				return ttlCreateRefused
			case spec.TTLClauseSourceFeatureComment:
				hasFeatureComment = true
			default:
				return ttlCreateUnproven
			}
		}
		if hasFeatureComment {
			return ttlCreateNone
		}
		return ttlCreateRefused
	}
	if dialect != spec.DialectTiDB {
		return ttlCreateUnproven
	}
	ttlCount := 0
	memberCounts := map[string]int{}
	for _, declaration := range ddl.TTL {
		memberCounts[declaration.Member]++
		if declaration.Member == "ttl" {
			ttlCount++
		}
		if len(declaration.Unmodeled) > 0 || declaration.Source == spec.TTLClauseSourceUnproven {
			return ttlCreateUnproven
		}
	}
	if ttlCount > 1 || (ttlCount == 0 && len(ddl.TTL) > 0) {
		return ttlCreateRefused
	}
	for member, count := range memberCounts {
		if member != "ttl" && count > 1 {
			return ttlCreateUnproven
		}
	}
	return ttlCreateApplied
}

func authoredTTLConfig(ddl *spec.DDL) *spec.TTLConfig {
	enabled := true
	jobInterval := "1h"
	config := &spec.TTLConfig{Enabled: &enabled, JobInterval: jobInterval}
	for _, declaration := range ddl.TTL {
		switch declaration.Member {
		case "ttl":
			config.TimeColumn = declaration.TimeColumn
			config.IntervalValue = declaration.IntervalValue
			config.IntervalUnit = declaration.IntervalUnit
		case "ttl_enable":
			config.Enabled = declaration.Enabled
		case "ttl_job_interval":
			config.JobInterval = declaration.JobInterval
		}
	}
	return config
}

func alterStatementHasTTL(ddl *spec.DDL) bool {
	if ddl == nil {
		return false
	}
	for _, alter := range ddl.Alter {
		if alter.Action == "remove_ttl" || len(alter.TTL) > 0 {
			return true
		}
	}
	return false
}

type mysqlTTLAlterResult int

const (
	mysqlTTLAlterRefused mysqlTTLAlterResult = iota
	mysqlTTLAlterUnproven
)

// mysqlTTLAlterOutcome separates the two MySQL vendor boundaries: a bare TTL
// clause is a proven refusal and keeps pre-state, while a feature-comment TTL
// clause may be ignored or executed until a native probe proves otherwise.
func mysqlTTLAlterOutcome(ddl *spec.DDL) mysqlTTLAlterResult {
	if ddl == nil {
		return mysqlTTLAlterRefused
	}
	hasFeatureComment := false
	for _, alter := range ddl.Alter {
		for _, declaration := range alter.TTL {
			switch declaration.Source {
			case spec.TTLClauseSourceNative:
				return mysqlTTLAlterRefused
			case spec.TTLClauseSourceFeatureComment:
				hasFeatureComment = true
			default:
				return mysqlTTLAlterUnproven
			}
		}
	}
	if hasFeatureComment {
		return mysqlTTLAlterUnproven
	}
	return mysqlTTLAlterRefused
}

func (s *batchState) applyTTLAlter(alter *spec.Alter, targets []spec.Table) {
	if s.dialect != spec.DialectTiDB || len(targets) == 0 {
		return
	}
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	if alter.Action == "remove_ttl" {
		if len(alter.TTL) > 0 && (len(alter.TTL[0].Unmodeled) > 0 || alter.TTL[0].Source == spec.TTLClauseSourceUnproven) {
			s.invalidateAll(targets)
			return
		}
		s.applyRemoveTTL(key, entry)
		return
	}
	if len(alter.TTL) != 1 {
		return
	}
	if entry == nil || entry.state == tableUnknown || entry.state == tableAbsent {
		// Unknown stays unknown; a proven-absent target keeps its refusal
		// pre-state instead of being rewritten as unknown.
		return
	}
	if entry.shape == nil {
		return
	}
	if len(alter.TTL[0].Unmodeled) > 0 || alter.TTL[0].Source == spec.TTLClauseSourceUnproven {
		s.invalidateAll(targets)
		return
	}
	declaration := alter.TTL[0]
	shape := cloneTableSnapshot(entry.shape)
	switch declaration.Member {
	case "ttl":
		s.applyTTLDefinition(shape, declaration)
	case "ttl_enable":
		s.applyTTLEnable(shape, declaration.Enabled)
	case "ttl_job_interval":
		s.applyTTLJobInterval(shape, declaration.JobInterval)
	default:
		s.invalidateAll(targets)
		return
	}
	s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
}

func (s *batchState) applyRemoveTTL(key batchTableKey, entry *batchTableEntry) {
	if entry == nil || entry.state == tableUnknown || entry.state == tableAbsent {
		return
	}
	shape := projectEntry(key, entry)
	shape.TTL = nil
	shape.TTLKnown = true
	s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
}

func (s *batchState) applyTTLDefinition(shape *spec.TableSnapshot, declaration spec.TTLDeclaration) {
	outcome := spec.TTLDefinitionLandingOutcome(shape, declaration)
	if !shape.TTLKnown {
		if outcome != spec.TTLDefinitionApplied {
			return
		}
		// Prior state could be either no TTL (defaults apply) or an existing
		// config (enabled/job interval are preserved): only the definition
		// members are provable after the statement lands.
		shape.TTL = &spec.TTLConfig{
			TimeColumn:    declaration.TimeColumn,
			IntervalValue: declaration.IntervalValue,
			IntervalUnit:  declaration.IntervalUnit,
		}
		shape.TTLKnown = true
		return
	}
	if shape.TTL == nil {
		switch outcome {
		case spec.TTLDefinitionApplied:
			enabled := true
			shape.TTL = &spec.TTLConfig{
				TimeColumn:    declaration.TimeColumn,
				IntervalValue: declaration.IntervalValue,
				IntervalUnit:  declaration.IntervalUnit,
				Enabled:       &enabled,
				JobInterval:   "1h",
			}
		case spec.TTLDefinitionRefused:
			// A known-broken premise keeps the proven no-TTL fact.
			return
		default:
			// The statement may create TTL or refuse; the post-state can no
			// longer claim either existence answer.
			shape.TTLKnown = false
		}
		return
	}
	switch outcome {
	case spec.TTLDefinitionApplied:
		shape.TTL.TimeColumn = declaration.TimeColumn
		shape.TTL.IntervalValue = declaration.IntervalValue
		shape.TTL.IntervalUnit = declaration.IntervalUnit
	case spec.TTLDefinitionRefused:
		return
	default:
		// Existing TTL remains whether the statement lands or refuses; only
		// the authored definition members become unproven while enable/job
		// members keep their catalog answers.
		shape.TTL.TimeColumn = ""
		shape.TTL.IntervalValue = ""
		shape.TTL.IntervalUnit = ""
	}
}

func (s *batchState) applyTTLEnable(shape *spec.TableSnapshot, enabled *bool) {
	if !shape.TTLKnown {
		return
	}
	if shape.TTL == nil || enabled == nil {
		return
	}
	value := *enabled
	shape.TTL.Enabled = &value
}

func (s *batchState) applyTTLJobInterval(shape *spec.TableSnapshot, interval string) {
	if !shape.TTLKnown || shape.TTL == nil || strings.TrimSpace(interval) == "" {
		return
	}
	shape.TTL.JobInterval = interval
}

func ttlTimeColumnType(declared string) bool {
	return spec.TTLTimeColumnType(declared)
}

// ttlColumnActionOutcome is the three-way safety answer a column action gets
// from TTL facts: the action is independent of the time-column binding, the
// outcome cannot be proven, or the action is a proven atomic refusal.
type ttlColumnActionOutcome int

const (
	ttlColumnActionSafe ttlColumnActionOutcome = iota
	ttlColumnActionUnsafe
	ttlColumnActionRefused
)

// ttlColumnTypeOutcome reports whether replacing columnName's type with
// targetType can be published honestly. A proven bound column must remain a
// TTL-capable temporal type; an unobserved binding (TTLKnown=false or a
// TTLConfig without its time-column member) can tolerate only a temporal
// replacement, since a non-temporal successor could have made a hidden TTL
// binding refuse the statement.
func ttlColumnTypeOutcome(dialect spec.Dialect, shape *spec.TableSnapshot, columnName, targetType string) ttlColumnActionOutcome {
	if dialect != spec.DialectTiDB || shape == nil {
		return ttlColumnActionSafe
	}
	if shape.TTL != nil {
		if shape.TTL.TimeColumn == "" {
			if ttlTimeColumnType(targetType) {
				return ttlColumnActionSafe
			}
			return ttlColumnActionUnsafe
		}
		if strings.EqualFold(shape.TTL.TimeColumn, columnName) && !ttlTimeColumnType(targetType) {
			return ttlColumnActionUnsafe
		}
		return ttlColumnActionSafe
	}
	if !shape.TTLKnown && !ttlTimeColumnType(targetType) {
		return ttlColumnActionUnsafe
	}
	return ttlColumnActionSafe
}

// ttlTemporalIdentityTemplate widens the ordinary MODIFY/CHANGE type template
// for the one column a proven TTL config binds: TiDB keeps a TTL binding when
// the replacement is still a legal time type. It never widens unrelated
// temporal columns or a config whose binding member is unobserved.
func ttlTemporalIdentityTemplate(dialect spec.Dialect, shape *spec.TableSnapshot, columnName string, target spec.Column) bool {
	return dialect == spec.DialectTiDB && shape != nil && shape.TTL != nil &&
		strings.EqualFold(shape.TTL.TimeColumn, columnName) && ttlTimeColumnType(target.Type)
}

// ttlDropColumnOutcome adjudicates DROP COLUMN against TTL state. A proven
// bound time column is the probe-backed refusal (the pre-state survives
// atomically). An unobserved TTL binding may be that column, so the drop is
// withheld; a proven no-TTL or unrelated binding leaves the ordinary drop
// template unchanged.
func ttlDropColumnOutcome(dialect spec.Dialect, shape *spec.TableSnapshot, columnName string) ttlColumnActionOutcome {
	if dialect != spec.DialectTiDB || shape == nil {
		return ttlColumnActionSafe
	}
	if shape.TTL != nil {
		if shape.TTL.TimeColumn == "" {
			return ttlColumnActionUnsafe
		}
		if strings.EqualFold(shape.TTL.TimeColumn, columnName) {
			return ttlColumnActionRefused
		}
		return ttlColumnActionSafe
	}
	if !shape.TTLKnown {
		return ttlColumnActionUnsafe
	}
	return ttlColumnActionSafe
}

// scrubTTLIdentityMembers withholds only the binding member whose source or
// destination identity may have changed while the column set itself was
// withheld. TTL existence remains proven; the stale name is never preserved.
func scrubTTLIdentityMembers(shape *spec.TableSnapshot, oldName, newName string) {
	if shape == nil || shape.TTL == nil || shape.TTL.TimeColumn == "" {
		return
	}
	if strings.EqualFold(shape.TTL.TimeColumn, oldName) || strings.EqualFold(shape.TTL.TimeColumn, newName) {
		shape.TTL.TimeColumn = ""
	}
}
