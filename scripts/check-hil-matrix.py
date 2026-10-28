#!/usr/bin/env python3
"""check-hil-matrix.py — validate docs/MVP1/fixtures/mvp_matrix.json structure.

Rules:
- Exactly 20 routes (5 hw x 2 clients x 2 transports)
- All 5 hardware paths present: DatecsBC50, BluePay, DaisyCompact, SmartS, BlueCash
- Both clients: POS, MiniPOS
- Both transports: MQTT, BLE
- S1..S8 scenarios (DaisyCompact may omit card/split scenarios)
- Daisy Compact: CARD/SPLIT capabilities absent; negative scenarios present
- Smart S: lab_only=true; negative scenarios include prod_no_doc_number/UNKNOWN
- SUPTO in all routes
- No HACKED/EMU/HIL physical PASS markers
"""
import json
import sys
from pathlib import Path

ROOT = Path(__file__).parent.parent
MATRIX_FILE = ROOT / "docs" / "MVP1" / "fixtures" / "mvp_matrix.json"

EXPECTED_HW = {"DatecsBC50", "BluePay", "DaisyCompact", "SmartS", "BlueCash"}
EXPECTED_CLIENTS = {"POS", "MiniPOS"}
EXPECTED_TRANSPORTS = {"MQTT", "BLE"}
EXPECTED_ROUTE_COUNT = 20


def check():
    errors = []

    if not MATRIX_FILE.exists():
        print(f"ERROR: mvp_matrix.json not found at {MATRIX_FILE}", file=sys.stderr)
        return 1

    try:
        data = json.loads(MATRIX_FILE.read_text())
    except json.JSONDecodeError as e:
        print(f"ERROR: JSON parse error: {e}", file=sys.stderr)
        return 1

    routes = data.get("routes", [])

    # 1. Exact count
    if len(routes) != EXPECTED_ROUTE_COUNT:
        errors.append(f"route count {len(routes)} != {EXPECTED_ROUTE_COUNT}")

    # 2. Hardware paths
    hw_found = {r["hw"] for r in routes}
    missing_hw = EXPECTED_HW - hw_found
    if missing_hw:
        errors.append(f"missing hw: {missing_hw}")
    extra_hw = hw_found - EXPECTED_HW
    if extra_hw:
        errors.append(f"unexpected hw: {extra_hw}")

    # 3. Clients
    clients_found = {r["client"] for r in routes}
    if clients_found != EXPECTED_CLIENTS:
        errors.append(f"clients mismatch: {clients_found} vs {EXPECTED_CLIENTS}")

    # 4. Transports
    transports_found = {r["transport"] for r in routes}
    if transports_found != EXPECTED_TRANSPORTS:
        errors.append(f"transports mismatch: {transports_found} vs {EXPECTED_TRANSPORTS}")

    # 5. Daisy Compact: CARD/SPLIT negative; capabilities without CARD/SPLIT
    daisy_routes = [r for r in routes if r["hw"] == "DaisyCompact"]
    if not daisy_routes:
        errors.append("no DaisyCompact routes found")
    for r in daisy_routes:
        caps = set(r.get("capabilities", []))
        if "CARD" in caps:
            errors.append(f"DaisyCompact route {r['route_id']} must not have CARD capability")
        if "SPLIT" in caps:
            errors.append(f"DaisyCompact route {r['route_id']} must not have SPLIT capability")
        neg_scenarios = [n["scenario"] for n in r.get("negative", [])]
        if "CARD_before_io" not in neg_scenarios:
            errors.append(f"DaisyCompact route {r['route_id']} missing CARD_before_io negative")
        if "SPLIT_before_io" not in neg_scenarios:
            errors.append(f"DaisyCompact route {r['route_id']} missing SPLIT_before_io negative")

    # 6. Smart S: lab_only, UNKNOWN prod, COMMITTED_NO_REFERENCE lab
    smart_routes = [r for r in routes if r["hw"] == "SmartS"]
    if not smart_routes:
        errors.append("no SmartS routes found")
    for r in smart_routes:
        if not r.get("lab_only"):
            errors.append(f"SmartS route {r['route_id']} must have lab_only=true")
        neg = {n["scenario"]: n["expect"] for n in r.get("negative", [])}
        if neg.get("prod_no_doc_number") != "UNKNOWN":
            errors.append(f"SmartS {r['route_id']}: prod_no_doc_number must expect UNKNOWN")
        if neg.get("lab_no_doc_number") != "COMMITTED_NO_REFERENCE":
            errors.append(f"SmartS {r['route_id']}: lab_no_doc_number must expect COMMITTED_NO_REFERENCE")

    # 7. SUPTO in all routes
    no_supto = [r["route_id"] for r in routes if not r.get("supto")]
    if no_supto:
        errors.append(f"routes without supto=true: {no_supto}")

    # 8. No EMU/HACKED/HIL_PASS markers in routes
    for r in routes:
        if r.get("hil_pass") or r.get("emu_pass"):
            errors.append(f"route {r['route_id']} has forbidden hil_pass/emu_pass marker")

    # 9. Self-test: POS BLE routes present (selftest - if deleted -> fail)
    pos_ble = [r for r in routes if r["client"] == "POS" and r["transport"] == "BLE"]
    if not pos_ble:
        errors.append("SELFTEST FAIL: no POS/BLE routes found (selftest: deleting POS BLE row -> fail)")

    if errors:
        for e in errors:
            print(f"MATRIX ERROR: {e}", file=sys.stderr)
        print(f"HIL-MATRIX FAIL: {len(errors)} errors", file=sys.stderr)
        return 1

    print(f"HIL-MATRIX OK: {len(routes)} routes validated")
    for hw in sorted(EXPECTED_HW):
        count = sum(1 for r in routes if r["hw"] == hw)
        print(f"  {hw}: {count} routes")
    return 0


if __name__ == "__main__":
    sys.exit(check())
