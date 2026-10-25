import copy
import hashlib
import json
from pathlib import Path

DEP = 'ddl.alter.column.dependency.require'
COMPAT = 'ddl.column.advanced.compatibility.require'
INDEX = 'ddl.create_index.columns.exists.require'
RENAME = 'ddl.alter.rename_column.version.require'
VENDOR_REASON = 'parsed by the shared parser but outside the supported statement surface for this dialect'
UNAUDITED_REASON = 'parsed by the shared parser but not covered by audited semantics'
VERSIONS = {'mysql57':'5.7.44','mysql80':'8.0.46','mysql84':'8.4.10','tidb85':'8.5.0'}
PORTS = {'mysql57':23357,'mysql80':23380,'mysql84':23384,'tidb85':24000}
MARKER = 'T07_PRIVATE_LITERAL_6b4d1e'
FROZEN_REQUIRED_CASES = 207
FROZEN_IDS_SHA256 = 'd723ac9553bf4e6ccc6ba1a73be2e4ba036903bb77a32c72f6b49878502f1d25'


def column(name, data_type='int', storage='', expr='', default=None, invisible=False):
    return {'name':name,'data_type':data_type,'storage':storage,'expression':expr,'default':default,'invisible':invisible}


def create(definition):
    return 'CREATE TABLE t ('+definition+') ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;'


def role(name, definition, actions=(), states=(), *, profile='t07-all', indexes=()):
    initial = list(states[0]) if states else []
    return {'role':name,'sql':[create(definition),*actions],'states':[initial,*[list(value) for value in states[1:]]],'profile':profile,'indexes':list(indexes)}


