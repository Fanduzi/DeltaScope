package spec

import (
	"encoding/json"
	"testing"
)

func TestMetadataStatusConstants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status MetadataStatus
		want   string
	}{
		{MetadataStatusConfirmed, "confirmed"},
		{MetadataStatusNotFound, "not_found"},
		{MetadataStatusUnavailable, "unavailable"},
		{MetadataStatusAmbiguous, "ambiguous"},
	}
	for _, tc := range cases {
		if string(tc.status) != tc.want {
			t.Errorf("expected %q, got %q", tc.want, tc.status)
		}
	}
}

func TestObjectSnapshotStatusHelpers(t *testing.T) {
	t.Parallel()
	confirmed := ObjectSnapshot{Status: MetadataStatusConfirmed}
	if !confirmed.IsConfirmed() {
		t.Fatal("expected IsConfirmed true for confirmed status")
	}
	if confirmed.IsNotFound() || confirmed.IsUnavailable() || confirmed.IsAmbiguous() {
		t.Fatal("expected only IsConfirmed true for confirmed status")
	}

	notFound := ObjectSnapshot{Status: MetadataStatusNotFound}
	if !notFound.IsNotFound() {
		t.Fatal("expected IsNotFound true for not_found status")
	}

	unavailable := ObjectSnapshot{Status: MetadataStatusUnavailable}
	if !unavailable.IsUnavailable() {
		t.Fatal("expected IsUnavailable true for unavailable status")
	}

	ambiguous := ObjectSnapshot{Status: MetadataStatusAmbiguous}
	if !ambiguous.IsAmbiguous() {
		t.Fatal("expected IsAmbiguous true for ambiguous status")
	}
}

func TestObjectSnapshotZeroValueIsUnavailable(t *testing.T) {
	t.Parallel()
	var zero ObjectSnapshot
	if !zero.IsUnavailable() {
		t.Fatal("expected zero-value ObjectSnapshot to be unavailable")
	}
	if zero.IsConfirmed() || zero.IsNotFound() || zero.IsAmbiguous() {
		t.Fatal("expected zero-value ObjectSnapshot to be unavailable only")
	}
}

