# input: fixed T09 index-governance roles, native four-anchor observations, and independently declared schema/result expectations
# output: immutable role specs for the T09 manifest and full-entry artifact validation; never reads product actuals
# pos: independent T09 acceptance oracle beside the existing DDL Golden runner
# note: if this file changes, update this header and scripts/README.md.
import copy
import hashlib
import json
from pathlib import Path

IDX_COLS = 'ddl.index.columns.max_count'
ALTER_COLS = 'ddl.alter.add_index.columns.max_count'
IDX_SEC_PFX = 'ddl.index.secondary.prefix.require'
ALTER_SEC_PFX = 'ddl.alter.add_index.secondary.prefix.require'
IDX_UNQ_PFX = 'ddl.index.unique.prefix.require'
ALTER_UNQ_PFX = 'ddl.alter.add_index.unique.prefix.require'
CONSTR_UNQ_PFX = 'ddl.constraint.unique_key.name.prefix.require'
IDX_PATTERN = 'ddl.index.name.pattern.require'
IDX_KEYWORD = 'ddl.index.name.keyword.forbid'
IDX_DUP = 'ddl.index.duplicate.forbid'
ALTER_DUP = 'ddl.alter.add_index.duplicate.forbid'
IDX_LP = 'ddl.index.redundant_left_prefix.forbid'
ALTER_LP = 'ddl.alter.add_index.redundant_left_prefix.forbid'
IDX_UO = 'ddl.index.redundant_unique_overlap.forbid'
ALTER_UO = 'ddl.alter.add_index.redundant_unique_overlap.forbid'
ADD_EXISTS = 'ddl.alter.add_index.exists.forbid'
DROP_EXISTS = 'ddl.alter.drop_index.exists.require'
REN_EXISTS = 'ddl.alter.rename_index.exists.require'
PK_EXISTS = 'ddl.alter.drop_primary_key.exists.require'
COLS_EXISTS = 'ddl.create_index.columns.exists.require'
TBL_EXISTS = 'ddl.table.exists.alter.require'
DROP_FORBID = 'ddl.alter.drop_index.forbid'
REN_FORBID = 'ddl.alter.rename_index.forbid'
CI_NOTICE = 'ddl.create_index.notice'
DI_NOTICE = 'ddl.drop_index.notice'
AC_NOTICE = 'ddl.alter.add_constraint.notice'
IDX_GAP_FACTS = ['target_table.existence', 'target_table.indexes']

VERSIONS = {'mysql57': '5.7.44', 'mysql80': '8.0.46', 'mysql84': '8.4.10', 'tidb85': 'v8.5.0'}
PORTS = {'mysql57': 23357, 'mysql80': 23380, 'mysql84': 23384, 'tidb85': 24000}
DIALECTS = {'mysql57': 'mysql', 'mysql80': 'mysql', 'mysql84': 'mysql', 'tidb85': 'tidb'}
ALL4 = tuple(VERSIONS)
MYSQL3 = ('mysql57', 'mysql80', 'mysql84')
VISIBLE = ('mysql80', 'mysql84', 'tidb85')  # anchors exposing STATISTICS.IS_VISIBLE

FROZEN_REQUIRED_CASES = 125
FROZEN_IDS_SHA256 = '1f7c3ea3dd95c217d1c894cddd537db73606d6dd9fc81d41f2280f5d81ffc805'

FIXTURE_DEFS = {
    't9': 'CREATE TABLE t9 (a INT, b INT, c INT, d INT, e INT, f INT, g INT, h INT, i INT);',
    't9i': 'CREATE TABLE t9i (a INT, b INT, c INT, d INT, KEY i_ab(a,b), UNIQUE KEY uq_cd(c,d), KEY i_abc(a,b,c));',
    't9p': 'CREATE TABLE t9p (a INT NOT NULL, b INT, PRIMARY KEY (a));',
    't9o': 'CREATE TABLE t9o (a INT, b INT);',
}
# name:ordinal_position:is_nullable:IFNULL(HEX(column_default),'x00'):LOWER(extra)
FIXTURE_COLS = {
    't9': 'a:1:YES:x00:|b:2:YES:x00:|c:3:YES:x00:|d:4:YES:x00:|e:5:YES:x00:|f:6:YES:x00:|g:7:YES:x00:|h:8:YES:x00:|i:9:YES:x00:',
    't9i': 'a:1:YES:x00:|b:2:YES:x00:|c:3:YES:x00:|d:4:YES:x00:',
    't9p': 'a:1:NO:x00:|b:2:YES:x00:',
    't9o': 'a:1:YES:x00:|b:2:YES:x00:',
}
# Index membership as name:non_unique:seq_in_index:column_name rows, binary-name order.
FIXTURE_IDX = {
    't9': '',
    't9i': 'i_ab:1:1:a,i_ab:1:2:b,i_abc:1:1:a,i_abc:1:2:b,i_abc:1:3:c,uq_cd:0:1:c,uq_cd:0:2:d',
    't9p': 'PRIMARY:0:1:a',
    't9o': '',
}
NO_PK_FIXTURES = ('t9', 't9i', 't9o')

E1054 = ('ERROR 1054',)
E1061 = ('ERROR 1061',)
E1064 = ('ERROR 1064',)
E1072 = ('ERROR 1072',)
E1091 = ('ERROR 1091',)
E1146 = ('ERROR 1146',)
E1176 = ('ERROR 1176',)
E8200 = ('ERROR 8200',)


def on(level, **params):
    return {'enabled': True, 'level': level, 'params': params}


def profiles():
    raw = {
        # First-path column-count isolation: create and alter entries enabled
        # separately so each rule's attribution is pinned on its own row.
        't09-cols8': {IDX_COLS: on('warning', limit=8), ALTER_COLS: on('warning', limit=8)},
        't09-cols8-create-off': {ALTER_COLS: on('warning', limit=8)},
        't09-cols8-alter-off': {IDX_COLS: on('warning', limit=8)},
        't09-naming': {
            IDX_SEC_PFX: on('blocker', prefix='idx_'), ALTER_SEC_PFX: on('blocker', prefix='idx_'),
            IDX_UNQ_PFX: on('blocker', prefix='uq_'), ALTER_UNQ_PFX: on('blocker', prefix='uq_'),
            CONSTR_UNQ_PFX: on('blocker', prefix='uc_')},
        't09-naming-custom': {
            IDX_SEC_PFX: on('blocker', prefix='ix_'), ALTER_SEC_PFX: on('blocker', prefix='ix_'),
            IDX_UNQ_PFX: on('blocker', prefix='uk_'), ALTER_UNQ_PFX: on('blocker', prefix='uk_'),
            CONSTR_UNQ_PFX: on('blocker', prefix='cu_')},
        't09-naming-pattern': {
            IDX_PATTERN: on('blocker', required=True, pattern='^idx_'),
            IDX_KEYWORD: on('blocker', forbid=True)},
        't09-dup': {IDX_DUP: on('warning'), ALTER_DUP: on('warning', forbid=True)},
        't09-redundant': {IDX_LP: on('warning'), ALTER_LP: on('warning', forbid=True)},
        't09-overlap': {IDX_UO: on('warning'), ALTER_UO: on('warning', forbid=True)},
        't09-exists': {
            ADD_EXISTS: on('blocker'), DROP_EXISTS: on('blocker'), REN_EXISTS: on('blocker'),
            PK_EXISTS: on('blocker'), COLS_EXISTS: on('blocker'), TBL_EXISTS: on('blocker')},
        't09-forbid': {DROP_FORBID: on('blocker', forbid=True), REN_FORBID: on('blocker', forbid=True)},
        't09-notice': {CI_NOTICE: on('notice'), DI_NOTICE: on('notice'), AC_NOTICE: on('notice')},
    }
    return {name: {'enable': rules} for name, rules in raw.items()}


