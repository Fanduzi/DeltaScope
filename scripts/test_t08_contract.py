#!/usr/bin/env python3
import copy
import json
import pathlib
import tempfile

import ddl_golden as runner
import t08_contract as contract
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
    for row in expect['finding_metadata']:
        metadata.setdefault(row['index'], []).append(row['metadata'])
    for want in expect['finding_entries']:
        idx = want['index']
        statements[idx]['findings'].append({'rule_id': want['rule_id'], 'level': want['level'],
                                            'message': 'constructed finding', 'statement_index': idx,
                                            'statement_kind': 'ddl',
                                            'location': {'line': 1, 'column': 1},
                                            'metadata': copy.deepcopy((metadata.get(idx) or [{}]).pop(0))})
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
    profile = spec.get('policy', 'all-rules-disabled')
    if profile != 'all-rules-disabled':
        parsed['rule_summary'] = {'loaded': len(contract.profiles()[profile]['enable']),
                                  'applicable': expect.get('rule_summary_applicable', 0)}
    raw = runner.spec_target_version_raw(spec)
    if raw:
        parsed['version'] = {'product': spec['dialect'], 'version': runner.canonical_target_version(raw),
                             'major': 8, 'minor': 0, 'patch': 0, 'source': 'target', 'validated_range': True}
    return parsed


