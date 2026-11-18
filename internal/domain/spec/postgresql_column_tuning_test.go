package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPGColumnTuningIsPrivate(t *testing.T) {
	n := int64(10001)
	value := -0.5
	a := Alter{Action: "set_statistics", PGColumnTuning: &PGColumnTuning{StatisticsValue: &n, Options: []PGColumnOption{{Name: "private_option_name", ValueKind: PGColumnOptionNumeric, Numeric: &value}}}}
	raw, err := json.Marshal(a)
	if err != nil || strings.Contains(string(raw), "10001") || strings.Contains(string(raw), "private_option_name") || strings.Contains(string(raw), "PGColumnTuning") {
		t.Fatalf("private facts leaked: %s %v", raw, err)
	}
}
