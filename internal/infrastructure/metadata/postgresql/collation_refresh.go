package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveCollationRefresh(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGColumnCollation{Schema: req.Schema, Name: req.Name}
	if !target.Qualified() {
		return nil, nil
	}
	state := spec.PGCollationRefreshState{Target: target}
	err := p.db.QueryRowContext(ctx, `
		select col.oid is null or col.collencoding OPERATOR(pg_catalog.=) enc.encoding
		       or col.collprovider OPERATOR(pg_catalog.<>) 'i'
		       or enc.encoding OPERATOR(pg_catalog.=) pg_catalog.pg_char_to_encoding('UTF8'),
		       col.oid is not null, coalesce(col.oid OPERATOR(pg_catalog.=) 100, false),
		       coalesce(version.incompatible is not null, false), coalesce(version.incompatible, false)
		from (select pg_catalog.pg_char_to_encoding(pg_catalog.current_setting('server_encoding')) as encoding) enc
		left join pg_catalog.pg_namespace n on n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join lateral (
		    select c.oid, c.collencoding, c.collprovider, c.collversion, c.collcollate, c.colllocale from pg_catalog.pg_collation c
		    where c.collnamespace OPERATOR(pg_catalog.=) n.oid and c.collname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		      and c.collencoding OPERATOR(pg_catalog.=) any(array[enc.encoding,-1])
		    order by c.collencoding desc limit 1
		) col on true
		left join lateral (
		    select case when (col.collprovider OPERATOR(pg_catalog.=) 'c'
		                           and col.collcollate OPERATOR(pg_catalog.=) any(array['C','POSIX']::pg_catalog.text[]))
		                      or (col.collprovider OPERATOR(pg_catalog.=) 'b'
		                           and col.colllocale OPERATOR(pg_catalog.=) any(array['C','C.UTF-8']::pg_catalog.text[]))
		                then (col.collversion is null) is distinct from (pg_catalog.pg_collation_actual_version(col.oid) is null)
		           end as incompatible
		) version on true
	`, target.Schema, target.Name).Scan(&state.Known, &state.Exists, &state.Default, &state.VersionKnown, &state.VersionIncompatible)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL collation refresh target: %w", err)
	}
	return &spec.ObjectSnapshot{PGCollationRefresh: &state}, nil
}
