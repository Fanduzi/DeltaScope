package postgresqlmeta

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveTablePersistence(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	var toLogged bool
	switch req.Qualifiers["action"] {
	case "set_logged":
		toLogged = true
	case "set_unlogged":
	default:
		return nil, nil
	}
	if req.Schema == "" || req.Schema == "pg_temp" || req.Name == "" {
		return nil, nil
	}
	partitionRoot := req.Qualifiers["kind"] == "p"
	if partitionRoot && toLogged {
		return nil, nil
	}
	// Match ATPrepChangePersistence: same-persistence requests bypass
	// dependencies, and only direct publication entries affect UNLOGGED.
	row := p.db.QueryRowContext(ctx, `
		select c.relpersistence::pg_catalog.text,
		       case when not $3 and c.relpersistence OPERATOR(pg_catalog.=) 'p'
		            then exists (select 1 from pg_catalog.pg_publication_rel pr
		                         where pr.prrelid OPERATOR(pg_catalog.=) c.oid)
		            else false end,
		       coalesce(fk.conflict, false), coalesce(fk.unknown_peer, false)
		from pg_catalog.pg_class c
		join pg_catalog.pg_namespace n on n.oid OPERATOR(pg_catalog.=) c.relnamespace
		left join lateral (
		  select pg_catalog.bool_or(case when $3 then peer.relpersistence OPERATOR(pg_catalog.<>) 'p'
		                                  else peer.relpersistence OPERATOR(pg_catalog.=) 'p' end) as conflict,
		         pg_catalog.bool_or(peer.oid is null) as unknown_peer
		  from pg_catalog.pg_constraint con
		  left join pg_catalog.pg_class peer on peer.oid OPERATOR(pg_catalog.=)
		    case when $3 then con.confrelid else con.conrelid end
		  where con.contype OPERATOR(pg_catalog.=) 'f'
		    and con.conrelid OPERATOR(pg_catalog.<>) con.confrelid
		    and (($3 and c.relpersistence OPERATOR(pg_catalog.=) 'u' and con.conrelid OPERATOR(pg_catalog.=) c.oid)
		         or (not $3 and c.relpersistence OPERATOR(pg_catalog.=) 'p' and con.confrelid OPERATOR(pg_catalog.=) c.oid))
		) fk on true
		where n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		  and c.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		  and not c.relisshared and not c.relispartition
		  and c.reloftype OPERATOR(pg_catalog.=) 0::pg_catalog.oid
		  and (($4::pg_catalog.bool and c.relkind OPERATOR(pg_catalog.=) 'p'
		        and c.relpersistence OPERATOR(pg_catalog.=) 'p')
		    or (not $4::pg_catalog.bool and c.relkind OPERATOR(pg_catalog.=) 'r'
		        and c.relpersistence OPERATOR(pg_catalog.=) any(array['p','u']::pg_catalog."char"[])
		        and c.relfilenode OPERATOR(pg_catalog.<>) 0::pg_catalog.oid
		        and not exists (select 1 from pg_catalog.pg_inherits h
		                        where h.inhrelid OPERATOR(pg_catalog.=) c.oid or h.inhparent OPERATOR(pg_catalog.=) c.oid)))
	`, req.Schema, req.Name, toLogged, partitionRoot)
	facts := &spec.PGTablePersistence{Schema: req.Schema, Table: req.Name, Logged: toLogged, PartitionRoot: partitionRoot}
	if err := row.Scan(&facts.Current, &facts.DirectPublication, &facts.ForeignKeyConflict, &facts.ForeignKeyUnknown); errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("read table persistence preconditions: %w", err)
	}
	return &spec.ObjectSnapshot{PGTablePersistence: facts}, nil
}
