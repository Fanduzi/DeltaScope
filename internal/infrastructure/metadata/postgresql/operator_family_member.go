package postgresqlmeta

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func (p *Provider) resolveOperatorFamilyMember(ctx context.Context, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.PGOperatorFamilyMember == nil {
		return nil, nil
	}
	member := *req.PGOperatorFamilyMember
	state := spec.PGOperatorFamilyMemberState{
		Target: spec.PGOperatorFamilyTarget{Schema: req.Schema, Name: req.Name, Method: req.Qualifiers["method"]}, Member: member,
		OperandTypesRead: member.Left.Qualified() && member.Right.Qualified() && !member.Left.Array && !member.Right.Array,
	}
	// PostgreSQL 17 pins pg_amop objects below FirstUnpinnedObjectId.
	err := p.db.QueryRowContext(ctx, `
		select ns.oid is not null, coalesce(am.amtype::pg_catalog.text, ''), fam.oid is not null,
		       coalesce(am.oid OPERATOR(pg_catalog.=) 403 and am.amname OPERATOR(pg_catalog.=) 'btree', false),
		       lt.oid is not null and rt.oid is not null, oper.oid is not null,
		       coalesce(oper.oprresult OPERATOR(pg_catalog.=) 16, false), slot.oid is not null,
		       coalesce(slot.oid OPERATOR(pg_catalog.<) 12000, false) or exists (select 1 from pg_catalog.pg_depend d
		               where d.classid OPERATOR(pg_catalog.=) 'pg_catalog.pg_amop'::pg_catalog.regclass
		                 and d.objid OPERATOR(pg_catalog.=) slot.oid
		                 and d.deptype OPERATOR(pg_catalog.=) 'i')
		from (values (1)) as witness(dummy)
		left join pg_catalog.pg_namespace ns on ns.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text
		left join pg_catalog.pg_am am on am.amname OPERATOR(pg_catalog.=) $3::pg_catalog.text
		left join pg_catalog.pg_opfamily fam on fam.opfnamespace OPERATOR(pg_catalog.=) ns.oid
		 and fam.opfname OPERATOR(pg_catalog.=) $2::pg_catalog.text and fam.opfmethod OPERATOR(pg_catalog.=) am.oid
		left join pg_catalog.pg_namespace ln on ln.nspname OPERATOR(pg_catalog.=) $4::pg_catalog.text
		left join pg_catalog.pg_type lt on lt.typnamespace OPERATOR(pg_catalog.=) ln.oid and lt.typname OPERATOR(pg_catalog.=) $5::pg_catalog.text and lt.typisdefined
		left join pg_catalog.pg_namespace rn on rn.nspname OPERATOR(pg_catalog.=) $6::pg_catalog.text
		left join pg_catalog.pg_type rt on rt.typnamespace OPERATOR(pg_catalog.=) rn.oid and rt.typname OPERATOR(pg_catalog.=) $7::pg_catalog.text and rt.typisdefined
		left join pg_catalog.pg_namespace ons on ons.nspname OPERATOR(pg_catalog.=) $8::pg_catalog.text
		left join pg_catalog.pg_operator oper on oper.oprnamespace OPERATOR(pg_catalog.=) ons.oid and oper.oprname OPERATOR(pg_catalog.=) $9::pg_catalog.text
		 and oper.oprleft OPERATOR(pg_catalog.=) lt.oid and oper.oprright OPERATOR(pg_catalog.=) rt.oid
		left join pg_catalog.pg_amop slot on slot.amopfamily OPERATOR(pg_catalog.=) fam.oid
		 and slot.amoplefttype OPERATOR(pg_catalog.=) lt.oid and slot.amoprighttype OPERATOR(pg_catalog.=) rt.oid
		 and slot.amopstrategy OPERATOR(pg_catalog.=) $10::pg_catalog.int4
	`, req.Schema, req.Name, state.Target.Method, member.Left.Schema, member.Left.Name, member.Right.Schema, member.Right.Name,
		member.Operator.Schema, member.Operator.Name, member.Number).Scan(&state.SchemaExists, &state.MethodKind, &state.FamilyExists,
		&state.BTree, &state.OperandTypesOK, &state.OperatorExists, &state.OperatorBoolean, &state.SlotExists, &state.SlotProtected)
	if err != nil {
		return nil, fmt.Errorf("read PostgreSQL operator family member: %w", err)
	}
	return &spec.ObjectSnapshot{PGOperatorFamilyMember: &state}, nil
}
