// Package mcpapi exposes the MCP adapter for DeltaScope.
// input: MCP success payload types that need explicit output-schema publication
// output: resolved JSON Schema objects for official DeltaScope MCP tool outputs
// pos: schema publication helpers for MCP tool registration metadata
// note: if this file changes, update this header and module README.md.
package mcpapi

import "github.com/google/jsonschema-go/jsonschema"

var (
	auditSQLResultSchema     = mustOutputSchema[AuditSQLResult]()
	describeRuleOutputSchema = mustOutputSchema[ruleDescription]()
	listRulesOutputSchema    = mustOutputSchema[listRulesResponse]()
	capabilitiesOutputSchema = mustOutputSchema[capabilitiesResponse]()
)

func mustOutputSchema[T any]() *jsonschema.Schema {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		panic(err)
	}
	return schema
}
