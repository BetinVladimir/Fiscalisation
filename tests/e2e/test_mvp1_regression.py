"""REGRESSION-01.1 — unified offline regression harness.

Runs all module test suites via subprocess. Each suite is PASS, FAIL, or BLOCKED.
BLOCKED = required env var missing or tool not installed.
A suite with zero tests and rc=0 is FAIL (not PASS).
"""
import json
import os
import re
import shutil
import subprocess
import sys
import unittest
import xml.etree.ElementTree as ET
from pathlib import Path
from typing import NamedTuple

ROOT = Path(__file__).parent.parent.parent


class SuiteResult(NamedTuple):
    name: str
    status: str   # PASS | FAIL | BLOCKED
    test_count: int
    message: str


def _find_tool(name: str) -> bool:
    return shutil.which(name) is not None


def _count_go_tests(output: str) -> int:
    """Count passed tests from 'go test -v' or JSON output."""
    count = 0
    for line in output.splitlines():
        # go test -v: "--- PASS: TestFoo (0.00s)"
        if re.match(r'\s*--- (PASS|FAIL):', line):
            count += 1
        # go test JSON: {"Action":"pass","Test":"TestFoo",...}
        elif line.startswith('{"Action":"pass"') and '"Test"' in line:
            count += 1
    # fallback: "ok  pkg  0.00s" lines
    if count == 0:
        count = len(re.findall(r'^ok\s+', output, re.MULTILINE))
    return count


def _count_junit_xml(cwd: Path, since_mtime: float) -> int:
    count = 0
    for xml_file in cwd.rglob('TEST-*.xml'):
        try:
            if xml_file.stat().st_mtime < since_mtime:
                continue
            root = ET.parse(xml_file).getroot()
            for tc in root.iter('testcase'):
                count += 1
        except (ET.ParseError, OSError):
            pass
    return count


def _count_pytest(output: str) -> int:
    m = re.search(r'(\d+) passed', output)
    return int(m.group(1)) if m else 0


def _count_tap(output: str) -> int:
    return len(re.findall(r'^ok \d+', output, re.MULTILINE))


def _count_native_pass(output: str) -> int:
    """Count =pass and ': OK' markers from IoT native test scripts."""
    count = len(re.findall(r'=pass', output))
    count += len(re.findall(r':\s+OK', output))
    return count


def _run_go_backend() -> SuiteResult:
    name = "go-backend"
    if not _find_tool('go'):
        return SuiteResult(name, 'BLOCKED', 0, 'go not found')
    cwd = ROOT / 'fiscal-backend'
    r = subprocess.run(
        ['go', 'test', '-count=1', '-v', './...'],
        cwd=str(cwd), capture_output=True, text=True, timeout=300,
    )
    combined = r.stdout + r.stderr
    count = _count_go_tests(combined)
    if r.returncode != 0:
        return SuiteResult(name, 'FAIL', count, combined[-2000:])
    if count == 0:
        return SuiteResult(name, 'FAIL', 0, 'zero tests reported')
    return SuiteResult(name, 'PASS', count, f'{count} tests')


def _run_kotlin_bluecash() -> SuiteResult:
    name = "kotlin-bluecash"
    gradlew = ROOT / 'SmartDevices' / 'gradlew'
    if not gradlew.exists():
        return SuiteResult(name, 'BLOCKED', 0, 'gradlew not found')
    import time
    since = time.time()
    r = subprocess.run(
        [str(gradlew), ':bluecash-app:testProdDebugUnitTest', '--no-daemon'],
        cwd=str(ROOT / 'SmartDevices'), capture_output=True, text=True, timeout=600,
    )
    count = _count_junit_xml(ROOT / 'SmartDevices', since)
    if r.returncode != 0:
        return SuiteResult(name, 'FAIL', count, (r.stdout + r.stderr)[-2000:])
    if count == 0:
        return SuiteResult(name, 'FAIL', 0, 'zero JUnit test cases found')
    return SuiteResult(name, 'PASS', count, f'{count} tests')


def _run_s3_native() -> SuiteResult:
    name = "s3-native"
    script = ROOT / 'IoT/firmware/edge-agent-s3/idf/tests/run_native_serial.sh'
    if not script.exists():
        return SuiteResult(name, 'BLOCKED', 0, 'run_native_serial.sh not found')
    if not _find_tool('bash'):
        return SuiteResult(name, 'BLOCKED', 0, 'bash not found')
    r = subprocess.run(
        ['bash', str(script)],
        cwd=str(ROOT), capture_output=True, text=True, timeout=120,
    )
    combined = r.stdout + r.stderr
    count = _count_native_pass(combined)
    if count == 0:
        count = _count_tap(combined)
    if count == 0:
        count = _count_go_tests(combined)
    if r.returncode != 0:
        return SuiteResult(name, 'FAIL', count, combined[-2000:])
    if count == 0:
        return SuiteResult(name, 'FAIL', 0, 'zero tests reported')
    return SuiteResult(name, 'PASS', count, f'{count} tests')


