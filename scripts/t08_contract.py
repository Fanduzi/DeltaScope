import copy
import hashlib
import json
from pathlib import Path

SET_FORBID = 'ddl.alter.set_default.explicit_default_change.forbid'
DROP_FORBID = 'ddl.alter.drop_default.explicit_default_change.forbid'
MOD_DEF = 'ddl.alter.modify_column.explicit_default_change.forbid'
CHG_DEF = 'ddl.alter.change_column.explicit_default_change.forbid'
MOD_NULL = 'ddl.alter.modify_column.explicit_nullability_change.forbid'
CHG_NULL = 'ddl.alter.change_column.explicit_nullability_change.forbid'
NULL_ADV = 'ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory'
MOD_AI = 'ddl.alter.modify_column.explicit_auto_increment_change.forbid'
CHG_AI = 'ddl.alter.change_column.explicit_auto_increment_change.forbid'
MOD_COMPAT = 'ddl.alter.modify_column.compatibility.require'
CHG_COMPAT = 'ddl.alter.change_column.compatibility.require'
SET_EXISTS = 'ddl.alter.set_default.exists.require'
DROP_EXISTS = 'ddl.alter.drop_default.exists.require'
POS_EXISTS = 'ddl.alter.column_position.exists.require'
TBL_EXISTS = 'ddl.table.exists.alter.require'
MOD_EXISTS = 'ddl.alter.modify_column.exists.require'
CHG_EXISTS = 'ddl.alter.change_column.exists.require'
MOD_FAMILY = 'ddl.alter.modify_column.target_type_family.allowlist'
CHG_FAMILY = 'ddl.alter.change_column.target_type_family.allowlist'
REN_VER = 'ddl.alter.rename_column.version.require'
UNAUDITED_REASON = 'parsed by the shared parser but not covered by audited semantics'
ADV = 'explicit_nullability_change'

VERSIONS = {'mysql57': '5.7.44', 'mysql80': '8.0.46', 'mysql84': '8.4.10', 'tidb85': 'v8.5.0'}
PORTS = {'mysql57': 23357, 'mysql80': 23380, 'mysql84': 23384, 'tidb85': 24000}
DIALECTS = {'mysql57': 'mysql', 'mysql80': 'mysql', 'mysql84': 'mysql', 'tidb85': 'tidb'}
ALL4 = tuple(VERSIONS)
NO80 = ('mysql57', 'mysql84', 'tidb85')
M57_84 = ('mysql57', 'mysql84')
M84_T = ('mysql84', 'tidb85')
NO57 = ('mysql80', 'mysql84', 'tidb85')
MYSQL3 = ('mysql57', 'mysql80', 'mysql84')
M57 = ('mysql57',)
M84 = ('mysql84',)

FROZEN_REQUIRED_CASES = 172
FROZEN_IDS_SHA256 = 'fb4721077cb94d0a6dab9cc826fea8f16d3bb13615da20d71d9749e22d31b18c'

FIXTURE_DEFS = {
    't': 'CREATE TABLE t (c INT NULL DEFAULT 0, keep_c INT NULL);',
    't8': 'CREATE TABLE t8 (pk INT NOT NULL AUTO_INCREMENT, c INT NULL DEFAULT 0, keep_c INT NULL, v VARCHAR(32) NULL, u INT UNSIGNED NOT NULL DEFAULT 0, s VARCHAR(16) NULL, PRIMARY KEY (pk));',
    't8p': 'CREATE TABLE t8p (a INT, b INT, c INT);',
    't8e': 'CREATE TABLE t8e (keep_c INT NULL);',
}
# name:ordinal_position:is_nullable:IFNULL(HEX(column_default),'x00'):LOWER(extra)
FIXTURE_COLS = {
    't': 'c:1:YES:30:|keep_c:2:YES:x00:',
    't8': 'pk:1:NO:x00:auto_increment|c:2:YES:30:|keep_c:3:YES:x00:|v:4:YES:x00:|u:5:NO:30:|s:6:YES:x00:',
    't8p': 'a:1:YES:x00:|b:2:YES:x00:|c:3:YES:x00:',
    't8e': 'keep_c:1:YES:x00:',
}
IDX_FIXTURE = {'t': '', 't8': 'PRIMARY:1:pk', 't8p': '', 't8e': ''}

E1054 = ('ERROR 1054',)
E1146 = ('ERROR 1146',)
E1060 = ('ERROR 1060',)
E1171 = ('ERROR 1171',)
E1064 = ('ERROR 1064',)
E8200 = ('ERROR 8200',)


def on(level, **params):
    return {'enabled': True, 'level': level, 'params': params}


def profiles():
    raw = {
        't08-set-default': {SET_FORBID: on('blocker', forbid=True)},
        't08-drop-default': {DROP_FORBID: on('blocker', forbid=True)},
        't08-modify-default': {MOD_DEF: on('blocker', forbid=True)},
        't08-change-default': {CHG_DEF: on('blocker', forbid=True)},
        't08-default-all': {
            SET_FORBID: on('blocker', forbid=True), DROP_FORBID: on('blocker', forbid=True),
            MOD_DEF: on('blocker', forbid=True), CHG_DEF: on('blocker', forbid=True)},
        't08-set-default-param-off': {SET_FORBID: on('blocker', forbid=False)},
        't08-nullability-all': {
            MOD_NULL: on('blocker', forbid=True), CHG_NULL: on('blocker', forbid=True),
            NULL_ADV: on('notice')},
        't08-ai': {MOD_AI: on('blocker', forbid=True), CHG_AI: on('blocker', forbid=True)},
        't08-compat': {MOD_COMPAT: on('warning', required=True), CHG_COMPAT: on('warning', required=True)},
        't08-existence': {
            SET_EXISTS: on('blocker'), DROP_EXISTS: on('blocker'),
            POS_EXISTS: on('blocker'), TBL_EXISTS: on('blocker')},
        # t08-all is the frozen T08 review surface: metadata compat warnings,
        # member/table existence blockers, the family allowlist, and the rename
        # version gate. Explicit-op forbid rules stay out so reviewed
        # transitions surface as warnings, not blockers.
        't08-all': {
            MOD_COMPAT: on('warning', required=True), CHG_COMPAT: on('warning', required=True),
            MOD_EXISTS: on('blocker'), CHG_EXISTS: on('blocker'),
            SET_EXISTS: on('blocker'), DROP_EXISTS: on('blocker'),
            POS_EXISTS: on('blocker'), TBL_EXISTS: on('blocker'),
            MOD_FAMILY: on('warning'), CHG_FAMILY: on('warning'),
            REN_VER: on('blocker', required=True)},
    }
    return {name: {'enable': rules} for name, rules in raw.items()}


def connect(anchor):
    conn = {'host': '127.0.0.1', 'port': PORTS[anchor], 'user': 'root', 'schema': 'golden'}
    if anchor != 'tidb85':
        conn['password_env'] = 'T08_MYSQL_PASSWORD'
    return conn


def colq(table):
    return ("SELECT COALESCE(GROUP_CONCAT(CONCAT(column_name,':',ordinal_position,':',"
            "is_nullable,':',IFNULL(HEX(column_default),'x00'),':',LOWER(extra)) "
            "ORDER BY ordinal_position SEPARATOR '|'),'') FROM information_schema.columns "
            "WHERE table_schema='golden' AND table_name='" + table + "'")


def idxq(table):
    return ("SELECT COALESCE(GROUP_CONCAT(CONCAT(index_name,':',seq_in_index,':',column_name) "
            "ORDER BY CAST(index_name AS BINARY),seq_in_index SEPARATOR ','),'') "
            "FROM information_schema.statistics WHERE table_schema='golden' AND table_name='" + table + "'")