def connect(anchor):
    conn = {'host': '127.0.0.1', 'port': PORTS[anchor], 'user': 'root', 'schema': 'golden'}
    if anchor != 'tidb85':
        conn['password_env'] = 'T09_MYSQL_PASSWORD'
    return conn


def colq(table):
    return ("SELECT COALESCE(GROUP_CONCAT(CONCAT(column_name,':',ordinal_position,':',"
            "is_nullable,':',IFNULL(HEX(column_default),'x00'),':',LOWER(extra)) "
            "ORDER BY ordinal_position SEPARATOR '|'),'') FROM information_schema.columns "
            "WHERE table_schema='golden' AND table_name='" + table + "'")


def idxq(table):
    return ("SELECT COALESCE(GROUP_CONCAT(CONCAT(index_name,':',non_unique,':',seq_in_index,"
            "':',column_name,':',index_type) ORDER BY CAST(index_name AS BINARY),seq_in_index "
            "SEPARATOR ','),'') FROM information_schema.statistics "
            "WHERE table_schema='golden' AND table_name='" + table + "'")


def idxvq(table):
    return ("SELECT COALESCE(GROUP_CONCAT(CONCAT(index_name,':',non_unique,':',seq_in_index,"
            "':',column_name,':',index_type,':',is_visible) ORDER BY CAST(index_name AS BINARY),"
            "seq_in_index SEPARATOR ','),'') FROM information_schema.statistics "
            "WHERE table_schema='golden' AND table_name='" + table + "'")


def colcheck(table, want):
    return {'assert': table + ' ordered column set', 'sql': colq(table), 'expect': want}


def tcount(table, want):
    return {'assert': table + ' bound table count',
            'sql': "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='golden' AND table_name='" + table + "'",
            'expect': want}


def idx_render(anchor, spec):
    """Render the pinned index-membership spec for one anchor: each row is
    name:non_unique:seq:column, then :index_type[:is_visible] per anchor
    capability (MySQL 5.7 STATISTICS has no IS_VISIBLE column)."""
    if not spec:
        return ''
    suffix = ':BTREE:YES' if anchor in VISIBLE else ':BTREE'
    return ','.join(row + suffix for row in spec.split(','))


def idxcheck(anchor, table, spec):
    query = idxvq(table) if anchor in VISIBLE else idxq(table)
    return {'assert': table + ' index identity', 'sql': query, 'expect': idx_render(anchor, spec)}


def expect(sqls, verdict='pass', coverage='complete', findings=0, gap_entries=(),
           finding_entries=(), finding_meta=(), unsupported_entries=(), diagnostics=0,
           loaded=None, applicable=None, exit_code=None, fail_on=None):
    e = {'exit': exit_code if exit_code is not None
         else (1 if (verdict == 'reject' or coverage == 'incomplete') else 0),
         'verdict': verdict, 'statements': len(sqls),
         'findings': findings, 'diagnostics': diagnostics, 'unsupported': len(unsupported_entries),
         'coverage': coverage,
         'statement_coverage': [coverage] * len(sqls),
         'statement_sql': list(sqls), 'statement_indices': list(range(len(sqls))),
         'evidence_gaps': len(gap_entries), 'evidence_gap_entries': list(gap_entries),
         'finding_entries': list(finding_entries), 'finding_metadata': list(finding_meta),
         'unsupported_entries': list(unsupported_entries)}
    if loaded is not None:
        e['rule_summary_loaded'] = loaded
    if applicable is not None:
        e['rule_summary_applicable'] = applicable
    if fail_on is not None:
        e['fail_on_triggered'] = fail_on
    return e


def finding(index, rule_id, level):
    return {'index': index, 'rule_id': rule_id, 'level': level}


def meta(stmt_index, rule_id, **metadata):
    return {'index': stmt_index, 'rule_id': rule_id, 'metadata': metadata}


def dup_meta(table, dup_of, name, columns):
    return {'columns': columns, 'duplicate': dup_of, 'index': name, 'kind': 'secondary',
            'signature': 'secondary:' + ','.join(columns), 'table': table}


def lp_meta(table, name, covered_by, columns, covering):
    return {'columns': columns, 'covering': covering, 'heuristic': 'left_prefix',
            'index': name, 'index_kind': 'secondary', 'redundant': covered_by, 'table': table}


def uo_meta(table, name, redundant_with, columns):
    return {'columns': columns, 'heuristic': 'unique_overlap', 'index': name,
            'index_kind': 'secondary', 'redundant': redundant_with, 'table': table}


def exists_forbid_meta(table, action, name):
    return {'table': table, 'action': action, 'name': name, 'exists': True}


def exists_require_meta(table, action, name):
    return {'table': table, 'action': action, 'name': name, 'exists': False}


def gap(index, rule_id):
    return {'index': index, 'rule_id': rule_id, 'reason_code': 'unknown_table_state',
            'required_facts': IDX_GAP_FACTS}


def setup_fixture(anchor, tables):
    """Absence proof, then create each owned fixture and verify its declared
    column set plus the created index membership — "known" is created
    knowledge, never assumed emptiness (contract §8.1)."""
    steps = []
    for table in tables:
        steps.append({'name': 'prove-prior-absence-' + table, 'sql': tcount(table, '0')['sql'],
                      'expect_rc': 0, 'verify': [tcount(table, '0')]})
    for table in tables:
        steps.append({'name': 'create-fixture-' + table, 'sql': FIXTURE_DEFS[table], 'expect_rc': 0,
                      'verify': [tcount(table, '1'), colcheck(table, FIXTURE_COLS[table]),
                                 idxcheck(anchor, table, FIXTURE_IDX[table])]})
    return steps


def teardown_fixture(*tables):
    return [{'name': 'drop-' + table, 'sql': 'DROP TABLE ' + table + ';', 'expect_rc': 0,
             'verify': [tcount(table, '0')]} for table in tables]


