// Package cli exposes the command-line adapter for DeltaScope.
// input: config command invocations, YAML files, and built-in default policy metadata
// output: config lint results, stable default-config rendering, and wiring for rule config status
// pos: CLI config command group for policy inspection and validation
// note: if this file changes, update this header and module README.md.
package cli

import (
	"fmt"
	"os"
	"reflect"
	"sort"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

func newConfigCmd(options *cliOptions, exitCode *int) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Generate and validate DeltaScope config",
	}
	cmd.AddCommand(newConfigInitCmd(exitCode))
	cmd.AddCommand(newConfigLintCmd(exitCode))
	cmd.AddCommand(newConfigShowDefaultCmd(exitCode))
	cmd.AddCommand(newConfigStatusCmd(options, exitCode))
	return cmd
}

func newConfigLintCmd(exitCode *int) *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Validate a DeltaScope YAML config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePath == "" {
				*exitCode = exitUser
				return newUserError("config lint requires --file")
			}
			if err := lintConfigFile(filePath); err != nil {
				*exitCode = exitUser
				return err
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Config OK"); err != nil {
				*exitCode = exitInternal
				return err
			}
			*exitCode = exitOK
			return nil
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "path to the YAML config file to lint")
	return cmd
}

func newConfigShowDefaultCmd(exitCode *int) *cobra.Command {
	return &cobra.Command{
		Use:   "show-default",
		Short: "Print the built-in default config",
		RunE: func(cmd *cobra.Command, args []string) error {
			output, err := renderExampleConfig()
			if err != nil {
				*exitCode = exitInternal
				return err
			}
			if _, err := cmd.OutOrStdout().Write(output); err != nil {
				*exitCode = exitInternal
				return err
			}
			*exitCode = exitOK
			return nil
		},
	}
}

type rawConfigFile struct {
	Rules map[string]rawRuleConfig `yaml:"rules"`
}

type rawRuleConfig struct {
	Enabled *bool          `yaml:"enabled"`
	Level   string         `yaml:"level"`
	Params  map[string]any `yaml:"params"`
}

func lintConfigFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return newUserError(fmt.Sprintf("read config file: %v", err))
	}

	var raw rawConfigFile
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return newUserError(fmt.Sprintf("parse yaml: %v", err))
	}

	defaults := policy.Default()
	for ruleID, rawRule := range raw.Rules {
		defaultRule, ok := defaults.Rules[ruleID]
		if !ok {
			return newUserError(fmt.Sprintf("unknown rule %q", ruleID))
		}
		if rawRule.Level != "" && rawRule.Level != "blocker" && rawRule.Level != "warning" && rawRule.Level != "notice" {
			return newUserError(fmt.Sprintf("invalid level %q for rule %q", rawRule.Level, ruleID))
		}
		if err := lintRuleParams(ruleID, rawRule.Params, defaultRule.Params); err != nil {
			return err
		}
	}
	return nil
}

func lintRuleParams(ruleID string, rawParams map[string]any, defaultParams map[string]any) error {
	keys := make([]string, 0, len(rawParams))
	for key := range rawParams {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		rawValue := rawParams[key]
		defaultValue, ok := defaultParams[key]
		if !ok {
			return newUserError(fmt.Sprintf("unknown param %q for rule %q", key, ruleID))
		}
		if !paramTypeMatches(rawValue, defaultValue) {
			return newUserError(fmt.Sprintf("invalid type for %s.%s: got %T, want %T", ruleID, key, rawValue, defaultValue))
		}
	}
	return nil
}

func paramTypeMatches(rawValue any, defaultValue any) bool {
	switch defaultValue.(type) {
	case []string:
		items, ok := rawValue.([]any)
		if !ok {
			if typed, ok := rawValue.([]string); ok {
				return len(typed) >= 0
			}
			return false
		}
		for _, item := range items {
			if _, ok := item.(string); !ok {
				return false
			}
		}
		return true
	case int:
		switch rawValue.(type) {
		case int, int64, uint64:
			return true
		default:
			return false
		}
	case bool:
		_, ok := rawValue.(bool)
		return ok
	case string:
		_, ok := rawValue.(string)
		return ok
	default:
		return reflect.TypeOf(rawValue) == reflect.TypeOf(defaultValue)
	}
}