def native_roles():
    a,b,c=column('a'),column('b',storage='stored',expr='a+1'),column('c')
    v=column('b',storage='virtual',expr='a+1')
    rows=[]
    first=role('g01-stored-dependent-drop','a INT,b INT GENERATED ALWAYS AS (a+1) STORED',['ALTER TABLE t DROP COLUMN a;'],[[a,b],[a,b]])
    first['sql'][0]='CREATE TABLE t (\n  a INT,\n  b INT GENERATED ALWAYS AS (a + 1) STORED\n);'
    first['profile']='t07-dependency'
    rows.append(first)
    rows += [
      role('g02-stored-unrelated-drop','a INT,b INT AS (a+1) STORED,c INT',['ALTER TABLE t DROP COLUMN c;'],[[a,b,c],[a,b]]),
      role('g03-virtual-transitive','a INT,b INT AS (a+1) VIRTUAL,c INT AS (b+1) VIRTUAL',['ALTER TABLE t DROP COLUMN a;'],[[a,v,column('c',storage='virtual',expr='b+1')],[a,v,column('c',storage='virtual',expr='b+1')]]),
      role('g04-omitted-storage','a INT,b INT AS (a+1)',states=[[a,v]]),
      role('g05-literal-not-reference',"a INT,b VARCHAR(8) AS ('a') VIRTUAL,c INT",['ALTER TABLE t DROP COLUMN a;'],[[a,column('b','varchar','virtual',"'a'"),c],[column('b','varchar','virtual',"'a'"),c]]),
      role('g06-quoted-identifier','`a b` INT,b INT AS (`a b`+1) STORED',['ALTER TABLE t DROP COLUMN `a b`;'],[[column('a b'),column('b',storage='stored',expr='a b+1')],[column('a b'),column('b',storage='stored',expr='a b+1')]]),
      role('g07-generated-index','a INT,b INT AS (a+1) STORED',['CREATE INDEX idx_b ON t(b);'],[[a,b],[a,b]],indexes=['','idx_b:1:b']),
      role('g08-add-virtual','a INT,c INT',['ALTER TABLE t ADD COLUMN b INT AS (a+1) VIRTUAL;','ALTER TABLE t DROP COLUMN a;'],[[a,c],[a,c,v],[a,c,v]]),
      role('g09-add-stored','a INT,c INT',['ALTER TABLE t ADD COLUMN b INT AS (a+1) STORED;'],[[a,c],[a,c,b]]),
      role('g10-modify-stored-expression','a INT,b INT AS (a+1) STORED',['ALTER TABLE t MODIFY COLUMN b INT AS (a+2) STORED;'],[[a,b],[a,column('b',storage='stored',expr='a+2')]]),
      role('g11-change-stored-name','a INT,b INT AS (a+1) STORED',['ALTER TABLE t CHANGE COLUMN b b2 INT AS (a+1) STORED;'],[[a,b],[a,column('b2',storage='stored',expr='a+1')]]),
      role('g12-virtual-to-stored','a INT,b INT AS (a+1) VIRTUAL',['ALTER TABLE t MODIFY COLUMN b INT AS (a+1) STORED;'],[[a,v],[a,v]]),
      role('g13-ordinary-to-stored','a INT,b INT',['ALTER TABLE t MODIFY COLUMN b INT AS (a+1) STORED;'],[[a,column('b')],[a,b]]),
      role('g14-stored-to-ordinary','a INT,b INT AS (a+1) STORED',['ALTER TABLE t MODIFY COLUMN b INT;'],[[a,b],[a,column('b')]]),
      role('g15-drop-generated-leaf','a INT,b INT AS (a+1) STORED',['ALTER TABLE t DROP COLUMN b;'],[[a,b],[a]]),
      role('g16-unrelated-modify','a INT,b INT AS (a+1) STORED,c INT',['ALTER TABLE t MODIFY COLUMN c BIGINT;'],[[a,b,c],[a,b,column('c','bigint')]]),
      role('g17-change-referenced-base','a INT,b INT AS (a+1) STORED',['ALTER TABLE t CHANGE COLUMN a a2 INT;'],[[a,b],[a,b]]),
      role('g18-rename-referenced-base','a INT,b INT AS (a+1) STORED',['ALTER TABLE t RENAME COLUMN a TO a2;'],[[a,b],[a,b]]),
      role('d01-literal','c INT DEFAULT 1',states=[[column('c',default='literal:1')]]),
      role('d02-sql-null','c INT DEFAULT NULL',states=[[column('c')]]),
      role('d03-constant-expression','c INT DEFAULT (1+2)',states=[[column('c',default='expr:1+2')]]),
      role('d04-column-expression','a INT,b INT DEFAULT (a+1)',['ALTER TABLE t DROP COLUMN a;'],[[a,column('b',default='expr:a+1')],[a,column('b',default='expr:a+1')]]),
      role('d05-json-object',"j JSON DEFAULT (JSON_OBJECT('k',1))",states=[[column('j','json',default='expr:json_object')]]),
      role('d06-alter-set-expression','a INT,b INT',['ALTER TABLE t ALTER COLUMN b SET DEFAULT (a+1);'],[[a,column('b')],[a,column('b',default='expr:a+1')]]),
      role('d07-alter-literal-drop','c INT DEFAULT 1',['ALTER TABLE t ALTER COLUMN c SET DEFAULT 2;','ALTER TABLE t ALTER COLUMN c DROP DEFAULT;'],[[column('c',default='literal:1')],[column('c',default='literal:2')],[column('c')]]),
      role('d08-alter-json-set','j JSON',["ALTER TABLE t ALTER COLUMN j SET DEFAULT (JSON_OBJECT('k',1));"],[[column('j','json')],[column('j','json',default='expr:json_object')]]),
      role('d09-dependent-generated-chain','a INT,b INT DEFAULT (a+1),c INT AS (b+1) STORED',['ALTER TABLE t DROP COLUMN a;'],[[a,column('b',default='expr:a+1'),column('c',storage='stored',expr='b+1')],[a,column('b',default='expr:a+1'),column('c',storage='stored',expr='b+1')]]),
      role('d10-alter-uuid-default','c VARCHAR(36),x INT',['ALTER TABLE t ALTER COLUMN c SET DEFAULT (UUID());','ALTER TABLE t ALTER COLUMN c DROP DEFAULT;'],[[column('c','varchar'),column('x')],[column('c','varchar',default='expr:uuid'),column('x')],[column('c','varchar'),column('x')]]),
      role('d11-drop-json-expression-default',"j JSON DEFAULT (JSON_OBJECT('k',1))",['ALTER TABLE t ALTER COLUMN j DROP DEFAULT;'],[[column('j','json',default='expr:json_object')],[column('j','json')]]),
    ]
    base=[column('id'),column('hidden_c',invisible=True),column('visible_c')]
    visible=[column('id'),column('hidden_c'),column('visible_c')]
    rows += [
      role('v01-create-invisible','id INT PRIMARY KEY,hidden_c INT INVISIBLE,visible_c INT',states=[base],indexes=['PRIMARY:1:id']),
      role('v02-create-explicit-visible','id INT PRIMARY KEY,hidden_c INT VISIBLE,visible_c INT',states=[visible],indexes=['PRIMARY:1:id']),
      role('v03-create-omitted','id INT PRIMARY KEY,hidden_c INT,visible_c INT',states=[visible],indexes=['PRIMARY:1:id']),
      role('v04-alter-visibility','id INT PRIMARY KEY,hidden_c INT,visible_c INT',['ALTER TABLE t ALTER COLUMN hidden_c SET INVISIBLE;','ALTER TABLE t ALTER COLUMN hidden_c SET VISIBLE;'],[visible,base,visible],indexes=['PRIMARY:1:id']*3),
      role('v05-modify-omitted','id INT PRIMARY KEY,hidden_c INT INVISIBLE,visible_c INT',['ALTER TABLE t MODIFY COLUMN hidden_c INT;'],[base,visible],indexes=['PRIMARY:1:id']*2),
      role('v06-change-omitted','id INT PRIMARY KEY,hidden_c INT INVISIBLE,visible_c INT',['ALTER TABLE t CHANGE COLUMN hidden_c hidden_new INT;'],[base,[column('id'),column('hidden_new'),column('visible_c')]],indexes=['PRIMARY:1:id']*2),
      role('v07-all-invisible','hidden_c INT INVISIBLE',states=[[column('hidden_c',invisible=True)]]),
      role('v08-hidden-index','id INT PRIMARY KEY,hidden_c INT INVISIBLE,visible_c INT',['CREATE INDEX idx_hidden ON t(hidden_c);'],[base,base],indexes=['PRIMARY:1:id','PRIMARY:1:id,idx_hidden:1:hidden_c']),
      role('x01-generated-invisible','a INT,b INT AS (a+1) STORED INVISIBLE,c INT',['CREATE INDEX idx_b ON t(b);','ALTER TABLE t DROP COLUMN a;'],[[a,column('b',storage='stored',expr='a+1',invisible=True),c]]*3,indexes=['','idx_b:1:b','idx_b:1:b']),
    ]
    assert len(rows)==38 and len({row['role'] for row in rows})==38
    return rows


def dependency(index, column_name='a', dependent='b', kind='generated', relation='direct', action='drop_column'):
    verb='drop' if action=='drop_column' else 'rename'
    label='generated' if kind=='generated' else 'expression-default'
    return {'index':index,'rule_id':DEP,'level':'blocker','message':f'cannot {verb} column "{column_name}": {label} column "{dependent}" depends on it','metadata':{'table':'t','column':column_name,'dependent_column':dependent,'dependency_kind':kind,'dependency':relation,'action':action}}