def exec_step(name, sql, rc=0, stderr=(), verify=()):
    step = {'name': name, 'sql': sql, 'expect_rc': rc}
    if stderr:
        step['stderr_contains'] = list(stderr)
    if verify:
        # Either a list of query dicts, or a callable(anchor) resolved inside
        # meta_case so per-anchor queries never reach the manifest.
        step['verify'] = verify
    return step


def meta_case(role, anchor, tables, sqls, exp, exec_steps, policy=None, final_idx=None,
              final_cols=None, gipk=False, extra_structure=()):
    """One metadata case: setup proves absence and creates the owned fixtures,
    the CLI audits against the live catalog, post_verify proves the audit did
    not mutate it, execute applies the audited statements natively, structure
    pins the resulting STATISTICS membership, teardown drops owned tables."""
    tables = list(tables)
    final_idx = final_idx or {}
    final_cols = final_cols or {}
    case = {'id': 't09-' + anchor + '-' + role, 'anchor': anchor, 'dialect': DIALECTS[anchor],
            'sql': '\n'.join(sqls), 'connect': connect(anchor), 'expect': copy.deepcopy(exp)}
    if policy:
        case['policy'] = policy
    case['setup'] = setup_fixture(anchor, tables)
    case['post_verify'] = [check for table in tables
                           for check in (colcheck(table, FIXTURE_COLS[table]),
                                         idxcheck(anchor, table, FIXTURE_IDX[table]))]
    resolved = []
    for step in exec_steps:
        step = copy.deepcopy(step)
        if callable(step.get('verify')):
            step['verify'] = step['verify'](anchor)
        resolved.append(step)
    case['execute'] = resolved
    structure = []
    for table in tables:
        want_idx = final_idx.get(table, FIXTURE_IDX[table])
        structure.append(idxcheck(anchor, table, want_idx))
        structure.append(colcheck(table, final_cols.get(table, FIXTURE_COLS[table])))
    structure += list(extra_structure)
    case['structure'] = structure
    case['teardown'] = teardown_fixture(*tables)
    if gipk and anchor in ('mysql80', 'mysql84') and any(t in NO_PK_FIXTURES for t in tables):
        case['expect']['instance_facts'] = {'sql_generate_invisible_primary_key': 'OFF'}
    return case


