// Package audit orchestrates audit use cases at the application layer.
// input: normalized SQL text and its selected MySQL, TiDB, or PostgreSQL dialect
// output: ordered top-level statement slices with 1-based source start locations
// pos: bounded lexical statement-boundary scanner before dialect parser adapters
// note: if this file changes, update this header and module README.md.
package audit

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type statementChunk struct {
	SQL    string
	Line   int
	Column int
}

const (
	scanNormal = iota
	scanSingleQuote
	scanDoubleQuote
	scanBacktick
	scanLineComment
	scanBlockComment
	scanDollarQuote
)

func splitSQLStatements(sql string, dialect spec.Dialect) []statementChunk {
	chunks := make([]statementChunk, 0)
	state, start, blockDepth := scanNormal, 0, 0
	dollarQuote := ""
	quoteBackslashEscapes := false
	for i := 0; i < len(sql); {
		switch state {
		case scanNormal:
			switch {
			case sql[i] == ';':
				chunks = appendStatementChunk(chunks, sql, start, i+1)
				start = i + 1
				i++
			case sql[i] == '\'':
				state = scanSingleQuote
				quoteBackslashEscapes = dialect != spec.DialectPostgreSQL || isPostgreSQLEscapeString(sql, i)
				i++
			case sql[i] == '"':
				state = scanDoubleQuote
				i++
			case sql[i] == '`' && dialect != spec.DialectPostgreSQL:
				state = scanBacktick
				i++
			case startsLineComment(sql, i, dialect):
				state = scanLineComment
				i += 2
			case sql[i] == '#' && dialect != spec.DialectPostgreSQL:
				state = scanLineComment
				i++
			case i+1 < len(sql) && sql[i:i+2] == "/*":
				state, blockDepth = scanBlockComment, 1
				i += 2
			case dialect == spec.DialectPostgreSQL && sql[i] == '$':
				if delimiter := dollarQuoteDelimiter(sql, i); delimiter != "" {
					state, dollarQuote = scanDollarQuote, delimiter
					i += len(delimiter)
				} else {
					i++
				}
			default:
				i++
			}
		case scanSingleQuote:
			i = scanQuoted(sql, i, '\'', quoteBackslashEscapes, &state)
		case scanDoubleQuote:
			i = scanQuoted(sql, i, '"', dialect != spec.DialectPostgreSQL, &state)
		case scanBacktick:
			i = scanQuoted(sql, i, '`', true, &state)
		case scanLineComment:
			if sql[i] == '\n' {
				state = scanNormal
			}
			i++
		case scanBlockComment:
			switch {
			case dialect == spec.DialectPostgreSQL && i+1 < len(sql) && sql[i:i+2] == "/*":
				blockDepth++
				i += 2
			case i+1 < len(sql) && sql[i:i+2] == "*/":
				blockDepth--
				i += 2
				if blockDepth == 0 {
					state = scanNormal
				}
			default:
				i++
			}
		case scanDollarQuote:
			if strings.HasPrefix(sql[i:], dollarQuote) {
				state = scanNormal
				i += len(dollarQuote)
			} else {
				i++
			}
		}
	}
	return appendStatementChunk(chunks, sql, start, len(sql))
}

func isPostgreSQLEscapeString(sql string, quote int) bool {
	if quote == 0 || sql[quote-1] != 'E' && sql[quote-1] != 'e' {
		return false
	}
	if quote == 1 {
		return true
	}
	previous, _ := utf8.DecodeLastRuneInString(sql[:quote-1])
	return !isDollarTagPart(previous) && previous != '$'
}

func appendStatementChunk(chunks []statementChunk, source string, start, end int) []statementChunk {
	raw := source[start:end]
	trimmedLeft := strings.TrimLeftFunc(raw, unicode.IsSpace)
	statement := strings.TrimSpace(trimmedLeft)
	if statement == "" {
		return chunks
	}
	offset := start + len(raw) - len(trimmedLeft)
	line, column := lineColumnAt(source, offset)
	return append(chunks, statementChunk{SQL: statement, Line: line, Column: column})
}

func scanQuoted(sql string, i int, quote byte, backslashEscapes bool, state *int) int {
	if backslashEscapes && sql[i] == '\\' && i+1 < len(sql) {
		return i + 2
	}
	if sql[i] != quote {
		return i + 1
	}
	if i+1 < len(sql) && sql[i+1] == quote {
		return i + 2
	}
	*state = scanNormal
	return i + 1
}

func startsLineComment(sql string, i int, dialect spec.Dialect) bool {
	if i+1 >= len(sql) || sql[i:i+2] != "--" {
		return false
	}
	return dialect == spec.DialectPostgreSQL || i+2 == len(sql) || sql[i+2] <= ' '
}

func dollarQuoteDelimiter(sql string, start int) string {
	if start+1 >= len(sql) {
		return ""
	}
	if start > 0 {
		previous, _ := utf8.DecodeLastRuneInString(sql[:start])
		if isDollarTagPart(previous) || previous == '$' {
			return ""
		}
	}
	if sql[start+1] == '$' {
		return "$$"
	}
	first, width := utf8.DecodeRuneInString(sql[start+1:])
	if !isDollarTagStart(first) {
		return ""
	}
	for i := start + 1 + width; i < len(sql); {
		if sql[i] == '$' {
			return sql[start : i+1]
		}
		char, width := utf8.DecodeRuneInString(sql[i:])
		if !isDollarTagPart(char) {
			return ""
		}
		i += width
	}
	return ""
}

func isDollarTagStart(char rune) bool {
	return char == '_' || unicode.IsLetter(char)
}

func isDollarTagPart(char rune) bool {
	return isDollarTagStart(char) || char >= '0' && char <= '9'
}

func lineColumnAt(source string, offset int) (int, int) {
	line, column := 1, 1
	for i := 0; i < offset && i < len(source); i++ {
		if source[i] == '\n' {
			line, column = line+1, 1
		} else {
			column++
		}
	}
	return line, column
}
