package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveIndexAttachment(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	target := spec.PGIndexAttachment{
		Parent: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}},
		Child:  spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Qualifiers["child_schema"], Name: req.Qualifiers["child"]}},
	}
	if !target.Parent.Qualified() || !target.Child.Qualified() {
		return nil, nil
	}
	state := spec.PGIndexAttachmentState{Target: target}
	err := p.db.QueryRowContext(ctx, `
		select coalesce(parent.relkind::pg_catalog.text, ''), coalesce(child.relkind::pg_catalog.text, ''),
		       exists (select 1 from pg_catalog.pg_inherits i
		         where i.inhrelid OPERATOR(pg_catalog.=) child.oid and i.inhparent OPERATOR(pg_catalog.=) parent.oid),
		       exists (select 1 from pg_catalog.pg_inherits i
		         join pg_catalog.pg_index other on other.indexrelid OPERATOR(pg_catalog.=) i.inhrelid
		         where i.inhparent OPERATOR(pg_catalog.=) parent.oid
		           and other.indrelid OPERATOR(pg_catalog.=) ci.indrelid and other.indislive
		           and other.indexrelid OPERATOR(pg_catalog.<>) child.oid),
		       exists (select 1 from pg_catalog.pg_inherits i
		         where i.inhrelid OPERATOR(pg_catalog.=) child.oid and i.inhparent OPERATOR(pg_catalog.<>) parent.oid),
		       exists (select 1 from pg_catalog.pg_inherits i
		         where i.inhrelid OPERATOR(pg_catalog.=) ci.indrelid and i.inhparent OPERATOR(pg_catalog.=) pi.indrelid),
		       exists (select 1 from pg_catalog.pg_inherits i
		         where i.inhrelid OPERATOR(pg_catalog.=) ci.indrelid and i.inhparent OPERATOR(pg_catalog.=) pi.indrelid
		           and i.inhdetachpending),
		       case
		         when pi.indexrelid is null or ci.indexrelid is null then ''
		         when pi.indisunique is distinct from ci.indisunique
		           or pi.indnullsnotdistinct is distinct from ci.indnullsnotdistinct
		           or parent.relam is distinct from child.relam
		           or pi.indnatts is distinct from ci.indnatts
		           or pi.indnkeyatts is distinct from ci.indnkeyatts
		           or (pi.indexprs is null) is distinct from (ci.indexprs is null)
		           or (pi.indpred is null) is distinct from (ci.indpred is null)
		           or pi.indisexclusion or ci.indisexclusion then 'mismatch'
		         when pi.indexprs is not null or pi.indpred is not null then ''
		         when exists (
		           select 1 from pg_catalog.unnest(pi.indkey) with ordinality as pk(attnum, position)
		           join pg_catalog.unnest(ci.indkey) with ordinality as ck(attnum, position)
		             on ck.position OPERATOR(pg_catalog.=) pk.position
		           left join pg_catalog.pg_attribute pa on pa.attrelid OPERATOR(pg_catalog.=) pi.indrelid
		             and pa.attnum OPERATOR(pg_catalog.=) pk.attnum and not pa.attisdropped
		           left join pg_catalog.pg_attribute ca on ca.attrelid OPERATOR(pg_catalog.=) ci.indrelid
		             and ca.attnum OPERATOR(pg_catalog.=) ck.attnum and not ca.attisdropped
		           left join pg_catalog.pg_opclass po on po.oid OPERATOR(pg_catalog.=) pi.indclass[pk.position OPERATOR(pg_catalog.-) 1]
		           left join pg_catalog.pg_opclass co on co.oid OPERATOR(pg_catalog.=) ci.indclass[ck.position OPERATOR(pg_catalog.-) 1]
		           where pa.attname is distinct from ca.attname
		             or (pk.position OPERATOR(pg_catalog.<=) pi.indnkeyatts and
		                 (po.opcfamily is distinct from co.opcfamily or
		                  pi.indcollation[pk.position OPERATOR(pg_catalog.-) 1] is distinct from ci.indcollation[ck.position OPERATOR(pg_catalog.-) 1]))
		         ) then 'mismatch'
		         when exists (select 1 from pg_catalog.pg_constraint con
		           where con.conrelid OPERATOR(pg_catalog.=) pi.indrelid and con.conindid OPERATOR(pg_catalog.=) pi.indexrelid
		             and con.contype OPERATOR(pg_catalog.=) any(array['p','u','x']::pg_catalog."char"[]))
		           and not exists (select 1 from pg_catalog.pg_constraint con
		           where con.conrelid OPERATOR(pg_catalog.=) ci.indrelid and con.conindid OPERATOR(pg_catalog.=) ci.indexrelid
		             and con.contype OPERATOR(pg_catalog.=) any(array['p','u','x']::pg_catalog."char"[])) then 'constraint_absent'
		         else 'match'
		       end
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace pn on pn.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_class parent on parent.relnamespace OPERATOR(pg_catalog.=) pn.oid
		 and parent.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text
		left join pg_catalog.pg_index pi on pi.indexrelid OPERATOR(pg_catalog.=) parent.oid
		left join pg_catalog.pg_namespace cn on cn.nspname OPERATOR(pg_catalog.=) $3::pg_catalog.text
		left join pg_catalog.pg_class child on child.relnamespace OPERATOR(pg_catalog.=) cn.oid
		 and child.relname OPERATOR(pg_catalog.=) $4::pg_catalog.text
		left join pg_catalog.pg_index ci on ci.indexrelid OPERATOR(pg_catalog.=) child.oid
	`, req.Schema, req.Name, target.Child.Table.Schema, target.Child.Table.Name).Scan(&state.ParentKind, &state.ChildKind,
		&state.SameParent, &state.PartitionInUse, &state.OtherParent, &state.TablePartition, &state.DetachPending, &state.Definition)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL index attachment: %w", err)
	}
	return &spec.ObjectSnapshot{PGIndexAttachment: &state}, nil
}
