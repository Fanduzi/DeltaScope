// Package audit orchestrates audit use cases at the application layer.
// input: audit requests carrying SQL text, dialect, and optional policy override paths
// output: end-to-end audit results assembled from policy loading, parsing, extraction, and rule evaluation
// pos: application service entrypoint for the core SQL audit use case
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	apppolicy "github.com/Fanduzi/DeltaScope/internal/application/policy"
	domainpolicy "github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	ddlrules "github.com/Fanduzi/DeltaScope/internal/domain/rule/ddl"
	dmlrules "github.com/Fanduzi/DeltaScope/internal/domain/rule/dml"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

var (
	// ErrEmptySQL indicates the request did not include auditable SQL text.
	ErrEmptySQL = errors.New("audit SQL must not be empty")
	// ErrUnknownDialect indicates the request did not specify a supported dialect.
	ErrUnknownDialect = errors.New("audit dialect must be mysql or tidb")
)

// Request describes one application-level audit invocation.
type Request struct {
	SQL        string
	Dialect    spec.Dialect
	ConfigPath string
}

// Service coordinates the full audit use case.
type Service struct{}

// NewService returns a ready-to-use audit service.
func NewService() Service {
	return Service{}
}

// AuditSQL is the convenience application entrypoint used by outer adapters.
func AuditSQL(ctx context.Context, request Request) (report.Result, error) {
	return NewService().Audit(ctx, request)
}

// Audit executes the full offline SQL audit flow.
func (s Service) Audit(ctx context.Context, request Request) (report.Result, error) {
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}
	if strings.TrimSpace(request.SQL) == "" {
		return report.Result{}, ErrEmptySQL
	}
	if request.Dialect != spec.DialectMySQL && request.Dialect != spec.DialectTiDB {
		return report.Result{}, ErrUnknownDialect
	}

	policyCfg, err := apppolicy.Load(request.ConfigPath)
	if err != nil {
		return report.Result{}, fmt.Errorf("load policy: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}

	parsed, err := Parse(request.SQL, request.Dialect)
	if err != nil {
		return report.Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}

	statements, err := Extract(parsed)
	if err != nil {
		return report.Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}

	registry, err := buildRegistry(policyCfg)
	if err != nil {
		return report.Result{}, err
	}

	return EvaluateStatements(registry, statements)
}

func buildRegistry(cfg domainpolicy.Policy) (*rule.Registry, error) {
	registry := rule.NewRegistry()
	if err := ddlrules.Register(registry, cfg); err != nil {
		return nil, fmt.Errorf("register ddl rules: %w", err)
	}
	if err := dmlrules.Register(registry, cfg); err != nil {
		return nil, fmt.Errorf("register dml rules: %w", err)
	}
	return registry, nil
}
