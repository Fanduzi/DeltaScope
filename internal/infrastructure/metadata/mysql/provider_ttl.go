// Package mysqlmeta reads TiDB TTL catalog facts.
package mysqlmeta

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func quoteIdentifier(identifier string) string {
	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
}

// loadTiDBTTL reads the authoritative SHOW CREATE surface TiDB exposes for
// TTL configuration and the TiDB option feature comments. A clean read
// establishes the existence answer; a TTL member whose catalog text is
// unrecognizable stays unmodeled inside a proven-present config rather than
// being guessed, and an absent option comment is the upstream-proven zero
// (TiDB emits the comment only for a stored non-zero value).
func (p *Provider) loadTiDBTTL(ctx context.Context, snapshot *spec.TableSnapshot) error {
	var objectName, createSQL string
	if err := p.db.QueryRowContext(ctx,
		"show create table "+quoteIdentifier(snapshot.Schema)+"."+quoteIdentifier(snapshot.Table.Name),
	).Scan(&objectName, &createSQL); err != nil {
		return fmt.Errorf("query show create table: %w", err)
	}
	config, _ := parseCatalogTTLConfig(createSQL)
	snapshot.TTLKnown = true
	snapshot.TTL = config
	options := parseCatalogLayoutOptions(createSQL)
	status, err := p.loadTiDBCacheStatus(ctx, snapshot, options["cache_status"] == "enable")
	if err != nil {
		return err
	}
	if status != "" {
		options["cache_status"] = status
	}
	for name, value := range options {
		snapshot.Options[name] = value
	}
	return nil
}

// loadTiDBCacheStatus resolves the table's cache status from
// mysql.table_cache_meta — the lock record upstream writes before the
// cache DDL job (state_remote.go), never deleted afterwards. The row
// only proves the table once entered the cache path; it is not a status
// projection — a completed NOCACHE leaves it behind, a pending CACHE
// shares the same shape, and Switching carries no marker either, so
// row-present without the bare /* CACHED ON */ marker stays unobserved
// rather than claiming a non-Disable status. The LEFT JOIN anchors on
// the information_schema table row itself: zero rows means the table
// identity was not observed (no claim), a NULL lock column proves
// never-cached Disable, and the bare marker alone proves Enable.
// Query and scan failures propagate — an auxiliary read never degrades
// into a status claim (issue #102 T23-R7 B1).
func (p *Provider) loadTiDBCacheStatus(ctx context.Context, snapshot *spec.TableSnapshot, cachedOn bool) (string, error) {
	var tid sql.NullInt64
	err := p.db.QueryRowContext(ctx,
		"select c.tid from information_schema.tables t left join mysql.table_cache_meta c on c.tid = t.tidb_table_id where t.table_schema = ? and t.table_name = ?",
		snapshot.Schema, snapshot.Table.Name).Scan(&tid)
	switch {
	case err == sql.ErrNoRows:
		// The anchor table row itself was not observed — the snapshot's
		// own existence answer owns that claim. The bare marker still
		// proves Enable on its own; anything else stays unobserved.
		if cachedOn {
			return "enable", nil
		}
		return "", nil
	case err != nil:
		return "", fmt.Errorf("query table cache status: %w", err)
	case cachedOn:
		return "enable", nil
	case !tid.Valid:
		return "disable", nil
	default:
		return "", nil
	}
}

