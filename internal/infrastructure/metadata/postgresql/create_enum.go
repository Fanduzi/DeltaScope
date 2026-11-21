package postgresqlmeta

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveEnum(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}
	if !target.Qualified() {
		return nil, nil
	}
	state := spec.PGEnumState{Target: target}
	var labels []byte
	err := p.db.QueryRowContext(ctx, `
		select ns.oid is not null,
		       coalesce(t.typisdefined and not coalesce(e.typarray OPERATOR(pg_catalog.=) t.oid, false), false),
		       pg_catalog.current_setting('server_encoding'), pg_catalog.current_setting('max_identifier_length')::pg_catalog.int4,
		       t.oid is not null, coalesce(t.typisdefined, false), coalesce(t.typtype::pg_catalog.text, ''),
		       coalesce(t.typstorage::pg_catalog.text, ''), coalesce(t.typlen OPERATOR(pg_catalog.=) -1, false),
		       coalesce(t.typsubscript OPERATOR(pg_catalog.=) 6179::pg_catalog.oid and t.typelem OPERATOR(pg_catalog.<>) 0::pg_catalog.oid, false),
		       case when $3::pg_catalog.bool and t.typtype OPERATOR(pg_catalog.=) 'e' then
		           pg_catalog.to_jsonb(array(select l.enumlabel::pg_catalog.text from pg_catalog.pg_enum l
		               where l.enumtypid OPERATOR(pg_catalog.=) t.oid order by l.enumsortorder)) end
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace ns on ns.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_type t on t.typnamespace OPERATOR(pg_catalog.=) ns.oid
		 and t.typname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_type e on e.oid OPERATOR(pg_catalog.=) t.typelem
		 and t.typsubscript OPERATOR(pg_catalog.=) 6179::pg_catalog.oid
	`, req.Schema, req.Name, req.Type == "pg_alter_enum").Scan(&state.SchemaExists, &state.NameOccupied, &state.Encoding, &state.LabelBytes, &state.TypeExists, &state.Defined, &state.TypeKind, &state.Storage, &state.VariableLength, &state.TrueArray, &labels)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL enum type: %w", err)
	}
	if labels != nil {
		if err := json.Unmarshal(labels, &state.Labels); err != nil {
			return nil, fmt.Errorf("decode PostgreSQL enum labels: %w", err)
		}
	}
	return &spec.ObjectSnapshot{PGEnum: &state}, nil
}