def metadata_cases():
    cases = []

    def add(role, anchors, tables, sqls, exp, exec_steps, policy=None, **kw):
        for anchor in anchors:
            cases.append(meta_case(role, anchor, tables, sqls, exp, exec_steps,
                                   policy=policy, **kw))

    wide_t9i = 'i_ab:1:1:a,i_ab:1:2:b,i_abc:1:1:a,i_abc:1:2:b,i_abc:1:3:c,i_new:1:1:a,i_new:1:2:b,uq_cd:0:1:c,uq_cd:0:2:d'

    # ---- physical-set roles against the live fixture -------------------------
    add('dup-create', ALL4, ['t9i'], ['CREATE INDEX i_new ON t9i(a,b);'],
        expect(['CREATE INDEX i_new ON t9i(a,b);'], verdict='review', findings=1,
               finding_entries=[finding(0, IDX_DUP, 'warning')],
               finding_meta=[meta(0, IDX_DUP, **dup_meta('t9i', 'i_ab', 'i_new', ['a', 'b']))],
               loaded=2, applicable=1),
        [exec_step('apply', 'CREATE INDEX i_new ON t9i(a,b);')],
        policy='t09-dup', final_idx={'t9i': wide_t9i}, gipk=True)

    add('dup-alter', ALL4, ['t9i'], ['ALTER TABLE t9i ADD INDEX i_new(a,b);'],
        expect(['ALTER TABLE t9i ADD INDEX i_new(a,b);'], verdict='review', findings=1,
               finding_entries=[finding(0, ALTER_DUP, 'warning')],
               finding_meta=[meta(0, ALTER_DUP, **dup_meta('t9i', 'i_ab', 'i_new', ['a', 'b']))],
               loaded=2, applicable=1),
        [exec_step('apply', 'ALTER TABLE t9i ADD INDEX i_new(a,b);')],
        policy='t09-dup', final_idx={'t9i': wide_t9i}, gipk=True)

    lp_final = 'i_a:1:1:a,i_ab:1:1:a,i_ab:1:2:b,i_abc:1:1:a,i_abc:1:2:b,i_abc:1:3:c,uq_cd:0:1:c,uq_cd:0:2:d'
    add('lp-create', ALL4, ['t9i'], ['CREATE INDEX i_a ON t9i(a);'],
        expect(['CREATE INDEX i_a ON t9i(a);'], verdict='review', findings=1,
               finding_entries=[finding(0, IDX_LP, 'warning')],
               finding_meta=[meta(0, IDX_LP, **lp_meta('t9i', 'i_a', 'i_ab', ['a'], ['a', 'b']))],
               loaded=2, applicable=1),
        [exec_step('apply', 'CREATE INDEX i_a ON t9i(a);')],
        policy='t09-redundant', final_idx={'t9i': lp_final}, gipk=True)

    add('lp-alter', ALL4, ['t9i'], ['ALTER TABLE t9i ADD INDEX i_a(a);'],
        expect(['ALTER TABLE t9i ADD INDEX i_a(a);'], verdict='review', findings=1,
               finding_entries=[finding(0, ALTER_LP, 'warning')],
               finding_meta=[meta(0, ALTER_LP, **lp_meta('t9i', 'i_a', 'i_ab', ['a'], ['a', 'b']))],
               loaded=2, applicable=1),
        [exec_step('apply', 'ALTER TABLE t9i ADD INDEX i_a(a);')],
        policy='t09-redundant', final_idx={'t9i': lp_final}, gipk=True)

    uo_final = 'i_ab:1:1:a,i_ab:1:2:b,i_abc:1:1:a,i_abc:1:2:b,i_abc:1:3:c,s_cd:1:1:c,s_cd:1:2:d,uq_cd:0:1:c,uq_cd:0:2:d'
    add('uo-create', ALL4, ['t9i'], ['CREATE INDEX s_cd ON t9i(c,d);'],
        expect(['CREATE INDEX s_cd ON t9i(c,d);'], verdict='review', findings=1,
               finding_entries=[finding(0, IDX_UO, 'warning')],
               finding_meta=[meta(0, IDX_UO, **uo_meta('t9i', 's_cd', 'uq_cd', ['c', 'd']))],
               loaded=2, applicable=1),
        [exec_step('apply', 'CREATE INDEX s_cd ON t9i(c,d);')],
        policy='t09-overlap', final_idx={'t9i': uo_final}, gipk=True)

    # THE un-deferred rule: ALTER ADD INDEX vs an existing UNIQUE index is
    # semantic evidence that ddl.alter.add_index.redundant_unique_overlap
    # reads the provider snapshot, not just same-statement candidates.
    add('uo-alter', ALL4, ['t9i'], ['ALTER TABLE t9i ADD INDEX s_cd(c,d);'],
        expect(['ALTER TABLE t9i ADD INDEX s_cd(c,d);'], verdict='review', findings=1,
               finding_entries=[finding(0, ALTER_UO, 'warning')],
               finding_meta=[meta(0, ALTER_UO, **uo_meta('t9i', 's_cd', 'uq_cd', ['c', 'd']))],
               loaded=2, applicable=1),
        [exec_step('apply', 'ALTER TABLE t9i ADD INDEX s_cd(c,d);')],
        policy='t09-overlap', final_idx={'t9i': uo_final}, gipk=True)

    # ---- existence roles ------------------------------------------------------
    add('exists-create', ALL4, ['t9i'], ['CREATE INDEX i_ab ON t9i(a);'],
        expect(['CREATE INDEX i_ab ON t9i(a);'], verdict='reject', findings=1,
               finding_entries=[finding(0, ADD_EXISTS, 'blocker')],
               finding_meta=[meta(0, ADD_EXISTS, **exists_forbid_meta('t9i', 'create_index', 'i_ab'))],
               loaded=6, applicable=2),
        [exec_step('apply', 'CREATE INDEX i_ab ON t9i(a);', rc=1, stderr=E1061)],
        policy='t09-exists', gipk=True)

    add('exists-alter', ALL4, ['t9i'], ['ALTER TABLE t9i ADD INDEX i_ab(d);'],
        expect(['ALTER TABLE t9i ADD INDEX i_ab(d);'], verdict='reject', findings=1,
               finding_entries=[finding(0, ADD_EXISTS, 'blocker')],
               finding_meta=[meta(0, ADD_EXISTS, **exists_forbid_meta('t9i', 'add_index', 'i_ab'))],
               loaded=6, applicable=2),
        [exec_step('apply', 'ALTER TABLE t9i ADD INDEX i_ab(d);', rc=1, stderr=E1061)],
        policy='t09-exists', gipk=True)

    add('cols-exists-create', ALL4, ['t9i'], ['CREATE INDEX i_bad ON t9i(no_col);'],
        expect(['CREATE INDEX i_bad ON t9i(no_col);'], verdict='reject', findings=1,
               finding_entries=[finding(0, COLS_EXISTS, 'blocker')],
               finding_meta=[meta(0, COLS_EXISTS, **{'column': 'no_col', 'exists': False,
                                                     'index': 'i_bad', 'schema': 'golden', 'table': 't9i'})],
               loaded=6, applicable=2),
        [exec_step('apply', 'CREATE INDEX i_bad ON t9i(no_col);', rc=1, stderr=E1072)],
        policy='t09-exists', gipk=True)

    add('drop-missing', ALL4, ['t9i'], ['ALTER TABLE t9i DROP INDEX gone;'],
        expect(['ALTER TABLE t9i DROP INDEX gone;'], verdict='reject', findings=1,
               finding_entries=[finding(0, DROP_EXISTS, 'blocker')],
               finding_meta=[meta(0, DROP_EXISTS, **exists_require_meta('t9i', 'drop_index', 'gone'))],
               loaded=6, applicable=2),
        [exec_step('apply', 'ALTER TABLE t9i DROP INDEX gone;', rc=1, stderr=E1091)],
        policy='t09-exists', gipk=True)

    add('rename-missing', ALL4, ['t9i'], ['ALTER TABLE t9i RENAME INDEX gone TO g2;'],
        expect(['ALTER TABLE t9i RENAME INDEX gone TO g2;'], verdict='reject', findings=1,
               finding_entries=[finding(0, REN_EXISTS, 'blocker')],
               finding_meta=[meta(0, REN_EXISTS, **exists_require_meta('t9i', 'rename_index', 'gone'))],
               loaded=6, applicable=2),
        [exec_step('apply', 'ALTER TABLE t9i RENAME INDEX gone TO g2;', rc=1, stderr=E1176)],
        policy='t09-exists', gipk=True)

    # ---- ordered-state chains --------------------------------------------------
    add('state-add-then-dup', ALL4, ['t9'],
        ['ALTER TABLE t9 ADD INDEX i1(a);', 'CREATE INDEX i1 ON t9(a);'],
        expect(['ALTER TABLE t9 ADD INDEX i1(a);', 'CREATE INDEX i1 ON t9(a);'],
               verdict='reject', findings=1,
               finding_entries=[finding(1, ADD_EXISTS, 'blocker')],
               finding_meta=[meta(1, ADD_EXISTS, **exists_forbid_meta('t9', 'create_index', 'i1'))],
               loaded=6, applicable=3),
        [exec_step('s0', 'ALTER TABLE t9 ADD INDEX i1(a);',
                   verify=lambda a: [idxcheck(a, 't9', 'i1:1:1:a')]),
         exec_step('s1', 'CREATE INDEX i1 ON t9(a);', rc=1, stderr=E1061)],
        policy='t09-exists', final_idx={'t9': 'i1:1:1:a'}, gipk=True)

    add('state-add-rename-add', ALL4, ['t9'],
        ['ALTER TABLE t9 ADD INDEX i1(a);', 'ALTER TABLE t9 RENAME INDEX i1 TO i2;',
         'CREATE INDEX i1 ON t9(a);'],
        expect(['ALTER TABLE t9 ADD INDEX i1(a);', 'ALTER TABLE t9 RENAME INDEX i1 TO i2;',
                'CREATE INDEX i1 ON t9(a);'], loaded=6, applicable=4),
        [exec_step('s0', 'ALTER TABLE t9 ADD INDEX i1(a);'),
         exec_step('s1', 'ALTER TABLE t9 RENAME INDEX i1 TO i2;',
                   verify=lambda a: [idxcheck(a, 't9', 'i2:1:1:a')]),
         exec_step('s2', 'CREATE INDEX i1 ON t9(a);')],
        policy='t09-exists', final_idx={'t9': 'i1:1:1:a,i2:1:1:a'}, gipk=True)

    add('state-add-drop-add', ALL4, ['t9'],
        ['ALTER TABLE t9 ADD INDEX i1(a);', 'ALTER TABLE t9 DROP INDEX i1;',
         'ALTER TABLE t9 ADD INDEX i1(a);'],
        expect(['ALTER TABLE t9 ADD INDEX i1(a);', 'ALTER TABLE t9 DROP INDEX i1;',
                'ALTER TABLE t9 ADD INDEX i1(a);'], loaded=6, applicable=3),
        [exec_step('s0', 'ALTER TABLE t9 ADD INDEX i1(a);'),
         exec_step('s1', 'ALTER TABLE t9 DROP INDEX i1;',
                   verify=lambda a: [idxcheck(a, 't9', '')]),
         exec_step('s2', 'ALTER TABLE t9 ADD INDEX i1(a);')],
        policy='t09-exists', final_idx={'t9': 'i1:1:1:a'}, gipk=True)

    add('state-create-drop-create', ALL4, ['t9'],
        ['CREATE INDEX i1 ON t9(a);', 'DROP INDEX i1 ON t9;', 'CREATE INDEX i1 ON t9(a);'],
        expect(['CREATE INDEX i1 ON t9(a);', 'DROP INDEX i1 ON t9;', 'CREATE INDEX i1 ON t9(a);'],
               loaded=6, applicable=3),
        [exec_step('s0', 'CREATE INDEX i1 ON t9(a);'),
         exec_step('s1', 'DROP INDEX i1 ON t9;', verify=lambda a: [idxcheck(a, 't9', '')]),
         exec_step('s2', 'CREATE INDEX i1 ON t9(a);')],
        policy='t09-exists', final_idx={'t9': 'i1:1:1:a'}, gipk=True)

    add('state-constraint-then-overlap', ALL4, ['t9'],
        ['ALTER TABLE t9 ADD CONSTRAINT uq_ab UNIQUE(a,b);', 'ALTER TABLE t9 ADD INDEX s_ab(a,b);'],
        expect(['ALTER TABLE t9 ADD CONSTRAINT uq_ab UNIQUE(a,b);', 'ALTER TABLE t9 ADD INDEX s_ab(a,b);'],
               verdict='review', findings=1,
               finding_entries=[finding(1, ALTER_UO, 'warning')],
               finding_meta=[meta(1, ALTER_UO, **uo_meta('t9', 's_ab', 'uq_ab', ['a', 'b']))],
               loaded=2, applicable=1),
        [exec_step('s0', 'ALTER TABLE t9 ADD CONSTRAINT uq_ab UNIQUE(a,b);'),
         exec_step('s1', 'ALTER TABLE t9 ADD INDEX s_ab(a,b);')],
        policy='t09-overlap',
        final_idx={'t9': 's_ab:1:1:a,s_ab:1:2:b,uq_ab:0:1:a,uq_ab:0:2:b'}, gipk=True)

    add('cross-table-samename', ALL4, ['t9', 't9o'],
        ['CREATE INDEX same_i ON t9(a);', 'CREATE INDEX same_i ON t9o(a);'],
        expect(['CREATE INDEX same_i ON t9(a);', 'CREATE INDEX same_i ON t9o(a);'],
               loaded=6, applicable=2),
        [exec_step('s0', 'CREATE INDEX same_i ON t9(a);'),
         exec_step('s1', 'CREATE INDEX same_i ON t9o(a);')],
        policy='t09-exists',
        final_idx={'t9': 'same_i:1:1:a', 't9o': 'same_i:1:1:a'}, gipk=True)

    # ---- lifecycle -------------------------------------------------------------
    # TiDB rejects DROP PRIMARY KEY on a clustered-index table (8200); MySQL
    # anchors accept it. Audit is identical — only the native outcome splits.
    for anchor in MYSQL3:
        cases.append(meta_case('drop-primary', anchor, ['t9p'],
                               ['ALTER TABLE t9p DROP PRIMARY KEY;'],
                               expect(['ALTER TABLE t9p DROP PRIMARY KEY;'], loaded=6, applicable=2),
                               [exec_step('apply', 'ALTER TABLE t9p DROP PRIMARY KEY;')],
                               policy='t09-exists', final_idx={'t9p': ''}))
    cases.append(meta_case('drop-primary', 'tidb85', ['t9p'],
                           ['ALTER TABLE t9p DROP PRIMARY KEY;'],
                           expect(['ALTER TABLE t9p DROP PRIMARY KEY;'], loaded=6, applicable=2),
                           [exec_step('apply', 'ALTER TABLE t9p DROP PRIMARY KEY;', rc=1, stderr=E8200)],
                           policy='t09-exists', final_idx={'t9p': 'PRIMARY:0:1:a'}))

    # MySQL 5.7 accepts RENAME INDEX natively (present since 5.7.0) — all four
    # anchors land the rename and keep the definition's column order.
    renamed = 'i_ab2:1:1:a,i_ab2:1:2:b,i_abc:1:1:a,i_abc:1:2:b,i_abc:1:3:c,uq_cd:0:1:c,uq_cd:0:2:d'
    add('rename-index', ALL4, ['t9i'], ['ALTER TABLE t9i RENAME INDEX i_ab TO i_ab2;'],
        expect(['ALTER TABLE t9i RENAME INDEX i_ab TO i_ab2;'], loaded=3, applicable=0),
        [exec_step('apply', 'ALTER TABLE t9i RENAME INDEX i_ab TO i_ab2;')],
        policy='t09-notice', final_idx={'t9i': renamed}, gipk=True)

    # ---- conditional grammar ----------------------------------------------------
    # IF [NOT] EXISTS is a server-side conditional: the audit still judges the
    # declared premise, while native grammar support splits per product line.
    for anchor in MYSQL3:
        cases.append(meta_case('add-ine', anchor, ['t9i'],
                               ['ALTER TABLE t9i ADD INDEX IF NOT EXISTS i1(a);'],
                               expect(['ALTER TABLE t9i ADD INDEX IF NOT EXISTS i1(a);'],
                                      loaded=6, applicable=2),
                               [exec_step('apply', 'ALTER TABLE t9i ADD INDEX IF NOT EXISTS i1(a);',
                                          rc=1, stderr=E1064)],
                               policy='t09-exists', gipk=True))
    cases.append(meta_case('add-ine', 'tidb85', ['t9i'],
                           ['ALTER TABLE t9i ADD INDEX IF NOT EXISTS i1(a);'],
                           expect(['ALTER TABLE t9i ADD INDEX IF NOT EXISTS i1(a);'],
                                  loaded=6, applicable=2),
                           [exec_step('apply', 'ALTER TABLE t9i ADD INDEX IF NOT EXISTS i1(a);')],
                           policy='t09-exists',
                           final_idx={'t9i': 'i1:1:1:a,' + FIXTURE_IDX['t9i']}, gipk=False))

    for anchor in MYSQL3:
        cases.append(meta_case('drop-ie', anchor, ['t9i'],
                               ['ALTER TABLE t9i DROP INDEX IF EXISTS gone;'],
                               expect(['ALTER TABLE t9i DROP INDEX IF EXISTS gone;'],
                                      verdict='reject', findings=1,
                                      finding_entries=[finding(0, DROP_EXISTS, 'blocker')],
                                      finding_meta=[meta(0, DROP_EXISTS, **exists_require_meta('t9i', 'drop_index', 'gone'))],
                                      loaded=6, applicable=2),
                               [exec_step('apply', 'ALTER TABLE t9i DROP INDEX IF EXISTS gone;',
                                          rc=1, stderr=E1064)],
                               policy='t09-exists', gipk=True))
    cases.append(meta_case('drop-ie', 'tidb85', ['t9i'],
                           ['ALTER TABLE t9i DROP INDEX IF EXISTS gone;'],
                           expect(['ALTER TABLE t9i DROP INDEX IF EXISTS gone;'],
                                  verdict='reject', findings=1,
                                  finding_entries=[finding(0, DROP_EXISTS, 'blocker')],
                                  finding_meta=[meta(0, DROP_EXISTS, **exists_require_meta('t9i', 'drop_index', 'gone'))],
                                  loaded=6, applicable=2),
                           [exec_step('apply', 'ALTER TABLE t9i DROP INDEX IF EXISTS gone;')],
                           policy='t09-exists', gipk=False))

    return cases


