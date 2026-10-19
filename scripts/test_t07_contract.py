#!/usr/bin/env python3
# input: independent frozen T07 roles and explicitly constructed offline validator artifacts
# output: full validate_artifact valid-control/adversarial sensitivity plus owned fixture and environment-only credential controls; no product/database acceptance claim
# pos: T07 artifact-integrity tests beside the existing T02–T06 validator suite
# note: if this file changes, update this header and scripts/README.md.
import copy
import json
import os
import pathlib
import tempfile
from unittest import mock

import ddl_golden as runner
import t07_contract as contract
from test_ddl_golden import make_artifact


def queries(specs):
    return [{'assert':q['assert'],'sql':q['sql'],'rc':0,'output':q['expect'],'stderr':''} for q in specs]


def steps(specs):
    return [{'name':s['name'],'sql':s['sql'],'rc':s.get('expect_rc',0),'stdout':'','stderr':' '.join(s.get('stderr_contains',[])),'verify':queries(s.get('verify',[]))} for s in specs]


def identity(product,version,source):
    major,minor,patch=map(int,version.split('.'))
    valid=(product=='mysql' and (major,minor) in ((5,7),(8,0),(8,4))) or (product=='tidb' and (major,minor)==(8,5))
    return {'product':product,'version':version,'major':major,'minor':minor,'patch':patch,'source':source,'validated_range':valid}


def parsed_for(spec):
    expect=spec['expect'];statements=[]
    for idx,sql in enumerate(expect['statement_sql']):
        statements.append({'index':idx,'kind':'ddl','raw_sql':sql,'normalized_sql':sql.removesuffix(';'),'coverage':{'status':expect['statement_coverage'][idx]},'findings':[],'evidence_gaps':[]})
    for want in expect['t07_exact_findings']:
        idx=want['index'];location=next(item for item in expect['finding_locations'] if item['index']==idx)
        statements[idx]['findings'].append({'rule_id':want['rule_id'],'level':want['level'],'message':want['message'],'statement_index':idx,'statement_kind':'ddl','location':{'line':location['line'],'column':location['column']},'metadata':copy.deepcopy(want['metadata'])})
    for want in expect['evidence_gap_entries']:
        statements[want['index']]['evidence_gaps'].append({key:copy.deepcopy(value) for key,value in want.items() if key!='index'})
    unsupported=[{**copy.deepcopy(item),'sql':expect['statement_sql'][item['index']]} for item in expect['unsupported_entries']]
    parsed={'verdict':expect['verdict'],'summary':{'statements':expect['statements'],'blockers':expect['findings'],'warnings':0,'notices':0},'coverage':{'status':expect['coverage']},'statements':statements,'global_findings':[],'unsupported':unsupported,'diagnostics':[{'classification':'vendor_boundary' if item['reason']==contract.VENDOR_REASON else 'unaudited','reason':item['reason'],'action_hint':'inspect bounded capability','audited':False,'dialect':spec['dialect']} for item in unsupported],'fail_on_triggered':bool(expect['findings'])}
    profile=spec.get('policy','all-rules-disabled')
    if profile!='all-rules-disabled':parsed['rule_summary']={'loaded':len(contract.profiles()[profile]['enable']),'applicable':expect.get('rule_summary_applicable',0)}
    raw=runner.spec_target_version_raw(spec)
    if raw:parsed['version']=identity(spec['dialect'],runner.canonical_target_version(raw),'target')
    return parsed


