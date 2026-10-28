#!/usr/bin/env python3
"""TST-META-01: проверка спецификаций тестов. exit 0 = всё в порядке.
  python3 docs/MVP1/tools/check-test-matrix.py"""
import json,os,re,glob,sys
H=os.path.dirname(os.path.abspath(__file__));M=os.path.dirname(H)
def rd(n): return open(os.path.join(M,n),encoding='utf-8').read()
S=json.load(open(os.path.join(M,'execution','status.json')));T=S['tasks'];T=T if isinstance(T,list) else list(T.values())
ids={t['id'] for t in T};problems=[]
spec=rd('ARCH_15_TEST_SPECS_WITH_LOGIC.md')+rd('ARCH_22_TEST_E2E_REVIEW_ADDENDUM.md')
# 1. каждый сценарий TST-E2E-NN в статусе описан в ARCH_15/22 и содержит Then
for i in sorted(x for x in ids if re.fullmatch(r'TST-E2E-\d\d',x)):
    m=re.search(r'^### '+i+r'\b(.*?)(?=^### |\Z)',spec,re.M|re.S)
    if not m: problems.append(i+': нет карточки'); continue
    if '**Then' not in m.group(1): problems.append(i+': нет блока Then')
# 2. у каждой TST-/SEQ-/PIPE- задачи есть карточка в каком-либо ARCH_*.md
cards=''.join(open(f,encoding='utf-8').read() for f in glob.glob(os.path.join(M,'ARCH_*.md')))
for i in sorted(ids):
    if re.match(r'(TST|SEQ|PIPE)-',i) and not re.search(r'^### .*\b'+re.escape(i)+r'\b',cards,re.M) and not re.search(r'^### '+re.escape(i.rsplit('-',1)[0])+r'-\d+\s*…',cards,re.M) and not re.search(r'^### (TST-E2E-21 … TST-E2E-26)',cards,re.M):
        problems.append(i+': нет карточки ###')
# 3. фикстуры
mins={'rounding_golden.json':34,'datecs_golden_frames.json':100,'daisy_golden_frames.json':15,'datecspay_golden_frames.json':50}
for f,n in mins.items():
    p=os.path.join(M,'fixtures',f)
    try:
        d=json.load(open(p,encoding='utf-8'));k=len(d) if isinstance(d,list) else len(d.get('vectors',d.get('cases',d)))
        if k<n: problems.append('%s: %d < %d'%(f,k,n))
    except Exception as e: problems.append(f+': '+str(e))
d=json.load(open(os.path.join(M,'fixtures','daisy_golden_frames.json')))
if sum(1 for x in d if x.get('negative'))!=1 or any(x.get('bcc_verified') is False and not x.get('negative') for x in d): problems.append('daisy: ложный негатив или нет «PDF typo»')
for p in problems: print('PROBLEM',p)
print('test-matrix problems=%d'%len(problems));sys.exit(1 if problems else 0)