def cli_cases():
    fo_warn = ('--fail-on', 'warning')
    parser_neg_expect = {'exit': 2, 'verdict': '', 'statements': 0, 'findings': 0, 'diagnostics': 1,
                         'unsupported': 0, 'coverage': 'incomplete', 'statement_coverage': [],
                         'statement_sql': [], 'statement_indices': [], 'evidence_gaps': 0,
                         'evidence_gap_entries': [], 'finding_entries': [], 'finding_metadata': [],
                         'unsupported_entries': []}
    wide = 'CREATE INDEX idx_wide ON t9(a,b,c,d,e,f,g,h,i); ALTER TABLE t9 ADD INDEX idx_wide2(a,b,c,d,e,f,g,h,i);'
    wide_sqls = ['CREATE INDEX idx_wide ON t9(a,b,c,d,e,f,g,h,i);',
                 'ALTER TABLE t9 ADD INDEX idx_wide2(a,b,c,d,e,f,g,h,i);']
    ok8 = 'CREATE INDEX idx_ok ON t9(a,b,c,d,e,f,g,h); ALTER TABLE t9 ADD INDEX idx_ok2(a,b,c,d,e,f,g,h);'
    ok8_sqls = ['CREATE INDEX idx_ok ON t9(a,b,c,d,e,f,g,h);',
                'ALTER TABLE t9 ADD INDEX idx_ok2(a,b,c,d,e,f,g,h);']
    rows = []
    # First golden path: the same bad-prefix/wide index reports under both
    # entries with per-rule attribution, --fail-on warning exits 1.
    for dialect in ('mysql', 'tidb'):
        rows.append(cli_case('firstpath-' + dialect, dialect, wide,
                             expect(wide_sqls, verdict='review', findings=2,
                                    finding_entries=[finding(0, IDX_COLS, 'warning'),
                                                     finding(1, ALTER_COLS, 'warning')],
                                    finding_meta=[
                                        meta(0, IDX_COLS, **{'actual': 9, 'index': 'idx_wide',
                                                             'kind': 'secondary', 'limit': 8, 'table': 't9'}),
                                        meta(1, ALTER_COLS, **{'actual': 9, 'index': 'idx_wide2',
                                                               'kind': 'secondary', 'limit': 8, 'table': 't9'})],
                                    loaded=2, applicable=2, exit_code=1, fail_on=True),
                             policy='t09-cols8', args=fo_warn))
        rows.append(cli_case('firstpath-8col-' + dialect, dialect, ok8,
                             expect(ok8_sqls, loaded=2, applicable=2),
                             policy='t09-cols8'))
    rows.append(cli_case('firstpath-create-off', 'mysql',
                         'CREATE INDEX idx_wide ON t9(a,b,c,d,e,f,g,h,i);',
                         expect(['CREATE INDEX idx_wide ON t9(a,b,c,d,e,f,g,h,i);'],
                                loaded=1, applicable=0),
                         policy='t09-cols8-create-off'))
    rows.append(cli_case('firstpath-alter-off', 'mysql',
                         'ALTER TABLE t9 ADD INDEX idx_wide2(a,b,c,d,e,f,g,h,i);',
                         expect(['ALTER TABLE t9 ADD INDEX idx_wide2(a,b,c,d,e,f,g,h,i);'],
                                loaded=1, applicable=0),
                         policy='t09-cols8-alter-off'))

    # Naming family: index syntax judged by index rules, constraint syntax by
    # the constraint family — never conflated.
    sec_bad = {'index': 'bad', 'kind': 'secondary', 'prefix': 'idx_', 'table': 't9'}
    for dialect in ('mysql', 'tidb'):
        rows.append(cli_case('name-bad-create-' + dialect, dialect, 'CREATE INDEX bad ON t9(a);',
                             expect(['CREATE INDEX bad ON t9(a);'], verdict='reject', findings=1,
                                    finding_entries=[finding(0, IDX_SEC_PFX, 'blocker')],
                                    finding_meta=[meta(0, IDX_SEC_PFX, **sec_bad)],
                                    loaded=5, applicable=2),
                             policy='t09-naming', args=('--fail-on', 'blocker')))
    rows.append(cli_case('name-good-create', 'mysql', 'CREATE INDEX idx_a ON t9(a);',
                         expect(['CREATE INDEX idx_a ON t9(a);'], loaded=5, applicable=2),
                         policy='t09-naming'))
    for dialect in ('mysql', 'tidb'):
        rows.append(cli_case('name-bad-alter-' + dialect, dialect, 'ALTER TABLE t9 ADD INDEX bad(a);',
                             expect(['ALTER TABLE t9 ADD INDEX bad(a);'], verdict='reject', findings=1,
                                    finding_entries=[finding(0, ALTER_SEC_PFX, 'blocker')],
                                    finding_meta=[meta(0, ALTER_SEC_PFX, **sec_bad)],
                                    loaded=5, applicable=1),
                             policy='t09-naming', args=('--fail-on', 'blocker')))
    rows.append(cli_case('name-unique-create', 'mysql', 'CREATE UNIQUE INDEX bad ON t9(a);',
                         expect(['CREATE UNIQUE INDEX bad ON t9(a);'], verdict='reject', findings=1,
                                finding_entries=[finding(0, IDX_UNQ_PFX, 'blocker')],
                                finding_meta=[meta(0, IDX_UNQ_PFX, **{'index': 'bad', 'kind': 'unique',
                                                                      'prefix': 'uq_', 'table': 't9'})],
                                loaded=5, applicable=2),
                         policy='t09-naming', args=('--fail-on', 'blocker')))
    rows.append(cli_case('name-unique-alter', 'mysql', 'ALTER TABLE t9 ADD UNIQUE KEY bad(a);',
                         expect(['ALTER TABLE t9 ADD UNIQUE KEY bad(a);'], verdict='reject', findings=1,
                                finding_entries=[finding(0, ALTER_UNQ_PFX, 'blocker')],
                                finding_meta=[meta(0, ALTER_UNQ_PFX, **{'index': 'bad', 'kind': 'unique',
                                                                        'prefix': 'uq_', 'table': 't9'})],
                                loaded=5, applicable=1),
                         policy='t09-naming', args=('--fail-on', 'blocker')))
    constraint_meta = {'name': 'badname', 'prefix': 'uc_', 'subject': 'constraint.unique_key', 'table': 't9'}
    for dialect in ('mysql', 'tidb'):
        rows.append(cli_case('constraint-separation-' + dialect, dialect,
                             'ALTER TABLE t9 ADD CONSTRAINT badname UNIQUE(a,b);',
                             expect(['ALTER TABLE t9 ADD CONSTRAINT badname UNIQUE(a,b);'],
                                    verdict='reject', findings=1,
                                    finding_entries=[finding(0, CONSTR_UNQ_PFX, 'blocker')],
                                    finding_meta=[meta(0, CONSTR_UNQ_PFX, **constraint_meta)],
                                    loaded=5, applicable=1),
                             policy='t09-naming', args=('--fail-on', 'blocker')))
    rows.append(cli_case('constraint-good', 'mysql', 'ALTER TABLE t9 ADD CONSTRAINT uc_ab UNIQUE(a,b);',
                         expect(['ALTER TABLE t9 ADD CONSTRAINT uc_ab UNIQUE(a,b);'],
                                loaded=5, applicable=1),
                         policy='t09-naming'))
    rows.append(cli_case('name-custom', 'mysql',
                         'CREATE INDEX idx_a ON t9(a); ALTER TABLE t9 ADD UNIQUE KEY idx_u(b);',
                         expect(['CREATE INDEX idx_a ON t9(a);', 'ALTER TABLE t9 ADD UNIQUE KEY idx_u(b);'],
                                verdict='reject', findings=2,
                                finding_entries=[finding(0, IDX_SEC_PFX, 'blocker'),
                                                 finding(1, ALTER_UNQ_PFX, 'blocker')],
                                finding_meta=[
                                    meta(0, IDX_SEC_PFX, **{'index': 'idx_a', 'kind': 'secondary',
                                                            'prefix': 'ix_', 'table': 't9'}),
                                    meta(1, ALTER_UNQ_PFX, **{'index': 'idx_u', 'kind': 'unique',
                                                              'prefix': 'uk_', 'table': 't9'})],
                                loaded=5, applicable=3),
                         policy='t09-naming-custom', args=('--fail-on', 'blocker')))
    rows.append(cli_case('name-pattern-keyword', 'mysql', 'CREATE INDEX bad ON t9(a);',
                         expect(['CREATE INDEX bad ON t9(a);'], verdict='reject', findings=1,
                                finding_entries=[finding(0, IDX_PATTERN, 'blocker')],
                                finding_meta=[meta(0, IDX_PATTERN, **{'name': 'bad', 'pattern': '^idx_',
                                                                      'subject': 'index', 'table': 't9'})],
                                loaded=2, applicable=2),
                         policy='t09-naming-pattern', args=('--fail-on', 'blocker')))

    # Physical-set roles: within-statement pairs, key-order distinction, and
    # offline evidence gaps for the unknowable existing set.
    for dialect in ('mysql', 'tidb'):
        rows.append(cli_case('dup-create-table-' + dialect, dialect,
                             'CREATE TABLE t9x(a INT, b INT, KEY i1(a,b), KEY i2(a,b));',
                             expect(['CREATE TABLE t9x(a INT, b INT, KEY i1(a,b), KEY i2(a,b));'],
                                    verdict='review', findings=1,
                                    finding_entries=[finding(0, IDX_DUP, 'warning')],
                                    finding_meta=[meta(0, IDX_DUP, **dup_meta('t9x', 'i1', 'i2', ['a', 'b']))],
                                    loaded=2, applicable=1),
                             policy='t09-dup'))
    rows.append(cli_case('dup-alter-within', 'mysql',
                         'ALTER TABLE t9 ADD INDEX i1(a,b), ADD INDEX i2(a,b);',
                         expect(['ALTER TABLE t9 ADD INDEX i1(a,b), ADD INDEX i2(a,b);'],
                                verdict='review', coverage='unverified', findings=1,
                                finding_entries=[finding(0, ALTER_DUP, 'warning')],
                                finding_meta=[meta(0, ALTER_DUP, **dup_meta('t9', 'i1', 'i2', ['a', 'b']))],
                                gap_entries=[gap(0, ALTER_DUP)],
                                loaded=2, applicable=1),
                         policy='t09-dup'))
    rows.append(cli_case('keyorder-neg', 'mysql',
                         'CREATE TABLE t9x(a INT, b INT, KEY i_ab(a,b), KEY i_ba(b,a));',
                         expect(['CREATE TABLE t9x(a INT, b INT, KEY i_ab(a,b), KEY i_ba(b,a));'],
                                loaded=2, applicable=1),
                         policy='t09-dup'))
    rows.append(cli_case('lp-create-table', 'mysql',
                         'CREATE TABLE t9x(a INT, b INT, c INT, KEY i_a(a), KEY i_ab(a,b));',
                         expect(['CREATE TABLE t9x(a INT, b INT, c INT, KEY i_a(a), KEY i_ab(a,b));'],
                                verdict='review', findings=1,
                                finding_entries=[finding(0, IDX_LP, 'warning')],
                                finding_meta=[meta(0, IDX_LP, **lp_meta('t9x', 'i_a', 'i_ab', ['a'], ['a', 'b']))],
                                loaded=2, applicable=1),
                         policy='t09-redundant'))
    for dialect in ('mysql', 'tidb'):
        rows.append(cli_case('uo-create-table-' + dialect, dialect,
                             'CREATE TABLE t9x(a INT, b INT, UNIQUE KEY u_ab(a,b), KEY s_ab(a,b));',
                             expect(['CREATE TABLE t9x(a INT, b INT, UNIQUE KEY u_ab(a,b), KEY s_ab(a,b));'],
                                    verdict='review', findings=1,
                                    finding_entries=[finding(0, IDX_UO, 'warning')],
                                    finding_meta=[meta(0, IDX_UO, **uo_meta('t9x', 's_ab', 'u_ab', ['a', 'b']))],
                                    loaded=2, applicable=1),
                             policy='t09-overlap'))
    for dialect in ('mysql', 'tidb'):
        rows.append(cli_case('uo-alter-within-' + dialect, dialect,
                             'ALTER TABLE t9 ADD CONSTRAINT uq_ab UNIQUE(a,b), ADD INDEX s_ab(a,b);',
                             expect(['ALTER TABLE t9 ADD CONSTRAINT uq_ab UNIQUE(a,b), ADD INDEX s_ab(a,b);'],
                                    verdict='review', coverage='unverified', findings=1,
                                    finding_entries=[finding(0, ALTER_UO, 'warning')],
                                    finding_meta=[meta(0, ALTER_UO, **uo_meta('t9', 's_ab', 'uq_ab', ['a', 'b']))],
                                    gap_entries=[gap(0, ALTER_UO)],
                                    loaded=2, applicable=1),
                             policy='t09-overlap'))
    rows.append(cli_case('uo-keyorder-neg', 'mysql',
                         'ALTER TABLE t9 ADD CONSTRAINT uq_ba UNIQUE(b,a), ADD INDEX s_ab(a,b);',
                         expect(['ALTER TABLE t9 ADD CONSTRAINT uq_ba UNIQUE(b,a), ADD INDEX s_ab(a,b);'],
                                verdict='review', coverage='unverified',
                                gap_entries=[gap(0, ALTER_UO)],
                                loaded=2, applicable=1),
                         policy='t09-overlap'))
    rows.append(cli_case('gap-create-offline', 'mysql', 'CREATE INDEX s ON t9(a,b);',
                         expect(['CREATE INDEX s ON t9(a,b);'], verdict='review',
                                coverage='unverified', gap_entries=[gap(0, IDX_UO)],
                                loaded=2, applicable=1),
                         policy='t09-overlap'))
    rows.append(cli_case('gap-dup-offline', 'mysql', 'CREATE INDEX s ON t9(a,b);',
                         expect(['CREATE INDEX s ON t9(a,b);'], verdict='review',
                                coverage='unverified', gap_entries=[gap(0, IDX_DUP)],
                                loaded=2, applicable=1),
                         policy='t09-dup'))
    rows.append(cli_case('err-syntax', 'mysql', 'CREATE INDEX x ON t9(',
                         parser_neg_expect, policy='t09-cols8'))
    return rows


