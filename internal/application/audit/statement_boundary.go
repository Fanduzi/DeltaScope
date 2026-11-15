// Package audit orchestrates audit use cases at the application layer.
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

// delimChange records a client DELIMITER directive: at byte offset the
// statement terminator switches to delim. Directive lines are client
// framing, not database SQL (issue #106 T27).
type delimChange struct {
	offset int
	delim  string
}

func splitSQLStatements(sql string, dialect spec.Dialect) []statementChunk {
	chunks := make([]statementChunk, 0)
	// Client DELIMITER directives are blanked same-width before scanning
	// (byte offsets and line/column positions stay 1:1 with the authored
	// input); the terminator list they produce drives boundary matching.
	scanText, changes := blankDelimiterDirectives(sql, dialect)
	delim := ";"
	nextChange := 0

	state, start, blockDepth := scanNormal, 0, 0
	dollarQuote := ""
	quoteBackslashEscapes := false
	parenDepth := 0
	routineDepth := -1        // -1 = not inside a routine body
	endSuppressed := false    // an END's optional closing keyword pushes nothing
	memberStart := false      // next word begins a routine member — compound keywords only open there
	exprCase := 0             // open mid-member CASE expressions — their END pairs inside the member
	lastInitial := false      // previous word was member-initial — a ':' after it marks a label
	chunkHasRoutineHead := -1 // -1 unknown, 0 no, 1 yes
	isWordByte := func(c byte) bool {
		// '$' is deliberately not a word byte: no SQL keyword contains it,
		// and treating it as one would swallow a `$$` client terminator
		// into the preceding word (`END$$` → "end$$", never matching "end"
		// or the delimiter) (T27/#106).
		return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
	}
	isRoutineCreatePrefix := func(upto int) bool {
		if chunkHasRoutineHead >= 0 {
			return chunkHasRoutineHead == 1
		}
		// Decide once per chunk: the authored prefix up to this point is a
		// stored-routine CREATE when its first word is CREATE and it names
		// PROCEDURE or FUNCTION. Only meaningful at parenDepth 0 — inside
		// the parameter/option lists words like FUNCTION never arm it.
		// T29 adds TRIGGER and T30 EVENT under CREATE; ALTER arms only for
		// EVENT — ALTER EVENT ... DO BEGIN…END is the sole ALTER form whose
		// clause introduces a compound body (T30/#109).
		prefix := scanText[start:upto]
		word, words := "", 0
		firstWord, hasRoutineKind, hasEventKind := "", false, false
		for i := 0; i < len(prefix); {
			c := prefix[i]
			switch {
			case isWordByte(c):
				j := i
				for j < len(prefix) && isWordByte(prefix[j]) {
					j++
				}
				word = strings.ToLower(prefix[i:j])
				if words == 0 {
					firstWord = word
					if word != "create" && word != "alter" {
						chunkHasRoutineHead = 0
						return false
					}
				}
				words++
				if word == "procedure" || word == "function" || word == "trigger" {
					hasRoutineKind = true
				}
				if word == "event" {
					hasEventKind = true
				}
				i = j
			case c == '\'' || c == '"' || c == '`':
				// quoted text cannot contribute keywords
				q := c
				i++
				for i < len(prefix) && prefix[i] != q {
					i++
				}
				i++
			case c == '-' && i+1 < len(prefix) && prefix[i+1] == '-' || c == '#':
				for i < len(prefix) && prefix[i] != '\n' {
					i++
				}
			case c == '/' && i+1 < len(prefix) && prefix[i+1] == '*':
				end := strings.Index(prefix[i+2:], "*/")
				if end < 0 {
					i = len(prefix)
				} else {
					i += 2 + end + 2
				}
			default:
				i++
			}
		}
		if words > 0 && ((firstWord == "create" && (hasRoutineKind || hasEventKind)) || (firstWord == "alter" && hasEventKind)) {
			chunkHasRoutineHead = 1
		} else {
			chunkHasRoutineHead = 0
		}
		return chunkHasRoutineHead == 1
	}

	for i := 0; i < len(scanText); {
		if nextChange < len(changes) && i >= changes[nextChange].offset {
			delim = changes[nextChange].delim
			nextChange++
		}
		switch state {
		case scanNormal:
			switch {
			case delim == ";" && scanText[i] == ';' && routineDepth <= 0:
				chunks = appendStatementChunk(chunks, scanText, start, i+1)
				start = i + 1
				i++
				chunkHasRoutineHead = -1
				parenDepth = 0
				routineDepth = -1
				endSuppressed = false
				memberStart, exprCase, lastInitial = false, 0, false
			case delim != ";" && strings.HasPrefix(scanText[i:], delim):
				// Non-";" terminators are client framing (DELIMITER //, $$,
				// …): they bound the statement but are never part of the SQL
				// handed to the parser (T27/#106).
				chunks = appendStatementChunk(chunks, scanText, start, i)
				start = i + len(delim)
				i += len(delim)
				chunkHasRoutineHead = -1
				parenDepth = 0
				routineDepth = -1
				endSuppressed = false
				memberStart, exprCase, lastInitial = false, 0, false
			case routineDepth > 0 && scanText[i] == ';':
				// Member boundary inside a routine body — the next word
				// is member-initial and any open case-expression ends.
				memberStart, exprCase, lastInitial = true, 0, false
				i++
			case scanText[i] == '\'':
				state = scanSingleQuote
				quoteBackslashEscapes = dialect != spec.DialectPostgreSQL || isPostgreSQLEscapeString(scanText, i)
				i++
			case scanText[i] == '"':
				state = scanDoubleQuote
				i++
			case scanText[i] == '`' && dialect != spec.DialectPostgreSQL:
				state = scanBacktick
				i++
			case startsLineComment(scanText, i, dialect):
				state = scanLineComment
				i += 2
			case scanText[i] == '#' && dialect != spec.DialectPostgreSQL:
				state = scanLineComment
				i++
			case i+1 < len(scanText) && scanText[i:i+2] == "/*":
				state, blockDepth = scanBlockComment, 1
				i += 2
			case dialect == spec.DialectPostgreSQL && scanText[i] == '$':
				if delimiter := dollarQuoteDelimiter(scanText, i); delimiter != "" {
					state, dollarQuote = scanDollarQuote, delimiter
					i += len(delimiter)
				} else {
					i++
				}
			case routineDepth > 0 && scanText[i] == ':' && lastInitial:
				// Label prefix — the construct keyword after it still
				// sits in member-initial position (T28).
				memberStart, lastInitial = true, false
				i++
			case dialect != spec.DialectPostgreSQL && isWordByte(scanText[i]):
				j := i
				for j < len(scanText) && isWordByte(scanText[j]) {
					j++
				}
				word := strings.ToLower(scanText[i:j])
				if routineDepth > 0 {
					initial := memberStart
					memberStart = false
					switch {
					case word == "end" && exprCase > 0:
						exprCase--
					case word == "end":
						routineDepth--
						endSuppressed = true
					case endSuppressed:
						endSuppressed = false // END's optional closing keyword
					case initial && (word == "begin" || word == "if" || word == "case" || word == "loop" || word == "while" || word == "repeat"):
						routineDepth++
						if word == "begin" || word == "loop" || word == "repeat" {
							// These open the member list directly — the
							// word that follows sits member-initial too.
							memberStart = true
						}
					case word == "case" && !initial:
						exprCase++
					case (word == "then" || word == "else" || word == "do") && exprCase == 0:
						// Branch/loop connective — a member follows.
						memberStart = true
					}
					lastInitial = initial
				} else if word == "begin" && parenDepth == 0 && isRoutineCreatePrefix(i) {
					// The CREATE's compound body opens here; every BEGIN…END
					// and compound construct nests until the outer END —
					// semicolons inside are body separators, never outer
					// migration boundaries (T27/#106).
					routineDepth = 1
					memberStart = true // the body's first member follows
				}
				i = j
			case scanText[i] == '(':
				parenDepth++
				i++
			case scanText[i] == ')':
				if parenDepth > 0 {
					parenDepth--
				}
				i++
			default:
				i++
			}
		case scanSingleQuote:
			i = scanQuoted(scanText, i, '\'', quoteBackslashEscapes, &state)
		case scanDoubleQuote:
			i = scanQuoted(scanText, i, '"', dialect != spec.DialectPostgreSQL, &state)
		case scanBacktick:
			i = scanQuoted(scanText, i, '`', true, &state)
		case scanLineComment:
			if scanText[i] == '\n' {
				state = scanNormal
			}
			i++
		case scanBlockComment:
			switch {
			case dialect == spec.DialectPostgreSQL && i+1 < len(scanText) && scanText[i:i+2] == "/*":
				blockDepth++
				i += 2
			case i+1 < len(scanText) && scanText[i:i+2] == "*/":
				blockDepth--
				i += 2
				if blockDepth == 0 {
					state = scanNormal
				}
			default:
				i++
			}
		case scanDollarQuote:
			if strings.HasPrefix(scanText[i:], dollarQuote) {
				state = scanNormal
				i += len(dollarQuote)
			} else {
				i++
			}
		}
	}
	return appendStatementChunk(chunks, scanText, start, len(scanText))
}