def incompatibility(index, column_name, feature, action, reason, anchor, minimum=''):
    return {'index':index,'rule_id':COMPAT,'level':'blocker','message':f'column "{column_name}" advanced definition is not supported for the target ({reason})','metadata':{'table':'t','column':column_name,'feature':feature,'action':action,'reason_code':reason,'product':'tidb' if anchor=='tidb85' else 'mysql','target_version':VERSIONS.get(anchor,anchor),'minimum_version':minimum}}


def unsupported(index, action='create_table'):
    suffix='create_table.column.visibility' if action=='create_table' else 'alter_table.set_column_visibility.column.visibility'
    return {'index':index,'feature':suffix,'reason':VENDOR_REASON,'metadata':{'aspect':'column'}}


def expected(sqls, findings=(), gaps=(), unsupported_entries=()):
    coverage=['complete']*len(sqls)
    for gap in gaps: coverage[gap['index']]='unverified'
    for item in unsupported_entries: coverage[item['index']]='incomplete'
    total='incomplete' if 'incomplete' in coverage else 'unverified' if 'unverified' in coverage else 'complete'
    verdict='reject' if findings else 'review' if total!='complete' else 'pass'
    result={'exit':1 if findings or unsupported_entries else 0,'verdict':verdict,'statements':len(sqls),'findings':len(findings),'diagnostics':len(unsupported_entries),'unsupported':len(unsupported_entries),'coverage':total,'statement_coverage':coverage,'statement_sql':sqls,'evidence_gaps':len(gaps),'evidence_gap_entries':list(gaps),'finding_entries':[{'index':f['index'],'rule_id':f['rule_id'],'level':f['level']} for f in findings],'finding_metadata':[{'index':f['index'],'rule_id':f['rule_id'],'metadata':f['metadata']} for f in findings],'finding_locations':[],'unsupported_entries':list(unsupported_entries)}
    line=1;locations=[]
    for stmt in sqls: locations.append(line);line+=stmt.count('\n')+1
    for f in findings: result['finding_locations'].append({'index':f['index'],'line':locations[f['index']],'column':1})
    result['t07_exact_findings']=list(findings)
    return result


def native_variant(row, anchor):
    r=copy.deepcopy(row);name=r['role'];fail={};findings=[];uns=[]
    mysql57=anchor=='mysql57';tidb=anchor=='tidb85'
    if name in ('g01-stored-dependent-drop','g03-virtual-transitive','g06-quoted-identifier','g17-change-referenced-base','g18-rename-referenced-base'):
        stage=1;action='change_column' if name.startswith('g17') else 'rename_column' if name.startswith('g18') else 'drop_column'
        findings=[dependency(stage,'a b' if name.startswith('g06') else 'a',action=action)]
        if name.startswith('g03'): findings.append(dependency(stage,dependent='c',relation='transitive'))
        fail[stage]=1064 if mysql57 and name.startswith('g18') else 1054 if mysql57 and name.startswith('g17') else 3108
        if mysql57 and name.startswith('g18'):
            findings.append({'index':1,'rule_id':RENAME,'level':'blocker','message':'RENAME COLUMN requires MySQL 8.0.3 or newer; 5.7.44 is not supported','metadata':{'action':'rename_column','product':'mysql','target_version':'5.7.44','minimum_supported_version':'8.0.3'}})
    if name=='g08-add-virtual': findings=[dependency(2)];fail[2]=3108
    if name=='g09-add-stored' and tidb: fail[1]=3106;findings=[incompatibility(1,'b','generated','add_columns','tidb_add_stored',anchor)]
    if name=='g10-modify-stored-expression' and tidb: fail[1]=3106;findings=[incompatibility(1,'b','generated','modify_column','tidb_stored_expression_change',anchor)]
    if name=='g12-virtual-to-stored': fail[1]=3106;findings=[incompatibility(1,'b','generated','modify_column','generated_storage_change',anchor)]
    if name=='g13-ordinary-to-stored' and tidb: fail[1]=3106;findings=[incompatibility(1,'b','generated','modify_column','tidb_ordinary_to_stored',anchor)]
    if name in ('d03-constant-expression','d04-column-expression','d05-json-object','d09-dependent-generated-chain','d11-drop-json-expression-default') and mysql57:
        fail[0]=1064;r['sql']=r['sql'][:1];r['states']=[[]]
        col='c' if name.startswith('d03') else 'j' if name.startswith(('d05','d11')) else 'b'
        findings=[incompatibility(0,col,'expression_default','create_table','version_unsupported',anchor,'8.0.13')]
    elif name in ('d03-constant-expression','d04-column-expression','d09-dependent-generated-chain') and tidb:
        fail[0]=1064;r['sql']=r['sql'][:1];r['states']=[[]]
        findings=[incompatibility(0,'c' if name.startswith('d03') else 'b','expression_default','create_table','tidb_default_expression',anchor)]
    elif name in ('d04-column-expression','d09-dependent-generated-chain'):
        findings=[dependency(1,kind='expression_default')];fail[1]=3766
        if name.startswith('d09'): findings.append(dependency(1,dependent='c',relation='transitive'))
    if name in ('d06-alter-set-expression','d08-alter-json-set','d10-alter-uuid-default'):
        if mysql57:
            fail[1]=1064;r['sql']=r['sql'][:2];r['states']=r['states'][:2]
            findings=[incompatibility(1,'b' if name.startswith('d06') else 'j' if name.startswith('d08') else 'c','expression_default','set_column_default','version_unsupported',anchor,'8.0.13')]
        elif tidb and name in ('d06-alter-set-expression','d08-alter-json-set'):
            fail[1]=1054 if name.startswith('d06') else 1101
            findings=[incompatibility(1,'b' if name.startswith('d06') else 'j','expression_default','set_column_default','tidb_default_expression' if name.startswith('d06') else 'tidb_alter_json_default',anchor)]
    if name.startswith('v') and name!='v03-create-omitted' or name=='x01-generated-invisible':
        stage=1 if name=='v04-alter-visibility' else 0
        if mysql57 or tidb:
            if stage==0: r['sql']=r['sql'][:1];r['states']=[[]]
            elif mysql57: r['sql']=r['sql'][:2];r['states']=r['states'][:2]
            for idx in range(stage,len(r['sql'])):
                fail[idx]=1064
                if tidb: uns.append(unsupported(idx,'create_table' if idx==0 else 'set_column_visibility'))
            if mysql57: findings=[incompatibility(stage,'b' if name.startswith('x') else 'hidden_c','visibility','create_table' if stage==0 else 'set_column_visibility','version_unsupported',anchor,'8.0.23')]
        elif name=='v07-all-invisible': fail[0]=4028;r['states']=[[]];findings=[incompatibility(0,'hidden_c','visibility','create_table','no_visible_columns',anchor)]
        elif name=='x01-generated-invisible': fail[2]=3108;findings=[dependency(2)]
    for idx in sorted(fail):
        if idx>=len(r['states']): continue
        if idx==0: r['states'][idx]=[]
        else: r['states'][idx]=copy.deepcopy(r['states'][idx-1])
    r['failures']=fail;r['expect']=expected(r['sql'],findings,unsupported_entries=uns)
    return r


