#!/usr/bin/env python3
"""Opt-in: requires an EMPTY disposable PostgreSQL DB with canonical test tables.
Never points this at an application DB. Table setup is documented in remediation 25.
"""
import os,pathlib,subprocess,tempfile,time,json,sys
root=pathlib.Path(__file__).resolve().parents[4]
dsn=os.environ.get('EXECUTION_PIPELINE_POSTGRES_URL','')
if os.environ.get('EXECUTION_PIPELINE_DISPOSABLE_DB')!='YES' or not dsn:
    sys.exit('Set EXECUTION_PIPELINE_DISPOSABLE_DB=YES and EXECUTION_PIPELINE_POSTGRES_URL for an EMPTY disposable canonical-schema DB')
run=pathlib.Path(tempfile.mkdtemp(prefix='fiscal-http-pipeline-'));run.chmod(0o700)
env={**os.environ,'EXECUTION_PIPELINE_DIR':str(run)}; children=[];logs=[]
def wait(name):
    for _ in range(600):
        if (run/name).exists():return
        if any(p.poll() is not None for p in children):raise RuntimeError('test host exited; inspect '+str(run))
        time.sleep(.1)
    raise RuntimeError('test host timeout '+name)
try:
    if os.environ.get('EXECUTION_PIPELINE_INIT_SCHEMA')=='YES':
        model=pathlib.Path(os.environ.get('BEELOY_DB_MODEL_ROOT',root.parent/'BeeloyDB/model'))
        sql="create extension if not exists pgcrypto;create schema fiscal;create schema minipos;create function minipos.app_organization_id() returns uuid language sql stable as $$select nullif(current_setting('minipos.organization_id',true),'')::uuid$$;\n"
        for file in ['fiscal/tables/external_systems.sql','fiscal/tables/tenant_source_bindings.sql','fiscal/tables/tenant_integration_credentials.sql','fiscal/tables/integration_security_events.sql','minipos/tables/organizations.sql','minipos/tables/company_fiscal_bindings.sql','minipos/tables/company_fiscal_credentials.sql']:
            sql+=(model/file).read_text()+"\n"
        subprocess.run(['psql',dsn,'-v','ON_ERROR_STOP=1'],input=sql,text=True,check=True)
    for label,project in [('source',root/'minipos/beeminipos-backend'),('root',root/'fiscal-backend')]:
        subprocess.run(['go','build','-o',str(run/label),'./cmd/execution-pipeline-oracle'],cwd=project,env=env,check=True)
    log=open(run/'source.log','w');logs.append(log);children.append(subprocess.Popen([str(run/'source'),str(run),dsn],stdout=log,stderr=subprocess.STDOUT,env=env));wait('source.json')
    subprocess.run(['go','test','./internal/domain','-run','^TestExecutionHTTPPipelineFixture$','-count=1'],cwd=root/'fiscal-backend',env=env,check=True)
    log=open(run/'root.log','w');logs.append(log);children.append(subprocess.Popen([str(run/'root'),str(run),dsn],stdout=log,stderr=subprocess.STDOUT,env=env));wait('source-server.json')
    subprocess.run(['./node_modules/.bin/playwright','test','-c',('playwright.reconciliation-pipeline.config.mjs' if os.environ.get('RECONCILIATION_HTTP_PIPELINE')=='YES' else 'playwright.pipeline.config.mjs')],cwd=root/'minipos/miniposweb',env=env,check=True)
finally:
    for p in children:p.terminate()
    for p in children:
        try:p.wait(timeout=5)
        except subprocess.TimeoutExpired:p.kill();p.wait()
    for f in logs:f.close()
    print('Private test evidence:',run)