// parseCatalogLayoutOptions projects the SHOW CREATE feature comments for
// the T23 layout options. An absent /*T![auto_id_cache] comment is the
// upstream-proven zero — TiDB only emits it for a stored non-zero
// AutoIDCache — so a clean read publishes the proven catalog value.
func parseCatalogLayoutOptions(createSQL string) map[string]string {
	out := map[string]string{"auto_id_cache": "0"}
	for _, comment := range featureComments(createSQL, "/*t![auto_id_cache]") {
		member, body, ok := catalogMember(comment)
		if !ok || member != "auto_id_cache" {
			continue
		}
		if value, ok := parseCatalogScalar(body); ok && value != "" {
			out["auto_id_cache"] = value
		}
	}
	// Upstream emits a bare `/* CACHED ON */` comment — deliberately not a
	// feature comment — exactly when TableCacheStatusType is Enable. Its
	// absence cannot prove Disable: the status space is three-valued
	// (Disable/Enable/Switching) and Switching carries no comment, so a
	// missing token leaves cache_status unobserved rather than fabricated
	// (issue #102 T23-R7 B1). The scan is the same quote-aware pass: a
	// user-authored COMMENT echoing the token never matches.
	if plainCommentToken(createSQL, "cached on") {
		out["cache_status"] = "enable"
	}
	return out
}