def cli_case(cid, dialect, sql, exp, policy=None, args=()):
    case = {'id': cid, 'dialect': dialect, 'sql': sql, 'expect': exp}
    if policy:
        case['policy'] = policy
    if args:
        case['args'] = list(args)
    return case


def error_cases():
    return [
        {'id': 'policy-invalid', 'dialect': 'mysql', 'sql': 'CREATE INDEX i1 ON t9(a);',
         'args': ['--config', '/nonexistent-t09-policy.yaml'],
         'expect': {'exit': 2, 'stderr_contains': ['load policy']}},
        {'id': 'provider-failure', 'dialect': 'mysql', 'sql': 'ALTER TABLE t9i DROP INDEX gone;',
         'args': ['--host', '127.0.0.1', '--port', '1', '--user', 'root', '--schema', 'golden'],
         'expect': {'exit': 3, 'stderr_contains': ['connection refused']}},
    ]


def manifest():
    baseline = json.loads((Path(__file__).resolve().parents[1] / 'testdata/ddl-golden/T07.json').read_text())
    anchors = copy.deepcopy(baseline['anchors'])
    for name, anchor in anchors.items():
        if name != 'tidb85':
            anchor['exec_password_env'] = 'T09_MYSQL_PASSWORD'
        else:
            anchor.pop('exec_password_env', None)
    metas = metadata_cases()
    clis = cli_cases()
    errs = error_cases()
    ids = [f'T09.db.{anchor}.{suffix}' for anchor in VERSIONS for suffix in ('ddl', 'syntax_negative')]
    ids += ['T09.cli.' + row['id'] for row in clis]
    ids += ['T09.clierr.' + row['id'] for row in errs]
    ids += ['T09.meta.' + row['id'] for row in metas]
    assert len(ids) == len(set(ids)) == FROZEN_REQUIRED_CASES, (len(ids), len(set(ids)))
    assert hashlib.sha256('\n'.join(ids).encode()).hexdigest() == FROZEN_IDS_SHA256
    return {
        'task_id': 'T09',
        'issue': 'https://github.com/Fanduzi/DeltaScope/issues/88',
        'policy_profile': 'all-rules-disabled',
        'description': 'Frozen index-governance semantics: standalone CREATE INDEX unified with ALTER '
                       'ADD INDEX under one vocabulary — naming, column count, duplicate, left-prefix '
                       'and unique-overlap redundancy, existence, lifecycle forbids/notices, explicit '
                       'CONSTRAINT distinction, and per-statement ordered index state; independent T09 '
                       'acceptance contract',
        'anchors': anchors,
        'ddl_steps': copy.deepcopy(baseline['ddl_steps']),
        'syntax_negative': copy.deepcopy(baseline['syntax_negative']),
        'cli_audit': copy.deepcopy(baseline['cli_audit']),
        'policy': {'profiles': profiles()},
        'cli_cases': clis,
        'metadata_cases': metas,
        'error_cases': errs,
        'required_case_ids': ids,
    }


