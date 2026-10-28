#!/usr/bin/env bash
# mutate-vector.sh — mutation test for golden vectors (MUT-001)
# For each vector: mutate 1 byte at LEN, DATA, BCC positions; decoder must reject.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VECTORS_DIR="$ROOT/docs/MVP1/fixtures"
IOT_TESTS="$ROOT/IoT"

errors=0
total=0
killed=0

run_decoder_check() {
  local hex="$1"
  local profile="$2"
  # Use the native test suite decoder via a small Python wrapper
  python3 -c "
import sys, subprocess, json, os
hex_bytes = bytes.fromhex('$hex')
# Write to temp file
import tempfile
with tempfile.NamedTemporaryFile(delete=False, suffix='.bin') as tf:
    tf.write(hex_bytes)
    tmp = tf.name
# Accept means test failed (mutant was not rejected)
result = subprocess.run(
    ['python3', '$ROOT/scripts/tests/decode_frame_check.py', tmp, '$profile'],
    capture_output=True, timeout=5
)
os.unlink(tmp)
sys.exit(0 if result.returncode != 0 else 1)
" 2>/dev/null
  return $?
}

for vec_file in "$VECTORS_DIR/datecs_golden_vectors.json" "$VECTORS_DIR/daisy_golden_vectors.json"; do
  [ -f "$vec_file" ] || continue
  profile=$(basename "$vec_file" _golden_vectors.json)

  while IFS= read -r hex; do
    [ -z "$hex" ] && continue
    len=${#hex}
    # Mutate byte at position 0 (LEN), middle (DATA), last-6 (BCC)
    for pos in 0 $((len/2)) $((len-6)); do
      [ $pos -lt 0 ] && continue
      [ $pos -ge $len ] && continue
      total=$((total+1))
      # flip one nibble
      orig="${hex:$pos:2}"
      mutated=$(printf '%02x' $(( 0x$orig ^ 0x01 )))
      mutant="${hex:0:$pos}${mutated}${hex:$((pos+2))}"
      if ! run_decoder_check "$mutant" "$profile" 2>/dev/null; then
        killed=$((killed+1))
      else
        echo "MUT-001: mutant not killed at pos $pos in $vec_file: $mutant" >&2
        errors=$((errors+1))
      fi
    done
  done < <(python3 -c "
import json
d = json.load(open('$vec_file'))
items = d.get('vectors', d.get('cases', d.get('tests', [])))
for item in items:
    h = item.get('hex') or item.get('frame') or item.get('encoded')
    if h:
        print(h.replace(' ','').lower())
" 2>/dev/null || true)
done

if [ $total -eq 0 ]; then
  echo "MUT-ENV: no golden vectors found" >&2
  exit 3
fi

echo "mutants killed $killed/$total"
if [ $errors -eq 0 ]; then
  echo "MUTATE-VECTOR OK"
  exit 0
else
  echo "MUTATE-VECTOR FAIL: $errors mutants survived" >&2
  exit 1
fi
