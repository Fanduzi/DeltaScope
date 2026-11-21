package spec

import "strings"

// PGRefreshTarget retains the authored materialized-view identity.
type PGRefreshTarget struct {
	Schema, Name     string
	CatalogQualified bool
}

// Qualified reports whether the target has an exact persistent namespace identity.
func (t PGRefreshTarget) Qualified() bool {
	return t.Schema != "" && t.Name != "" && !t.CatalogQualified && t.Schema != "pg_temp" &&
		!strings.HasPrefix(t.Schema, "pg_temp_") && !strings.HasPrefix(t.Schema, "pg_toast_temp_")
}

// PGRefreshState binds the required catalog observations to one exact target.
// An empty Kind proves the relation is absent in its schema.
type PGRefreshState struct {
	Target                 PGRefreshTarget
	Kind                   string
	Populated, UniqueIndex bool
}