def fixture(tmp):
    manifest = contract.manifest()
    artifact = make_artifact(tmp)
    binary = pathlib.Path(artifact['cli']['path'])
    catalog = json.dumps({'rules': [{'rule_id': rule_id}
                                    for profile in contract.profiles().values()
                                    for rule_id in profile['enable']]})
    binary.write_text("#!/bin/sh\nprintf '%s' '" + catalog + "'\n")
    artifact['cli']['sha256'] = runner.sha256_file(binary)
    policies = runner.make_policies(str(binary), tmp, manifest)
    artifact.update(task_id='T08', policies=list(policies.values()),
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
        record = common('T08.db.' + anchor + '.ddl', 'db_ddl',
                        [step['sql'] for step in manifest['ddl_steps']],
                        {'steps': [{'name': step['name'], 'rc': step.get('expect_rc', 0),
                                    'verify': step.get('verify', [])} for step in manifest['ddl_steps']]})
        record.update(anchor=anchor, actual={'database': database(anchor), 'steps': steps(manifest['ddl_steps'])})
        artifact['cases'].append(record)
        record = common('T08.db.' + anchor + '.syntax_negative', 'db_syntax_negative',
                        manifest['syntax_negative']['sql'], manifest['syntax_negative']['expect'])
        record.update(anchor=anchor, actual={'rc': 1, 'stdout': '',
                                             'stderr': 'ERROR 1064 (42000): controlled syntax rejection',
                                             'error_class': '1064'})
        artifact['cases'].append(record)

    def audit_record(spec, online=False):
        name = 'T08.meta.' + spec['id'] if online else 'T08.cli.' + spec['id']
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
            record.update(anchor=anchor, connect=copy.deepcopy(spec['connect']))
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
    for spec in manifest['error_cases']:
        profile_name = spec.get('policy', 'all-rules-disabled')
        policy = policies[profile_name]
        command = [str(binary), 'audit', '--dialect', spec['dialect'], '--sql', spec['sql'],
                   '--config', policy['path'], '--format', 'json'] + list(spec.get('args') or [])
        record = common('T08.clierr.' + spec['id'], 'cli_error', spec['sql'], spec['expect'])
        record.update(cli_case=spec['id'], dialect=spec['dialect'], policy_profile=profile_name,
                      policy_path=policy['path'], command=command,
                      actual={'exit': spec['expect']['exit'], 'stdout': '',
                              'stderr': ' '.join(spec['expect']['stderr_contains']),
                              'version_evidence': runner.version_evidence_record(spec, None)})
        artifact['cases'].append(record)
    artifact['executed_count'] = len(artifact['cases'])
    return artifact, manifest


def main():
    results = []
    with tempfile.TemporaryDirectory(prefix='t08-validator-') as directory:
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

        check('T08 complete constructed control', artifact, reject=False)

        def take(suffix):
            a = copy.deepcopy(artifact)
            record = next(row for row in a['cases'] if row['case_id'].endswith(suffix))
            return a, record

        def sync(record):
            record['actual']['stdout'] = json.dumps(record['actual']['parsed'])

        # Frozen-contract oracle: every mutation below must be rejected.
        a = copy.deepcopy(artifact)
        a['executed_count'] = contract.FROZEN_REQUIRED_CASES - 1
        check('T08 strict denominator drift', a)
        a = copy.deepcopy(artifact)
        a['cases'].pop()
        a['required_case_ids'].pop()
        a['executed_count'] -= 1
        check('T08 double-side deletion', a)
        a = copy.deepcopy(artifact)
        a['cases'][-1] = copy.deepcopy(a['cases'][0])
        check('T08 duplicate case ID', a)
        a = copy.deepcopy(artifact)
        a['cases'].pop()
        a['executed_count'] -= 1
        check('T08 missing executed case', a)
        a = copy.deepcopy(artifact)
        a['cases'][-1]['expected'] = {}
        check('T08 frozen expectation drift', a)
        a = copy.deepcopy(artifact)
        a['cases'][-1]['status'] = 'skip'
        check('T08 fabricated status', a)
        a = copy.deepcopy(artifact)
        a['cases'][-1]['assertions'][0]['ok'] = False
        check('T08 failed assertion', a)

        a, c = take('mysql84-pos-add-after-missing')
        c['actual']['parsed']['statements'][0]['findings'][0]['metadata']['name'] = 'phantom'
        sync(c)
        check('T08 wrong finding metadata', a)
        a, c = take('mysql84-pos-add-after-missing')
        c['actual']['parsed']['statements'][0]['findings'] = []
        c['actual']['parsed']['summary']['blockers'] = 0
        c['actual']['parsed']['verdict'] = 'pass'
        c['actual']['exit'] = 0
        sync(c)
        check('T08 exists blocker laundered', a)
        a, c = take('mysql84-pos-add-after-missing')
        c['actual']['parsed']['rule_summary']['applicable'] = 3
        sync(c)
        check('T08 applicable drift', a)
        a, c = take('mysql84-pos-add-after-missing')
        c['actual']['execute'][0]['rc'] = 0
        c['actual']['execute'][0]['stderr'] = ''
        check('T08 native rejection erased', a)
        a, c = take('mysql84-pos-add-after-missing')
        c['actual']['execute'][0]['stderr'] = 'ERROR 9999'
        check('T08 native rejection class drift', a)
        for category in ('setup', 'execute', 'structure', 'post_verify', 'teardown'):
            a, c = take('mysql84-pos-add-after-missing')
            c['actual'][category] = []
            check('T08 skipped ' + category, a)
        a, c = take('mysql84-pos-add-after-missing')
        c['anchor'] = 'mysql57'
        check('T08 anchor rebind', a)
        a, c = take('tidb85-multi-add2-after-sibling')
        c['actual']['parsed']['verdict'] = 'pass'
        c['actual']['parsed']['summary']['blockers'] = 0
        c['actual']['parsed']['statements'][0]['findings'] = []
        c['actual']['parsed']['rule_summary']['applicable'] = 2
        c['actual']['exit'] = 0
        c['actual']['execute'][0]['rc'] = 0
        c['actual']['execute'][0]['stderr'] = ''
        c['actual']['structure'][0]['output'] = 'a:1:YES:x00:|b:2:YES:x00:|c:3:YES:x00:|c1:4:YES:x00:|c2:5:YES:x00:'
        sync(c)
        check('T08 dialect split flattened', a)
        a, c = take('tidb85-multi-add2-after-sibling')
        c['actual']['structure'][0]['output'] = 'a:1:YES:x00:|b:2:YES:x00:|c:3:YES:x00:|c1:4:YES:x00:|c2:5:YES:x00:'
        check('T08 failed execute side effects claimed', a)
        a, c = take('cli.unsupported-aspect')
        c['actual']['parsed']['unsupported'] = []
        c['actual']['parsed']['diagnostics'] = []
        c['actual']['parsed']['coverage']['status'] = 'complete'
        c['actual']['parsed']['statements'][0]['coverage']['status'] = 'complete'
        c['actual']['parsed']['verdict'] = 'pass'
        sync(c)
        check('T08 unsupported aspect erased', a)
        a, c = take('cli.parser-neg')
        c['actual']['parsed']['statements'] = [{'index': 0, 'kind': 'ddl',
                                               'raw_sql': 'ALTER TABLE t ALTER COLUMN c SET DEFAULT;',
                                               'normalized_sql': 'ALTER TABLE t ALTER COLUMN c SET DEFAULT',
                                               'coverage': {'status': 'incomplete'},
                                               'findings': [], 'evidence_gaps': []}]
        sync(c)
        check('T08 parser-negative fabricated statement', a)
        a, c = take('cli.multi-add-setter')
        c['actual']['parsed']['rule_summary']['applicable'] = 0
        sync(c)
        check('T08 applicable counter laundered', a)
        m = copy.deepcopy(manifest)
        m['metadata_cases'][0]['expect']['verdict'] = 'pass'
        a = copy.deepcopy(artifact)
        check('T08 manifest drift', a, m)

    failures = sum(1 for result in results if not result)
    print(f'{len(results) - failures}/{len(results)} constructed-control checks passed')
    if failures:
        raise SystemExit(1)


if __name__ == '__main__':
    main()