def quote(value): return "'"+str(value).replace("'","''")+"'"


def expression_forms(key):
    if key in ('a+1','a+2','b+1','a b+1'):
        left,right=key.rsplit('+',1)
        return [f'`{left}` + {right}',f'`{left}`+{right}',f'(`{left}` + {right})',f'(`{left}`+{right})']
    if key=="'a'": return ["'a'","_utf8mb4'a'","('a')","(_utf8mb4'a')",r"_utf8mb4\'a\'"]
    if key=='1+2': return ['(1 + 2)','(1+2)','1 + 2','1+2']
    if key=='json_object': return ["json_object(_utf8mb4'k',1)","json_object(_utf8mb4'k', 1)","json_object('k',1)","json_object('k', 1)",r"json_object(_utf8mb4\'k\',1)"]
    if key=='uuid': return ['uuid()','(uuid())']
    raise ValueError('unfrozen expression form '+key)


def hex_predicate(field,key):
    return 'HEX('+field+') IN ('+','.join(quote(value.encode().hex().upper()) for value in expression_forms(key))+')'


def columns_check(columns):
    branches=[]
    for col in columns:
        attrs=[]
        if col['storage']: attrs.append(col['storage']+' generated')
        if col['default'] and col['default'].startswith('expr:'): attrs.append('default_generated')
        if col['invisible']: attrs.append('invisible')
        extra=' '.join(attrs)
        tests=['CAST(column_name AS BINARY)='+quote(col['name']),'data_type='+quote(col['data_type']),'CAST(LOWER(extra) AS BINARY)='+quote(extra)]
        tests.append(hex_predicate('generation_expression',col['expression']) if col['expression'] else "COALESCE(generation_expression,'')=''")
        value=col['default']
        tests.append('column_default IS NULL' if value is None else hex_predicate('column_default',value[5:]) if value.startswith('expr:') else 'CAST(column_default AS BINARY)='+quote(value[8:]))
        branches.append('WHEN '+' AND '.join(tests)+" THEN 'ok'")
    body='CASE '+' '.join(branches)+" ELSE 'bad' END" if branches else "'bad'"
    query="SELECT COALESCE(GROUP_CONCAT(CONCAT(column_name,':',ordinal_position,':',"+body+") ORDER BY ordinal_position SEPARATOR ','),'') FROM information_schema.columns WHERE table_schema='golden' AND table_name='t'"
    return {'assert':'complete advanced column set','sql':query,'expect':','.join(f"{col['name']}:{i+1}:ok" for i,col in enumerate(columns))}


def table_check(value):
    return {'assert':'bound table count','sql':"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='golden' AND table_name='t'",'expect':str(value)}


def indexes_check(value):
    return {'assert':'complete index identity','sql':"SELECT COALESCE(GROUP_CONCAT(CONCAT(index_name,':',seq_in_index,':',column_name) ORDER BY CAST(index_name AS BINARY),seq_in_index SEPARATOR ','),'') FROM information_schema.statistics WHERE table_schema='golden' AND table_name='t'",'expect':value}


def policy(enable):
    return {name:{'enabled':True,'level':'blocker','params':params} for name,params in enable.items()}


def profiles():
    both={DEP:{'required':True},COMPAT:{'required':True},INDEX:{},RENAME:{'required':True}}
    return {'t07-all':{'enable':policy(both)},'t07-dependency':{'enable':policy({DEP:{'required':True}})},'t07-compat':{'enable':policy({COMPAT:{'required':True}})},'t07-not-required':{'enable':policy({DEP:{'required':False},COMPAT:{'required':False}})}}


