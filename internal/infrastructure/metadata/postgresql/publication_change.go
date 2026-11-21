package postgresqlmeta

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolvePublicationChange(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	q := req.Qualifiers
	state := spec.PGPublicationChangeState{Name: req.Name, RenameTo: q["rename_to"],
		Owner: spec.PGTableOwnerTarget{Kind: q["role_kind"], Name: q["role_name"]}}
	type member struct {
		Position int    `json:"position"`
		Kind     string `json:"kind"`
		Schema   string `json:"schema_name"`
		Name     string `json:"relation_name"`
	}
	members := make([]member, len(req.PGPublicationMembers))
	for i, target := range req.PGPublicationMembers {
		members[i] = member{Position: i, Kind: target.Kind, Schema: target.Schema}
		if target.Kind == "table" && target.Table.Qualified() {
			members[i].Schema, members[i].Name = target.Table.Table.Schema, target.Table.Table.Name
		}
	}
	input, err := json.Marshal(members)
	if err != nil {
		return nil, fmt.Errorf("encode PostgreSQL publication member targets: %w", err)
	}
	var facts []byte
	err = p.db.QueryRowContext(ctx, `
		select p.oid is not null, coalesce(p.puballtables,false),
		       exists (select 1 from pg_catalog.pg_publication_rel pr where pr.prpubid OPERATOR(pg_catalog.=) p.oid and pr.prattrs is not null),
		       exists (select 1 from pg_catalog.pg_publication_rel pr
		               join pg_catalog.pg_class part on part.oid OPERATOR(pg_catalog.=) pr.prrelid
		               where pr.prpubid OPERATOR(pg_catalog.=) p.oid and part.relkind OPERATOR(pg_catalog.=) 'p'
		                 and (pr.prattrs is not null or pr.prqual is not null)),
		       taken.oid is not null, r.oid is not null,
		       coalesce((select pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object(
		          'Exists', case when wanted.kind OPERATOR(pg_catalog.=) 'schema' then n.oid is not null else t.oid is not null end,
		          'Member', case when wanted.kind OPERATOR(pg_catalog.=) 'schema' then exists (
		             select 1 from pg_catalog.pg_publication_namespace m where m.pnpubid OPERATOR(pg_catalog.=) p.oid and m.pnnspid OPERATOR(pg_catalog.=) n.oid)
		             else exists (select 1 from pg_catalog.pg_publication_rel m where m.prpubid OPERATOR(pg_catalog.=) p.oid and m.prrelid OPERATOR(pg_catalog.=) t.oid) end,
		          'Eligible', case when wanted.kind OPERATOR(pg_catalog.=) 'schema' then coalesce(n.oid OPERATOR(pg_catalog.<>) all(array[11,99]::pg_catalog.oid[])
		             and n.nspname::pg_catalog.text OPERATOR(pg_catalog.!~) '^pg_(toast_)?temp_',false)
		             else coalesce(t.relkind OPERATOR(pg_catalog.=) any(array['r','p']::pg_catalog."char"[])
		                  and t.relpersistence OPERATOR(pg_catalog.=) 'p' and t.oid OPERATOR(pg_catalog.>=) 12000::pg_catalog.oid,false) end,
		          'Inherits', coalesce(t.relkind OPERATOR(pg_catalog.<>) 'p',false) and exists (
		             select 1 from pg_catalog.pg_inherits i where i.inhparent OPERATOR(pg_catalog.=) t.oid)) order by wanted.position)
		          from pg_catalog.jsonb_to_recordset($5::pg_catalog.jsonb) as wanted(position pg_catalog.int4, kind pg_catalog.text, schema_name pg_catalog.text, relation_name pg_catalog.text)
		          left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) wanted.schema_name
		          left join pg_catalog.pg_class t on t.relnamespace OPERATOR(pg_catalog.=) n.oid and t.relname OPERATOR(pg_catalog.=) wanted.relation_name), '[]'::pg_catalog.jsonb)
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_publication p on p.pubname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_publication taken on taken.pubname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_roles r on r.rolname OPERATOR(pg_catalog.=) case
		    when $3::pg_catalog.text OPERATOR(pg_catalog.=) 'named' then $4::pg_catalog.text
		    when $3::pg_catalog.text OPERATOR(pg_catalog.=) 'session_user' then session_user::pg_catalog.text
		    when $3::pg_catalog.text OPERATOR(pg_catalog.=) any(array['current_user','current_role']::pg_catalog.text[]) then current_user::pg_catalog.text end
	`, req.Name, state.RenameTo, state.Owner.Kind, state.Owner.Name, string(input)).Scan(&state.Exists, &state.AllTables, &state.HasColumns,
		&state.HasPartitionSpec, &state.DestinationOccupied, &state.OwnerExists, &facts)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL publication metadata: %w", err)
	}
	if err := json.Unmarshal(facts, &state.Members); err != nil {
		return nil, fmt.Errorf("decode PostgreSQL publication member facts: %w", err)
	}
	for i := range state.Members {
		state.Members[i].Target = req.PGPublicationMembers[i]
	}
	return &spec.ObjectSnapshot{PGPublicationChange: &state}, nil
}