func TestObjectSnapshotRoundTrip(t *testing.T) {
	t.Parallel()
	snapshot := ObjectSnapshot{
		Schema: "public",
		Type:   "extension",
		Name:   "pg_trgm",
		Status: MetadataStatusConfirmed,
		Exists: true,
		Attributes: map[string]string{
			"extension_version": "1.6",
			"schema":            "public",
		},
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var rt ObjectSnapshot
	if err := json.Unmarshal(data, &rt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if rt.Schema != "public" {
		t.Fatalf("expected schema public, got %q", rt.Schema)
	}
	if rt.Type != "extension" {
		t.Fatalf("expected type extension, got %q", rt.Type)
	}
	if rt.Name != "pg_trgm" {
		t.Fatalf("expected name pg_trgm, got %q", rt.Name)
	}
	if rt.Status != MetadataStatusConfirmed {
		t.Fatalf("expected status confirmed, got %q", rt.Status)
	}
	if !rt.Exists {
		t.Fatal("expected exists true")
	}
	if rt.Attributes["extension_version"] != "1.6" {
		t.Fatalf("expected extension_version 1.6, got %q", rt.Attributes["extension_version"])
	}
}

func TestObjectSnapshotOmitsEmptyFields(t *testing.T) {
	t.Parallel()
	snapshot := ObjectSnapshot{
		Type:   "publication",
		Name:   "pub_all",
		Status: MetadataStatusNotFound,
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if _, ok := payload["schema"]; ok {
		t.Fatal("expected schema to be omitted when empty")
	}
	if _, ok := payload["attributes"]; ok {
		t.Fatal("expected attributes to be omitted when nil")
	}
	if _, ok := payload["ambiguous_candidates"]; ok {
		t.Fatal("expected ambiguous_candidates to be omitted when nil")
	}
	if got := payload["exists"]; got != false {
		t.Fatalf("expected exists to serialize as false, got %#v", got)
	}
}

func TestObjectSnapshotAmbiguousWithCandidates(t *testing.T) {
	t.Parallel()
	snapshot := ObjectSnapshot{
		Type:                "type",
		Name:                "address",
		Status:              MetadataStatusAmbiguous,
		AmbiguousCandidates: []string{"public.address", "app.address"},
	}

	if !snapshot.IsAmbiguous() {
		t.Fatal("expected IsAmbiguous true")
	}
	if len(snapshot.AmbiguousCandidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(snapshot.AmbiguousCandidates))
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var rt ObjectSnapshot
	if err := json.Unmarshal(data, &rt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(rt.AmbiguousCandidates) != 2 {
		t.Fatalf("expected candidates to round-trip, got %d", len(rt.AmbiguousCandidates))
	}
}

func TestSafeAttributesBlocksSensitiveKeys(t *testing.T) {
	t.Parallel()
	snapshot := ObjectSnapshot{
		Status: MetadataStatusConfirmed,
		Exists: true,
		Attributes: map[string]string{
			"extension_version": "1.6",
			"schema":            "public",
			"enabled":           "true",
			"password":          "should_not_appear",
			"secret":            "should_not_appear",
			"token":             "should_not_appear",
			"api_key":           "should_not_appear",
			"connection":        "should_not_appear",
			"dsn":               "should_not_appear",
			"connstr":           "should_not_appear",
			"body":              "should_not_appear",
			"definition":        "should_not_appear",
			"comment":           "should_not_appear",
			"label":             "should_not_appear",
			"query":             "should_not_appear",
			"action_sql":        "should_not_appear",
			"options":           "should_not_appear",
		},
	}

	safe := snapshot.SafeAttributes()
	sensitive := []string{
		"password", "secret", "token", "api_key",
		"connection", "dsn", "connstr",
		"body", "definition", "comment", "label",
		"query", "action_sql", "options",
	}
	for _, key := range sensitive {
		if _, ok := safe[key]; ok {
			t.Errorf("expected sensitive key %q to be filtered out", key)
		}
	}

	expected := []string{"extension_version", "schema", "enabled"}
	for _, key := range expected {
		if _, ok := safe[key]; !ok {
			t.Errorf("expected safe key %q to be present", key)
		}
	}
}

func TestSafeAttributesNilAttributes(t *testing.T) {
	t.Parallel()
	snapshot := ObjectSnapshot{Status: MetadataStatusUnavailable}
	if safe := snapshot.SafeAttributes(); safe != nil {
		t.Fatalf("expected nil for nil attributes, got %#v", safe)
	}
}

func TestSafeAttributesEmptyMap(t *testing.T) {
	t.Parallel()
	snapshot := ObjectSnapshot{Attributes: map[string]string{}}
	safe := snapshot.SafeAttributes()
	if len(safe) != 0 {
		t.Fatalf("expected empty map, got %#v", safe)
	}
}

func TestFindObjectByTypeAndName(t *testing.T) {
	t.Parallel()
	meta := &Metadata{
		Objects: []ObjectSnapshot{
			{Type: "extension", Name: "pg_trgm", Status: MetadataStatusConfirmed, Exists: true},
			{Type: "publication", Name: "pub_all", Status: MetadataStatusNotFound},
			{Type: "extension", Name: "pgcrypto", Status: MetadataStatusUnavailable},
		},
	}

	found := meta.FindObject("extension", "pg_trgm")
	if found == nil {
		t.Fatal("expected to find pg_trgm extension")
	}
	if !found.IsConfirmed() {
		t.Fatal("expected confirmed status")
	}

	found = meta.FindObject("EXTENSION", "PG_TRGM")
	if found == nil {
		t.Fatal("expected case-insensitive lookup to find pg_trgm")
	}

	found = meta.FindObject("publication", "pub_all")
	if found == nil {
		t.Fatal("expected to find pub_all publication")
	}
	if !found.IsNotFound() {
		t.Fatal("expected not_found status")
	}

	found = meta.FindObject("extension", "pgcrypto")
	if found == nil {
		t.Fatal("expected to find pgcrypto extension")
	}
	if !found.IsUnavailable() {
		t.Fatal("expected unavailable status")
	}

	found = meta.FindObject("event_trigger", "trg_missing")
	if found != nil {
		t.Fatal("expected nil for missing object type")
	}
}

func TestFindObjectEdgeCases(t *testing.T) {
	t.Parallel()
	var nilMeta *Metadata
	if found := nilMeta.FindObject("extension", "pg_trgm"); found != nil {
		t.Fatal("expected nil for nil metadata")
	}

	emptyMeta := &Metadata{}
	if found := emptyMeta.FindObject("extension", "pg_trgm"); found != nil {
		t.Fatal("expected nil for empty objects")
	}

	meta := &Metadata{Objects: []ObjectSnapshot{{Type: "extension", Name: "pg_trgm"}}}
	if found := meta.FindObject("", "pg_trgm"); found != nil {
		t.Fatal("expected nil for empty type")
	}
	if found := meta.FindObject("extension", ""); found != nil {
		t.Fatal("expected nil for empty name")
	}
	if found := meta.FindObject("  ", "  "); found != nil {
		t.Fatal("expected nil for whitespace-only input")
	}
}

func TestFindObjectsByType(t *testing.T) {
	t.Parallel()
	meta := &Metadata{
		Objects: []ObjectSnapshot{
			{Type: "extension", Name: "pg_trgm", Status: MetadataStatusConfirmed},
			{Type: "extension", Name: "pgcrypto", Status: MetadataStatusUnavailable},
			{Type: "publication", Name: "pub_all", Status: MetadataStatusNotFound},
		},
	}

	exts := meta.FindObjectsByType("extension")
	if len(exts) != 2 {
		t.Fatalf("expected 2 extensions, got %d", len(exts))
	}

	pubs := meta.FindObjectsByType("publication")
	if len(pubs) != 1 {
		t.Fatalf("expected 1 publication, got %d", len(pubs))
	}

	triggers := meta.FindObjectsByType("event_trigger")
	if triggers != nil {
		t.Fatalf("expected nil for missing type, got %#v", triggers)
	}

	var nilMeta *Metadata
	if result := nilMeta.FindObjectsByType("extension"); result != nil {
		t.Fatal("expected nil for nil metadata")
	}
}

func TestMetadataObjectsFieldRoundTrip(t *testing.T) {
	t.Parallel()
	meta := &Metadata{
		Schema:   "public",
		Instance: &InstanceFacts{Version: "PostgreSQL 16.0"},
		TargetTable: &TableSnapshot{
			Exists: true,
			Table:  &Table{Name: "users"},
		},
		Objects: []ObjectSnapshot{
			{
				Type:   "extension",
				Name:   "pg_trgm",
				Status: MetadataStatusConfirmed,
				Exists: true,
				Attributes: map[string]string{
					"extension_version": "1.6",
					"schema":            "public",
				},
			},
			{
				Type:   "event_trigger",
				Name:   "trg_ddl",
				Status: MetadataStatusNotFound,
			},
		},
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var rt Metadata
	if err := json.Unmarshal(data, &rt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if rt.Schema != "public" {
		t.Fatalf("expected schema public, got %q", rt.Schema)
	}
	if rt.TargetTable == nil || !rt.TargetTable.Exists {
		t.Fatal("expected target table to survive round-trip")
	}
	if len(rt.Objects) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(rt.Objects))
	}
	if rt.Objects[0].Type != "extension" || rt.Objects[0].Name != "pg_trgm" {
		t.Fatalf("expected first object pg_trgm extension, got %+v", rt.Objects[0])
	}
	if rt.Objects[0].Status != MetadataStatusConfirmed {
		t.Fatalf("expected confirmed status, got %q", rt.Objects[0].Status)
	}
	if rt.Objects[1].Type != "event_trigger" || rt.Objects[1].Status != MetadataStatusNotFound {
		t.Fatalf("expected second object event_trigger not_found, got %+v", rt.Objects[1])
	}
}

func TestMetadataObjectsOittedWhenNil(t *testing.T) {
	t.Parallel()
	meta := &Metadata{Schema: "public"}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := payload["objects"]; ok {
		t.Fatal("expected objects to be omitted when nil")
	}
}

// TestPGObjectTypes covers all PostgreSQL object families from v0.90.0 scope
// with table-driven cases that verify the model can represent each family.
func TestPGObjectTypes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		objectType string
		name       string
		status     MetadataStatus
		attrs      map[string]string
	}{
		{"type", "address", MetadataStatusConfirmed, map[string]string{"type_kind": "composite"}},
		{"type", "color", MetadataStatusConfirmed, map[string]string{"type_kind": "enum"}},
		{"domain", "email", MetadataStatusConfirmed, map[string]string{"base_type": "text"}},
		{"extension", "pg_trgm", MetadataStatusConfirmed, map[string]string{"extension_version": "1.6"}},
		{"extension", "missing_ext", MetadataStatusNotFound, nil},
		{"publication", "pub_all", MetadataStatusConfirmed, nil},
		{"subscription", "sub", MetadataStatusConfirmed, map[string]string{"enabled": "true"}},
		{"subscription", "sub", MetadataStatusConfirmed, map[string]string{"enabled": "false"}},
		{"foreign_table", "ft_users", MetadataStatusConfirmed, map[string]string{"server": "srv"}},
		{"foreign_server", "srv", MetadataStatusConfirmed, map[string]string{"foreign_data_wrapper": "postgres_fdw"}},
		{"user_mapping", "app@srv", MetadataStatusConfirmed, map[string]string{"server": "srv"}},
		{"foreign_data_wrapper", "fdw", MetadataStatusConfirmed, nil},
		{"event_trigger", "trg_ddl", MetadataStatusConfirmed, map[string]string{"enabled": "true"}},
		{"event_trigger", "trg_ddl", MetadataStatusConfirmed, map[string]string{"enabled": "false"}},
		{"rule", "users_insert", MetadataStatusConfirmed, map[string]string{"table": "users"}},
		{"comment", "users", MetadataStatusUnavailable, nil},
		{"security_label", "users", MetadataStatusUnavailable, nil},
		{"schema", "app", MetadataStatusUnavailable, nil},
		{"sequence", "seq_order_id", MetadataStatusUnavailable, nil},
		{"materialized_view", "mv_stats", MetadataStatusUnavailable, nil},
	}

	for _, tc := range cases {
		snapshot := ObjectSnapshot{
			Type:       tc.objectType,
			Name:       tc.name,
			Status:     tc.status,
			Attributes: tc.attrs,
		}

		data, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatalf("marshal %s/%s: %v", tc.objectType, tc.name, err)
		}

		var rt ObjectSnapshot
		if err := json.Unmarshal(data, &rt); err != nil {
			t.Fatalf("unmarshal %s/%s: %v", tc.objectType, tc.name, err)
		}

		if rt.Type != tc.objectType {
			t.Errorf("%s/%s: expected type %q, got %q", tc.objectType, tc.name, tc.objectType, rt.Type)
		}
		if rt.Name != tc.name {
			t.Errorf("%s/%s: expected name %q, got %q", tc.objectType, tc.name, tc.name, rt.Name)
		}
		if rt.Status != tc.status {
			t.Errorf("%s/%s: expected status %q, got %q", tc.objectType, tc.name, tc.status, rt.Status)
		}
		for k, v := range tc.attrs {
			if rt.Attributes[k] != v {
				t.Errorf("%s/%s: expected attr %s=%q, got %q", tc.objectType, tc.name, k, v, rt.Attributes[k])
			}
		}

		safe := snapshot.SafeAttributes()
		for k := range safe {
			if isSensitiveAttributeKey(k) {
				t.Errorf("%s/%s: SafeAttributes leaked sensitive key %q", tc.objectType, tc.name, k)
			}
		}
	}
}