// plainCommentToken reports whether the SHOW CREATE text carries a
// /* token */ comment (any case) outside quoted regions. Unlike
// featureComments it matches plain comments, not /*T![ns] markers.
func plainCommentToken(text, token string) bool {
	lower := strings.ToLower(text)
	for i := 0; i < len(text); {
		switch c := text[i]; c {
		case '\'', '"', '`':
			i++
			for i < len(text) {
				if text[i] == '\\' && c != '`' {
					i += 2
					continue
				}
				if text[i] == c {
					if i+1 < len(text) && text[i+1] == c {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		default:
			if strings.HasPrefix(lower[i:], "/*") {
				end := strings.Index(text[i+2:], "*/")
				if end < 0 {
					return false
				}
				if strings.EqualFold(strings.TrimSpace(text[i+2:i+2+end]), token) {
					return true
				}
				i += end + 4
				continue
			}
			i++
		}
	}
	return false
}

// featureComments extracts every /*T![ns] ... */ comment for one feature
// namespace from the SHOW CREATE text. The scan is quote-aware: markers
// inside single-quoted or double-quoted string literals and backtick
// quoted identifiers are user-authored text, not generated catalog
// comments, and never match. String literals honour backslash escapes
// and doubled quote characters; backtick identifiers only the doubled
// backtick.
func featureComments(text, marker string) []string {
	var comments []string
	lower := strings.ToLower(text)
	for i := 0; i < len(text); {
		switch c := text[i]; c {
		case '\'', '"', '`':
			i++
			for i < len(text) {
				if text[i] == '\\' && c != '`' {
					i += 2
					continue
				}
				if text[i] == c {
					if i+1 < len(text) && text[i+1] == c {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		default:
			if !strings.HasPrefix(lower[i:], marker) {
				i++
				continue
			}
			start := i + len(marker)
			end := strings.Index(text[start:], "*/")
			if end < 0 {
				return comments
			}
			comments = append(comments, text[start:start+end])
			i = start + end + 2
		}
	}
	return comments
}

// catalogMember splits a feature-comment body into the leading member
// name and the value text after '='.
func catalogMember(comment string) (string, string, bool) {
	comment = strings.TrimSpace(comment)
	i := 0
	for i < len(comment) && isCatalogBareWord(comment[i]) {
		i++
	}
	if i == 0 {
		return "", "", false
	}
	member := strings.ToLower(comment[:i])
	i = skipCatalogSpace(comment, i)
	if i >= len(comment) || comment[i] != '=' {
		return "", "", false
	}
	return member, comment[i+1:], true
}

func parseCatalogTTLConfig(createSQL string) (*spec.TTLConfig, bool) {
	var config *spec.TTLConfig
	comments := ttlFeatureComments(createSQL)
	for _, comment := range comments {
		member, body, ok := ttlCatalogMember(comment)
		if !ok {
			continue
		}
		if config == nil {
			config = &spec.TTLConfig{}
		}
		switch member {
		case "ttl":
			column, value, unit, ok := parseCatalogTTLDefinition(body)
			if !ok {
				continue
			}
			config.TimeColumn = strings.ToLower(column)
			config.IntervalValue = value
			config.IntervalUnit = strings.ToLower(unit)
		case "ttl_enable":
			if value, ok := parseCatalogScalar(body); ok {
				switch {
				case strings.EqualFold(value, "on"):
					enabled := true
					config.Enabled = &enabled
				case strings.EqualFold(value, "off"):
					enabled := false
					config.Enabled = &enabled
				}
			}
		case "ttl_job_interval":
			if value, ok := parseCatalogScalar(body); ok {
				config.JobInterval = value
			}
		}
	}
	if config == nil && len(comments) > 0 {
		// The TTL feature namespace is present even when no member parsed:
		// retain proven existence with every member unobserved.
		config = &spec.TTLConfig{}
	}
	return config, config != nil
}

func ttlFeatureComments(text string) []string {
	return featureComments(text, "/*t![ttl]")
}

func ttlCatalogMember(comment string) (string, string, bool) {
	comment = strings.TrimSpace(comment)
	i := 0
	for i < len(comment) && isCatalogBareWord(comment[i]) {
		i++
	}
	if i == 0 {
		return "", "", false
	}
	member := strings.ToLower(comment[:i])
	if member != "ttl" && member != "ttl_enable" && member != "ttl_job_interval" {
		return "", "", false
	}
	i = skipCatalogSpace(comment, i)
	if i >= len(comment) || comment[i] != '=' {
		return "", "", false
	}
	return member, comment[i+1:], true
}

func parseCatalogTTLDefinition(body string) (column, value, unit string, ok bool) {
	i := skipCatalogSpace(body, 0)
	if i < len(body) && body[i] == '`' {
		i++
		var name strings.Builder
		for i < len(body) {
			if body[i] == '`' {
				if i+1 < len(body) && body[i+1] == '`' {
					name.WriteByte('`')
					i += 2
					continue
				}
				i++
				break
			}
			name.WriteByte(body[i])
			i++
		}
		if i > len(body) {
			return "", "", "", false
		}
		column = name.String()
	} else {
		start := i
		for i < len(body) && isCatalogBareWord(body[i]) {
			i++
		}
		column = body[start:i]
	}
	if column == "" {
		return "", "", "", false
	}
	i = skipCatalogSpace(body, i)
	if i >= len(body) || body[i] != '+' {
		return "", "", "", false
	}
	i = skipCatalogSpace(body, i+1)
	keyword, next := catalogBareWord(body, i)
	if !strings.EqualFold(keyword, "interval") {
		return "", "", "", false
	}
	i = skipCatalogSpace(body, next)
	start := i
	for i < len(body) && unicode.IsDigit(rune(body[i])) {
		i++
	}
	if start == i {
		return "", "", "", false
	}
	value = body[start:i]
	i = skipCatalogSpace(body, i)
	unit, _ = catalogBareWord(body, i)
	if unit == "" {
		return "", "", "", false
	}
	return column, value, unit, true
}

func parseCatalogScalar(body string) (string, bool) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", false
	}
	if body[0] == '\'' {
		var out strings.Builder
		for i := 1; i < len(body); i++ {
			if body[i] == '\'' {
				if i+1 < len(body) && body[i+1] == '\'' {
					out.WriteByte('\'')
					i++
					continue
				}
				return out.String(), true
			}
			if body[i] == '\\' && i+1 < len(body) {
				i++
			}
			out.WriteByte(body[i])
		}
		return "", false
	}
	value, _ := catalogBareWord(body, 0)
	return value, value != ""
}

func skipCatalogSpace(text string, i int) int {
	for i < len(text) && unicode.IsSpace(rune(text[i])) {
		i++
	}
	return i
}

func catalogBareWord(text string, i int) (string, int) {
	i = skipCatalogSpace(text, i)
	start := i
	for i < len(text) && isCatalogBareWord(text[i]) {
		i++
	}
	return text[start:i], i
}

func isCatalogBareWord(c byte) bool {
	return c == '_' || c == '$' || unicode.IsLetter(rune(c)) || unicode.IsDigit(rune(c))
}