def colcheck(table, want):
    return {'assert': table + ' ordered column set', 'sql': colq(table), 'expect': want}


def tcount(table, want):
    return {'assert': table + ' bound table count',
            'sql': "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='golden' AND table_name='" + table + "'",
            'expect': want}


def idxcheck(table, want):
    return {'assert': table + ' index identity', 'sql': idxq(table), 'expect': want}


def expect(sqls, verdict='pass', coverage='complete', findings=0, gap_entries=(),
           finding_entries=(), finding_meta=(), unsupported_entries=(), diagnostics=0,
           loaded=None, applicable=None):
    e = {'exit': 1 if (verdict == 'reject' or coverage == 'incomplete') else 0,
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
    return e


def finding(index, rule_id, level):
    return {'index': index, 'rule_id': rule_id, 'level': level}


def meta(index, rule_id, **metadata):
    return {'index': index, 'rule_id': rule_id, 'metadata': metadata}


def forbid_meta(table, action, name, column, kind):
    return {'table': table, 'action': action, 'name': name, 'column_name': column, 'change_kind': kind}


def exists_meta(table, action, name):
    return {'table': table, 'action': action, 'name': name, 'exists': False}


def compat_meta(table, name, **extra):
    return dict({'table': table, 'action': 'modify_column', 'name': name, 'column_name': name}, **extra)


def setup_fixture(table):
    """Absence proof, then create the owned fixture and verify its column set —
    the audit below runs metadata-aware, so the fixture must exist in setup."""
    return [
        {'name': 'prove-prior-absence-' + table, 'sql': tcount(table, '0')['sql'], 'expect_rc': 0,
         'verify': [tcount(table, '0')]},
        {'name': 'create-fixture-' + table, 'sql': FIXTURE_DEFS[table], 'expect_rc': 0,
         'verify': [tcount(table, '1'), colcheck(table, FIXTURE_COLS[table])]},
    ]


def teardown_fixture(*tables):
    return [{'name': 'drop-' + table, 'sql': 'DROP TABLE ' + table + ';', 'expect_rc': 0,
             'verify': [tcount(table, '0')]} for table in tables]


def exec_step(name, sql, rc=0, stderr=()):
    step = {'name': name, 'sql': sql, 'expect_rc': rc}
    if stderr:
        step['stderr_contains'] = list(stderr)
    return step


def meta_case(role, anchor, table, sqls, exp, exec_steps, final_cols, policy=None,
              structure_extra=(), gipk_check=False):
    """One metadata case: setup proves absence and creates the fixture, the CLI
    audits against the live table, post_verify proves the audit did not mutate
    it, execute applies the audited statements natively, structure pins the
    resulting catalog, teardown drops owned tables."""
    case = {'id': 't08-' + anchor + '-' + role, 'anchor': anchor, 'dialect': DIALECTS[anchor],
            'sql': '\n'.join(sqls), 'connect': connect(anchor), 'expect': copy.deepcopy(exp)}
    if policy:
        case['policy'] = policy
    case['setup'] = setup_fixture(table) if table else []
    case['post_verify'] = ([colcheck(table, FIXTURE_COLS[table]), idxcheck(table, IDX_FIXTURE[table])]
                           if table else [])
    case['execute'] = list(exec_steps)
    structure = []
    if final_cols is not None:
        structure.append(colcheck(table, final_cols))
    structure += list(structure_extra)
    case['structure'] = structure
    case['teardown'] = teardown_fixture(table) if table else []
    if gipk_check and anchor in ('mysql80', 'mysql84'):
        case['expect']['instance_facts'] = {'sql_generate_invisible_primary_key': 'OFF'}
    return case


def metadata_cases():
    cases = []

    def add(role, anchors, table, sqls, exp, exec_steps, final_cols, policy=None, **kw):
        for anchor in anchors:
            cases.append(meta_case(role, anchor, table, sqls, exp, exec_steps, final_cols,
                                   policy=policy, **kw))

    # ---- explicit-op forbid first path ------------------------------------
    add('fp-set-blocker', ALL4, 't', ['ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;'],
        expect(['ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;'], verdict='reject', findings=1,
               finding_entries=[finding(0, SET_FORBID, 'blocker')],
               finding_meta=[meta(0, SET_FORBID, **forbid_meta('t', 'set_column_default', 'c', 'c', 'explicit_default_change'))],
               loaded=1, applicable=1),
        [exec_step('apply', 'ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;')],
        'c:1:YES:31:|keep_c:2:YES:x00:', policy='t08-set-default')

    add('fp-modify-equiv', M84_T, 't', ['ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 1;'],
        expect(['ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 1;'], verdict='reject', findings=1,
               finding_entries=[finding(0, MOD_DEF, 'blocker')],
               finding_meta=[meta(0, MOD_DEF, **forbid_meta('t', 'modify_column', 'c', 'c', 'explicit_default_change'))],
               loaded=1, applicable=1),
        [exec_step('apply', 'ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 1;')],
        'c:1:YES:31:|keep_c:2:YES:x00:', policy='t08-modify-default')

    add('fp-drop-blocker', M84_T, 't', ['ALTER TABLE t ALTER COLUMN c DROP DEFAULT;'],
        expect(['ALTER TABLE t ALTER COLUMN c DROP DEFAULT;'], verdict='reject', findings=1,
               finding_entries=[finding(0, DROP_FORBID, 'blocker')],
               finding_meta=[meta(0, DROP_FORBID, **forbid_meta('t', 'drop_column_default', 'c', 'c', 'explicit_default_change'))],
               loaded=1, applicable=1),
        [exec_step('apply', 'ALTER TABLE t ALTER COLUMN c DROP DEFAULT;')],
        'c:1:YES:x00:|keep_c:2:YES:x00:', policy='t08-drop-default')

    add('fp-non-target', M84, 't', ['ALTER TABLE t ADD COLUMN other_c INT NULL;'],
        expect(['ALTER TABLE t ADD COLUMN other_c INT NULL;'], loaded=4, applicable=0),
        [exec_step('apply', 'ALTER TABLE t ADD COLUMN other_c INT NULL;')],
        'c:1:YES:30:|keep_c:2:YES:x00:|other_c:3:YES:x00:', policy='t08-default-all')

    # ---- setter catalog facts ----------------------------------------------
    add('setter-set-default', ALL4, 't', ['ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;'],
        expect(['ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;']),
        [exec_step('apply', 'ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;')],
        'c:1:YES:31:|keep_c:2:YES:x00:')

    add('setter-set-null', NO80, 't', ['ALTER TABLE t ALTER COLUMN c SET DEFAULT NULL;'],
        expect(['ALTER TABLE t ALTER COLUMN c SET DEFAULT NULL;']),
        [exec_step('apply', 'ALTER TABLE t ALTER COLUMN c SET DEFAULT NULL;')],
        'c:1:YES:x00:|keep_c:2:YES:x00:')

    add('setter-set-strnull', NO80, 't8', ["ALTER TABLE t8 ALTER COLUMN s SET DEFAULT 'NULL';"],
        expect(["ALTER TABLE t8 ALTER COLUMN s SET DEFAULT 'NULL';"]),
        [exec_step('apply', "ALTER TABLE t8 ALTER COLUMN s SET DEFAULT 'NULL';")],
        'pk:1:NO:x00:auto_increment|c:2:YES:30:|keep_c:3:YES:x00:|v:4:YES:x00:|u:5:NO:30:|s:6:YES:4E554C4C:')

    add('setter-drop', NO80, 't', ['ALTER TABLE t ALTER COLUMN c DROP DEFAULT;'],
        expect(['ALTER TABLE t ALTER COLUMN c DROP DEFAULT;']),
        [exec_step('apply', 'ALTER TABLE t ALTER COLUMN c DROP DEFAULT;')],
        'c:1:YES:x00:|keep_c:2:YES:x00:')

    # ---- existence rules -----------------------------------------------------
    add('setter-missing-column', ALL4, 't8e', ['ALTER TABLE t8e ALTER COLUMN c SET DEFAULT 1;'],
        expect(['ALTER TABLE t8e ALTER COLUMN c SET DEFAULT 1;'], verdict='reject', findings=1,
               finding_entries=[finding(0, SET_EXISTS, 'blocker')],
               finding_meta=[meta(0, SET_EXISTS, **exists_meta('t8e', 'set_column_default', 'c'))],
               loaded=4, applicable=2),
        [exec_step('apply', 'ALTER TABLE t8e ALTER COLUMN c SET DEFAULT 1;', rc=1, stderr=E1054)],
        'keep_c:1:YES:x00:', policy='t08-existence', gipk_check=True)

    add('setter-missing-table', ALL4, None, ['ALTER TABLE no_such_t ALTER COLUMN c SET DEFAULT 1;'],
        expect(['ALTER TABLE no_such_t ALTER COLUMN c SET DEFAULT 1;'], verdict='reject', findings=1,
               finding_entries=[finding(0, TBL_EXISTS, 'blocker')],
               loaded=4, applicable=2),
        [exec_step('apply', 'ALTER TABLE no_such_t ALTER COLUMN c SET DEFAULT 1;', rc=1, stderr=E1146)],
        None, policy='t08-existence', structure_extra=[tcount('no_such_t', '0')])

    add('drop-missing-column', NO80, 't8e', ['ALTER TABLE t8e ALTER COLUMN c DROP DEFAULT;'],
        expect(['ALTER TABLE t8e ALTER COLUMN c DROP DEFAULT;'], verdict='reject', findings=1,
               finding_entries=[finding(0, DROP_EXISTS, 'blocker')],
               finding_meta=[meta(0, DROP_EXISTS, **exists_meta('t8e', 'drop_column_default', 'c'))],
               loaded=4, applicable=2),
        [exec_step('apply', 'ALTER TABLE t8e ALTER COLUMN c DROP DEFAULT;', rc=1, stderr=E1054)],
        'keep_c:1:YES:x00:', policy='t08-existence', gipk_check=True)

    # ---- modify transitions under t08-all ------------------------------------
    add('modify-notnull', ALL4, 't8', ['ALTER TABLE t8 MODIFY COLUMN c INT NOT NULL;'],
        expect(['ALTER TABLE t8 MODIFY COLUMN c INT NOT NULL;'], verdict='review', findings=2,
               finding_entries=[finding(0, MOD_COMPAT, 'warning'), finding(0, MOD_COMPAT, 'warning')],
               finding_meta=[
                   meta(0, MOD_COMPAT, **compat_meta('t8', 'c', source_not_null=False, target_not_null=True)),
                   meta(0, MOD_COMPAT, **compat_meta('t8', 'c', source_default='literal', target_default='none'))],
               loaded=11),
        [exec_step('apply', 'ALTER TABLE t8 MODIFY COLUMN c INT NOT NULL;')],
        'pk:1:NO:x00:auto_increment|c:2:NO:x00:|keep_c:3:YES:x00:|v:4:YES:x00:|u:5:NO:30:|s:6:YES:x00:',
        policy='t08-all')

    add('modify-null', NO80, 't8', ['ALTER TABLE t8 MODIFY COLUMN u INT NULL;'],
        expect(['ALTER TABLE t8 MODIFY COLUMN u INT NULL;'], verdict='review', findings=2,
               finding_entries=[finding(0, MOD_COMPAT, 'warning'), finding(0, MOD_COMPAT, 'warning')],
               finding_meta=[
                   meta(0, MOD_COMPAT, **compat_meta('t8', 'u', source_unsigned=True, target_unsigned=False)),
                   meta(0, MOD_COMPAT, **compat_meta('t8', 'u', source_default='literal', target_default='none'))],
               loaded=11),
        [exec_step('apply', 'ALTER TABLE t8 MODIFY COLUMN u INT NULL;')],
        'pk:1:NO:x00:auto_increment|c:2:YES:30:|keep_c:3:YES:x00:|v:4:YES:x00:|u:5:YES:x00:|s:6:YES:x00:',
        policy='t08-all')

    add('modify-shrink', M57_84, 't8', ['ALTER TABLE t8 MODIFY COLUMN v VARCHAR(8);'],
        expect(['ALTER TABLE t8 MODIFY COLUMN v VARCHAR(8);'], verdict='review', findings=1,
               finding_entries=[finding(0, MOD_COMPAT, 'warning')],
               finding_meta=[meta(0, MOD_COMPAT, **compat_meta('t8', 'v', source_length=32, target_length=8))],
               loaded=11),
        [exec_step('apply', 'ALTER TABLE t8 MODIFY COLUMN v VARCHAR(8);')],
        'pk:1:NO:x00:auto_increment|c:2:YES:30:|keep_c:3:YES:x00:|v:4:YES:x00:|u:5:NO:30:|s:6:YES:x00:',
        policy='t08-all')

    add('modify-widen', M57, 't8', ['ALTER TABLE t8 MODIFY COLUMN v VARCHAR(64);'],
        expect(['ALTER TABLE t8 MODIFY COLUMN v VARCHAR(64);'], loaded=11),
        [exec_step('apply', 'ALTER TABLE t8 MODIFY COLUMN v VARCHAR(64);')],
        'pk:1:NO:x00:auto_increment|c:2:YES:30:|keep_c:3:YES:x00:|v:4:YES:x00:|u:5:NO:30:|s:6:YES:x00:',
        policy='t08-all')

    add('modify-family-change', M57_84, 't8', ['ALTER TABLE t8 MODIFY COLUMN s DATETIME;'],
        expect(['ALTER TABLE t8 MODIFY COLUMN s DATETIME;'], verdict='review', findings=1,
               finding_entries=[finding(0, MOD_COMPAT, 'warning')],
               finding_meta=[meta(0, MOD_COMPAT, **compat_meta('t8', 's', source_family='string', target_family='time'))],
               loaded=11),
        [exec_step('apply', 'ALTER TABLE t8 MODIFY COLUMN s DATETIME;')],
        'pk:1:NO:x00:auto_increment|c:2:YES:30:|keep_c:3:YES:x00:|v:4:YES:x00:|u:5:NO:30:|s:6:YES:x00:',
        policy='t08-all')

    add('modify-unsigned-drop', M57_84, 't8', ['ALTER TABLE t8 MODIFY COLUMN u INT;'],
        expect(['ALTER TABLE t8 MODIFY COLUMN u INT;'], verdict='review', findings=2,
               finding_entries=[finding(0, MOD_COMPAT, 'warning'), finding(0, MOD_COMPAT, 'warning')],
               finding_meta=[
                   meta(0, MOD_COMPAT, **compat_meta('t8', 'u', source_unsigned=True, target_unsigned=False)),
                   meta(0, MOD_COMPAT, **compat_meta('t8', 'u', source_default='literal', target_default='none'))],
               loaded=11),
        [exec_step('apply', 'ALTER TABLE t8 MODIFY COLUMN u INT;')],
        'pk:1:NO:x00:auto_increment|c:2:YES:30:|keep_c:3:YES:x00:|v:4:YES:x00:|u:5:YES:x00:|s:6:YES:x00:',
        policy='t08-all')

    add('modify-ai-keep', M57, 't8', ['ALTER TABLE t8 MODIFY COLUMN pk INT NOT NULL AUTO_INCREMENT;'],
        expect(['ALTER TABLE t8 MODIFY COLUMN pk INT NOT NULL AUTO_INCREMENT;'], loaded=11),
        [exec_step('apply', 'ALTER TABLE t8 MODIFY COLUMN pk INT NOT NULL AUTO_INCREMENT;')],
        FIXTURE_COLS['t8'], policy='t08-all')

    # MySQL drops auto_increment on an omitted AUTO_INCREMENT clause; TiDB 8.5
    # rejects the same statement natively (8200). The audit expectation is
    # identical — the warning is about the declared transition — while the
    # native outcome and resulting catalog split per anchor.
    for anchor, rc, err, final in (('mysql57', 0, (), 'pk:1:NO:x00:|c:2:YES:30:|keep_c:3:YES:x00:|v:4:YES:x00:|u:5:NO:30:|s:6:YES:x00:'),
                                   ('mysql84', 0, (), 'pk:1:NO:x00:|c:2:YES:30:|keep_c:3:YES:x00:|v:4:YES:x00:|u:5:NO:30:|s:6:YES:x00:'),
                                   ('tidb85', 1, E8200, FIXTURE_COLS['t8'])):
        cases.append(meta_case('modify-ai-drop', anchor, 't8',
                               ['ALTER TABLE t8 MODIFY COLUMN pk INT NOT NULL;'],
                               expect(['ALTER TABLE t8 MODIFY COLUMN pk INT NOT NULL;'], verdict='review', findings=1,
                                      finding_entries=[finding(0, MOD_COMPAT, 'warning')],
                                      finding_meta=[meta(0, MOD_COMPAT, **compat_meta('t8', 'pk',
                                                                                    source_auto_increment=True, target_auto_increment=False))],
                                      loaded=11),
                               [exec_step('apply', 'ALTER TABLE t8 MODIFY COLUMN pk INT NOT NULL;', rc=rc, stderr=err)],
                               final, policy='t08-all'))

    add('modify-omit-default', ALL4, 't', ['ALTER TABLE t MODIFY COLUMN c INT NULL;'],
        expect(['ALTER TABLE t MODIFY COLUMN c INT NULL;'], verdict='review', findings=1,
               finding_entries=[finding(0, MOD_COMPAT, 'warning')],
               finding_meta=[meta(0, MOD_COMPAT, **compat_meta('t', 'c', source_default='literal', target_default='none'))],
               loaded=11),
        [exec_step('apply', 'ALTER TABLE t MODIFY COLUMN c INT NULL;')],
        'c:1:YES:x00:|keep_c:2:YES:x00:', policy='t08-all', gipk_check=True)

    add('modify-default-same', M57_84, 't', ['ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 0;'],
        expect(['ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 0;'], loaded=11),
        [exec_step('apply', 'ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 0;')],
        'c:1:YES:30:|keep_c:2:YES:x00:', policy='t08-all', gipk_check=True)

    add('modify-default-diff', ALL4, 't', ['ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 7;'],
        expect(['ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 7;'], verdict='review', findings=1,
               finding_entries=[finding(0, MOD_COMPAT, 'warning')],
               finding_meta=[meta(0, MOD_COMPAT, **compat_meta('t', 'c', source_default='literal', target_default='literal'))],
               loaded=11),
        [exec_step('apply', 'ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 7;')],
        'c:1:YES:37:|keep_c:2:YES:x00:', policy='t08-all', gipk_check=True)

    add('modify-default-added', M57, 't8', ['ALTER TABLE t8 MODIFY COLUMN keep_c INT NULL DEFAULT 5;'],
        expect(['ALTER TABLE t8 MODIFY COLUMN keep_c INT NULL DEFAULT 5;'], verdict='review', findings=1,
               finding_entries=[finding(0, MOD_COMPAT, 'warning')],
               finding_meta=[meta(0, MOD_COMPAT, **compat_meta('t8', 'keep_c', source_default='none', target_default='literal'))],
               loaded=11),
        [exec_step('apply', 'ALTER TABLE t8 MODIFY COLUMN keep_c INT NULL DEFAULT 5;')],
        'pk:1:NO:x00:auto_increment|c:2:YES:30:|keep_c:3:YES:35:|v:4:YES:x00:|u:5:NO:30:|s:6:YES:x00:',
        policy='t08-all')

    # Omitting AUTO_INCREMENT on the PK column is the auto_increment-removal
    # warning; the native engines then refuse the NULL redefinition (1171).
    add('modify-pk-null', NO80, 't8', ['ALTER TABLE t8 MODIFY COLUMN pk INT NULL;'],
        expect(['ALTER TABLE t8 MODIFY COLUMN pk INT NULL;'], verdict='review', findings=1,
               finding_entries=[finding(0, MOD_COMPAT, 'warning')],
               finding_meta=[meta(0, MOD_COMPAT, **compat_meta('t8', 'pk',
                                                             source_auto_increment=True, target_auto_increment=False))],
               loaded=11),
        [exec_step('apply', 'ALTER TABLE t8 MODIFY COLUMN pk INT NULL;', rc=1, stderr=E1171)],
        FIXTURE_COLS['t8'], policy='t08-all')

    # ---- position ------------------------------------------------------------
    add('pos-add-first', ALL4, 't8p', ['ALTER TABLE t8p ADD COLUMN x INT FIRST;'],
        expect(['ALTER TABLE t8p ADD COLUMN x INT FIRST;']),
        [exec_step('apply', 'ALTER TABLE t8p ADD COLUMN x INT FIRST;')],
        'x:1:YES:x00:|a:2:YES:x00:|b:3:YES:x00:|c:4:YES:x00:', gipk_check=True)

    add('pos-add-after', ALL4, 't8p', ['ALTER TABLE t8p ADD COLUMN x INT AFTER b;'],
        expect(['ALTER TABLE t8p ADD COLUMN x INT AFTER b;']),
        [exec_step('apply', 'ALTER TABLE t8p ADD COLUMN x INT AFTER b;')],
        'a:1:YES:x00:|b:2:YES:x00:|x:3:YES:x00:|c:4:YES:x00:', gipk_check=True)

    add('pos-add-after-missing', ALL4, 't8p', ['ALTER TABLE t8p ADD COLUMN x INT AFTER missing;'],
        expect(['ALTER TABLE t8p ADD COLUMN x INT AFTER missing;'], verdict='reject', findings=1,
               finding_entries=[finding(0, POS_EXISTS, 'blocker')],
               finding_meta=[meta(0, POS_EXISTS, **exists_meta('t8p', 'add_columns', 'missing'))],
               loaded=4, applicable=2),
        [exec_step('apply', 'ALTER TABLE t8p ADD COLUMN x INT AFTER missing;', rc=1, stderr=E1054)],
        FIXTURE_COLS['t8p'], policy='t08-existence', gipk_check=True)

    add('pos-modify-first', ALL4, 't8p', ['ALTER TABLE t8p MODIFY COLUMN c INT FIRST;'],
        expect(['ALTER TABLE t8p MODIFY COLUMN c INT FIRST;']),
        [exec_step('apply', 'ALTER TABLE t8p MODIFY COLUMN c INT FIRST;')],
        'c:1:YES:x00:|a:2:YES:x00:|b:3:YES:x00:', gipk_check=True)

    add('pos-change-after', M57_84, 't8p', ['ALTER TABLE t8p CHANGE COLUMN a a2 INT AFTER c;'],
        expect(['ALTER TABLE t8p CHANGE COLUMN a a2 INT AFTER c;']),
        [exec_step('apply', 'ALTER TABLE t8p CHANGE COLUMN a a2 INT AFTER c;')],
        'b:1:YES:x00:|c:2:YES:x00:|a2:3:YES:x00:', gipk_check=True)

    add('pos-derived-chain', ALL4, 't8p',
        ['ALTER TABLE t8p ADD COLUMN x INT FIRST;',
         'ALTER TABLE t8p ADD COLUMN y INT AFTER x;',
         'ALTER TABLE t8p ADD COLUMN z INT AFTER phantom;'],
        expect(['ALTER TABLE t8p ADD COLUMN x INT FIRST;',
                'ALTER TABLE t8p ADD COLUMN y INT AFTER x;',
                'ALTER TABLE t8p ADD COLUMN z INT AFTER phantom;'],
               verdict='reject', findings=1,
               finding_entries=[finding(2, POS_EXISTS, 'blocker')],
               finding_meta=[meta(2, POS_EXISTS, **exists_meta('t8p', 'add_columns', 'phantom'))],
               loaded=4),
        [exec_step('s0', 'ALTER TABLE t8p ADD COLUMN x INT FIRST;'),
         exec_step('s1', 'ALTER TABLE t8p ADD COLUMN y INT AFTER x;'),
         exec_step('s2', 'ALTER TABLE t8p ADD COLUMN z INT AFTER phantom;', rc=1, stderr=E1054)],
        'x:1:YES:x00:|y:2:YES:x00:|a:3:YES:x00:|b:4:YES:x00:|c:5:YES:x00:',
        policy='t08-existence', gipk_check=True)

    # ---- multi-action ----------------------------------------------------------
    add('multi-add2', ALL4, 't8p', ['ALTER TABLE t8p ADD COLUMN m1 INT, ADD COLUMN m2 INT;'],
        expect(['ALTER TABLE t8p ADD COLUMN m1 INT, ADD COLUMN m2 INT;']),
        [exec_step('apply', 'ALTER TABLE t8p ADD COLUMN m1 INT, ADD COLUMN m2 INT;')],
        'a:1:YES:x00:|b:2:YES:x00:|c:3:YES:x00:|m1:4:YES:x00:|m2:5:YES:x00:', gipk_check=True)

    add('multi-add-setter', NO80, 't', ['ALTER TABLE t ADD COLUMN a INT, ALTER COLUMN c SET DEFAULT 2;'],
        expect(['ALTER TABLE t ADD COLUMN a INT, ALTER COLUMN c SET DEFAULT 2;']),
        [exec_step('apply', 'ALTER TABLE t ADD COLUMN a INT, ALTER COLUMN c SET DEFAULT 2;')],
        'c:1:YES:32:|keep_c:2:YES:x00:|a:3:YES:x00:', gipk_check=True)

    # Sibling AFTER: MySQL validates left-to-right (c1 visible), TiDB validates
    # against the pre-state (c1 absent -> 1054 and the exists blocker).
    sib_sql = 'ALTER TABLE t8p ADD COLUMN c1 INT, ADD COLUMN c2 INT AFTER c1;'
    for anchor in MYSQL3:
        cases.append(meta_case('multi-add2-after-sibling', anchor, 't8p', [sib_sql],
                               expect([sib_sql], loaded=4, applicable=2),
                               [exec_step('apply', sib_sql)],
                               'a:1:YES:x00:|b:2:YES:x00:|c:3:YES:x00:|c1:4:YES:x00:|c2:5:YES:x00:',
                               policy='t08-existence', gipk_check=True))
    cases.append(meta_case('multi-add2-after-sibling', 'tidb85', 't8p', [sib_sql],
                           expect([sib_sql], verdict='reject', findings=1,
                                  finding_entries=[finding(0, POS_EXISTS, 'blocker')],
                                  finding_meta=[meta(0, POS_EXISTS, **exists_meta('t8p', 'add_columns', 'c1'))],
                                  loaded=4, applicable=2),
                           [exec_step('apply', sib_sql, rc=1, stderr=E1054)],
                           FIXTURE_COLS['t8p'], policy='t08-existence'))

    same_sql = 'ALTER TABLE t8p MODIFY COLUMN a INT, MODIFY COLUMN a BIGINT;'
    for anchor, err in (('mysql57', E1054), ('mysql84', E1054), ('tidb85', E8200)):
        cases.append(meta_case('multi-same-obj', anchor, 't8p', [same_sql],
                               expect([same_sql]),
                               [exec_step('apply', same_sql, rc=1, stderr=err)],
                               FIXTURE_COLS['t8p'], gipk_check=True))

    das_sql = 'ALTER TABLE t8p DROP COLUMN b, ADD COLUMN b INT;'
    for anchor in M57_84:
        cases.append(meta_case('multi-drop-add-same', anchor, 't8p', [das_sql],
                               expect([das_sql]),
                               [exec_step('apply', das_sql)],
                               'a:1:YES:x00:|c:2:YES:x00:|b:3:YES:x00:', gipk_check=True))
    cases.append(meta_case('multi-drop-add-same', 'tidb85', 't8p', [das_sql],
                           expect([das_sql]),
                           [exec_step('apply', das_sql, rc=1, stderr=E1060)],
                           FIXTURE_COLS['t8p']))

    add('multi-modify-x2', M57_84, 't',
        ['ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 9, MODIFY COLUMN keep_c INT NOT NULL;'],
        expect(['ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 9, MODIFY COLUMN keep_c INT NOT NULL;'],
               verdict='review', findings=2,
               finding_entries=[finding(0, MOD_COMPAT, 'warning'), finding(0, MOD_COMPAT, 'warning')],
               finding_meta=[
                   meta(0, MOD_COMPAT, **compat_meta('t', 'c', source_default='literal', target_default='literal')),
                   meta(0, MOD_COMPAT, **compat_meta('t', 'keep_c', source_not_null=False, target_not_null=True))],
               loaded=11),
        [exec_step('apply', 'ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 9, MODIFY COLUMN keep_c INT NOT NULL;')],
        'c:1:YES:39:|keep_c:2:NO:x00:', policy='t08-all', gipk_check=True)

    # ---- ordered-state chains ---------------------------------------------------
    add('chain-setter-then-modify', ALL4, 't',
        ['ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;',
         'ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 1;'],
        expect(['ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;',
                'ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 1;'], loaded=11),
        [exec_step('s0', 'ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;'),
         exec_step('s1', 'ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 1;')],
        'c:1:YES:31:|keep_c:2:YES:x00:', policy='t08-all', gipk_check=True)

    add('chain-drop-then-modify', M57_84, 't',
        ['ALTER TABLE t ALTER COLUMN c DROP DEFAULT;',
         'ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 9;'],
        expect(['ALTER TABLE t ALTER COLUMN c DROP DEFAULT;',
                'ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 9;'],
               verdict='review', findings=1,
               finding_entries=[finding(1, MOD_COMPAT, 'warning')],
               finding_meta=[meta(1, MOD_COMPAT, **compat_meta('t', 'c', source_default='none', target_default='literal'))],
               loaded=11),
        [exec_step('s0', 'ALTER TABLE t ALTER COLUMN c DROP DEFAULT;'),
         exec_step('s1', 'ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 9;')],
        'c:1:YES:39:|keep_c:2:YES:x00:', policy='t08-all', gipk_check=True)

    add('chain-add-rename-index', NO57, 't8p',
        ['ALTER TABLE t8p ADD COLUMN n INT;',
         'ALTER TABLE t8p RENAME COLUMN n TO n2;',
         'CREATE INDEX idx_n ON t8p (n2);'],
        expect(['ALTER TABLE t8p ADD COLUMN n INT;',
                'ALTER TABLE t8p RENAME COLUMN n TO n2;',
                'CREATE INDEX idx_n ON t8p (n2);']),
        [exec_step('s0', 'ALTER TABLE t8p ADD COLUMN n INT;'),
         exec_step('s1', 'ALTER TABLE t8p RENAME COLUMN n TO n2;'),
         exec_step('s2', 'CREATE INDEX idx_n ON t8p (n2);')],
        'a:1:YES:x00:|b:2:YES:x00:|c:3:YES:x00:|n2:4:YES:x00:',
        structure_extra=[idxcheck('t8p', 'idx_n:1:n2')], gipk_check=True)

    add('chain-change-oldname', ALL4, 't8p',
        ['ALTER TABLE t8p CHANGE COLUMN b b2 INT;',
         'ALTER TABLE t8p ALTER COLUMN b SET DEFAULT 1;'],
        expect(['ALTER TABLE t8p CHANGE COLUMN b b2 INT;',
                'ALTER TABLE t8p ALTER COLUMN b SET DEFAULT 1;'],
               verdict='reject', findings=1,
               finding_entries=[finding(1, SET_EXISTS, 'blocker')],
               finding_meta=[meta(1, SET_EXISTS, **exists_meta('t8p', 'set_column_default', 'b'))],
               loaded=4),
        [exec_step('s0', 'ALTER TABLE t8p CHANGE COLUMN b b2 INT;'),
         exec_step('s1', 'ALTER TABLE t8p ALTER COLUMN b SET DEFAULT 1;', rc=1, stderr=E1054)],
        'a:1:YES:x00:|b2:2:YES:x00:|c:3:YES:x00:', policy='t08-existence', gipk_check=True)

    cases.append(meta_case('rename-mysql57-gate', 'mysql57', 't8p',
                           ['ALTER TABLE t8p RENAME COLUMN a TO a2;'],
                           expect(['ALTER TABLE t8p RENAME COLUMN a TO a2;'], verdict='reject', findings=1,
                                  finding_entries=[finding(0, REN_VER, 'blocker')], loaded=11),
                           [exec_step('apply', 'ALTER TABLE t8p RENAME COLUMN a TO a2;', rc=1, stderr=E1064)],
                           FIXTURE_COLS['t8p'], policy='t08-all'))

    add('chain-drop-col', M57_84, 't8p',
        ['ALTER TABLE t8p DROP COLUMN c;',
         'ALTER TABLE t8p ADD COLUMN z INT AFTER b;'],
        expect(['ALTER TABLE t8p DROP COLUMN c;',
                'ALTER TABLE t8p ADD COLUMN z INT AFTER b;'], loaded=4, applicable=2),
        [exec_step('s0', 'ALTER TABLE t8p DROP COLUMN c;'),
         exec_step('s1', 'ALTER TABLE t8p ADD COLUMN z INT AFTER b;')],
        'a:1:YES:x00:|b:2:YES:x00:|z:3:YES:x00:', policy='t08-existence', gipk_check=True)

    return cases


def cli_cases():
    fo = ('--fail-on', 'blocker')
    fb = forbid_meta
    parser_neg_expect = {'exit': 2, 'verdict': '', 'statements': 0, 'findings': 0, 'diagnostics': 1,
                         'unsupported': 0, 'coverage': 'incomplete', 'statement_coverage': [],
                         'statement_sql': [], 'statement_indices': [], 'evidence_gaps': 0,
                         'evidence_gap_entries': [], 'finding_entries': [], 'finding_metadata': [],
                         'unsupported_entries': []}
    return [
        cli_case('mysql', 'mysql', 'CREATE TABLE t (a INT);',
                 expect(['CREATE TABLE t (a INT);'])),
        cli_case('tidb', 'tidb', 'CREATE TABLE t (a INT);',
                 expect(['CREATE TABLE t (a INT);'])),
        cli_case('set-default-blocker', 'mysql', 'ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;',
                 expect(['ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;'], verdict='reject', findings=1,
                        finding_entries=[finding(0, SET_FORBID, 'blocker')],
                        finding_meta=[meta(0, SET_FORBID, **fb('t', 'set_column_default', 'c', 'c', 'explicit_default_change'))],
                        loaded=1, applicable=1),
                 policy='t08-set-default', args=fo),
        cli_case('set-default-blocker-tidb', 'tidb', 'ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;',
                 expect(['ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;'], verdict='reject', findings=1,
                        finding_entries=[finding(0, SET_FORBID, 'blocker')],
                        finding_meta=[meta(0, SET_FORBID, **fb('t', 'set_column_default', 'c', 'c', 'explicit_default_change'))],
                        loaded=1, applicable=1),
                 policy='t08-set-default', args=fo),
        cli_case('modify-default-blocker', 'mysql', 'ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 1;',
                 expect(['ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 1;'], verdict='reject', findings=1,
                        finding_entries=[finding(0, MOD_DEF, 'blocker')],
                        finding_meta=[meta(0, MOD_DEF, **fb('t', 'modify_column', 'c', 'c', 'explicit_default_change'))],
                        loaded=1, applicable=1),
                 policy='t08-modify-default', args=fo),
        cli_case('change-default-blocker', 'mysql', 'ALTER TABLE t CHANGE COLUMN c c2 INT NULL DEFAULT 1;',
                 expect(['ALTER TABLE t CHANGE COLUMN c c2 INT NULL DEFAULT 1;'], verdict='reject', findings=1,
                        finding_entries=[finding(0, CHG_DEF, 'blocker')],
                        finding_meta=[meta(0, CHG_DEF, **fb('t', 'change_column', 'c', 'c2', 'explicit_default_change'))],
                        loaded=1, applicable=1),
                 policy='t08-change-default', args=fo),
        cli_case('drop-default-blocker', 'mysql', 'ALTER TABLE t ALTER COLUMN c DROP DEFAULT;',
                 expect(['ALTER TABLE t ALTER COLUMN c DROP DEFAULT;'], verdict='reject', findings=1,
                        finding_entries=[finding(0, DROP_FORBID, 'blocker')],
                        finding_meta=[meta(0, DROP_FORBID, **fb('t', 'drop_column_default', 'c', 'c', 'explicit_default_change'))],
                        loaded=1, applicable=1),
                 policy='t08-drop-default', args=fo),
        cli_case('set-under-drop-policy', 'mysql', 'ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;',
                 expect(['ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;'], loaded=1, applicable=0),
                 policy='t08-drop-default'),
        cli_case('drop-under-set-policy', 'mysql', 'ALTER TABLE t ALTER COLUMN c DROP DEFAULT;',
                 expect(['ALTER TABLE t ALTER COLUMN c DROP DEFAULT;'], loaded=1, applicable=0),
                 policy='t08-set-default'),
        cli_case('default-non-target', 'mysql', 'ALTER TABLE t ADD COLUMN other_c INT NULL;',
                 expect(['ALTER TABLE t ADD COLUMN other_c INT NULL;'], loaded=4, applicable=0),
                 policy='t08-default-all'),
        cli_case('set-default-off', 'mysql', 'ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;',
                 expect(['ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;'])),
        cli_case('set-default-forbid-false', 'mysql', 'ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;',
                 expect(['ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;'], loaded=1, applicable=0),
                 policy='t08-set-default-param-off'),
        cli_case('set-default-samevalue', 'mysql', 'ALTER TABLE t ALTER COLUMN c SET DEFAULT 0;',
                 expect(['ALTER TABLE t ALTER COLUMN c SET DEFAULT 0;'], verdict='reject', findings=1,
                        finding_entries=[finding(0, SET_FORBID, 'blocker')],
                        loaded=1, applicable=1),
                 policy='t08-set-default', args=fo),
        cli_case('set-default-strnull', 'mysql', "ALTER TABLE t ALTER COLUMN c SET DEFAULT 'NULL';",
                 expect(["ALTER TABLE t ALTER COLUMN c SET DEFAULT 'NULL';"], verdict='reject', findings=1,
                        finding_entries=[finding(0, SET_FORBID, 'blocker')],
                        loaded=1, applicable=1),
                 policy='t08-set-default', args=fo),
        cli_case('expr-default-off-tidb', 'tidb', 'ALTER TABLE t ALTER COLUMN c SET DEFAULT (uuid());',
                 expect(['ALTER TABLE t ALTER COLUMN c SET DEFAULT (uuid());'])),
        cli_case('expr-default-policy-tidb', 'tidb', 'ALTER TABLE t ALTER COLUMN c SET DEFAULT (uuid());',
                 expect(['ALTER TABLE t ALTER COLUMN c SET DEFAULT (uuid());'], verdict='reject', findings=1,
                        finding_entries=[finding(0, SET_FORBID, 'blocker')],
                        loaded=1, applicable=1),
                 policy='t08-set-default', args=fo),
        # The eval-unproven gap needs the target column's type; offline the
        # boundary stays silent and only the explicit-op finding fires.
        # Blocker+gap coexistence on the provider path is pinned by
        # TestAuditSQLT08EvalGapCoexistsWithBlocker (the live-anchor cell is
        # unpinnable because the eval-unproven boundary is value-dependent).
        cli_case('eval-gap-coexist-tidb', 'tidb', "ALTER TABLE t ALTER COLUMN c SET DEFAULT (lower('X'));",
                 expect(["ALTER TABLE t ALTER COLUMN c SET DEFAULT (lower('X'));"], verdict='reject', findings=1,
                        finding_entries=[finding(0, SET_FORBID, 'blocker')],
                        loaded=1, applicable=1),
                 policy='t08-set-default', args=fo),
        cli_case('modify-null-advisory', 'mysql', 'ALTER TABLE t MODIFY COLUMN c INT NULL;',
                 expect(['ALTER TABLE t MODIFY COLUMN c INT NULL;'], findings=1,
                        finding_entries=[finding(0, NULL_ADV, 'notice')],
                        finding_meta=[meta(0, NULL_ADV, **fb('t', 'modify_column', 'c', 'c', ADV),
                                         **{'prior_state': 'unknown', 'requested_not_null': False})],
                        loaded=3, applicable=2),
                 policy='t08-nullability-all'),
        cli_case('modify-notnull-advisory', 'mysql', 'ALTER TABLE t MODIFY COLUMN c INT NOT NULL;',
                 expect(['ALTER TABLE t MODIFY COLUMN c INT NOT NULL;'], findings=1,
                        finding_entries=[finding(0, NULL_ADV, 'notice')],
                        finding_meta=[meta(0, NULL_ADV, **fb('t', 'modify_column', 'c', 'c', ADV),
                                         **{'prior_state': 'unknown', 'requested_not_null': True})],
                        loaded=3, applicable=2),
                 policy='t08-nullability-all'),
        cli_case('change-notnull-blocker', 'mysql', 'ALTER TABLE t CHANGE COLUMN c c INT NOT NULL;',
                 expect(['ALTER TABLE t CHANGE COLUMN c c INT NOT NULL;'], verdict='reject', findings=1,
                        finding_entries=[finding(0, CHG_NULL, 'blocker')],
                        finding_meta=[meta(0, CHG_NULL, **fb('t', 'change_column', 'c', 'c', ADV))],
                        loaded=3, applicable=1),
                 policy='t08-nullability-all', args=fo),
        cli_case('change-null-blocker', 'mysql', 'ALTER TABLE t CHANGE COLUMN c c INT NULL;',
                 expect(['ALTER TABLE t CHANGE COLUMN c c INT NULL;'], verdict='reject', findings=1,
                        finding_entries=[finding(0, CHG_NULL, 'blocker')],
                        finding_meta=[meta(0, CHG_NULL, **fb('t', 'change_column', 'c', 'c', ADV))],
                        loaded=3, applicable=1),
                 policy='t08-nullability-all', args=fo),
        cli_case('modify-omit-null', 'mysql', 'ALTER TABLE t MODIFY COLUMN c INT;',
                 expect(['ALTER TABLE t MODIFY COLUMN c INT;'], loaded=3, applicable=2),
                 policy='t08-nullability-all'),
        cli_case('modify-ai-blocker', 'mysql', 'ALTER TABLE t MODIFY COLUMN c INT AUTO_INCREMENT;',
                 expect(['ALTER TABLE t MODIFY COLUMN c INT AUTO_INCREMENT;'], verdict='reject', findings=1,
                        finding_entries=[finding(0, MOD_AI, 'blocker')],
                        finding_meta=[meta(0, MOD_AI, **fb('t', 'modify_column', 'c', 'c', 'explicit_auto_increment_change'))],
                        loaded=2, applicable=1),
                 policy='t08-ai', args=fo),
        cli_case('change-ai-blocker', 'mysql', 'ALTER TABLE t CHANGE COLUMN c c INT AUTO_INCREMENT;',
                 expect(['ALTER TABLE t CHANGE COLUMN c c INT AUTO_INCREMENT;'], verdict='reject', findings=1,
                        finding_entries=[finding(0, CHG_AI, 'blocker')],
                        finding_meta=[meta(0, CHG_AI, **fb('t', 'change_column', 'c', 'c', 'explicit_auto_increment_change'))],
                        loaded=2, applicable=1),
                 policy='t08-ai', args=fo),
        cli_case('modify-json-family', 'mysql', 'ALTER TABLE t MODIFY COLUMN c JSON;',
                 expect(['ALTER TABLE t MODIFY COLUMN c JSON;'], verdict='review', findings=1,
                        coverage='unverified',
                        finding_entries=[finding(0, MOD_FAMILY, 'warning')],
                        finding_meta=[meta(0, MOD_FAMILY, **{'table': 't', 'action': 'modify_column', 'name': 'c',
                                                             'column_name': 'c', 'target_type': 'json', 'type_family': 'text'})],
                        gap_entries=[
                            {'index': 0, 'rule_id': MOD_COMPAT, 'reason_code': 'missing_source_column',
                             'required_facts': ['source_column.definition']},
                            {'index': 0, 'rule_id': TBL_EXISTS, 'reason_code': 'unknown_table_state',
                             'required_facts': ['target_table.existence']},
                            {'index': 0, 'rule_id': MOD_EXISTS, 'reason_code': 'unknown_table_state',
                             'required_facts': ['target_table.columns', 'target_table.existence']}],
                        loaded=11, applicable=4),
                 policy='t08-all'),
        cli_case('modify-bigint-gap', 'mysql', 'ALTER TABLE t MODIFY COLUMN c BIGINT;',
                 expect(['ALTER TABLE t MODIFY COLUMN c BIGINT;'], verdict='review',
                        coverage='unverified',
                        gap_entries=[
                            {'index': 0, 'rule_id': MOD_COMPAT, 'reason_code': 'missing_source_column',
                             'required_facts': ['source_column.definition']},
                            {'index': 0, 'rule_id': TBL_EXISTS, 'reason_code': 'unknown_table_state',
                             'required_facts': ['target_table.existence']},
                            {'index': 0, 'rule_id': MOD_EXISTS, 'reason_code': 'unknown_table_state',
                             'required_facts': ['target_table.columns', 'target_table.existence']}],
                        loaded=11, applicable=4),
                 policy='t08-all'),
        cli_case('add-first', 'mysql', 'ALTER TABLE t ADD COLUMN x INT FIRST;',
                 expect(['ALTER TABLE t ADD COLUMN x INT FIRST;'])),
        cli_case('add-after', 'mysql', 'ALTER TABLE t ADD COLUMN x INT AFTER b;',
                 expect(['ALTER TABLE t ADD COLUMN x INT AFTER b;'])),
        cli_case('modify-after', 'mysql', 'ALTER TABLE t MODIFY COLUMN b INT AFTER a;',
                 expect(['ALTER TABLE t MODIFY COLUMN b INT AFTER a;'])),
        cli_case('change-first', 'mysql', 'ALTER TABLE t CHANGE COLUMN b b2 INT FIRST;',
                 expect(['ALTER TABLE t CHANGE COLUMN b b2 INT FIRST;'])),
        cli_case('multi-add2', 'mysql', 'ALTER TABLE t ADD COLUMN a INT, ADD COLUMN b INT;',
                 expect(['ALTER TABLE t ADD COLUMN a INT, ADD COLUMN b INT;'])),
        cli_case('multi-add-setter', 'mysql', 'ALTER TABLE t ADD COLUMN a INT, ALTER COLUMN c SET DEFAULT 1;',
                 expect(['ALTER TABLE t ADD COLUMN a INT, ALTER COLUMN c SET DEFAULT 1;'], verdict='reject', findings=1,
                        finding_entries=[finding(0, SET_FORBID, 'blocker')],
                        loaded=1, applicable=1),
                 policy='t08-set-default', args=fo),
        cli_case('multi-modify-x2', 'mysql', 'ALTER TABLE t MODIFY COLUMN a INT NULL, MODIFY COLUMN b INT NOT NULL;',
                 expect(['ALTER TABLE t MODIFY COLUMN a INT NULL, MODIFY COLUMN b INT NOT NULL;'], findings=2,
                        finding_entries=[finding(0, NULL_ADV, 'notice'), finding(0, NULL_ADV, 'notice')],
                        loaded=3, applicable=2),
                 policy='t08-nullability-all'),
        cli_case('multi-same-obj', 'mysql', 'ALTER TABLE t MODIFY COLUMN a INT, MODIFY COLUMN a BIGINT;',
                 expect(['ALTER TABLE t MODIFY COLUMN a INT, MODIFY COLUMN a BIGINT;'])),
        cli_case('multi-add-drop', 'mysql', 'ALTER TABLE t ADD COLUMN a INT, DROP COLUMN b;',
                 expect(['ALTER TABLE t ADD COLUMN a INT, DROP COLUMN b;'])),
        cli_case('parser-neg', 'mysql', 'ALTER TABLE t ALTER COLUMN c SET DEFAULT;',
                 parser_neg_expect),
        cli_case('err-syntax', 'mysql', 'ALTER TABLE t ALTER COLUMN c SET DEFAULT 1 2;',
                 copy.deepcopy(parser_neg_expect)),
        cli_case('offline-no-connect', 'mysql', 'ALTER TABLE t MODIFY COLUMN c INT;',
                 expect(['ALTER TABLE t MODIFY COLUMN c INT;'])),
        cli_case('unsupported-aspect', 'mysql', 'ALTER TABLE t MODIFY COLUMN c INT, ALGORITHM=INPLACE;',
                 expect(['ALTER TABLE t MODIFY COLUMN c INT, ALGORITHM=INPLACE;'], verdict='review',
                        coverage='incomplete', diagnostics=1,
                        unsupported_entries=[{'index': 0, 'feature': 'alter_table.algorithm', 'reason': UNAUDITED_REASON}])),
    ]


def cli_case(cid, dialect, sql, exp, policy=None, args=()):
    case = {'id': cid, 'dialect': dialect, 'sql': sql, 'expect': exp}
    if policy:
        case['policy'] = policy
    if args:
        case['args'] = list(args)
    return case


def error_cases():
    return [
        {'id': 'err-bad-dialect', 'dialect': 'bogus', 'sql': 'ALTER TABLE t ADD COLUMN a INT;',
         'expect': {'exit': 2, 'stderr_contains': ['dialect must be']}},
        {'id': 'err-policy-invalid', 'dialect': 'mysql', 'sql': 'ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;',
         'args': ['--config', '/nonexistent-t08-policy.yaml'],
         'expect': {'exit': 2, 'stderr_contains': ['load policy']}},
        {'id': 'err-provider-failure', 'dialect': 'mysql', 'sql': 'ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;',
         'args': ['--host', '127.0.0.1', '--port', '1', '--user', 'root', '--schema', 'golden'],
         'expect': {'exit': 3, 'stderr_contains': ['connection refused']}},
    ]


def manifest():
    baseline = json.loads((Path(__file__).resolve().parents[1] / 'testdata/ddl-golden/T07.json').read_text())
    anchors = copy.deepcopy(baseline['anchors'])
    for name, anchor in anchors.items():
        if name != 'tidb85':
            anchor['exec_password_env'] = 'T08_MYSQL_PASSWORD'
        else:
            anchor.pop('exec_password_env', None)
    metas = metadata_cases()
    clis = cli_cases()
    errs = error_cases()
    ids = [f'T08.db.{anchor}.{suffix}' for anchor in VERSIONS for suffix in ('ddl', 'syntax_negative')]
    ids += ['T08.cli.' + row['id'] for row in clis]
    ids += ['T08.clierr.' + row['id'] for row in errs]
    ids += ['T08.meta.' + row['id'] for row in metas]
    assert len(ids) == len(set(ids)) == FROZEN_REQUIRED_CASES, (len(ids), len(set(ids)))
    assert hashlib.sha256('\n'.join(ids).encode()).hexdigest() == FROZEN_IDS_SHA256
    return {
        'task_id': 'T08',
        'issue': 'https://github.com/Fanduzi/DeltaScope/issues/87',
        'policy_profile': 'all-rules-disabled',
        'description': 'Frozen column-governance semantics: explicit default/nullability/auto_increment '
                       'operations, effective-default compare, column position, and bounded multi-action '
                       'ordered state; independent T08 acceptance contract',
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


def t08_manifest_failures(data):
    if data.get('task_id') != 'T08':
        return []
    want = manifest()
    return [] if strict_equal(data, want) else ['T08 manifest differs from independent frozen role/oracle constants']


def t08_artifact_failures(artifact, data):
    if artifact.get('task_id') != 'T08' and data.get('task_id') != 'T08':
        return []
    if data.get('task_id') != 'T08':
        return ['T08 manifest task identity']
    errors = t08_manifest_failures(data)
    if errors:
        return errors

    def need(condition, message):
        if not condition:
            errors.append('T08 ' + message)

    need(artifact.get('task_id') == 'T08', 'task identity')
    records = artifact.get('cases')
    if not isinstance(records, list) or not all(isinstance(row, dict) for row in records):
        return errors + ['T08 malformed case collection']
    need(type(artifact.get('executed_count')) is int and artifact['executed_count'] == FROZEN_REQUIRED_CASES,
         'strict executed_count/denominator')
    ids = [row.get('case_id') for row in records]
    if not all(type(value) is str for value in ids):
        return errors + ['T08 case IDs must be strings']
    need(len(ids) == FROZEN_REQUIRED_CASES and len(set(ids)) == FROZEN_REQUIRED_CASES
         and set(ids) == set(data['required_case_ids']), 'complete unique case identity set')
    if errors:
        return errors
    spec_by_id = {}
    for spec in metadata_cases():
        spec_by_id['T08.meta.' + spec['id']] = spec
    for spec in cli_cases():
        spec_by_id['T08.cli.' + spec['id']] = spec
    for spec in error_cases():
        spec_by_id['T08.clierr.' + spec['id']] = spec
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
    (root / 'testdata/ddl-golden/T08.json').write_text(json.dumps(data, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'required_cases': len(data['required_case_ids']),
                      'cli_cases': len(data['cli_cases']),
                      'metadata_cases': len(data['metadata_cases']),
                      'error_cases': len(data['error_cases']),
                      'id_sha256': hashlib.sha256('\n'.join(data['required_case_ids']).encode()).hexdigest()}, indent=2))


if __name__ == '__main__':
    write_frozen_files()