def metadata_case(row,anchor):
    variant=native_variant(row,anchor)
    execute=[];last_index=''
    for idx,sql in enumerate(variant['sql']):
        fail=variant['failures'].get(idx)
        indexes=variant['indexes'][min(idx,len(variant['indexes'])-1)] if variant['indexes'] else ''
        if fail: indexes=last_index
        state=variant['states'][idx]
        step={'name':f'step-{idx}','sql':sql,'expect_rc':1 if fail else 0,'verify':[table_check(1 if state else 0),columns_check(state),indexes_check(indexes)]}
        if idx==0:step['creates_fixture']=True
        if fail: step['stderr_contains']=['ERROR '+str(fail)]
        execute.append(step);last_index=indexes
        if fail and not state: break
    connect={'host':'127.0.0.1','port':PORTS[anchor],'user':'root','schema':'golden'}
    if anchor!='tidb85': connect['password_env']='T07_MYSQL_PASSWORD'
    exists=bool(variant['states'][len(execute)-1])
    teardown=[{'name':'drop-owned-t','sql':'DROP TABLE t;','expect_rc':0,'drops_fixture':True,'verify':[table_check(0)]}] if exists else [{'name':'verify-absent-t','sql':table_check(0)['sql'],'expect_rc':0,'verify':[table_check(0)]}]
    return {'id':'t07-'+anchor+'-'+row['role'],'anchor':anchor,'dialect':'tidb' if anchor=='tidb85' else 'mysql','sql':'\n'.join(variant['sql']),'policy':variant['profile'],'connect':connect,'setup':[{'name':'prove-prior-absence','sql':table_check(0)['sql'],'expect_rc':0,'requires_absence':True,'verify':[table_check(0)]}],'expect':variant['expect'],'post_verify':[table_check(0)],'execute':execute,'structure':[table_check(1 if exists else 0),columns_check(variant['states'][len(execute)-1]),indexes_check(last_index)],'teardown':teardown}


def cli_cases():
    rows=[]
    seed=native_roles()[0]['sql'];default_sql=[create('a INT,b INT DEFAULT (a+1)'), 'ALTER TABLE t DROP COLUMN a;'];visible_sql=[create('id INT PRIMARY KEY,hidden_c INT INVISIBLE,visible_c INT')]
    for dialect,version in [('mysql','8.0.46'),('tidb','8.5.0')]:
        for label,sqls in [('generated',seed),('expression-default',default_sql),('visibility',visible_sql)]:
            for switch,profile in [('not-required','t07-not-required'),('disabled','all-rules-disabled')]:
                # Disabling a check never erases the known TiDB visibility boundary.
                uns=[unsupported(0)] if dialect=='tidb' and label=='visibility' else []
                rows.append({'id':f't07-{dialect}-{label}-{switch}','dialect':dialect,'sql':'\n'.join(sqls),'policy':profile,'args':['--target-version',version,'--fail-on','blocker'],'expect':expected(sqls,unsupported_entries=uns)})
        for label,sqls in [('generated',[seed[0]]),('expression-default',[create("j JSON DEFAULT (JSON_OBJECT('k',1))")]),('visibility',visible_sql)]:
            if dialect=='tidb' and label=='visibility': uns=[unsupported(0)];gaps=[]
            else: uns=[];gaps=[{'index':0,'rule_id':COMPAT,'reason_code':'missing_target_version','required_facts':['target.version']}]
            rows.append({'id':f't07-{dialect}-{label}-missing-version','dialect':dialect,'sql':'\n'.join(sqls),'policy':'t07-compat','args':['--fail-on','blocker'],'expect':expected(sqls,gaps=gaps,unsupported_entries=uns)})
    for feature,sqls,versions,column_name,minimum in [('generated',[seed[0]],('5.7.5','5.7.6'),'b','5.7.6'),('expression_default',[create('c INT DEFAULT (1+2)')],('8.0.12','8.0.13'),'c','8.0.13'),('visibility',visible_sql,('8.0.22','8.0.23'),'hidden_c','8.0.23')]:
        for idx,version in enumerate(versions):
            hits=[] if idx else [incompatibility(0,column_name,feature,'create_table','version_unsupported',version,minimum)]
            rows.append({'id':f't07-mysql-{feature}-boundary-{version}','dialect':'mysql','sql':'\n'.join(sqls),'policy':'t07-compat','args':['--target-version',version,'--fail-on','blocker'],'expect':expected(sqls,hits)})
    # Privacy is checked independently by the T07 validator, excluding only existing input-echo paths.
    for dialect,version in [('mysql','8.0.46'),('tidb','8.5.0')]:
        sqls=[create("a INT,b VARCHAR(64) AS ('"+MARKER+"') VIRTUAL"),'ALTER TABLE t DROP COLUMN a;']
        rows.append({'id':f't07-{dialect}-privacy-literal-nonreference','dialect':dialect,'sql':'\n'.join(sqls),'policy':'t07-all','args':['--target-version',version,'--fail-on','blocker'],'expect':expected(sqls)})
        unknown=['ALTER TABLE t DROP COLUMN a;']
        gap=[{'index':0,'rule_id':DEP,'reason_code':'unknown_table_state','required_facts':['target_table.columns']}]
        rows.append({'id':f't07-{dialect}-dependency-unknown-source','dialect':dialect,'sql':unknown[0],'policy':'t07-dependency','args':['--target-version',version,'--fail-on','blocker'],'expect':expected(unknown,gaps=gap)})
        opaque=[create("a INT,b VARCHAR(64) AS (T07_UNKNOWN_FN('"+MARKER+"')) VIRTUAL")]
        boundary=[{'index':0,'feature':'create_table.column.expression_dependencies','reason':UNAUDITED_REASON,'metadata':{'aspect':'column','reason':'unknown_function'}}]
        rows.append({'id':f't07-{dialect}-unknown-expression-privacy','dialect':dialect,'sql':opaque[0],'policy':'all-rules-disabled','args':['--target-version',version,'--fail-on','none'],'expect':expected(opaque,unsupported_entries=boundary)})
        future=[{'index':0,'rule_id':COMPAT,'reason_code':'target_version_out_of_validated_range','required_facts':['target.version.validated_range']}]
        rows.append({'id':f't07-{dialect}-out-of-validated-series','dialect':dialect,'sql':seed[0],'policy':'t07-compat','args':['--target-version','9.0.0','--fail-on','blocker'],'expect':expected([seed[0]],gaps=future)})
        missing=[{'index':0,'rule_id':COMPAT,'reason_code':'missing_target_version','required_facts':['target.version']}]
        rows.append({'id':f't07-{dialect}-dependency-independent-of-version','dialect':dialect,'sql':'\n'.join(seed),'policy':'t07-all','args':['--fail-on','blocker'],'expect':expected(seed,[dependency(1)],missing)})
    return rows


