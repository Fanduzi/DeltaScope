// Package policy loads audit policy for application use cases.
package policy

import (
	domainpolicy "github.com/Fanduzi/DeltaScope/internal/domain/policy"
	viperconfig "github.com/Fanduzi/DeltaScope/internal/infrastructure/config/viper"
)

// Load returns the effective policy using defaults plus any optional file override.
func Load(path string) (domainpolicy.Policy, error) {
	return viperconfig.LoadPolicy(path)
}
