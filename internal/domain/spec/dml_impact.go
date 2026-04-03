// Package spec defines normalized statement specifications for rule evaluation.
// input: parser-neutral DML impact estimation facts attached during audit orchestration
// output: shared impact contract reused across domain, report, and public API layers
// pos: domain DML impact model under the unified Statement spec
// note: if this file changes, update this header and module README.md.
package spec

// ImpactSource identifies where a DML impact estimate came from.
type ImpactSource string

const (
	ImpactSourceShape    ImpactSource = "shape"
	ImpactSourceMetadata ImpactSource = "metadata"
	ImpactSourcePlan     ImpactSource = "plan"
)

// ImpactRisk identifies the conservative risk bucket for a DML statement.
type ImpactRisk string

const (
	ImpactRiskLow     ImpactRisk = "low"
	ImpactRiskMedium  ImpactRisk = "medium"
	ImpactRiskHigh    ImpactRisk = "high"
	ImpactRiskUnknown ImpactRisk = "unknown"
)

// ImpactConfidence identifies how reliable the estimate source is.
type ImpactConfidence string

const (
	ImpactConfidenceLow    ImpactConfidence = "low"
	ImpactConfidenceMedium ImpactConfidence = "medium"
	ImpactConfidenceHigh   ImpactConfidence = "high"
)

// PredicateShape identifies the normalized WHERE-clause or join pattern for DML.
type PredicateShape string

const (
	PredicateShapeUnknown               PredicateShape = "unknown"
	PredicateShapeMissingWhere          PredicateShape = "missing_where"
	PredicateShapeUniqueEquality        PredicateShape = "unique_equality"
	PredicateShapeIndexedPrefixEquality PredicateShape = "indexed_prefix_equality"
	PredicateShapeIndexedRange          PredicateShape = "indexed_range"
	PredicateShapeJoin                  PredicateShape = "join"
	PredicateShapeNonSargable           PredicateShape = "non_sargable"
	PredicateShapeSubquery              PredicateShape = "subquery"
)

// ImpactEstimate stores the conservative DML impact estimate attached to a statement.
type ImpactEstimate struct {
	EstimatedRows  *int64           `json:"estimated_rows,omitempty"`
	EstimatedRatio *float64         `json:"estimated_ratio,omitempty"`
	RiskLevel      ImpactRisk       `json:"risk_level,omitempty"`
	Confidence     ImpactConfidence `json:"confidence,omitempty"`
	Source         ImpactSource     `json:"source,omitempty"`
	ReasonCodes    []string         `json:"reason_codes,omitempty"`
	Notes          []string         `json:"notes,omitempty"`
}
