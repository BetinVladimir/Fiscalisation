"""W0-1.1 — mvp1-baseline.sh assembler: report.json schema, good/bad/blocked fixtures."""
import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCRIPT = ROOT / "scripts" / "mvp1-baseline.sh"


def _run_baseline(fixture_data, timeout=30, outdir=None):
    """Helper: write fixture JSON, run baseline script, return (rc, report_dict)."""
    artifact_dir = outdir or os.environ.get("MVP1_ARTIFACT_DIR")
    if artifact_dir:
        out = Path(artifact_dir) / ("run_" + str(os.getpid()))
        out.mkdir(parents=True, exist_ok=True)
    else:
        out = Path(tempfile.mkdtemp(prefix="mvp1_baseline_"))

    with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as fj:
        json.dump(fixture_data, fj)
        fixture_path = fj.name

    try:
        result = subprocess.run(
            ["bash", str(SCRIPT),
             "--output-dir", str(out),
             "--fixture", fixture_path,
             "--timeout", str(timeout)],
            capture_output=True, text=True, timeout=60
        )
        report_path = out / "report.json"
        if report_path.exists():
            with open(report_path) as f:
                report = json.load(f)
        else:
            report = None
        return result.returncode, report
    finally:
        os.unlink(fixture_path)


def _make_good_fixture(tmpdir):
    """7 commands that succeed (write a marker file each)."""
    cmds = []
    for i in range(7):
        marker = str(tmpdir / f"marker_{i}.txt")
        cmds.append({
            "id": f"mod-{i}",
            "cwd": str(tmpdir),
            "argv": ["python3", "-c", f"open('{marker}','w').write('ok'); print('done')"]
        })
    return cmds


def _make_bad_fixture(tmpdir):
    """7 commands; one fails with rc=1."""
    cmds = _make_good_fixture(tmpdir)
    cmds[3]["argv"] = ["python3", "-c", "import sys; sys.exit(1)"]
    return cmds


def _make_blocked_fixture(tmpdir):
    """7 commands; one has a missing executable, one has a missing cwd."""
    cmds = _make_good_fixture(tmpdir)
    cmds[0]["argv"] = ["__no_such_executable_xyz__", "arg"]
    cmds[1]["cwd"] = "/nonexistent_path_xyz_12345"
    return cmds


