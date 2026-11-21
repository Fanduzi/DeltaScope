package postgresqlmeta

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveCreateSources(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}
	if !target.Qualified() || len(req.PGDestructiveTargets) == 0 {
		return nil, nil
	}
	type sourceName struct {
		Schema string `json:"schema_name"`
		Name   string `json:"relation_name"`
		Order  int    `json:"position"`
	}
	names := make([]sourceName, len(req.PGDestructiveTargets))
	for i, source := range req.PGDestructiveTargets {
		names[i].Order = i + 1
		if source.Qualified() {
			names[i].Schema, names[i].Name = source.Table.Schema, source.Table.Name
		}
	}
	encoded, err := json.Marshal(names)
	if err != nil {
		return nil, fmt.Errorf("encode PostgreSQL LIKE sources: %w", err)
	}
	state := spec.PGCreateSources{Target: target}
	var raw []byte
	err = p.db.QueryRowContext(ctx, `
		select ns.oid is not null, dst.oid is not null, (
		    select pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object('Kind', coalesce(c.relkind::pg_catalog.text, ''), 'Partition', c.relispartition, 'Columns', array(
		        select a.attname::pg_catalog.text from pg_catalog.pg_attribute a
		        where a.attrelid OPERATOR(pg_catalog.=) c.oid and a.attnum OPERATOR(pg_catalog.>) 0 and not a.attisdropped
		        order by a.attnum)) order by src.position)
		    from pg_catalog.jsonb_to_recordset($3::pg_catalog.jsonb) as src(schema_name pg_catalog.text, relation_name pg_catalog.text, position pg_catalog.int4)
		    left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) src.schema_name
		    left join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid and c.relname OPERATOR(pg_catalog.=) src.relation_name)
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace ns on ns.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_class dst on dst.relnamespace OPERATOR(pg_catalog.=) ns.oid and dst.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
	`, req.Schema, req.Name, string(encoded)).Scan(&state.SchemaExists, &state.Exists, &raw)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL table creation sources: %w", err)
	}
	if err := json.Unmarshal(raw, &state.Sources); err != nil {
		return nil, fmt.Errorf("decode PostgreSQL table creation sources: %w", err)
	}
	if len(state.Sources) != len(req.PGDestructiveTargets) {
		return nil, nil
	}
	for i := range state.Sources {
		state.Sources[i].Target = req.PGDestructiveTargets[i]
	}
	if req.Type == "pg_create_inheritance" {
		return &spec.ObjectSnapshot{PGCreateInheritance: &state}, nil
	}
	return &spec.ObjectSnapshot{PGCreateLike: &state}, nil
}
