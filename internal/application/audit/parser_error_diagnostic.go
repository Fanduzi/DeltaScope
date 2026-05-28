package audit

import "errors"

// errParserUnsupported is returned when the selected dialect parser cannot parse
// the statement. It communicates that no audit was performed and no findings
// were inferred from the unparsed SQL, without exposing the raw parser error
// or any SQL payload.
var errParserUnsupported = errors.New(
	"statement was not audited because the selected dialect parser could not parse it; no audit findings were inferred",
)
