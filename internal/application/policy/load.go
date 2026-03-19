// Package policy loads audit policy for application use cases.
// input: optional config file paths and infrastructure config loaders
// output: application-facing policy loading entrypoint
// pos: application wrapper over infrastructure-backed policy loading
// note: if this file changes, update this header and module README.md.
package policy

import (
	domainpolicy "github.com/Fanduzi/DeltaScope/internal/domain/policy"
	viperconfig "github.com/Fanduzi/DeltaScope/internal/infrastructure/config/viper"
)

// Load returns the effective policy using defaults plus any optional file override.
func Load(path string) (domainpolicy.Policy, error) {
	return viperconfig.LoadPolicy(path)
}
