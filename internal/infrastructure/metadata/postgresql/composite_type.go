package postgresqlmeta

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveCompositeType(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}
	if !target.Qualified() {
		return nil, nil
	}
	state := spec.PGCompositeState{Target: target}
	var attributes []byte
	err := p.db.QueryRowContext(ctx, `
		select ns.oid is not null,
		       coalesce(t.typisdefined and not coalesce(e.typarray OPERATOR(pg_catalog.=) t.oid, false), false),
		       coalesce(c.relkind::pg_catalog.text, ''),
		       case when $3::pg_catalog.bool and c.relkind OPERATOR(pg_catalog.=) 'c' then
		           pg_catalog.to_jsonb(array(select a.attname::pg_catalog.text from pg_catalog.pg_attribute a
		               where a.attrelid OPERATOR(pg_catalog.=) c.oid and a.attnum OPERATOR(pg_catalog.>) 0
		                 and not a.attisdropped order by a.attnum)) end
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace ns on ns.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_type t on t.typnamespace OPERATOR(pg_catalog.=) ns.oid
		 and t.typname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_type e on e.oid OPERATOR(pg_catalog.=) t.typelem
		 and t.typsubscript OPERATOR(pg_catalog.=) 6179::pg_catalog.oid
		left join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) ns.oid
		 and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
	`, req.Schema, req.Name, req.Qualifiers["attributes"] == "true").Scan(&state.SchemaExists, &state.TypeOccupied, &state.RelationKind, &attributes)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL composite type facts: %w", err)
	}
	if attributes != nil {
		if err := json.Unmarshal(attributes, &state.Attributes); err != nil {
			return nil, fmt.Errorf("decode PostgreSQL composite attributes: %w", err)
		}
	}
	return &spec.ObjectSnapshot{PGComposite: &state}, nil
}
