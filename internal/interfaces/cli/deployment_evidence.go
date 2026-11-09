package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// deploymentEvidenceMaxBytes bounds the local evidence document — the
// file is a small JSON fact bundle, not a script; anything larger is an
// input error rather than a partial read.
const deploymentEvidenceMaxBytes = 64 << 10

// loadDeploymentEvidenceFile reads one bounded JSON evidence document
// and validates only its shape: the binding anchors (source, product,
// version) must be present, product must name mysql, and no unknown
// keys may appear. Semantic binding (exact version equality, target
// match, fill-only-unproven) stays with the shared
// spec.DeploymentEvidence contract downstream — the CLI never
// re-adjudicates the facts and never echoes file contents into output
// or errors (T24/#103 R6).
func loadDeploymentEvidenceFile(path string) (*spec.DeploymentEvidence, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("deployment evidence file is not readable")
	}
	if info.Size() > deploymentEvidenceMaxBytes {
		return nil, fmt.Errorf("deployment evidence file exceeds the %d-byte limit", deploymentEvidenceMaxBytes)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("deployment evidence file is not readable")
	}
	var evidence spec.DeploymentEvidence
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&evidence); err != nil {
		return nil, fmt.Errorf("deployment evidence file is not valid JSON")
	}
	if decoder.More() {
		return nil, fmt.Errorf("deployment evidence file carries trailing content")
	}
	if evidence.Source == "" || evidence.Product == "" || evidence.Version == "" {
		return nil, fmt.Errorf("deployment evidence file must carry source, product, and version")
	}
	if evidence.Product != "mysql" {
		return nil, fmt.Errorf("deployment evidence product must be mysql")
	}
	return &evidence, nil
}
