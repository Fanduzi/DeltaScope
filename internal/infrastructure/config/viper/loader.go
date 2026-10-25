// Package viperconfig loads YAML policy files through Viper.
package viperconfig

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

// LoadPolicy returns the default policy or applies file-based overrides when a YAML path is provided.
func LoadPolicy(path string) (policy.Policy, error) {
	cfg := policy.Default()
	if path == "" {
		return cfg, nil
	}

	v := viper.NewWithOptions(viper.KeyDelimiter("::"))
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		return policy.Policy{}, fmt.Errorf("read policy config %q: %w", path, err)
	}

	if err := v.Unmarshal(&cfg, func(dc *mapstructure.DecoderConfig) {
		dc.TagName = "yaml"
		dc.ErrorUnused = true
	}); err != nil {
		return policy.Policy{}, fmt.Errorf("unmarshal policy config: %w", err)
	}

	return cfg, nil
}