def _run_protocol_abstraction() -> SuiteResult:
    name = "protocol-abstraction"
    script = ROOT / 'IoT/protocol-abstraction/run-tests.sh'
    if not script.exists():
        return SuiteResult(name, 'BLOCKED', 0, 'run-tests.sh not found')
    if not _find_tool('bash'):
        return SuiteResult(name, 'BLOCKED', 0, 'bash not found')
    r = subprocess.run(
        ['bash', str(script)],
        cwd=str(ROOT / 'IoT/protocol-abstraction'), capture_output=True, text=True, timeout=120,
    )
    combined = r.stdout + r.stderr
    count = _count_native_pass(combined)
    if count == 0:
        count = _count_tap(combined)
    if count == 0:
        count = _count_go_tests(combined)
    if r.returncode != 0:
        return SuiteResult(name, 'FAIL', count, combined[-2000:])
    if count == 0:
        return SuiteResult(name, 'FAIL', 0, 'zero tests reported')
    return SuiteResult(name, 'PASS', count, f'{count} tests')


def _run_pos_typescript() -> SuiteResult:
    name = "pos-typescript"
    pos_dir = Path('/Users/freelancer/Documents/Beeloy/POS/pos-app')
    if not pos_dir.exists():
        return SuiteResult(name, 'BLOCKED', 0, 'POS repo not found')
    if not _find_tool('npx'):
        return SuiteResult(name, 'BLOCKED', 0, 'npx not found')
    r = subprocess.run(
        ['npx', 'jest', '--no-coverage', '--json'],
        cwd=str(pos_dir), capture_output=True, text=True, timeout=300,
    )
    count = 0
    try:
        result_json = json.loads(r.stdout)
        count = result_json.get('numPassedTests', 0) + result_json.get('numFailedTests', 0)
    except (json.JSONDecodeError, ValueError):
        count = _count_tap(r.stdout + r.stderr)
    if r.returncode != 0:
        return SuiteResult(name, 'FAIL', count, (r.stdout + r.stderr)[-2000:])
    if count == 0:
        return SuiteResult(name, 'FAIL', 0, 'zero tests reported')
    return SuiteResult(name, 'PASS', count, f'{count} tests')


def _run_mvp1_acceptance() -> SuiteResult:
    name = "mvp1-acceptance"
    if not _find_tool('pytest'):
        # try python3 -m pytest
        r0 = subprocess.run(
            [sys.executable, '-m', 'pytest', '--version'],
            capture_output=True, text=True,
        )
        if r0.returncode != 0:
            return SuiteResult(name, 'BLOCKED', 0, 'pytest not found')
    r = subprocess.run(
        [sys.executable, '-m', 'pytest', 'tests/mvp1/', '-q', '--tb=no', '--no-header'],
        cwd=str(ROOT), capture_output=True, text=True, timeout=1800,
    )
    combined = r.stdout + r.stderr
    count = _count_pytest(combined)
    if count == 0:
        # check for "x passed" in any format
        m = re.search(r'(\d+) (?:passed|failed)', combined)
        if m:
            count = int(m.group(1))
    if r.returncode != 0:
        return SuiteResult(name, 'FAIL', count, combined[-3000:])
    if count == 0:
        return SuiteResult(name, 'FAIL', 0, 'zero tests reported')
    return SuiteResult(name, 'PASS', count, f'{count} tests')


SUITES = [
    _run_go_backend,
    _run_kotlin_bluecash,
    _run_s3_native,
    _run_protocol_abstraction,
    _run_pos_typescript,
    _run_mvp1_acceptance,
]


def run_all_suites() -> list:
    results = []
    for fn in SUITES:
        try:
            result = fn()
        except subprocess.TimeoutExpired as e:
            result = SuiteResult(fn.__name__.lstrip('_run_'), 'FAIL', 0, f'timeout: {e}')
        except Exception as e:
            result = SuiteResult(fn.__name__.lstrip('_run_'), 'FAIL', 0, f'exception: {e}')
        print(f"  [{result.status:7s}] {result.name}: {result.message[:120]}")
        results.append(result)
    return results


class TestMVP1Regression(unittest.TestCase):
    def test_all_suites(self):
        print()
        results = run_all_suites()

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out_dir = Path(artifact_dir) / 'regression'
            out_dir.mkdir(parents=True, exist_ok=True)
            lines = [f"{r.status} {r.name} ({r.test_count} tests): {r.message[:200]}" for r in results]
            (out_dir / 'results.txt').write_text('\n'.join(lines))

        failures = [r for r in results if r.status == 'FAIL']
        blocked = [r for r in results if r.status == 'BLOCKED']
        passed = [r for r in results if r.status == 'PASS']

        print(f"\nSummary: {len(passed)} PASS, {len(blocked)} BLOCKED, {len(failures)} FAIL")

        self.assertEqual(
            failures, [],
            'Suites with FAIL status:\n' +
            '\n'.join(f'  {r.name}: {r.message[:500]}' for r in failures),
        )
        # At least one suite must actually PASS (not all BLOCKED)
        self.assertGreater(
            len(passed), 0,
            f'All {len(blocked)} suites are BLOCKED — no real test coverage confirmed',
        )


if __name__ == '__main__':
    unittest.main(verbosity=2)
