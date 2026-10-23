#!/usr/bin/env python3
# input: independent frozen T10 roles and explicitly constructed offline validator artifacts
# output: full validate_artifact valid-control plus T10 oracle sensitivity; no product/database acceptance claim
# pos: T10 artifact-integrity tests beside the existing T02–T09 validator suite
# note: if this file changes, update this header and scripts/README.md.
import copy
import json
import pathlib
import tempfile

import ddl_golden as runner
import t10_contract as contract
from test_ddl_golden import make_artifact


def queries(specs):
    return [{'assert': q['assert'], 'sql': q['sql'], 'rc': 0, 'output': q['expect'], 'stderr': ''} for q in specs]


def steps(specs):
    return [{'name': s['name'], 'sql': s['sql'], 'rc': s.get('expect_rc', 0), 'stdout': '',
             'stderr': ' '.join(s.get('stderr_contains', [])),
             'verify': queries(s.get('verify', []))} for s in specs]


def identity(product, version, source):
    major, minor, patch = map(int, version.lstrip('v').split('.'))
    valid = ((product == 'mysql' and (major, minor) in ((5, 7), (8, 0), (8, 4)))
             or (product == 'tidb' and (major, minor) == (8, 5)))
    return {'product': product, 'version': version, 'major': major, 'minor': minor, 'patch': patch,
            'source': source, 'validated_range': valid}


def parsed_for(spec):
    expect = spec['expect']
    statements = []
    for idx, sql in enumerate(expect['statement_sql']):
        statements.append({'index': idx, 'kind': 'ddl', 'raw_sql': sql,
                           'normalized_sql': sql.removesuffix(';'),
                           'coverage': {'status': expect['statement_coverage'][idx]},
                           'findings': [], 'evidence_gaps': []})
    metadata = {}
    for row in expect.get('finding_metadata', []):
        metadata.setdefault((row['index'], row['rule_id']), []).append(row['metadata'])
    for want in expect['finding_entries']:
        idx = want['index']
        statements[idx]['findings'].append({'rule_id': want['rule_id'], 'level': want['level'],
                                            'message': 'constructed finding', 'statement_index': idx,
                                            'statement_kind': 'ddl',
                                            'location': {'line': 1, 'column': 1},
                                            'metadata': copy.deepcopy((metadata.get((idx, want['rule_id'])) or [{}]).pop(0))})
    for want in expect['evidence_gap_entries']:
        statements[want['index']]['evidence_gaps'].append(
            {key: copy.deepcopy(value) for key, value in want.items() if key != 'index'})
    unsupported = [{**copy.deepcopy(item), 'sql': expect['statement_sql'][item['index']]}
                   for item in expect['unsupported_entries']]
    blockers = sum(1 for want in expect['finding_entries'] if want['level'] == 'blocker')
    warnings = sum(1 for want in expect['finding_entries'] if want['level'] == 'warning')
    notices = sum(1 for want in expect['finding_entries'] if want['level'] == 'notice')
    parsed = {'verdict': expect['verdict'],
              'summary': {'statements': expect['statements'], 'blockers': blockers,
                          'warnings': warnings, 'notices': notices},
              'coverage': {'status': expect['coverage']}, 'statements': statements,
              'global_findings': [], 'unsupported': unsupported,
              'diagnostics': [{'classification': 'unaudited', 'reason': item['reason'],
                               'action_hint': 'inspect bounded capability', 'audited': False,
                               'dialect': spec['dialect']} for item in unsupported],
              'fail_on_triggered': expect['exit'] == 1}
    while len(parsed['diagnostics']) < expect['diagnostics']:
        parsed['diagnostics'].append({'classification': 'parser_error',
                                      'reason': 'statement was not audited', 'action_hint': 'fix syntax',
                                      'audited': False, 'dialect': spec['dialect']})
    if 'rule_summary_loaded' in expect:
        parsed['rule_summary'] = {'loaded': expect['rule_summary_loaded'],
                                  'applicable': expect['rule_summary_applicable']}
    return parsed


