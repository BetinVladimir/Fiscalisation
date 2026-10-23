#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""check-doc-refs.py — проверка ссылок документации на код (только чтение).
  python3 docs/MVP1/tools/check-doc-refs.py docs/MVP1      # ссылки path:line
  python3 docs/MVP1/tools/check-doc-refs.py --status       # целостность status.json
  python3 docs/MVP1/tools/check-doc-refs.py --scenarios    # 48 ключей сценариев ARCH_19
Правила ссылки `path:line` (path с каталогом): файл существует и единственный; строка в пределах файла;
если в той же строке документа есть `символ`, он встречается в окне ±30 строк. Строка документа с «(создать)»/«создать»/«новый» —
ссылка на будущий файл, не ошибка. Исключаются .store-build, node_modules, build, .git."""
import os, re, sys, json, glob, subprocess
HERE = os.path.dirname(os.path.abspath(__file__)); MVP = os.path.dirname(HERE)
ROOT = os.path.dirname(os.path.dirname(MVP)); BASES = [ROOT, os.path.dirname(ROOT)]
SKIP = ('.store-build', 'node_modules', '/build/', '/.git/')
REF = re.compile(r'`?((?:[A-Za-z0-9_.\-]+/)+[A-Za-z0-9_.\-]+\.[A-Za-z0-9]{1,5}):(\d+)(?:[-–](\d+))?`?')
SYM = re.compile(r'`([A-Za-z_][A-Za-z0-9_:.]{2,60})`')
_idx = {}
def find(path):
    for b in BASES:
        p = os.path.join(b, path)
        if os.path.isfile(p): return [p]
    base = os.path.basename(path)
    if base not in _idx:
        out = []
        for b in BASES[:1]:
            for dp, dn, fn in os.walk(b):
                dn[:] = [d for d in dn if d not in ('node_modules', '.store-build', 'build', '.git')]
                if base in fn: out.append(os.path.join(dp, base))
        _idx[base] = out
    return [p for p in _idx[base] if p.replace('\\', '/').endswith(path)]

def refs(dirpath):
    bad = []; n = 0
    for f in sorted(glob.glob(os.path.join(dirpath, '*.md'))):
        for ln, line in enumerate(open(f, encoding='utf-8'), 1):
            future = bool(re.search(r'созда(ть|ё|ется)|новый файл|\(new\)', line, re.I))
            for m in REF.finditer(line):
                path, l1 = m.group(1), int(m.group(2)); n += 1
                tail = line[m.end():m.end() + 70]
                nxt = REF.search(tail)
                syms = SYM.findall(tail[:nxt.start()] if nxt else tail)[:1]
                if '...' in path or any(s in path for s in SKIP) or path.startswith(('http', 'docs/MVP1/fixtures')): continue
                hits = find(path)
                where = f'{os.path.basename(f)}:{ln}'
                if not hits:
                    if not future: bad.append(f'{where}: нет файла {path}')
                    continue
                if len(hits) > 1 and '/' not in path.strip('/').rsplit('/', 1)[0] + '/':
                    bad.append(f'{where}: неоднозначно {path} ({len(hits)})'); continue
                if len(hits) > 1 and len({os.path.relpath(h, ROOT) for h in hits}) > 1 and not any(h.endswith(path) for h in hits):
                    bad.append(f'{where}: неоднозначно {path}'); continue
                lines = open(hits[0], errors='ignore').read().split('\n')
                if l1 > len(lines): bad.append(f'{where}: {path}:{l1} за пределами файла ({len(lines)})'); continue
                if syms:
                    win = '\n'.join(lines[max(0, l1 - 31): l1 + 30])
                    ok = any(s.split('::')[-1].split('.')[-1].rstrip('()') in win for s in syms)
                    if not ok: bad.append(f'{where}: символ {syms[0]} не найден рядом с {path}:{l1}')
    return n, bad

def status():
    d = json.load(open(os.path.join(MVP, 'execution', 'status.json'), encoding='utf-8'))
    ids = [t['id'] for t in d['tasks']]; bad = []
    if len(ids) != len(set(ids)): bad.append('дубликаты task id')
    S = set(ids); allowed = set(d.get('statuses', []))
    for t in d['tasks']:
        for x in t.get('depends', []):
            if x not in S: bad.append(f"{t['id']}: depends {x} отсутствует")
        if allowed and t['status'] not in allowed: bad.append(f"{t['id']}: статус {t['status']}")
        for s in t.get('subtasks', []):
            for k in ('id', 'title', 'files', 'test', 'accept'):
                if k not in s: bad.append(f"{s.get('id')}: нет поля {k}")
            if any(g in (s.get('test', '') + s.get('accept', '')) for g in ('по подзадаче', 'по карточке', 'build+тест')):
                bad.append(f"{s['id']}: общая формулировка")
    for t in d['tasks']:
        if t['id'].startswith('VG-') and not t.get('doc'): bad.append(t['id'] + ': нет doc')
    return len(ids), bad

def scenarios():
    txt = open(os.path.join(MVP, 'ARCH_19_HIL_SCENARIOS.md'), encoding='utf-8').read()
    keys = re.findall(r'ключ `([^`]+)`', txt)
    return keys

if __name__ == '__main__':
    a = sys.argv[1:]
    if '--status' in a:
        n, bad = status(); print(f'tasks={n} problems={len(bad)}'); [print(' ', b) for b in bad[:50]]; sys.exit(1 if bad else 0)
    if '--scenarios' in a:
        k = scenarios(); print(f'paths={len(k)} scenarios={len(k)*8}'); sys.exit(0 if len(k) == 6 else 1)
    d = a[0] if a else MVP
    if not os.path.isabs(d): d = os.path.join(ROOT, os.path.relpath(d, 'docs')) if d.startswith('docs/') and not os.path.isdir(d) else d
    n, bad = refs(d); print(f'refs={n} problems={len(bad)}'); [print(' ', b) for b in bad[:80]]; sys.exit(1 if bad else 0)
