// Package configlint derives rule-level replacement warnings for a DeltaScope
// YAML config file without changing validation, audit, or policy behavior.
// input: a config file path and built-in default policy metadata
// output: deterministic replacement-hazard warnings for mentioned rules
// pos: application use case for config lint warnings, below the CLI surface
// note: if this file changes, update this header and module README.md.
package configlint

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Request selects one YAML config file to lint.
type Request struct {
	Path string
}

// Result is the lint outcome. Warnings are advisory and never include a
// "severity" field; level remains the public priority field.
type Result struct {
	Warnings []Warning `json:"warnings"`
}

// Warning describes one rule-level replacement hazard found in the config.
type Warning struct {
	RuleID  string `json:"rule_id"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Inspect loads a YAML config file, runs the same semantic validation the CLI
// `config lint` command performs, and then derives deterministic replacement
// warnings for mentioned rules. Validation errors (unknown rule, invalid level,
// unknown param, param type mismatch, malformed YAML, missing/unreadable file,
// empty path) are returned as errors and take precedence over warnings.
//
// It does not load policy through LoadPolicy, run an audit, parse SQL, touch a
// database, or mutate the default or any parsed policy map. ctx is reserved for
// future cancellation and is currently unused.
func Inspect(ctx context.Context, req Request) (Result, error) {
	_ = ctx

	if strings.TrimSpace(req.Path) == "" {
		return Result{}, errors.New("config path must not be empty")
	}

	content, err := os.ReadFile(req.Path)
	if err != nil {
		return Result{}, fmt.Errorf("read config file %q: %w", req.Path, err)
	}
	raw, err := parseRaw(content)
	if err != nil {
		return Result{}, fmt.Errorf("parse yaml: %w", err)
	}
	if err := validateRaw(raw); err != nil {
		return Result{}, err
	}

	return Result{Warnings: deriveWarnings(raw)}, nil
}