def fixture(tmp):
    manifest=contract.manifest();artifact=make_artifact(tmp);binary=pathlib.Path(artifact['cli']['path'])
    catalog=json.dumps({'rules':[{'rule_id':key} for key in (contract.DEP,contract.COMPAT,contract.INDEX,contract.RENAME)]})
    binary.write_text("#!/bin/sh\nprintf '%s' '"+catalog+"'\n")
    artifact['cli']['sha256']=runner.sha256_file(binary)
    policies=runner.make_policies(str(binary),tmp,manifest)
    artifact.update(task_id='T07',policies=list(policies.values()),policy_profile=policies['all-rules-disabled'],cases=[],required_case_ids=manifest['required_case_ids'])
    def database(anchor):
        source=manifest['anchors'][anchor];banner='8.0.11-TiDB-v8.5.0' if anchor=='tidb85' else source['version_contains']
        return {'product':source['product'],'image':source['image'],'image_digest':source['image'].split(':')[0]+'@sha256:constructed-validator-control','container':source['container'],'reachable':True,'version':banner}
    def common(case_id,kind,input_sql,expected):
        return {'case_id':case_id,'kind':kind,'input_sql':input_sql,'expected':copy.deepcopy(expected),'assertions':[{'name':'constructed control','ok':True,'detail':'not a product or database execution'}],'status':'pass'}
    for anchor in contract.VERSIONS:
        record=common('T07.db.'+anchor+'.ddl','db_ddl',[step['sql'] for step in manifest['ddl_steps']],{'steps':[{'name':step['name'],'rc':step.get('expect_rc',0),'verify':step.get('verify',[])} for step in manifest['ddl_steps']]})
        record.update(anchor=anchor,actual={'database':database(anchor),'steps':steps(manifest['ddl_steps'])});artifact['cases'].append(record)
        record=common('T07.db.'+anchor+'.syntax_negative','db_syntax_negative',manifest['syntax_negative']['sql'],manifest['syntax_negative']['expect'])
        record.update(anchor=anchor,actual={'rc':1,'stdout':'','stderr':'ERROR 1064 (42000): controlled syntax rejection','error_class':'1064'});artifact['cases'].append(record)
    def audit_record(spec,online=False,baseline=False):
        name='T07.meta.'+spec['id'] if online else 'T07.cli.'+spec['id']
        parsed=parsed_for(spec);profile=spec.get('policy','all-rules-disabled');policy=policies[profile]
        record=common(name,'cli_metadata' if online else 'cli_audit',spec['sql'],spec['expect'])
        record.update(dialect=spec['dialect'],policy_profile=profile,policy_path=policy['path'])
        if not baseline:record['cli_case']=spec['id']
        command=[str(binary),'audit','--dialect',spec['dialect'],'--sql',spec['sql'],'--config',policy['path'],'--format','json']
        actual={'exit':spec['expect']['exit'],'stdout':'','stderr':'','parsed':parsed}
        if online:
            anchor=spec['anchor'];record.update(anchor=anchor,connect=copy.deepcopy(spec['connect']));actual['database']=database(anchor)
            version=contract.VERSIONS[anchor];parsed['version']=identity(spec['dialect'],version,'observed')
            conn=spec['connect'];command+=['--host',conn['host'],'--port',str(conn['port']),'--user',conn['user']]
            if conn.get('password_env'):command+=['--password-env',conn['password_env']]
            if conn.get('schema'):command+=['--schema',conn['schema']]
            for key in ('setup','execute','teardown'):actual[key]=steps(spec.get(key,[]))
            for key in ('post_verify','structure'):actual[key]=queries(spec.get(key,[]))
            actual['version_evidence']=runner.version_evidence_record(spec,parsed,actual['database']['version'],spec['dialect'])
        else:actual['version_evidence']=runner.version_evidence_record(spec,parsed)
        command+=spec.get('args',[]);actual['stdout']=json.dumps(parsed);record.update(command=command,actual=actual)
        return record
    artifact['cases'] += [audit_record(spec) for spec in manifest['cli_cases']]
    artifact['cases'] += [audit_record(spec,online=True) for spec in manifest['metadata_cases']]
    artifact['executed_count']=len(artifact['cases'])
    return artifact,manifest


