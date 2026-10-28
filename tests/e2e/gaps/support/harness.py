"""
Subprocess/in-process integration support for E2E tests.

Provides runtime adapters (Go, Kotlin, Native/C++) that:
- Launch real test suites via subprocess
- Manage temporary ports and cleanup
- Fail explicitly on missing executables (never mocked success)
- Support fault injection via environment variables
- Track test counts from native output formats
"""
import json
import os
import re
import shutil
import socket
import subprocess
import tempfile
import time
from pathlib import Path
from typing import Dict, List, Optional, Tuple


def find_free_port() -> int:
    """Find and return an available TCP port."""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(('127.0.0.1', 0))
        return s.getsockname()[1]


class _RuntimeResult:
    """Holds the outcome of a test runtime execution."""
    def __init__(self, returncode: int, stdout: str, stderr: str, test_count: int):
        self.returncode = returncode
        self.stdout = stdout
        self.stderr = stderr
        self.test_count = test_count

    @property
    def passed(self) -> bool:
        return self.returncode == 0

    def assert_passed(self):
        assert self.returncode == 0, (
            f'Runtime failed (rc={self.returncode}):\n'
            f'{self.stdout[-3000:]}\n{self.stderr[-1000:]}'
        )
        assert self.test_count > 0, (
            f'Runtime exited 0 but test_count=0 (no tests ran):\n{self.stdout[-2000:]}'
        )


class GoTestRuntime:
    """
    Runs a Go test package via `go test -v -json`.

    Never mocks the `go` binary: if it is missing, `run()` raises FileNotFoundError.
    Supports fault injection via extra env vars passed to the child process.
    """

    def __init__(
        self,
        root: Path,
        package: str = './...',
        run_pattern: Optional[str] = None,
        extra_env: Optional[Dict[str, str]] = None,
        timeout: int = 300,
    ):
        self.root = Path(root)
        self.package = package
        self.run_pattern = run_pattern
        self.extra_env = extra_env or {}
        self.timeout = timeout
        self._proc: Optional[subprocess.Popen] = None

    def run(self, artifact_dir: Optional[str] = None) -> _RuntimeResult:
        go = shutil.which('go')
        if go is None:
            raise FileNotFoundError('go binary not found on PATH — cannot run Go test suite')

        cmd = [go, 'test', '-v', '-json', self.package]
        if self.run_pattern:
            cmd += ['-run', self.run_pattern]

        env = os.environ.copy()
        env.update(self.extra_env)

        proc = subprocess.run(
            cmd,
            cwd=str(self.root),
            capture_output=True,
            text=True,
            timeout=self.timeout,
            env=env,
        )

        test_count = _count_go_json_tests(proc.stdout)

        if artifact_dir:
            path = os.path.join(artifact_dir, f'go_test_{self.package.replace("/", "_")}.json')
            with open(path, 'w') as f:
                f.write(proc.stdout + proc.stderr)

        return _RuntimeResult(proc.returncode, proc.stdout, proc.stderr, test_count)

    def teardown(self):
        if self._proc and self._proc.poll() is None:
            self._proc.terminate()
            try:
                self._proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self._proc.kill()
                self._proc.wait()
        self._proc = None


class KotlinTestRuntime:
    """
    Runs a Kotlin/Gradle test suite via `./gradlew <task>`.

    Never mocks gradlew: if it is missing, `run()` raises FileNotFoundError.
    Parses JUnit XML test results for accurate test counts.
    """

    def __init__(
        self,
        root: Path,
        task: str = 'testProdDebugUnitTest',
        tests_filter: Optional[str] = None,
        extra_env: Optional[Dict[str, str]] = None,
        timeout: int = 600,
    ):
        self.root = Path(root)
        self.task = task
        self.tests_filter = tests_filter
        self.extra_env = extra_env or {}
        self.timeout = timeout

    def run(self, artifact_dir: Optional[str] = None) -> _RuntimeResult:
        gradlew = self.root / 'gradlew'
        if not gradlew.exists():
            raise FileNotFoundError(f'gradlew not found at {gradlew}')

        cmd = [str(gradlew), self.task, '-q', '--no-daemon']
        if self.tests_filter:
            cmd += ['--tests', self.tests_filter]

        env = os.environ.copy()
        env.update(self.extra_env)

        proc = subprocess.run(
            cmd,
            cwd=str(self.root),
            capture_output=True,
            text=True,
            timeout=self.timeout,
            env=env,
        )

        # Search for test results recursively under root (covers subproject builds)
        test_count = _count_junit_xml_tests(self.root)

        if artifact_dir:
            path = os.path.join(artifact_dir, f'kotlin_{self.task}.txt')
            with open(path, 'w') as f:
                f.write(proc.stdout[-4000:] + proc.stderr[-2000:])

        return _RuntimeResult(proc.returncode, proc.stdout, proc.stderr, test_count)


class NativeTestRuntime:
    """
    Compiles and runs a C++ native test suite via a bash script.

    Never mocks the compiler or script: if it is missing, `run()` raises FileNotFoundError.
    Parses stdout for TAP or marker lines to count tests.
    """

    def __init__(
        self,
        root: Path,
        script: Path,
        required_markers: Optional[List[str]] = None,
        extra_env: Optional[Dict[str, str]] = None,
        timeout: int = 300,
    ):
        self.root = Path(root)
        self.script = Path(script)
        self.required_markers = required_markers or []
        self.extra_env = extra_env or {}
        self.timeout = timeout

    def run(self, artifact_dir: Optional[str] = None) -> _RuntimeResult:
        if not self.script.exists():
            raise FileNotFoundError(f'test script not found: {self.script}')

        env = os.environ.copy()
        env.update(self.extra_env)

        proc = subprocess.run(
            ['bash', str(self.script)],
            cwd=str(self.root),
            capture_output=True,
            text=True,
            timeout=self.timeout,
            env=env,
        )

        test_count = _count_native_markers(proc.stdout, self.required_markers)

        if artifact_dir:
            path = os.path.join(artifact_dir, f'native_{self.script.stem}.txt')
            with open(path, 'w') as f:
                f.write(proc.stdout + proc.stderr)

        return _RuntimeResult(proc.returncode, proc.stdout, proc.stderr, test_count)


# ── Internal helpers ──────────────────────────────────────────────────────────

def _count_go_json_tests(output: str) -> int:
    """Count passed+failed test events from `go test -json` output."""
    count = 0
    for line in output.splitlines():
        try:
            obj = json.loads(line)
            if obj.get('Action') in ('pass', 'fail') and obj.get('Test'):
                count += 1
        except (json.JSONDecodeError, KeyError):
            continue
    return count


def _count_junit_xml_tests(results_dir: Path) -> int:
    """Sum tests from all JUnit XML files under results_dir."""
    import xml.etree.ElementTree as ET
    total = 0
    if not results_dir.exists():
        return 0
    for xml_file in results_dir.rglob('TEST-*.xml'):
        try:
            tree = ET.parse(xml_file)
            total += int(tree.getroot().get('tests', 0))
        except Exception:
            continue
    return total


def _count_native_markers(output: str, required_markers: List[str]) -> int:
    """Return the number of required markers found in output (as a test count proxy)."""
    if not required_markers:
        # Fallback: count TAP 'ok' lines
        return sum(1 for line in output.splitlines() if re.match(r'^ok\b', line))
    return sum(1 for m in required_markers if m in output)
