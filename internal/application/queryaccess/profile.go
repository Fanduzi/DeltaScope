// Package queryaccess defines the closed internal query access profile contract.
package queryaccess

import "errors"

// AnalysisProfile identifies a closed engine/version compatibility target.
type AnalysisProfile string

const (
	AnalysisProfileEmpty   AnalysisProfile = ""
	AnalysisProfileMySQL57 AnalysisProfile = "mysql-5.7"
	AnalysisProfileMySQL80 AnalysisProfile = "mysql-8.0"
	AnalysisProfileMySQL84 AnalysisProfile = "mysql-8.4"
	AnalysisProfileTiDB85  AnalysisProfile = "tidb-8.5"
)

var (
	ErrInvalidAnalysisProfile         = errors.New("invalid query access analysis profile")
	ErrAnalysisProfileDialectMismatch = errors.New("query access analysis profile does not match dialect")
)

// ValidateAnalysisProfile checks the closed profile set and dialect ownership.
func ValidateAnalysisProfile(profile AnalysisProfile, dialect string) error {
	switch profile {
	case AnalysisProfileEmpty:
		return nil
	case AnalysisProfileMySQL57, AnalysisProfileMySQL80, AnalysisProfileMySQL84:
		if dialect != "mysql" {
			return ErrAnalysisProfileDialectMismatch
		}
		return nil
	case AnalysisProfileTiDB85:
		if dialect != "tidb" {
			return ErrAnalysisProfileDialectMismatch
		}
		return nil
	default:
		return ErrInvalidAnalysisProfile
	}
}
