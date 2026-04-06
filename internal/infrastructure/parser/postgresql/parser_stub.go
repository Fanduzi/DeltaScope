//go:build !postgresql

package postgresql

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type Parser struct{}

type ExtractedStatement struct {
	Kind      spec.Kind
	RawSQL    string
	Extractor spec.StatementExtractor
}

type Result struct {
	Statements []ExtractedStatement
	Warnings   []string
}

func New() *Parser { return &Parser{} }

func (p *Parser) Parse(_ string) (Result, error) {
	return Result{}, fmt.Errorf("PostgreSQL support requires the PG-capable build; rebuild with -tags postgresql")
}