class TestW01BaselineScript(unittest.TestCase):

    def setUp(self):
        self.tmpdir = Path(tempfile.mkdtemp(prefix="w01_"))

    def test_script_exists(self):
        self.assertTrue(SCRIPT.exists(), f"scripts/mvp1-baseline.sh not found at {SCRIPT}")

    # --- good fixture: all PASS, rc=0 ---
    def test_good_fixture_rc0(self):
        fixture = _make_good_fixture(self.tmpdir)
        rc, report = _run_baseline(fixture)
        self.assertEqual(rc, 0, f"Expected rc=0 for all-pass fixture; got rc={rc}\nreport={report}")

    def test_good_fixture_report_schema(self):
        fixture = _make_good_fixture(self.tmpdir)
        _rc, report = _run_baseline(fixture)
        self.assertIsNotNone(report, "report.json not produced")
        self.assertEqual(report.get("schema"), 1, "report.schema must be 1")
        self.assertIn("commit", report)
        self.assertIn("tools", report)
        self.assertIsInstance(report["tools"], dict)
        self.assertGreater(len(report["tools"]), 0, "tools must be non-empty dict")
        self.assertEqual(report.get("overall_status"), "PASS")

    def test_good_fixture_seven_records(self):
        fixture = _make_good_fixture(self.tmpdir)
        _rc, report = _run_baseline(fixture)
        self.assertIsNotNone(report)
        modules = report.get("modules", [])
        self.assertEqual(len(modules), 7, f"Expected 7 module records, got {len(modules)}")

    def test_good_fixture_all_records_pass(self):
        fixture = _make_good_fixture(self.tmpdir)
        _rc, report = _run_baseline(fixture)
        self.assertIsNotNone(report)
        for rec in report.get("modules", []):
            self.assertEqual(rec["status"], "PASS",
                             f"Module {rec['id']} status={rec['status']} expected PASS")
            self.assertEqual(rec["rc"], 0)
            self.assertIn("stdout", rec)
            self.assertIn("stderr", rec)
            self.assertIn("duration_seconds", rec)

    def test_good_fixture_markers_written(self):
        """Real execution evidence: marker files must exist."""
        fixture = _make_good_fixture(self.tmpdir)
        _run_baseline(fixture)
        for i in range(7):
            marker = self.tmpdir / f"marker_{i}.txt"
            self.assertTrue(marker.exists(), f"marker_{i}.txt not created; command may not have run")

    # --- bad fixture: one rc=1, overall FAIL, rc=1 ---
    def test_bad_fixture_rc1(self):
        fixture = _make_bad_fixture(self.tmpdir)
        rc, report = _run_baseline(fixture)
        self.assertEqual(rc, 1, f"Expected rc=1 for failure fixture; got rc={rc}")

    def test_bad_fixture_overall_fail(self):
        fixture = _make_bad_fixture(self.tmpdir)
        _rc, report = _run_baseline(fixture)
        self.assertIsNotNone(report)
        self.assertEqual(report.get("overall_status"), "FAIL")

    def test_bad_fixture_failing_record(self):
        fixture = _make_bad_fixture(self.tmpdir)
        _rc, report = _run_baseline(fixture)
        self.assertIsNotNone(report)
        fail_recs = [r for r in report.get("modules", []) if r["status"] == "FAIL"]
        self.assertGreater(len(fail_recs), 0, "Expected at least one FAIL record")
        for fr in fail_recs:
            self.assertNotEqual(fr["rc"], 0)

    # --- blocked fixture: missing executable / missing cwd, rc=3 ---
    def test_blocked_fixture_rc3(self):
        fixture = _make_blocked_fixture(self.tmpdir)
        rc, report = _run_baseline(fixture)
        self.assertEqual(rc, 3, f"Expected rc=3 for blocked fixture; got rc={rc}")

    def test_blocked_fixture_overall_blocked(self):
        fixture = _make_blocked_fixture(self.tmpdir)
        _rc, report = _run_baseline(fixture)
        self.assertIsNotNone(report)
        self.assertEqual(report.get("overall_status"), "BLOCKED")

    def test_blocked_fixture_blocked_records(self):
        fixture = _make_blocked_fixture(self.tmpdir)
        _rc, report = _run_baseline(fixture)
        self.assertIsNotNone(report)
        blocked = [r for r in report.get("modules", []) if r["status"] == "BLOCKED"]
        self.assertGreater(len(blocked), 0, "Expected at least one BLOCKED record")
        for br in blocked:
            self.assertIsNone(br["rc"])
            self.assertIn("reason", br)
            self.assertGreater(len(br.get("reason", "")), 0)

    def test_report_written_even_on_failure(self):
        """report.json must be produced regardless of outcome."""
        for fixture in [_make_good_fixture(self.tmpdir),
                        _make_bad_fixture(self.tmpdir),
                        _make_blocked_fixture(self.tmpdir)]:
            with tempfile.TemporaryDirectory() as td:
                rc, report = _run_baseline(fixture, outdir=Path(td))
                self.assertIsNotNone(report, f"report.json missing for fixture rc={rc}")

    def test_tools_canonical_keys(self):
        """Tools dict must contain the canonical set of keys."""
        fixture = _make_good_fixture(self.tmpdir)
        _rc, report = _run_baseline(fixture)
        self.assertIsNotNone(report)
        tools = report.get("tools", {})
        canonical = {"python", "git", "go", "c++", "node", "npm", "java", "docker"}
        for key in canonical:
            self.assertIn(key, tools, f"tools missing key '{key}'")

    def test_commit_field(self):
        fixture = _make_good_fixture(self.tmpdir)
        _rc, report = _run_baseline(fixture)
        self.assertIsNotNone(report)
        # commit is a string (40-char hex) or None with commit_reason
        commit = report.get("commit")
        commit_reason = report.get("commit_reason")
        if commit is None:
            self.assertIsNotNone(commit_reason, "commit is null but commit_reason missing")
        else:
            self.assertRegex(commit, r"^[0-9a-f]{40}$", "commit must be 40-char hex SHA")


if __name__ == "__main__":
    unittest.main()