def fixture_guard_checks(manifest):
    results=[]
    policies={name:{'path':'/constructed/'+name+'.yaml'} for name in ('all-rules-disabled',*contract.profiles())}
    source=next(row for row in manifest['metadata_cases'] if row['id']=='t07-mysql80-g01-stored-dependent-drop')
    for fault in ('preexisting','count-error','count-empty','wrong-version','version-query-error','collision'):
        ledger=[];foreign=False
        verify_values={q['sql']:q['expect'] for group in ('setup','execute') for step in source[group] for q in step.get('verify',[])}
        def mysql(anchor,sql,database=None,silent=True):
            nonlocal foreign
            ledger.append(sql)
            if sql=='SELECT VERSION()':return (1,'','VERSION query failure') if fault=='version-query-error' else (0,'8.4.10' if fault=='wrong-version' else '8.0.46','')
            if sql.startswith('CREATE TABLE'):
                foreign=True;return 1,'','ERROR 1050 table already exists'
            if sql.startswith('SELECT COUNT(*)'):
                return (1,'','COUNT failure') if fault=='count-error' else (0,'','') if fault=='count-empty' else (0,'1' if fault=='preexisting' or foreign else '0','')
            return 0,verify_values.get(sql,''),''
        parsed=parsed_for(source)
        with mock.patch.object(runner,'mysql_exec',side_effect=mysql),mock.patch.object(runner,'image_digest',return_value='constructed'),mock.patch.object(runner,'run_cmd',return_value=(1,json.dumps(parsed),'')):
            record=runner.execute_metadata_case(manifest,'mysql80','/constructed/cli',policies,source)
        no_drop=not any(sql.startswith('DROP TABLE') for sql in ledger)
        no_early_create=fault=='collision' or not any(sql.startswith('CREATE TABLE') for sql in ledger)
        results.append((fault+' preserves unowned resources',record['status']=='fail' and no_drop and no_early_create))
    provider=next(row for row in manifest['metadata_cases'] if row['id']=='t07-provider-mysql80-g01-stored-dependent-drop')
    ledger=[];created=False;dropped=False
    values={q['sql']:q['expect'] for step in provider['setup'] for q in step.get('verify',[])}
    def owned_mysql(anchor,sql,database=None,silent=True):
        nonlocal created,dropped
        ledger.append(sql)
        if sql=='SELECT VERSION()':return 0,'8.0.46',''
        if sql.startswith('CREATE TABLE'):created=True;return 0,'',''
        if sql.startswith('DROP TABLE'):
            if not created:raise AssertionError('unowned DROP')
            dropped=True;return 0,'',''
        if sql.startswith('SELECT COUNT(*)'):
            return (1,'','intermediate COUNT failure') if created and not dropped else (0,'0','')
        return 0,values.get(sql,''),''
    with mock.patch.object(runner,'mysql_exec',side_effect=owned_mysql),mock.patch.object(runner,'image_digest',return_value='constructed'),mock.patch.object(runner,'run_cmd') as command:
        record=runner.execute_metadata_case(manifest,'mysql80','/constructed/cli',policies,provider)
    results.append(('owned intermediate failure cleans without audit',record['status']=='fail' and dropped and command.call_count==0 and record['actual']['owned_cleanup']['count']['stdout']=='0'))
    for exception_cleanup in (False,True):
        def fail_after_owned(*args,fixture=None,**kwargs):
            fixture['case']={'actual':{},'assertions':[],'status':'pass'};fixture['created']=True;raise RuntimeError('primary controlled failure')
        def cleanup_fail(*args,**kwargs):
            if exception_cleanup:raise RuntimeError('cleanup controlled failure')
            return 1,'','cleanup controlled failure'
        with mock.patch.object(runner,'_execute_metadata_case',side_effect=fail_after_owned),mock.patch.object(runner,'mysql_exec',side_effect=cleanup_fail):
            record=runner.execute_metadata_case(manifest,'mysql80','/constructed/cli',policies,source)
        results.append(('primary and cleanup errors preserved '+str(exception_cleanup),record['status']=='fail' and record['actual']['fixture_error']['message']=='primary controlled failure' and len(record['actual']['owned_cleanup'])==2))
    secret='T07_ENV_ONLY_CREDENTIAL_SENTINEL'
    anchor={'exec_client':['mysql','-uroot'],'container':'constructed','exec_password_env':'T07_PASSWORD_TEST'}
    with mock.patch.dict(os.environ,{'T07_PASSWORD_TEST':secret}),mock.patch.object(runner,'run_cmd',return_value=(0,'','')) as command:
        runner.mysql_exec(anchor,'SELECT 1')
    args,kwargs=command.call_args;results.append(('fixture credentials environment-only',secret not in repr(args) and kwargs['env']['MYSQL_PWD']==secret and args[0][:5]==['docker','exec','--env','MYSQL_PWD','constructed']))
    environment={key:value for key,value in os.environ.items() if key!='T07_PASSWORD_TEST'}
    with mock.patch.dict(os.environ,environment,clear=True),mock.patch.object(runner,'run_cmd') as command:
        result=runner.mysql_exec(anchor,'SELECT 1')
    results.append(('missing fixture environment fails closed',result[0]!=0 and command.call_count==0))
    return results


