#!/usr/bin/env python3
"""Read-only audit of current MVP1 sources, task coverage and generated packets."""
import argparse
import contextlib
import io
import json
from pathlib import Path
import re
import runpy
import tempfile

HERE=Path(__file__).resolve().parent

def compare_packets(generated,plan_path):
    errors=[];plan_path=Path(plan_path)
    for file in Path(generated).iterdir():
        expected=plan_path if file.name=='plan.json' else plan_path.parent/file.name
        if not expected.is_file() or file.read_bytes()!=expected.read_bytes():errors.append('stale generated packet '+file.name)
    return errors

def audit(root,plan_path,rebuild=True):
    root=Path(root);plan_path=Path(plan_path);docs=root/'docs/MVP1'
    plan=json.loads(plan_path.read_text());ids={t['id'] for t in plan['tasks']};errors=[]
    runner=runpy.run_path(str(plan_path.parent/'run_fixes.py'))
    errors.extend(runner['validate'](plan,root))
    cards=set();test_ids=set();declared=set()
    for file in docs.glob('*.md'):
        for value in re.findall(r'^#{2,4} ([A-Z][A-Z0-9]*(?:-[A-Z0-9]+)+)\b',file.read_text(),re.M):
            if re.fullmatch(r'E-[1-6]',value):value='EMU-0'+value[2:]
            if value.startswith('SCR-'):value='SCRIPT-'+value[4:]
            declared.add(value)
    for value in sorted(declared-ids):errors.append('uncovered current document requirement '+value)
    for file in docs.glob('ARCH_*.md'):
        text=file.read_text()
        for value in re.findall(r'^### ([A-Z][A-Z0-9]*(?:-[A-Z0-9]+)+)\b',text,re.M):cards.add('SCRIPT-'+value[4:] if value.startswith('SCR-') else value)
        if file.name in ('ARCH_03_TESTS_E2E.md','ARCH_18_TEST_LOGIC_SPECS.md'):
            test_ids.update(re.findall(r'^\| (TST-[A-Z0-9]+-\d+) \|',text,re.M))
    for value in sorted(cards-ids):errors.append('uncovered current ARCH card '+value)
    for value in sorted(cards-set(plan.get('source_cards',{}))):errors.append('unmapped current ARCH card '+value)
    for value in sorted(test_ids-ids):errors.append('uncovered current test row '+value)
    legacy={t['id'] for t in json.loads((docs/'execution/status.json').read_text())['tasks']}
    for value in sorted(legacy-ids):errors.append('uncovered current legacy task '+value)
    if set(plan['coverage']['legacy_tasks'])!=legacy:errors.append('legacy coverage inventory differs from current tracker')
    gaps={'E2E-'+v for v in re.findall(r'^\| (GAP-\d+) \|',(docs/'TRACE_EXISTING_REGISTERS.md').read_text(),re.M)}
    for value in sorted(gaps-ids):errors.append('uncovered current gap '+value)
    current_docs={f.name:f.read_text() for f in docs.glob('*.md')}
    if current_docs!=plan.get('source_documents'):errors.append('source snapshots differ from current MVP1 documents; rebuild plan')
    if rebuild:
        builder=runpy.run_path(str(plan_path.parent/'build_plan.py'))
        with tempfile.TemporaryDirectory(prefix='mvp1-audit-') as directory:
            out=Path(directory)
            with contextlib.redirect_stdout(io.StringIO()):builder['build'](root,out)
            errors.extend(compare_packets(out,plan_path))
    estimates=[(len(runner['prompt'](plan,t,u,root).encode())//2+1,u['id']) for t in plan['tasks'] for u in t['units']]
    return {'tasks':len(ids),'units':sum(len(t['units']) for t in plan['tasks']),'arch_cards':len(cards),'declared_document_ids':len(declared),'legacy_tasks':len(legacy),'test_rows':len(test_ids),'scripts':len(plan['coverage']['script_tasks']),'gap_scenarios':len(gaps),'routes':len(plan['coverage']['required_routes']),'max_packet_estimate':max(estimates,default=(0,'')),'errors':errors}

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root',type=Path,default=HERE.parents[2]);parser.add_argument('--plan',type=Path,default=HERE/'plan.json')
    args=parser.parse_args();result=audit(args.root,args.plan);print(json.dumps(result,ensure_ascii=False,indent=2));return int(bool(result['errors']))

if __name__=='__main__':raise SystemExit(main())
