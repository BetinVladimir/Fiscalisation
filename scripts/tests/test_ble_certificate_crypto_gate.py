#!/usr/bin/env python3
"""Fail-closed evidence and native no-tests/skip counterexamples; no network needed."""
from pathlib import Path
import importlib.util
import json
import os
import subprocess
import sys
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / "run-ble-certificate-crypto.py"
spec = importlib.util.spec_from_file_location("ble_crypto_gate", SCRIPT)
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)

class CryptoGateTests(unittest.TestCase):
    def test_missing_tool_is_failure_and_all_phases_have_evidence(self):
        with tempfile.TemporaryDirectory() as tmp:
            env = {"PATH": "/definitely/no/tools", "HOME": tmp}
            result = subprocess.run([sys.executable, str(SCRIPT), "--evidence", tmp], env=env,
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, timeout=10)
            self.assertNotEqual(result.returncode, 0)
            manifest = json.loads((Path(tmp) / "manifest.json").read_text())
            self.assertEqual(manifest["status"], "FAILED")
            self.assertEqual(set(manifest["phases"]), set(gate.PHASES))
            self.assertEqual(manifest["phases"]["preflight"]["status"], "FAILED")
            self.assertNotEqual(manifest["phases"]["preflight"]["exit_code"], 0)
            for name in gate.PHASES[1:]:
                self.assertEqual(manifest["phases"][name]["status"], "NOT_RUN")
                self.assertIsNone(manifest["phases"][name]["exit_code"])
            self.assertEqual(sorted(p.suffix for p in Path(tmp).rglob('*') if p.is_file()), ['.json', '.json', '.log'])
            self.assertNotIn("PRIVATE KEY", json.dumps(manifest))

    def test_successful_process_with_zero_or_skipped_cases_is_rejected(self):
        for text in ("ok domain [no tests to run]", "--- SKIP: TestBLECertificateActualFirmwareProofInterop", "PASS"):
            with self.subTest(text=text), self.assertRaises(gate.GateError):
                gate.require_native_cases(text, gate.PROOF_MARKERS)
        gate.require_native_cases("\n".join(gate.PROOF_MARKERS), gate.PROOF_MARKERS)

    def test_profile_requires_each_adversarial_case(self):
        for omitted in gate.PROFILE_MARKERS:
            with self.subTest(omitted=omitted), self.assertRaises(gate.GateError):
                gate.require_native_cases("\n".join(x for x in gate.PROFILE_MARKERS if x != omitted), gate.PROFILE_MARKERS)

    def test_dependency_hash_mismatch_is_not_accepted(self):
        with tempfile.TemporaryDirectory() as tmp:
            source = Path(tmp) / "cJSON.c"
            source.write_bytes(b"different upstream source")
            with self.assertRaises(gate.GateError):
                gate.require_hash(source, gate.CJSON["cJSON.c"])

if __name__ == "__main__":
    unittest.main()