def provider_case(row,anchor):
    variant=native_variant(row,anchor)
    assert not variant['failures'].get(0), 'provider fixture must have a legal CREATE'
    base=metadata_case(row,anchor)
    sqls=variant['sql'][1:]
    findings=[{**entry,'index':entry['index']-1} for entry in variant['expect']['t07_exact_findings']]
    gaps=[{**entry,'index':entry['index']-1} for entry in variant['expect']['evidence_gap_entries']]
    uns=[{**entry,'index':entry['index']-1} for entry in variant['expect']['unsupported_entries']]
    base['id']='t07-provider-'+anchor+'-'+row['role']
    base['sql']='\n'.join(sqls);base['expect']=expected(sqls,findings,gaps,uns)
    initial_indexes=variant['indexes'][0] if variant['indexes'] else ''
    base['setup'].append({'name':'create-provider-source','sql':variant['sql'][0],'expect_rc':0,'creates_fixture':True,'verify':[table_check(1),columns_check(variant['states'][0]),indexes_check(initial_indexes)]})
    base['post_verify']=[table_check(1),columns_check(variant['states'][0]),indexes_check(initial_indexes)]
    base['execute']=base['execute'][1:]
    return base


def provider_cases():
    roles={row['role']:row for row in native_roles()};rows=[]
    for anchor in VERSIONS: rows.append(provider_case(roles['g01-stored-dependent-drop'],anchor))
    for anchor in ('mysql80','mysql84'):
        rows.append(provider_case(roles['d04-column-expression'],anchor))
        rows.append(provider_case(roles['v08-hidden-index'],anchor))
    for anchor in ('mysql57','tidb85'): rows.append(provider_case(roles['v04-alter-visibility'],anchor))
    j=column('j','json',default='expr:json_object')
    independent=role('provider-json-independent',"x INT,j JSON DEFAULT (JSON_OBJECT('k',1))",['ALTER TABLE t DROP COLUMN x;','ALTER TABLE t ADD COLUMN z INT;'],[[column('x'),j],[j],[j,column('z')]])
    rows.append(provider_case(independent,'tidb85'))
    assert len(rows)==11
    return rows


def manifest():
    baseline=json.loads((Path(__file__).resolve().parents[1]/'testdata/ddl-golden/T06.json').read_text())
    anchors=copy.deepcopy(baseline['anchors'])
    for name,anchor in anchors.items():
        anchor['exec_client']=[arg for arg in anchor['exec_client'] if not arg.startswith('-p')]
        # The catalog records string literals inside generated/default
        # expressions with the DDL session's connection charset introducer;
        # the collected native evidence used utf8mb4 sessions. Pin the exec
        # client's charset so expression HEX whitelists are deterministic
        # instead of inheriting the image's latin1 client default.
        if '--default-character-set=utf8mb4' not in anchor['exec_client']:
            anchor['exec_client'].append('--default-character-set=utf8mb4')
        if name!='tidb85': anchor['exec_password_env']='T07_MYSQL_PASSWORD'
    native=[metadata_case(role,anchor) for role in native_roles() for anchor in VERSIONS]+provider_cases()
    cli=cli_cases()
    for case in native+cli:
        selected=case.get('policy','all-rules-disabled')
        case['expect']['statement_indices']=list(range(case['expect']['statements']))
        if selected!='all-rules-disabled':case['expect']['rule_summary_loaded']=len(profiles()[selected]['enable'])
        if selected=='t07-not-required':case['expect']['rule_summary_applicable']=0
        if selected=='t07-dependency':case['expect']['rule_summary_applicable']=1
    ids=[f'T07.db.{anchor}.{suffix}' for anchor in VERSIONS for suffix in ('ddl','syntax_negative')]+['T07.cli.mysql','T07.cli.tidb']
    ids += ['T07.cli.'+row['id'] for row in cli]+['T07.meta.'+row['id'] for row in native]
    assert len(ids)==len(set(ids))==FROZEN_REQUIRED_CASES
    assert hashlib.sha256('\n'.join(ids).encode()).hexdigest()==FROZEN_IDS_SHA256
    baseline_cli=[{'id':dialect,'dialect':dialect,'sql':'CREATE TABLE t (a INT);','expect':expected(['CREATE TABLE t (a INT);'])} for dialect in ('mysql','tidb')]
    return {'task_id':'T07','issue':'https://github.com/Fanduzi/DeltaScope/issues/86','policy_profile':'all-rules-disabled','description':'Frozen generated/default-expression/visibility semantics; independent T07 acceptance contract','anchors':anchors,'ddl_steps':copy.deepcopy(baseline['ddl_steps']),'syntax_negative':copy.deepcopy(baseline['syntax_negative']),'cli_audit':{'dialects':['mysql','tidb'],'sql':'CREATE TABLE t (a INT);','expect':{'exit':0,'verdict':'pass','statements':1,'findings':0,'diagnostics':0,'unsupported':0,'coverage':'complete'}},'policy':{'profiles':profiles()},'cli_cases':baseline_cli+cli,'metadata_cases':native,'required_case_ids':ids}


