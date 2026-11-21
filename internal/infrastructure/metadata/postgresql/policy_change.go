package postgresqlmeta

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolvePolicyChange(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	roles, err := json.Marshal(append([]string{}, req.PGPolicyRoles...))
	if err != nil {
		return nil, fmt.Errorf("encode PostgreSQL policy roles: %w", err)
	}
	state := spec.PGPolicyChangeState{Table: spec.Table{Schema: req.Schema, Name: req.Name},
		Name: req.Qualifiers["policy"], RenameTo: req.Qualifiers["rename_to"], NamedRoles: slices.Clone(req.PGPolicyRoles)}
	var columns []byte
	err = p.db.QueryRowContext(ctx, `
		select coalesce(c.relkind::pg_catalog.text, ''), coalesce(src.polcmd::pg_catalog.text, ''), dst.oid is not null,
		       exists (select 1 from pg_catalog.jsonb_array_elements_text($5::pg_catalog.jsonb) as wanted(name)
		           where not exists (select 1 from pg_catalog.pg_roles r where r.rolname OPERATOR(pg_catalog.=) wanted.name)),
		       pg_catalog.to_jsonb(array(select a.attname::pg_catalog.text from pg_catalog.pg_attribute a
		           where a.attrelid OPERATOR(pg_catalog.=) c.oid and not a.attisdropped order by a.attnum))
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_class c on c.relnamespace OPERATOR(pg_catalog.=) n.oid and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_policy src on src.polrelid OPERATOR(pg_catalog.=) c.oid and src.polname OPERATOR(pg_catalog.=) $3::pg_catalog.text
		left join pg_catalog.pg_policy dst on dst.polrelid OPERATOR(pg_catalog.=) c.oid and dst.polname OPERATOR(pg_catalog.=) $4::pg_catalog.text
	`, req.Schema, req.Name, state.Name, state.RenameTo, string(roles)).Scan(
		&state.RelationKind, &state.PolicyCommand, &state.DestinationOccupied, &state.MissingRole, &columns)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL policy change premises: %w", err)
	}
	if err := json.Unmarshal(columns, &state.Columns); err != nil {
		return nil, fmt.Errorf("decode PostgreSQL policy table columns: %w", err)
	}
	return &spec.ObjectSnapshot{PGPolicyChange: &state}, nil
}