def main():
    results=[]
    with tempfile.TemporaryDirectory(prefix='t07-validator-') as directory:
        artifact,manifest=fixture(pathlib.Path(directory));baseline=runner.load_baseline()
        def check(name,a,m=None,reject=True):
            try:failures=runner.validate_artifact(a,m or manifest,baseline=baseline,verify_binary=True)
            except Exception as error:
                results.append(False);print('FAIL '+name+': exception '+repr(error));return
            ok=bool(failures) if reject else not failures
            results.append(ok);print(('PASS ' if ok else 'FAIL ')+name)
            if not ok:print(json.dumps(failures,ensure_ascii=False,indent=2))
        check('T07 complete constructed control',artifact,reject=False)
        def take(suffix):
            a=copy.deepcopy(artifact);record=next(row for row in a['cases'] if row['case_id'].endswith(suffix));return a,record
        def sync(record):record['actual']['stdout']=json.dumps(record['actual']['parsed'])
        for field,value in [('column','c'),('dependent_column','c'),('dependency_kind','expression_default'),('dependency','transitive'),('action','rename_column')]:
            a,c=take('mysql80-g01-stored-dependent-drop');c['actual']['parsed']['statements'][1]['findings'][0]['metadata'][field]=value;sync(c);check('T07 wrong finding '+field,a)
        a,c=take('mysql80-g01-stored-dependent-drop');p=c['actual']['parsed'];p['statements'][0]['findings']=p['statements'][1]['findings'];p['statements'][1]['findings']=[];sync(c);check('T07 wrong statement ownership',a)
        for operation in ('remove','duplicate','wrong-rule','wrong-message','wrong-location'):
            a,c=take('mysql80-g01-stored-dependent-drop');findings=c['actual']['parsed']['statements'][1]['findings']
            if operation=='remove':findings.clear()
            elif operation=='duplicate':findings.append(copy.deepcopy(findings[0]))
            elif operation=='wrong-rule':findings[0]['rule_id']='ddl.table.exists.alter.require'
            elif operation=='wrong-message':findings[0]['message']='generic notice'
            else:findings[0]['location']['line']=1
            sync(c);check('T07 finding '+operation,a)
        a,c=take('mysql80-g03-virtual-transitive');p=c['actual']['parsed'];p['statements'][1]['findings'][1]['metadata']=copy.deepcopy(p['statements'][1]['findings'][0]['metadata']);sync(c);check('T07 cross-dependent pairing',a)
        a,c=take('t07-mysql-dependency-unknown-source');p=c['actual']['parsed'];p['statements'][0]['evidence_gaps']=[];p['statements'][0]['coverage']['status']='complete';p['coverage']['status']='complete';p['verdict']='pass';sync(c);check('T07 unknown dependencies laundered to none',a)
        a,c=take('mysql80-g01-stored-dependent-drop');p=c['actual']['parsed'];p['statements'][0]['evidence_gaps']=[{'rule_id':contract.DEP,'reason_code':'unknown_table_state','required_facts':['target_table.columns']}];sync(c);check('T07 declaration known laundered metadata unknown',a)
        a,c=take('t07-tidb-visibility-disabled');p=c['actual']['parsed'];p['unsupported']=[];p['diagnostics']=[];p['coverage']['status']='complete';p['statements'][0]['coverage']['status']='complete';p['verdict']='pass';c['actual']['exit']=0;sync(c);check('T07 vendor negative replaced by positive',a)
        for category in ('setup','execute','structure','post_verify','teardown'):
            a,c=take('mysql80-g01-stored-dependent-drop');c['actual'][category]=[];check('T07 skipped '+category,a)
        for change in ('missing-column','duplicate-column','missing-dependency','failed-query','false-rc'):
            a,c=take('mysql80-g01-stored-dependent-drop');query=c['actual']['execute'][0]['verify'][1]
            if change=='missing-column':query['output']='a:1:ok'
            elif change=='duplicate-column':query['output']='a:1:ok,a:1:ok'
            elif change=='missing-dependency':query['output']='a:1:ok,b:2:bad'
            elif change=='failed-query':query['rc']=1
            else:query['rc']=False
            check('T07 structure '+change,a)
        a,c=take('mysql80-g01-stored-dependent-drop');c['actual']['execute'][1]['stderr']='';check('T07 rejection reason skipped',a)
        a,c=take('mysql80-g01-stored-dependent-drop');c['anchor']='mysql84';check('T07 anchor rebind',a)
        a,c=take('t07-mysql-generated-not-required');c['policy_profile']='all-rules-disabled';check('T07 required=false profile swap',a)
        for value in (None,False,1.0,'1'):
            for field in ('loaded','applicable'):
                a,c=take('t07-mysql-generated-not-required');c['actual']['parsed']['rule_summary'][field]=value;sync(c);check('T07 strict '+field+' '+repr(value),a)
            a,c=take('mysql80-g01-stored-dependent-drop');c['actual']['parsed']['summary']['blockers']=value;sync(c);check('T07 strict raw summary '+repr(value),a)
            a=copy.deepcopy(artifact);a['executed_count']=value;check('T07 strict denominator '+repr(value),a)
        for value in (None,False,{},[],0):
            a,c=take('t07-mysql-generated-disabled');c['actual']['parsed']['rule_summary']=value;sync(c);check('T07 disabled summary key '+repr(value),a)
        a,c=take('t07-mysql-generated-not-required');c['actual']['parsed']['rule_summary']['applicable']=1;sync(c);check('T07 required=false applicable',a)
        a,c=take('t07-mysql-unknown-expression-privacy');p=c['actual']['parsed'];p['unsupported']=[];p['diagnostics']=[];p['statements'][0]['coverage']['status']='complete';p['coverage']['status']='complete';sync(c);check('T07 unknown expression disguised empty references',a)
        for channel in ('finding','gap','diagnostic','explanation'):
            a,c=take('t07-mysql-privacy-literal-nonreference');p=c['actual']['parsed']
            if channel=='finding':p['statements'][0]['findings']=[{'rule_id':contract.DEP,'message':contract.MARKER}]
            elif channel=='gap':p['statements'][0]['evidence_gaps']=[{'rule_id':contract.DEP,'reason_code':contract.MARKER}]
            elif channel=='diagnostic':p['diagnostics']=[{'reason':contract.MARKER}]
            else:p['explanation']={'summary':contract.MARKER}
            sync(c);check('T07 privacy '+channel,a)
        for field in ('verify','execute','case'):
            a,c=take('mysql80-g01-stored-dependent-drop');m=copy.deepcopy(manifest);spec=next(s for s in m['metadata_cases'] if s['id']==c['cli_case'])
            if field=='verify':spec['execute'][0]['verify'].pop();c['actual']['execute'][0]['verify'].pop()
            elif field=='execute':spec['execute'].pop();c['actual']['execute'].pop()
            else:
                m['metadata_cases'].remove(spec);m['required_case_ids'].remove(c['case_id']);a['required_case_ids'].remove(c['case_id']);a['cases'].remove(c);a['executed_count']-=1
            check('T07 double-side deletion '+field,a,m)
        a=copy.deepcopy(artifact);a['cases'][-1]=copy.deepcopy(a['cases'][0]);check('T07 duplicate case ID',a)
        a,c=take('mysql80-g01-stored-dependent-drop');c['actual']['parsed']['summary']['blockers']=0;check('T07 raw/parsed drift',a)
        for field in ('statements','warnings','notices'):
            for value in (False,0.0,None):
                a,c=take('mysql80-g01-stored-dependent-drop');c['actual']['parsed']['summary'][field]=value;sync(c);check('T07 raw summary '+field+' '+repr(value),a)
        a,c=take('t07-mysql-visibility-boundary-8.0.22');p=c['actual']['parsed'];p['version']=identity('mysql','8.0.23','target');sync(c);check('T07 boundary version role swap',a)
        a,c=take('tidb85-v01-create-invisible');positive=next(row for row in artifact['cases'] if row['case_id'].endswith('mysql80-v01-create-invisible'));c['actual']=copy.deepcopy(positive['actual']);check('T07 vendor negative cross-anchor positive',a)
        a,c=take('mysql80-g01-stored-dependent-drop');c['actual']['execute'].reverse();check('T07 native step reorder',a)
        a,c=take('t07-mysql-privacy-literal-nonreference');c['actual']['parsed']['explanation']={contract.MARKER:'fixed'};sync(c);check('T07 private literal payload key',a)
        a=copy.deepcopy(artifact);a['cases'][0]['case_id']={};check('T07 malformed case ID',a)
        for name,ok in fixture_guard_checks(manifest):
            results.append(ok);print(('PASS ' if ok else 'FAIL ')+'T07 fixture '+name)
        print(json.dumps({'constructed_checks':len(results),'failures':sum(not value for value in results),'note':'validator sensitivity only; not product or native execution'},indent=2))
        return 0 if all(results) else 1


if __name__=='__main__':raise SystemExit(main())