def fixture(tmp):
    manifest = contract.manifest()
    artifact = make_artifact(tmp)
    binary = pathlib.Path(artifact['cli']['path'])
    catalog = json.dumps({'rules': [{'rule_id': rule_id}
                                    for enable in contract.POLICIES.values()
                                    for rule_id in enable]})
    binary.write_text("#!/bin/sh\nprintf '%s' '" + catalog + "'\n")
    artifact['cli']['sha256'] = runner.sha256_file(binary)
    policies = runner.make_policies(str(binary), tmp, manifest)
    artifact.update(task_id='T10', policies=list(policies.values()),
                    policy_profile=policies['all-rules-disabled'], cases=[],
                    required_case_ids=manifest['required_case_ids'])

    def database(anchor):
        source = manifest['anchors'][anchor]
        banner = '8.0.11-TiDB-v8.5.0' if anchor == 'tidb85' else source['version_contains']
        return {'product': source['product'], 'image': source['image'],
                'image_digest': source['image'].split(':')[0] + '@sha256:constructed-validator-control',
                'container': source['container'], 'reachable': True, 'version': banner}

    def common(case_id, kind, input_sql, expected):
        return {'case_id': case_id, 'kind': kind, 'input_sql': input_sql,
                'expected': copy.deepcopy(expected),
                'assertions': [{'name': 'constructed control', 'ok': True,
                                'detail': 'not a product or database execution'}], 'status': 'pass'}

    for anchor in contract.VERSIONS:
        record = common('T10.db.' + anchor + '.ddl', 'db_ddl',
                        [step['sql'] for step in manifest['ddl_steps']],
                        {'steps': [{'name': step['name'], 'rc': step.get('expect_rc', 0),
                                    'verify': step.get('verify', [])} for step in manifest['ddl_steps']]})
        record.update(anchor=anchor, actual={'database': database(anchor), 'steps': steps(manifest['ddl_steps'])})
        artifact['cases'].append(record)
        record = common('T10.db.' + anchor + '.syntax_negative', 'db_syntax_negative',
                        manifest['syntax_negative']['sql'], manifest['syntax_negative']['expect'])
        record.update(anchor=anchor, actual={'rc': 1, 'stdout': '',
                                             'stderr': 'ERROR 1064 (42000): controlled syntax rejection',
                                             'error_class': '1064'})
        artifact['cases'].append(record)

    def audit_record(spec, online=False):
        name = 'T10.meta.' + spec['id'] if online else 'T10.cli.' + spec['id']
        parsed = parsed_for(spec)
        profile = spec.get('policy', 'all-rules-disabled')
        policy = policies[profile]
        record = common(name, 'cli_metadata' if online else 'cli_audit', spec['sql'], spec['expect'])
        record.update(dialect=spec['dialect'], policy_profile=profile, policy_path=policy['path'],
                      cli_case=spec['id'])
        command = [str(binary), 'audit', '--dialect', spec['dialect'], '--sql', spec['sql'],
                   '--config', policy['path'], '--format', 'json']
        actual = {'exit': spec['expect']['exit'], 'stdout': '', 'stderr': '', 'parsed': parsed}
        if online:
            anchor = spec['anchor']
            record.update(anchor=anchor,
                          connect={k: v for k, v in copy.deepcopy(spec['connect']).items()
                                   if k != 'password'})
            actual['database'] = database(anchor)
            parsed['version'] = identity(spec['dialect'], contract.VERSIONS[anchor].lstrip('v'), 'observed')
            conn = spec['connect']
            command += ['--host', conn['host'], '--port', str(conn['port']), '--user', conn['user']]
            if conn.get('password_env'):
                command += ['--password-env', conn['password_env']]
            if conn.get('schema'):
                command += ['--schema', conn['schema']]
            for key in ('setup', 'execute', 'teardown'):
                actual[key] = steps(spec.get(key, []))
            for key in ('post_verify', 'structure'):
                actual[key] = queries(spec.get(key, []))
            declared_facts = (spec.get('expect') or {}).get('instance_facts')
            if declared_facts:
                actual['instance_facts'] = dict(declared_facts)
            actual['version_evidence'] = runner.version_evidence_record(
                spec, parsed, actual['database']['version'], spec['dialect'])
        else:
            actual['version_evidence'] = runner.version_evidence_record(spec, parsed)
        command += spec.get('args', [])
        actual['stdout'] = json.dumps(parsed)
        record.update(command=command, actual=actual)
        return record

    artifact['cases'] += [audit_record(spec) for spec in manifest['cli_cases']]
    artifact['cases'] += [audit_record(spec, online=True) for spec in manifest['metadata_cases']]
    artifact['executed_count'] = len(artifact['cases'])
    return artifact, manifest