def strict_equal(left, right):
    if type(left) is not type(right): return False
    if isinstance(left,dict): return left.keys()==right.keys() and all(strict_equal(left[key],right[key]) for key in left)
    if isinstance(left,list): return len(left)==len(right) and all(strict_equal(a,b) for a,b in zip(left,right))
    return left==right


def t07_manifest_failures(data):
    if data.get('task_id')!='T07': return []
    want=manifest()
    return [] if strict_equal(data,want) else ['T07 manifest differs from independent frozen role/oracle constants']


def t07_artifact_failures(artifact,data):
    if artifact.get('task_id')!='T07' and data.get('task_id')!='T07': return []
    if data.get('task_id')!='T07': return ['T07 manifest task identity']
    errors=t07_manifest_failures(data)
    if errors: return errors
    def need(condition,message):
        if not condition: errors.append('T07 '+message)
    def integer(value): return type(value) is int
    need(artifact.get('task_id')=='T07','task identity')
    records=artifact.get('cases')
    if not isinstance(records,list) or not all(isinstance(row,dict) for row in records): return errors+['T07 malformed case collection']
    need(integer(artifact.get('executed_count')) and artifact['executed_count']==FROZEN_REQUIRED_CASES,'strict executed_count/denominator')
    ids=[row.get('case_id') for row in records]
    if not all(type(value) is str for value in ids):return errors+['T07 case IDs must be strings']
    need(len(ids)==FROZEN_REQUIRED_CASES and len(set(ids))==FROZEN_REQUIRED_CASES and set(ids)==set(data['required_case_ids']),'complete unique case identity set')
    if errors: return errors
    by_id={row['case_id']:row for row in records}
    profiles_by_name=profiles()
    def private_payload(value,path=()):
        if isinstance(value,str) and MARKER in value:
            allowed=len(path)==3 and path[0]=='statements' and type(path[1]) is int and path[2] in ('raw_sql','normalized_sql')
            allowed=allowed or len(path)==3 and path[0]=='unsupported' and type(path[1]) is int and path[2]=='sql'
            need(allowed,'private literal leaked outside existing input echo '+repr(path))
        elif isinstance(value,dict):
            for key,item in value.items():
                need(not isinstance(key,str) or MARKER not in key,'private literal leaked as a payload key')
                private_payload(item,path+(key,))
        elif isinstance(value,list):
            for idx,item in enumerate(value): private_payload(item,path+(idx,))
    def query(actual,want,label):
        if not isinstance(actual,dict): need(False,label+' query record is not an object');return
        need(actual.get('assert')==want['assert'] and actual.get('sql')==want['sql'],label+' query binding')
        need(integer(actual.get('rc')) and actual['rc']==0 and type(actual.get('output')) is str and actual['output']==want['expect'],label+' strict query result')
    def query_set(actual,want,label):
        if not isinstance(actual,list): need(False,label+' query collection');return
        need(len(actual)==len(want),label+' complete query set')
        for got,expected_query in zip(actual,want):query(got,expected_query,label)
    def steps(actual,want,label):
        if not isinstance(actual,list): need(False,label+' step collection');return
        need(len(actual)==len(want),label+' complete step set')
        for got,expected_step in zip(actual,want):
            if not isinstance(got,dict):need(False,label+' step object');continue
            need(got.get('name')==expected_step['name'] and got.get('sql')==expected_step['sql'],label+' step identity/SQL')
            need(integer(got.get('rc')) and got['rc']==expected_step.get('expect_rc',0),label+' strict driver return code')
            stderr=got.get('stderr')
            need(type(stderr) is str,label+' stderr type')
            for marker in expected_step.get('stderr_contains',[]):need(type(stderr) is str and marker in stderr,label+' native rejection reason')
            query_set(got.get('verify',[]),expected_step.get('verify',[]),label+' intermediate')
    specifications=[('T07.cli.'+row['id'],row,False) for row in data['cli_cases']]+[('T07.meta.'+row['id'],row,True) for row in data['metadata_cases']]
    for case_id,source,online in specifications:
        row=by_id[case_id];expect=source['expect'];actual=row.get('actual')
        need(row.get('input_sql')==source['sql'] and row.get('dialect')==source['dialect'],case_id+' input identity')
        need(row.get('policy_profile')==source.get('policy','all-rules-disabled'),case_id+' policy role binding')
        if not isinstance(actual,dict):need(False,case_id+' actual object');continue
        raw=actual.get('stdout');parsed=actual.get('parsed')
        if type(raw) is not str or not isinstance(parsed,dict):need(False,case_id+' raw/parsed payload types');continue
        try: decoded=json.loads(raw)
        except (ValueError,TypeError):need(False,case_id+' raw JSON');continue
        need(strict_equal(decoded,parsed),case_id+' raw/parsed mismatch')
        need(integer(actual.get('exit')) and actual['exit']==expect['exit'],case_id+' exact CLI exit')
        summary=parsed.get('summary');statements=parsed.get('statements')
        if not isinstance(summary,dict) or not isinstance(statements,list) or not all(isinstance(item,dict) for item in statements):need(False,case_id+' summary/statements shape');continue
        need(integer(summary.get('statements')) and summary['statements']==expect['statements'] and len(statements)==expect['statements'],case_id+' strict statement count')
        for key,want in [('blockers',expect['findings']),('warnings',0),('notices',0)]:need(integer(summary.get(key)) and summary[key]==want,case_id+' strict '+key+' count')
        coverage=parsed.get('coverage')
        need(isinstance(coverage,dict) and coverage.get('status')==expect['coverage'],case_id+' aggregate coverage')
        findings=[];gaps=[]
        for idx,(statement,sql) in enumerate(zip(statements,expect['statement_sql'])):
            need(integer(statement.get('index')) and statement['index']==idx and statement.get('raw_sql')==sql,case_id+' statement identity')
            cov=statement.get('coverage');need(isinstance(cov,dict) and cov.get('status')==expect['statement_coverage'][idx],case_id+' statement coverage')
            found=statement.get('findings',[]);missing=statement.get('evidence_gaps',[])
            if not isinstance(found,list) or not all(isinstance(value,dict) for value in found):need(False,case_id+' finding array');continue
            if not isinstance(missing,list) or not all(isinstance(value,dict) for value in missing):need(False,case_id+' gap array');continue
            for value in found:findings.append((idx,value))
            for value in missing:gaps.append({'index':idx,**value})
        need(parsed.get('global_findings',[])==[],case_id+' unexpected global finding')
        want_findings=expect['t07_exact_findings'];need(len(findings)==len(want_findings),case_id+' exact finding cardinality')
        consumed=set()
        for wanted in want_findings:
            matches=[]
            for position,(idx,found) in enumerate(findings):
                if position in consumed:continue
                if idx!=wanted['index'] or found.get('rule_id')!=wanted['rule_id']:continue
                if found.get('level')!=wanted['level'] or found.get('message')!=wanted['message'] or not strict_equal(found.get('metadata'),wanted['metadata']):continue
                statement_index=found.get('statement_index',0);location=found.get('location')
                expected_location=next(item for item in expect['finding_locations'] if item['index']==idx)
                if not integer(statement_index) or statement_index!=idx or not isinstance(location,dict):continue
                if not integer(location.get('line')) or not integer(location.get('column')) or location['line']!=expected_location['line'] or location['column']!=expected_location['column']:continue
                matches.append(position)
            need(len(matches)==1,case_id+' exact one-to-one finding/column/dependent/location')
            if matches:consumed.add(matches[0])
        need(strict_equal(gaps,expect['evidence_gap_entries']),case_id+' exact known/unknown dependency or version gap role')
        uns=parsed.get('unsupported',[]);diags=parsed.get('diagnostics',[])
        need(isinstance(uns,list) and len(uns)==expect['unsupported'] and isinstance(diags,list) and len(diags)==expect['diagnostics'],case_id+' unsupported/diagnostics count')
        if isinstance(uns,list):
            got=[]
            for item in uns:
                if not isinstance(item,dict):need(False,case_id+' unsupported object');continue
                got.append({'index':item.get('index',0),'feature':item.get('feature'),'reason':item.get('reason'),'metadata':item.get('metadata')})
            need(strict_equal(got,expect['unsupported_entries']),case_id+' vendor/tool unsupported role')
        rule_summary=parsed.get('rule_summary')
        selected=source.get('policy','all-rules-disabled')
        if selected=='all-rules-disabled':need('rule_summary' not in parsed,case_id+' disabled summary must be absent')
        else:
            enabled=profiles_by_name[selected]['enable']
            need(isinstance(rule_summary,dict),case_id+' loaded rule_summary object')
            if isinstance(rule_summary,dict):
                need(integer(rule_summary.get('loaded')) and rule_summary['loaded']==len(enabled) and integer(rule_summary.get('applicable')) and rule_summary['applicable']>=0,case_id+' strict aggregate rule_summary counters')
                if selected=='t07-not-required':need(type(rule_summary.get('applicable')) is int and rule_summary['applicable']==0,case_id+' required=false inapplicable')
        private_payload(parsed)
        if MARKER in source['sql']:need(any(MARKER in statement.get('raw_sql','') for statement in statements),case_id+' original input echo was removed')
        if online:
            need(row.get('anchor')==source['anchor'],case_id+' anchor swap')
            database=actual.get('database');anchor=data['anchors'][source['anchor']]
            need(isinstance(database,dict) and database.get('reachable') is True and database.get('product')==anchor['product'] and database.get('image')==anchor['image'] and type(database.get('version')) is str and anchor['version_contains'] in database['version'],case_id+' native anchor identity')
            steps(actual.get('setup'),source.get('setup',[]),case_id+' setup')
            steps(actual.get('execute'),source.get('execute',[]),case_id+' execute')
            steps(actual.get('teardown'),source.get('teardown',[]),case_id+' teardown')
            query_set(actual.get('post_verify'),source.get('post_verify',[]),case_id+' nonexecution')
            query_set(actual.get('structure'),source.get('structure',[]),case_id+' structure')
    return errors


def write_frozen_files():
    root=Path(__file__).resolve().parents[1];data=manifest()
    (root/'testdata/ddl-golden/T07.json').write_text(json.dumps(data,ensure_ascii=False,indent=2)+'\n')
    directory=root/'docs/work-plans/t07-advanced-columns'
    (directory/'roles.json').write_text(json.dumps({'role_count':38,'baseline_cli_cases':2,'offline_cli_cases':len(data['cli_cases'])-2,'cli_cases':len(data['cli_cases']),'metadata_cases':len(data['metadata_cases']),'required_cases':len(data['required_case_ids']),'required_case_ids':data['required_case_ids'],'native_roles':native_roles()},ensure_ascii=False,indent=2)+'\n')
    print(json.dumps({'required_cases':len(data['required_case_ids']),'cli_cases':len(data['cli_cases']),'metadata_cases':len(data['metadata_cases']),'id_sha256':hashlib.sha256('\n'.join(data['required_case_ids']).encode()).hexdigest()},indent=2))


if __name__=='__main__': write_frozen_files()
