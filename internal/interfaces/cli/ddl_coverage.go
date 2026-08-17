// Package cli exposes the command-line adapter for DeltaScope.
// input: ddl-coverage command flags, LoadEmbeddedCatalog, and optional LoadCatalogFile path override
// output: filtered catalog entries rendered as human-readable text or machine-readable JSON
// pos: CLI ddl-coverage command for querying the generated DDL coverage catalog
// note: if this file changes, update this header and module README.md.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/spf13/cobra"
)

type ddlCoverageJSONOutput struct {
	Version string                  `json:"version"`
	Summary ddlCoverageJSONSummary  `json:"summary"`
	Entries []appaudit.CatalogEntry `json:"entries"`
}

type ddlCoverageJSONSummary struct {
	Total    int               `json:"total"`
	Returned int               `json:"returned"`
	Filters  map[string]string `json:"filters,omitempty"`
}

func newDDLCoverageCmd(exitCode *int) *cobra.Command {
	var dialect, classification, guidanceCode, family, form, search, format string
	var limit int
	var catalogPath string

	cmd := &cobra.Command{
		Use:   "ddl-coverage",
		Short: "Query the DDL coverage catalog",
		Long:  "Query the generated DDL coverage catalog for verified DeltaScope entries.\nThe catalog is compiled into the binary and does not require a source checkout.\nDoes not execute audits, parse SQL, or call the audit service.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDDLCoverageFlags(format, limit); err != nil {
				*exitCode = exitUser
				return err
			}

			q := appaudit.CatalogQuery{
				Dialect:        dialect,
				Classification: classification,
				GuidanceCode:   guidanceCode,
				Family:         family,
				Form:           form,
				Search:         search,
				Limit:          limit,
			}
			if err := q.Validate(); err != nil {
				*exitCode = exitUser
				return err
			}

			version, entries, err := loadDDLCoverageCatalog(catalogPath)
			if err != nil {
				if catalogPath != "" {
					*exitCode = exitUser
					return newUserError(fmt.Sprintf("catalog unavailable: %v", err))
				}
				*exitCode = exitInternal
				return fmt.Errorf("load catalog: %w", err)
			}

			result := appaudit.QueryCatalog(entries, q)

			var output string
			if format == "json" {
				output = renderDDLCoverageJSON(version, result, q)
			} else {
				output = renderDDLCoverageText(result)
			}

			if _, err := fmt.Fprint(cmd.OutOrStdout(), output); err != nil {
				*exitCode = exitInternal
				return err
			}
			*exitCode = exitOK
			return nil
		},
	}

	cmd.Flags().StringVar(&dialect, "dialect", "", "filter by dialect: mysql, tidb, postgresql")
	cmd.Flags().StringVar(&classification, "classification", "", "filter by classification: finding_covered, normalized_silent, unsupported_boundary, parser_error, unclassified")
	cmd.Flags().StringVar(&guidanceCode, "guidance-code", "", "filter by guidance code: parser_upgrade_candidate")
	cmd.Flags().StringVar(&family, "family", "", "filter by family (case-insensitive substring)")
	cmd.Flags().StringVar(&form, "form", "", "filter by form (case-insensitive substring)")
	cmd.Flags().StringVar(&search, "search", "", "search across fields (case-insensitive substring)")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	cmd.Flags().IntVar(&limit, "limit", 0, "limit result count; 0 means no limit")
	cmd.Flags().StringVar(&catalogPath, "catalog", "", "path to catalog JSON file; default is the catalog compiled into the binary")
	_ = cmd.Flags().MarkHidden("catalog")

	return cmd
}

func validateDDLCoverageFlags(format string, limit int) error {
	if format != "text" && format != "json" {
		return newUserError(fmt.Sprintf("invalid format %q: must be text or json", format))
	}
	if limit < 0 {
		return newUserError(fmt.Sprintf("invalid limit %d: must be 0 or positive", limit))
	}
	return nil
}

func loadDDLCoverageCatalog(path string) (string, []appaudit.CatalogEntry, error) {
	if path == "" {
		return appaudit.LoadEmbeddedCatalog()
	}
	return appaudit.LoadCatalogFile(path)
}

func renderDDLCoverageJSON(version string, result appaudit.CatalogResult, q appaudit.CatalogQuery) string {
	filters := make(map[string]string)
	if q.Dialect != "" {
		filters["dialect"] = q.Dialect
	}
	if q.Classification != "" {
		filters["classification"] = q.Classification
	}
	if q.GuidanceCode != "" {
		filters["guidance_code"] = q.GuidanceCode
	}
	if q.Family != "" {
		filters["family"] = q.Family
	}
	if q.Form != "" {
		filters["form"] = q.Form
	}
	if q.Search != "" {
		filters["search"] = q.Search
	}
	if q.Limit > 0 {
		filters["limit"] = fmt.Sprintf("%d", q.Limit)
	}

	out := ddlCoverageJSONOutput{
		Version: version,
		Summary: ddlCoverageJSONSummary{
			Total:    result.Total,
			Returned: len(result.Entries),
			Filters:  filters,
		},
		Entries: result.Entries,
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "marshal failed: %v"}`, err)
	}
	return string(data) + "\n"
}

func renderDDLCoverageText(result appaudit.CatalogResult) string {
	var b strings.Builder

	headers := []string{"DIALECT", "CLASSIFICATION", "FAMILY", "FORM", "GUIDANCE"}
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}

	rows := make([][]string, len(result.Entries))
	for i, e := range result.Entries {
		guidance := e.GuidanceCode
		if guidance == "" {
			guidance = "-"
		}
		row := []string{e.Dialect, e.Classification, e.Family, e.Form, guidance}
		rows[i] = row
		for j, v := range row {
			if len(v) > widths[j] {
				widths[j] = len(v)
			}
		}
	}

	writeRow := func(cols []string) {
		for i, col := range cols {
			if i > 0 {
				b.WriteString("  ")
			}
			fmt.Fprintf(&b, "%-*s", widths[i], col)
		}
		b.WriteByte('\n')
	}

	writeSep := func() {
		for i, w := range widths {
			if i > 0 {
				b.WriteString("  ")
			}
			b.WriteString(strings.Repeat("-", w))
		}
		b.WriteByte('\n')
	}

	writeRow(headers)
	writeSep()
	for _, row := range rows {
		writeRow(row)
	}

	if len(result.Entries) == 0 {
		b.WriteString("\nNo entries matched.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "\n%d entries", len(result.Entries))
	if result.Total > len(result.Entries) {
		fmt.Fprintf(&b, " (%d total before limit)", result.Total)
	}
	b.WriteByte('\n')

	return b.String()
}
