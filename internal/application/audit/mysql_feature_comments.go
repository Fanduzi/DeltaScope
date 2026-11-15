package audit

import "strings"

// blankTiDBFeatureComments rewrites /*T! ... */ TiDB feature comments to
// same-width whitespace. MySQL has no feature-comment syntax: the server
// treats /*T! as an ordinary comment and ignores its contents, while the
// shared TiDB parser resolves it into real AST nodes. Blanking keeps byte
// offsets stable so statement positions, RawSQL, and extraction offsets
// still map to the original text.
//
// The scan is a real lexer pass: /*T! inside string literals, quoted
// identifiers, or other comments is content, never a comment opener.
// /*! executable comments, /*+ optimizer hints, and plain /* comments are
// untouched. Newlines are preserved so line numbers do not shift.
//
// /*T![ttl] ... */ regions are preserved, not blanked: the server ignores
// them identically, but a TTL clause is an authored automatic-deletion
// configuration whose inert form must stay visible to the audit surface as
// a vendor-boundary entry rather than disappear into a silent pass
// (issue #98 T19).
func blankTiDBFeatureComments(sql string) string {
	out := []byte(nil)
	state := scanNormal
	for i := 0; i < len(sql); {
		switch state {
		case scanNormal:
			switch {
			case sql[i] == '\'':
				state = scanSingleQuote
				i++
			case sql[i] == '"':
				state = scanDoubleQuote
				i++
			case sql[i] == '`':
				state = scanBacktick
				i++
			case i+1 < len(sql) && sql[i:i+2] == "--" &&
				(i+2 == len(sql) || sql[i+2] <= ' '):
				state = scanLineComment
				i += 2
			case sql[i] == '#':
				state = scanLineComment
				i++
			case i+3 < len(sql) && sql[i:i+4] == "/*T!" &&
				(i+9 > len(sql) || !strings.EqualFold(sql[i:i+9], "/*T![ttl]")):
				if out == nil {
					out = []byte(sql)
				}
				i = blankFeatureComment(out, sql, i)
			case i+1 < len(sql) && sql[i:i+2] == "/*":
				state = scanBlockComment
				i += 2
			default:
				i++
			}
		case scanSingleQuote:
			i = scanQuoted(sql, i, '\'', true, &state)
		case scanDoubleQuote:
			i = scanQuoted(sql, i, '"', true, &state)
		case scanBacktick:
			i = scanQuoted(sql, i, '`', true, &state)
		case scanLineComment:
			if sql[i] == '\n' {
				state = scanNormal
			}
			i++
		case scanBlockComment:
			if i+1 < len(sql) && sql[i:i+2] == "*/" {
				state = scanNormal
				i += 2
			} else {
				i++
			}
		}
	}
	if out == nil {
		return sql
	}
	return string(out)
}

// blankFeatureComment whites out sql[start:] through the closing */ of the
// feature comment opened at start (sql[start:start+4] == "/*T!"), keeping
// newlines. Unterminated comments blank to end of input, matching how the
// server consumes the truncated remainder. Returns the next scan offset.
func blankFeatureComment(out []byte, sql string, start int) int {
	i := start
	for i < len(sql) {
		if i+1 < len(sql) && sql[i:i+2] == "*/" {
			out[i], out[i+1] = ' ', ' '
			return i + 2
		}
		if sql[i] != '\n' {
			out[i] = ' '
		}
		i++
	}
	return i
}