def strict_equal(left, right):
    if type(left) is not type(right):
        return False
    if isinstance(left, dict):
        return left.keys() == right.keys() and all(strict_equal(left[key], right[key]) for key in left)
    if isinstance(left, list):
        return len(left) == len(right) and all(strict_equal(a, b) for a, b in zip(left, right))
    return left == right


def t09_manifest_failures(data):
    if data.get('task_id') != 'T09':
        return []
    want = manifest()
    return [] if strict_equal(data, want) else ['T09 manifest differs from independent frozen role/oracle constants']


def t09_artifact_failures(artifact, data):
    if artifact.get('task_id') != 'T09' and data.get('task_id') != 'T09':
        return []
    if data.get('task_id') != 'T09':
        return ['T09 manifest task identity']
    errors = t09_manifest_failures(data)
    if errors:
        return errors

    def need(condition, message):
        if not condition:
            errors.append('T09 ' + message)

    need(artifact.get('task_id') == 'T09', 'task identity')
    records = artifact.get('cases')
    if not isinstance(records, list) or not all(isinstance(row, dict) for row in records):
        return errors + ['T09 malformed case collection']
    need(type(artifact.get('executed_count')) is int and artifact['executed_count'] == FROZEN_REQUIRED_CASES,
         'strict executed_count/denominator')
    ids = [row.get('case_id') for row in records]
    if not all(type(value) is str for value in ids):
        return errors + ['T09 case IDs must be strings']
    need(len(ids) == FROZEN_REQUIRED_CASES and len(set(ids)) == FROZEN_REQUIRED_CASES
         and set(ids) == set(data['required_case_ids']), 'complete unique case identity set')
    if errors:
        return errors
    spec_by_id = {}
    for spec in metadata_cases():
        spec_by_id['T09.meta.' + spec['id']] = spec
    for spec in cli_cases():
        spec_by_id['T09.cli.' + spec['id']] = spec
    for spec in error_cases():
        spec_by_id['T09.clierr.' + spec['id']] = spec
    for row in records:
        cid = row['case_id']
        need(row.get('status') == 'pass', f'{cid} status must be pass')
        spec = spec_by_id.get(cid)
        if spec is None:
            continue
        need(row.get('expected') == spec['expect'], f'{cid} recorded expectation differs from frozen contract')
        need(row.get('input_sql') == spec['sql'], f'{cid} recorded input SQL differs from frozen contract')
        for assertion in row.get('assertions') or []:
            need(assertion.get('ok') is True, f"{cid} failed assertion {assertion.get('name')!r}")
    return errors


def write_frozen_files():
    root = Path(__file__).resolve().parents[1]
    data = manifest()
    (root / 'testdata/ddl-golden/T09.json').write_text(json.dumps(data, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'required_cases': len(data['required_case_ids']),
                      'cli_cases': len(data['cli_cases']),
                      'metadata_cases': len(data['metadata_cases']),
                      'error_cases': len(data['error_cases']),
                      'id_sha256': hashlib.sha256('\n'.join(data['required_case_ids']).encode()).hexdigest()}, indent=2))


if __name__ == '__main__':
    write_frozen_files()
