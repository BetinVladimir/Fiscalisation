#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""extract_vendor_frames.py — проверка golden-кадров (BCC, LEN) в fixtures/.
  python3 docs/MVP1/tools/extract_vendor_frames.py --check datecs|daisy
Datecs: BCC = 4 nibble(+0x30) суммы байт от LEN до 0x05; LEN запроса = nibbles(len(DATA)+0x2A).
Daisy:  LEN = len(DATA)+0x24 (один байт), BCC как у Datecs. Для генерации из PDF (pdftotext) см. ARCH_18 часть 3."""
import json, os, sys
MVP = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
def nib(v, n=4): return bytes(0x30 + ((v >> (4 * (n - 1 - i))) & 0xF) for i in range(n))
def check_pay():
    fr = json.load(open(os.path.join(MVP, 'fixtures', 'datecspay_golden_frames.json')))
    bad = 0
    for f in fr:
        b = bytes.fromhex(f['frame_hex']); x = 0
        for q in b[:-1]: x ^= q
        ln = (b[3] << 8) | b[4]
        if x != b[-1] or len(b) != 6 + ln: bad += 1
    print(f'datecspay: frames={len(fr)} bad={bad}')
    return bad == 0 and len(fr) >= 70

def check(kind):
    if kind == 'datecspay': return check_pay()
    fr = json.load(open(os.path.join(MVP, 'fixtures', f'{kind}_golden_frames.json')))
    ok = bad = 0
    for f in fr:
        b = bytes.fromhex(f['frame_hex']); core = b[1:-5]
        good = b[0] == 1 and b[-1] == 3 and b[-5:-1] == nib(sum(core) & 0xFFFF)
        if kind == 'datecs' and f['direction'] == 'request':
            good = good and b[1:5] == nib(len(bytes.fromhex(f['data_hex'])) + 0x2A)
        if f.get('bcc_verified') is False: good = not good  # негативный вектор должен не проходить
        ok += good; bad += (not good)
    print(f'{kind}: frames={len(fr)} ok={ok} bad={bad}')
    return bad == 0 and (len(fr) >= (181 if kind == 'datecs' else 15))
if __name__ == '__main__':
    k = sys.argv[sys.argv.index('--check') + 1] if '--check' in sys.argv else 'datecs'
    sys.exit(0 if check(k) else 1)