// blankDelimiterDirectives blanks client `DELIMITER <d>` directive lines
// same-width (every non-newline byte becomes a space) and returns the
// rewritten scan text plus the ordered terminator changes. A directive
// is a line whose first non-whitespace word is DELIMITER — the exact
// client framing mysqldump-style exports emit; it is never database SQL.
func blankDelimiterDirectives(sql string, dialect spec.Dialect) (string, []delimChange) {
	if dialect == spec.DialectPostgreSQL || !strings.Contains(strings.ToLower(sql), "delimiter") {
		return sql, nil
	}
	var changes []delimChange
	blank := []byte(sql)
	lineStart := 0
	for lineStart < len(sql) {
		lineEnd := strings.IndexByte(sql[lineStart:], '\n')
		end := len(sql)
		if lineEnd >= 0 {
			end = lineStart + lineEnd
		}
		line := sql[lineStart:end]
		rest := strings.TrimLeft(line, " \t")
		if len(rest) > len("delimiter") && strings.EqualFold(rest[:len("delimiter")], "delimiter") && (rest[len("delimiter")] == ' ' || rest[len("delimiter")] == '\t') {
			d := strings.TrimSpace(rest[len("delimiter"):])
			// A directive line's payload is the first non-space token;
			// mysql accepts anything here — only a non-empty token is a
			// valid terminator change.
			if token := strings.Fields(d); len(token) > 0 {
				d = token[0]
				if d != "" {
					// Blank the directive line except its newline so byte
					// offsets and line numbers are preserved exactly.
					for i := lineStart; i < end; i++ {
						blank[i] = ' '
					}
					changes = append(changes, delimChange{offset: end, delim: d})
				}
			}
		}
		if lineEnd < 0 {
			break
		}
		lineStart = end + 1
	}
	if len(changes) == 0 {
		return sql, nil
	}
	return string(blank), changes
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
