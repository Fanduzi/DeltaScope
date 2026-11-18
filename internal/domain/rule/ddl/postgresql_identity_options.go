package ddl

import (
	"math"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// pgAlterIdentityOptions shares one options verdict across blockers and risks.
// Stable definitions never stand in for current issuance position.
func pgAlterIdentityOptions(a spec.Alter, c *spec.PGColumnState, targetReason, targetMissing string) (reason, missing string, cycle, restart bool) {
	f := a.PGColumnTransition
	if f == nil {
		return "", "column.generation_transition", false, false
	}
	if f.ModeCount > 1 {
		return "identity_modes_duplicate", "", false, false
	}
	unknown := f.Unmodeled || f.ModeCount < 0 || f.ModeCount == 0 && f.Mode != "" || f.ModeCount == 1 && f.Mode != "a" && f.Mode != "d"
	if a.Action == "add_identity" && f.ModeCount != 1 {
		unknown = true
	}
	if a.Action == "set_generated" && f.ModeCount == 0 && len(f.Options) == 0 {
		unknown = true
	}
	values := map[spec.SequenceOptionKind]int64{}
	seen := map[spec.SequenceOptionKind]bool{}
	for _, o := range f.Options {
		key := o.Kind
		valueless := false
		switch key {
		case spec.SequenceOptionNoMinValue:
			key = spec.SequenceOptionMinValue
			valueless = true
		case spec.SequenceOptionNoMaxValue:
			key = spec.SequenceOptionMaxValue
			valueless = true
		case spec.SequenceOptionNoCycle:
			key = spec.SequenceOptionCycle
			valueless = true
		case spec.SequenceOptionCycle:
			valueless = true
		case spec.SequenceOptionRestart:
			key = spec.SequenceOptionRestartWith
			valueless = true
		case spec.SequenceOptionIncrementBy, spec.SequenceOptionStartWith, spec.SequenceOptionMinValue, spec.SequenceOptionMaxValue, spec.SequenceOptionCache, spec.SequenceOptionRestartWith:
		default:
			unknown = true
			continue
		}
		if valueless != (o.Value == nil) {
			unknown = true
			continue
		}
		if seen[key] {
			return "identity_options_duplicate", "", false, false
		}
		seen[key] = true
		if o.Value != nil {
			values[o.Kind] = *o.Value
		}
		cycle = cycle || o.Kind == spec.SequenceOptionCycle
		restart = restart || key == spec.SequenceOptionRestartWith
	}
	if v, ok := values[spec.SequenceOptionIncrementBy]; ok && v == 0 {
		return "identity_increment_zero", "", false, false
	}
	if v, ok := values[spec.SequenceOptionCache]; ok && v < 1 {
		return "identity_cache_invalid", "", false, false
	}
	min, hasMin := values[spec.SequenceOptionMinValue]
	max, hasMax := values[spec.SequenceOptionMaxValue]
	if hasMin && hasMax && min >= max {
		return "identity_bounds_invalid", "", false, false
	}
	for _, k := range []spec.SequenceOptionKind{spec.SequenceOptionStartWith, spec.SequenceOptionRestartWith} {
		if v, ok := values[k]; ok && (hasMin && v < min || hasMax && v > max) {
			return "identity_start_or_restart_out_of_bounds", "", false, false
		}
	}
	if unknown {
		return "", "column.identity_options", false, false
	}
	if targetReason != "" {
		return "", "", false, false
	}
	if targetMissing != "" {
		return "", targetMissing, false, false
	}
	if a.Action == "add_identity" {
		// This is the authored new identity declaration, not a guessed post-state.
		decl := spec.Column{PGGeneration: &spec.PGColumnGeneration{IdentityCount: f.ModeCount, Mode: f.Mode, TypeWidth: c.TypeWidth, Options: f.Options}}
		r, m, cy := pgGenerationAssessment(decl)
		return r, m, cy, false
	}
	if len(f.Options) == 0 {
		return "", "", false, false
	}
	seq := c.IdentitySequence
	if seq == nil || !pgIdentityDefinitionKnown(seq) {
		return "", "column.identity_sequence_definition", false, false
	}
	d := seq.Definition
	increment, start := *d.Increment, *d.Start
	min, max = *d.MinValue, *d.MaxValue
	cache := *d.CacheValue
	if v, ok := values[spec.SequenceOptionIncrementBy]; ok {
		increment = v
	}
	typeMin, typeMax := pgIdentityIntegerBounds(seq.TypeWidth)
	for _, o := range f.Options {
		switch o.Kind {
		case spec.SequenceOptionMinValue:
			min = *o.Value
		case spec.SequenceOptionMaxValue:
			max = *o.Value
		case spec.SequenceOptionNoMinValue:
			min = 1
			if increment < 0 {
				min = typeMin
			}
		case spec.SequenceOptionNoMaxValue:
			max = typeMax
			if increment < 0 {
				max = -1
			}
		case spec.SequenceOptionStartWith:
			start = *o.Value
		case spec.SequenceOptionCache:
			cache = *o.Value
		}
	}
	if min < typeMin || max > typeMax || min >= max {
		return "identity_bounds_invalid", "", false, false
	}
	if start < min || start > max {
		return "identity_start_out_of_bounds", "", false, false
	}
	if cache < 1 || increment == 0 {
		return "", "column.identity_sequence_definition", false, false
	}
	if restart {
		value := start
		if v, ok := values[spec.SequenceOptionRestartWith]; ok {
			value = v
		}
		if value < min || value > max {
			return "identity_restart_out_of_bounds", "", false, false
		}
	} else if min > *d.MinValue || max < *d.MaxValue {
		// Narrowed bounds can reject an unread current value even when START fits.
		return "", "column.identity_sequence.current_value", false, false
	}
	return "", "", cycle, restart
}

func pgIdentityIntegerBounds(width int) (int64, int64) {
	if width == 64 {
		return math.MinInt64, math.MaxInt64
	}
	max := (int64(1) << (width - 1)) - 1
	return -max - 1, max
}

func pgIdentityDefinitionKnown(s *spec.PGIdentitySequenceState) bool {
	if s.TypeWidth != 16 && s.TypeWidth != 32 && s.TypeWidth != 64 {
		return false
	}
	d := s.Definition
	if d.Start == nil || d.Increment == nil || d.MinValue == nil || d.MaxValue == nil || d.CacheValue == nil || d.Cycle == nil {
		return false
	}
	min, max := pgIdentityIntegerBounds(s.TypeWidth)
	return *d.Increment != 0 && *d.CacheValue > 0 && *d.MinValue >= min && *d.MaxValue <= max && *d.MinValue < *d.MaxValue && *d.Start >= *d.MinValue && *d.Start <= *d.MaxValue
}
