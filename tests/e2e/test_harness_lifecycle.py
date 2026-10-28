"""E2E-HARNESS-01.1 — harness lifecycle: real runtimes start/stop, missing exe → failure."""
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(Path(__file__).parent / 'gaps' / 'support'))

from harness import GoTestRuntime, KotlinTestRuntime, NativeTestRuntime, find_free_port


class TestHarnessLifecycle(unittest.TestCase):

    def test_find_free_port_returns_usable_port(self):
        import socket
        port = find_free_port()
        self.assertGreater(port, 0)
        self.assertLess(port, 65536)
        # Verify it's actually free (bind succeeds)
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
            s.bind(('127.0.0.1', port))

    def test_go_runtime_runs_real_tests(self):
        rt = GoTestRuntime(
            root=ROOT / 'fiscal-backend',
            package='./internal/config/...',
            run_pattern='TestSplitCSV',
            timeout=120,
        )
        result = rt.run()
        self.assertEqual(result.returncode, 0,
                         f'Go config test failed: {result.stdout[-2000:]}')
        self.assertGreater(result.test_count, 0,
                           f'test_count=0 — no tests ran: {result.stdout[-1000:]}')
        rt.teardown()

    def test_go_runtime_missing_binary_raises(self):
        rt = GoTestRuntime(root=ROOT / 'fiscal-backend', package='./internal/config/...')
        original = os.environ.get('PATH', '')
        try:
            os.environ['PATH'] = ''
            with self.assertRaises(FileNotFoundError):
                rt.run()
        finally:
            os.environ['PATH'] = original

    def test_go_runtime_writes_artifact(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            rt = GoTestRuntime(
                root=ROOT / 'fiscal-backend',
                package='./internal/config/...',
                run_pattern='TestSplitCSV',
                timeout=120,
            )
            result = rt.run(artifact_dir=tmpdir)
            self.assertEqual(result.returncode, 0)
            artifacts = list(Path(tmpdir).iterdir())
            self.assertTrue(artifacts, 'no artifact file written')

    def test_native_runtime_missing_script_raises(self):
        missing = ROOT / 'IoT' / 'firmware' / 'edge-agent-s3' / 'idf' / 'tests' / 'no_such_script.sh'
        rt = NativeTestRuntime(root=ROOT, script=missing)
        with self.assertRaises(FileNotFoundError):
            rt.run()

    def test_native_runtime_runs_real_suite(self):
        script = ROOT / 'IoT' / 'protocol-abstraction' / 'run-tests.sh'
        self.assertTrue(script.exists(), f'run-tests.sh not found: {script}')
        rt = NativeTestRuntime(
            root=ROOT,
            script=script,
            required_markers=['runtime vendor codec tests: OK', 'DaisyCp1251AndSynTest: OK'],
            timeout=180,
        )
        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR', tempfile.mkdtemp())
        result = rt.run(artifact_dir=artifact_dir)
        self.assertEqual(result.returncode, 0,
                         f'native suite failed:\n{result.stdout[-2000:]}\n{result.stderr[-500:]}')
        self.assertGreater(result.test_count, 0, 'no native test markers found')

    def test_kotlin_runtime_missing_gradlew_raises(self):
        rt = KotlinTestRuntime(root=ROOT / 'no-such-dir')
        with self.assertRaises(FileNotFoundError):
            rt.run()

    def test_kotlin_runtime_runs_real_tests(self):
        rt = KotlinTestRuntime(
            root=ROOT / 'SmartDevices',
            task=':bluecash-app:testProdDebugUnitTest',
            tests_filter='com.beeloy.fiscal.bluecash.DatecsProtocolTest',
            timeout=300,
        )
        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR', tempfile.mkdtemp())
        result = rt.run(artifact_dir=artifact_dir)
        self.assertEqual(
            result.returncode, 0,
            f'Kotlin DatecsProtocolTest failed:\n{result.stdout[-3000:]}\n{result.stderr[-1000:]}',
        )
        self.assertGreater(result.test_count, 0,
                           'Expected non-zero test count from Kotlin suite')


if __name__ == '__main__':
    unittest.main()
