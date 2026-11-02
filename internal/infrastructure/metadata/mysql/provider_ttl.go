// Package mysqlmeta reads TiDB TTL catalog facts.
package mysqlmeta

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func quoteIdentifier(identifier string) string {
	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
}

// loadTiDBTTL reads the authoritative SHOW CREATE surface TiDB exposes for
// TTL configuration. A clean read establishes the existence answer; a TTL
// member whose catalog text is unrecognizable stays unmodeled inside a
// proven-present config rather than being guessed.
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
	return nil
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
	var comments []string
	lower := strings.ToLower(text)
	for offset := 0; offset+9 <= len(text); {
		idx := strings.Index(lower[offset:], "/*t![ttl]")
		if idx < 0 {
			break
		}
		start := offset + idx + 9
		end := strings.Index(text[start:], "*/")
		if end < 0 {
			break
		}
		comments = append(comments, text[start:start+end])
		offset = start + end + 2
	}
	return comments
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
