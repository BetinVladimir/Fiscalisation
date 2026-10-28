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

    def time_log(self):
        return "\n".join(gate.TIME_ANCHOR_MARKERS).replace(
            "--- PASS: " + gate.TIME_ANCHOR_TEST,
            "--- PASS: " + gate.TIME_ANCHOR_TEST + " (0.25s)")

    def test_time_anchor_requires_actual_case_and_each_native_marker(self):
        valid = self.time_log()
        gate.require_time_anchor_cases(valid)
        for marker in gate.TIME_ANCHOR_MARKERS:
            with self.subTest(marker=marker), self.assertRaises(gate.GateError):
                gate.require_time_anchor_cases(valid.replace(marker, "missing"))
        for bad in ("ok domain [no tests to run]", "--- SKIP: " + gate.TIME_ANCHOR_TEST,
                    valid + "\n--- SKIP: child", "PASS", valid + "\n[no tests to run]",
                    valid + "\n=== RUN   " + gate.TIME_ANCHOR_TEST,
                    valid.replace("=== RUN   ", "quoted === RUN   "),
                    valid.replace("--- PASS: ", "quoted --- PASS: ")):
            with self.subTest(log=bad), self.assertRaises(gate.GateError):
                gate.require_time_anchor_cases(bad)

    def test_time_anchor_requires_same_fresh_libraries_and_real_crypto_profile(self):
        libraries = {"libmbedcrypto.a": "fresh-build-sha"}
        good = {"libraries_sha256": libraries, "binary_sha256": "a" * 64,
                "synthetic_core_permit_factory": False, "host_test_root_injection": True,
                "compile_flags": ["-DBEEFISCAL_TIME_ANCHOR_CRYPTO_HOST_TEST"],
                "cjson_sources_sha256": gate.CJSON}
        gate.require_native_manifest(good, libraries, time_anchor=True)
        for field, value in (("libraries_sha256", {"libmbedcrypto.a": "stale"}),
                             ("binary_sha256", ""), ("binary_sha256", "not-a-digest"),
                             ("synthetic_core_permit_factory", True),
                             ("synthetic_core_permit_factory", None),
                             ("host_test_root_injection", False),
                             ("compile_flags", []),
                             ("compile_flags", good["compile_flags"] + ["-DBEEFISCAL_TIME_ANCHOR_HOST_TEST"]),
                             ("cjson_sources_sha256", {})):
            with self.subTest(field=field, value=value), self.assertRaises(gate.GateError):
                gate.require_native_manifest(dict(good, **{field: value}), libraries, time_anchor=True)

    def test_time_phase_inventory_and_workflow_include_actual_crypto_sources(self):
        self.assertEqual(len(gate.PHASES), 10)
        self.assertLess(gate.PHASES.index("time-anchor"), gate.PHASES.index("source-recheck"))
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            fixture = repo / "IoT/firmware/edge-agent-s3/idf/tests/time_anchor_crypto_host/crypto_native.cpp"
            fixture.parent.mkdir(parents=True)
            fixture.write_text("actual fixture")
            self.assertIn(fixture, gate.source_files(repo))
            self.assertIn(repo / "scripts/verify-device-time-anchor-crypto-host.sh", gate.source_files(repo))
        workflow = (SCRIPT.parents[1] / ".github/workflows/ble-certificate-crypto.yml").read_text()
        self.assertEqual(workflow.count("'scripts/verify-device-time-anchor-crypto-host.sh'"), 2)
        self.assertIn("/**/time-anchor.log", workflow)
        self.assertIn("/**/time-anchor-build-manifest.json", workflow)

    def test_time_anchor_executes_nonexecutable_posix_runner(self):
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            script = repo / "fixture.sh"
            script.write_text("#!/bin/sh\nprintf 'actual-shell-runner\\n'\n")
            script.chmod(0o644)
            result = subprocess.run(gate.native_command(repo, "fixture.sh", time_anchor=True),
                                    capture_output=True, text=True, timeout=5)
            self.assertEqual(result.returncode, 0)
            self.assertEqual(result.stdout, "actual-shell-runner\n")

if __name__ == "__main__":
    unittest.main()