def main():
    results = []
    with tempfile.TemporaryDirectory(prefix='t10-validator-') as directory:
        artifact, manifest = fixture(pathlib.Path(directory))
        baseline = runner.load_baseline()

        def check(name, a, m=None, reject=True):
            try:
                failures = runner.validate_artifact(a, m or manifest, baseline=baseline, verify_binary=True)
            except Exception as error:
                results.append(False)
                print('FAIL ' + name + ': exception ' + repr(error))
                return
            ok = bool(failures) if reject else not failures
            results.append(ok)
            print(('PASS ' if ok else 'FAIL ') + name)
            if not ok:
                print(json.dumps(failures, ensure_ascii=False, indent=2))

        check('T10 complete constructed control', artifact, reject=False)

        def take(suffix):
            a = copy.deepcopy(artifact)
            record = next(row for row in a['cases'] if row['case_id'].endswith(suffix))
            return a, record

        def sync(record):
            record['actual']['stdout'] = json.dumps(record['actual']['parsed'])

        # Frozen-contract oracle: every mutation below must be rejected.
        a = copy.deepcopy(artifact)
        a['executed_count'] = contract.FROZEN_REQUIRED_CASES - 1
        check('T10 strict denominator drift', a)
        a = copy.deepcopy(artifact)
        a['cases'].pop()
        a['required_case_ids'].pop()
        a['executed_count'] -= 1
        check('T10 double-side deletion', a)
        a = copy.deepcopy(artifact)
        a['cases'][-1] = copy.deepcopy(a['cases'][0])
        check('T10 duplicate case ID', a)
        a = copy.deepcopy(artifact)
        a['cases'].pop()
        a['executed_count'] -= 1
        check('T10 missing executed case', a)
        a = copy.deepcopy(artifact)
        a['cases'][-1]['expected'] = {}
        check('T10 frozen expectation drift', a)
        a = copy.deepcopy(artifact)
        a['cases'][-1]['status'] = 'skip'
        check('T10 fabricated status', a)
        a = copy.deepcopy(artifact)
        a['cases'][-1]['assertions'][0]['ok'] = False
        check('T10 failed assertion', a)

        # First-path binding: the invisible warning must stay bound to the
        # exact index/table, and the version-gate blocker must keep its
        # reason-code metadata.
        a, c = take('t10-mysql84-firstpath')
        c['actual']['parsed']['statements'][0]['findings'][0]['metadata']['index'] = 'phantom'
        sync(c)
        check('T10 wrong index-name metadata', a)
        a, c = take('t10-mysql84-firstpath')
        c['actual']['parsed']['statements'][0]['findings'][0]['metadata']['visibility'] = 'visible'
        sync(c)
        check('T10 visibility direction flipped', a)
        a, c = take('t10-mysql57-firstpath')
        compat = c['actual']['parsed']['statements'][0]['findings'][1]
        compat['metadata']['reason_code'] = 'missing_target_version'
        sync(c)
        check('T10 version blocker laundered to gap reason', a)
        a, c = take('t10-mysql57-firstpath')
        c['actual']['parsed']['statements'][0]['findings'] = [
            f for f in c['actual']['parsed']['statements'][0]['findings']
            if f['rule_id'] != contract.IDX_COMPAT]
        c['actual']['parsed']['summary']['blockers'] = 0
        c['actual']['parsed']['verdict'] = 'review'
        c['actual']['exit'] = 0
        sync(c)
        check('T10 5.7 silent complete', a)
        a, c = take('t10-mysql84-missing')
        c['actual']['parsed']['statements'][0]['evidence_gaps'].append(
            {'rule_id': contract.ALTER_IDX_EXISTS, 'reason_code': 'unknown_table_state',
             'required_facts': ['target_table.existence', 'target_table.indexes']})
        c['actual']['parsed']['coverage']['status'] = 'unverified'
        c['actual']['parsed']['statements'][0]['coverage']['status'] = 'unverified'
        sync(c)
        check('T10 missing index laundered to gap', a)

        # Ordered-state binding: the chain and the drop-then-alter sequences
        # must keep per-statement attribution — a warning moved to the wrong
        # statement or only the first statement audited cannot launder.
        a, c = take('t10-mysql84-batch-chain')
        moved = c['actual']['parsed']['statements'][0]['findings'].pop(0)
        moved['statement_index'] = 1
        c['actual']['parsed']['statements'][1]['findings'].append(moved)
        sync(c)
        check('T10 finding borrowed across statements', a)
        a, c = take('t10-mysql84-drop-then-alter')
        c['actual']['parsed']['statements'] = [c['actual']['parsed']['statements'][0]]
        c['actual']['parsed']['summary']['statements'] = 1
        c['actual']['parsed']['verdict'] = 'pass'
        c['actual']['exit'] = 0
        sync(c)
        check('T10 only first statement audited', a)
        a, c = take('t10-mysql84-drop-then-alter')
        c['actual']['execute'][1]['rc'] = 0
        c['actual']['execute'][1]['stderr'] = ''
        check('T10 native missing-index accepted', a)

        # Product split: MySQL rejects primary-key invisibility natively while
        # TiDB resolves it through index existence — swapping the two oracles
        # must fail.
        a, c = take('t10-mysql84-primary')
        c['actual']['execute'][0]['rc'] = 0
        c['actual']['execute'][0]['stderr'] = ''
        check('T10 mysql primary invisibility accepted', a)
        a, c = take('t10-tidb-primary')
        c['actual']['execute'][0]['stderr'] = 'ERROR 3522 (HY000): A primary key index cannot be invisible'
        check('T10 tidb primary mapped to mysql error', a)
        a, c = take('t10-tidb-fulltext')
        c['actual']['execute'][0]['rc'] = 0
        c['actual']['execute'][0]['stderr'] = ''
        check('T10 tidb fulltext natively accepted', a)
        a, c = take('t10-tidb-desc')
        c['actual']['structure'][0]['output'] = 'D'
        check('T10 tidb desc stored descending', a)
        a, c = take('t10-mysql57-desc')
        c['actual']['structure'][1]['output'] = '0'
        check('T10 5.7 desc marker claimed', a)

        # Evidence accounting: provider-observed invisibility must not be
        # reported as a declared finding, and the unvouched-set role stays a
        # bounded gap, never a fabricated finding.
        a, c = take('t10-mysql84-provider-invisible')
        c['actual']['parsed']['statements'][0]['findings'].append(
            {'rule_id': contract.IDX_INVIS, 'level': 'warning', 'message': 'fabricated',
             'statement_index': 0, 'statement_kind': 'ddl', 'location': {'line': 1, 'column': 1},
             'metadata': {'table': 't10i', 'index': 'idx_i', 'action': 'alter_index',
                          'visibility': 'invisible'}})
        c['actual']['parsed']['summary']['warnings'] = 1
        sync(c)
        check('T10 observed state recorded as declaration', a)
        a, c = take('t10-cli-mv-ver57')
        c['actual']['parsed']['statements'][0]['evidence_gaps'].append(
            {'rule_id': contract.IDX_COMPAT, 'reason_code': 'missing_target_version',
             'required_facts': ['target.version']})
        c['actual']['parsed']['coverage']['status'] = 'unverified'
        c['actual']['parsed']['statements'][0]['coverage']['status'] = 'unverified'
        sync(c)
        check('T10 determined version finding softened to gap', a)
        a, c = take('t10-cli-pfx')
        c['actual']['parsed']['statements'][0]['findings'].append(
            {'rule_id': contract.IDX_COMPAT, 'level': 'blocker', 'message': 'fabricated',
             'statement_index': 0, 'statement_kind': 'ddl', 'location': {'line': 1, 'column': 1},
             'metadata': {'table': 't', 'index': 'i', 'feature': 'prefix_keys', 'action': 'create_index',
                          'reason_code': 'version_unsupported', 'product': 'mysql',
                          'target_version': '8.0.46', 'minimum_version': '8.0.0'}})
        c['actual']['parsed']['summary']['blockers'] = 1
        c['actual']['parsed']['verdict'] = 'reject'
        sync(c)
        check('T10 prefix fabricated as gated feature', a)

        a, c = take('t10-cli-alter-index-no-kw')
        c['actual']['parsed']['statements'] = [{'index': 0, 'kind': 'ddl',
                                               'raw_sql': 'ALTER TABLE t ALTER INDEX idx_c VISIBLE;',
                                               'normalized_sql': 'ALTER TABLE t ALTER INDEX idx_c VISIBLE',
                                               'coverage': {'status': 'complete'},
                                               'findings': [], 'evidence_gaps': []}]
        sync(c)
        check('T10 parser-negative fabricated statement', a)
        a, c = take('t10-cli-firstpath')
        c['actual']['parsed']['rule_summary']['applicable'] = 0
        sync(c)
        check('T10 applicable counter laundered', a)
        for category in ('setup', 'execute', 'structure', 'post_verify', 'teardown'):
            a, c = take('t10-mysql84-firstpath')
            c['actual'][category] = []
            check('T10 skipped ' + category, a)
        a, c = take('t10-mysql84-firstpath')
        c['anchor'] = 'mysql57'
        check('T10 anchor rebind', a)
        m = copy.deepcopy(manifest)
        m['metadata_cases'][0]['expect']['verdict'] = 'pass'
        a = copy.deepcopy(artifact)
        check('T10 manifest drift', a, m)

    failures = sum(1 for result in results if not result)
    print(f'{len(results) - failures}/{len(results)} constructed-control checks passed')
    if failures:
        raise SystemExit(1)


if __name__ == '__main__':
    main()
