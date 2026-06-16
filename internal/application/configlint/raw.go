package configlint

import (
	"go.yaml.in/yaml/v3"
)

// rawConfigFile mirrors the on-disk YAML closely enough to detect which fields a
// user wrote explicitly. Pointers distinguish "absent" from "empty/zero" so the
// warning layer can report omission rather than a deliberate empty value. This
// shape intentionally matches internal/application/configstatus so the two stay
// in lockstep; see the package README for the deferred consolidation note.
type rawConfigFile struct {
	Rules map[string]rawRuleConfig `yaml:"rules"`
}

type rawRuleConfig struct {
	Enabled *bool          `yaml:"enabled"`
	Level   *string        `yaml:"level"`
	Params  map[string]any `yaml:"params"`
}

// parseRaw unmarshals config bytes into a rawConfigFile. A nil rules map is
// normalized to an empty map so callers can range over it safely.
func parseRaw(content []byte) (rawConfigFile, error) {
	var raw rawConfigFile
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return rawConfigFile{}, err
	}
	if raw.Rules == nil {
		raw.Rules = map[string]rawRuleConfig{}
	}
	return raw, nil
}
