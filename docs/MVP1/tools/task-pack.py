#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""task-pack.py — собирает контекст-пакет задачи (<= 6000 токенов) для слабой модели.
Использование:
  python3 docs/MVP1/tools/task-pack.py BUG-001          # печатает пакет
  python3 docs/MVP1/tools/task-pack.py --all --check    # проверка бюджета для всех задач
Состав пакета: метаданные, карточка (### <ID> из ARCH_*.md), подзадачи, решения D-n, статусы зависимостей,
выдержки кода (±15 строк вокруг символов из подзадач). При превышении бюджета режутся выдержки, затем карточка; подзадачи не режутся.
Только чтение. Код продукта не изменяется."""
import json, re, sys, glob, os, argparse
HERE = os.path.dirname(os.path.abspath(__file__))
MVP = os.path.dirname(HERE)
ROOT = os.path.dirname(os.path.dirname(MVP))      # .../Fiscalisation
BASES = [ROOT, os.path.dirname(ROOT), os.path.join(ROOT, 'BeeloyBackend')]
BUDGET = 6000
CTX = 15

def tok(s):
    cy = sum(1 for ch in s if 'Ѐ' <= ch <= 'ӿ')
    return int((len(s) - cy) / 3.6 + cy / 1.9) + 1

def load():
    st = json.load(open(os.path.join(MVP, 'execution', 'status.json'), encoding='utf-8'))
    cards = {}
    for f in sorted(glob.glob(os.path.join(MVP, 'ARCH_*.md'))):
        txt = open(f, encoding='utf-8').read()
        for part in re.split(r'^### ', txt, flags=re.M)[1:]:
            m = re.match(r'([A-Z][A-Za-z0-9]*-[0-9A-Za-z]+(?:-[0-9A-Za-z]+)*)\b', part)
            if m and m.group(1) not in cards:
                cards[m.group(1)] = '### ' + part.strip()
    dec = {}
    p = os.path.join(MVP, 'DECISIONS.md')
    if os.path.isfile(p):
        for part in re.split(r'^#+ ', open(p, encoding='utf-8').read(), flags=re.M)[1:]:
            m = re.match(r'(D-\d+)\b', part)
            if m: dec[m.group(1)] = part.strip()[:700]
    return st, cards, dec

def resolve(path):
    path = path.strip().replace('...', '')
    if not path or '*' in path: return None
    for b in BASES:
        p = os.path.join(b, path)
        if os.path.isfile(p): return p
    return None

def excerpts(files, titles):
    syms = []
    for t in titles:
        syms += re.findall(r'`([A-Za-z_][A-Za-z0-9_:.]{2,60})`', t)
        syms += re.findall(r'\b([A-Za-z_][A-Za-z0-9_]*(?:::[A-Za-z_][A-Za-z0-9_]*)+)', t)
    seen, out = set(), []
    for f in files:
        p = resolve(f)
        if not p: continue
        lines = open(p, errors='ignore').read().split('\n')
        for s in syms:
            key = s.split('::')[-1].split('.')[-1]
            if (f, key) in seen: continue
            for i, ln in enumerate(lines):
                if key in ln and len(key) > 3:
                    a, b = max(0, i - CTX), min(len(lines), i + CTX)
                    out.append((f, i + 1, '\n'.join(f'{n+1}: {lines[n][:200]}' for n in range(a, b))))
                    seen.add((f, key)); break
    return out

def build(t, st, cards, dec, budget=BUDGET):
    byid = {x['id']: x for x in st['tasks']}
    head = f"# ЗАДАЧА {t['id']}  [{t.get('severity','')}] wave={t.get('wave')} status={t['status']}\nДокумент: {t['doc']}\nЗаметки: {t.get('notes','')}\n"
    deps = ''.join(f"- {d}: {byid[d]['status'] if d in byid else 'НЕТ В status.json'}\n" for d in t.get('depends', []))
    subs = ''
    for s in t.get('subtasks', []):
        subs += f"\n## {s['id']}\n{s['title']}\nФайлы: {', '.join(s.get('files', [])) or '—'}\nОжидание: {s.get('test','')}\nКоманда приёмки: {s.get('accept','')}\n"
    card = cards.get(t['id'], '')
    ids = set(re.findall(r'\bD-\d+\b', card + subs + t.get('notes', '')))
    decs = '\n'.join(dec[i] for i in sorted(ids) if i in dec)
    files = []
    for s in t.get('subtasks', []): files += s.get('files', [])
    ex = excerpts(list(dict.fromkeys(files)), [s['title'] for s in t.get('subtasks', [])])
    def assemble(card_s, ex_l):
        body = head + '\n## Зависимости\n' + (deps or '—\n') + '\n## Подзадачи' + subs
        body += '\n## Карточка\n' + (card_s or '(нет отдельной карточки — см. подзадачи)') + '\n'
        if decs: body += '\n## Решения\n' + decs + '\n'
        for f, ln, txt in ex_l:
            body += f'\n## Код {f} (строка {ln})\n```\n{txt}\n```\n'
        return body
    ex_l = list(ex); body = assemble(card, ex_l)
    while tok(body) > budget and ex_l: ex_l.pop(); body = assemble(card, ex_l)
    cs = card
    while tok(body) > budget and len(cs) > 400:
        cs = cs[:int(len(cs) * 0.8)]; body = assemble(cs + '\n…(карточка сокращена)', ex_l)
    return body

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('id', nargs='?'); ap.add_argument('--all', action='store_true')
    ap.add_argument('--check', action='store_true'); ap.add_argument('--budget', type=int, default=BUDGET)
    a = ap.parse_args()
    st, cards, dec = load()
    if a.all:
        over = []; rows = []
        for t in st['tasks']:
            n = tok(build(t, st, cards, dec, a.budget)); rows.append((t['id'], n))
            if n > a.budget: over.append((t['id'], n))
        if a.check:
            for i, n in rows: print(f'{i}\t{n}')
        print(f'tasks={len(rows)} max={max(n for _, n in rows)} over_budget={len(over)}')
        for i, n in over: print('OVER', i, n)
        sys.exit(1 if over else 0)
    if not a.id: ap.error('нужен ID или --all')
    t = next((x for x in st['tasks'] if x['id'] == a.id), None)
    if not t: print('нет задачи', a.id, file=sys.stderr); sys.exit(2)
    print(build(t, st, cards, dec, a.budget))

if __name__ == '__main__': main()
