"""TST-DEEP-13.1 — DaisyCodec against 17 golden frames from fixture (test executes)."""
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
IOT = ROOT / 'IoT' / 'protocol-abstraction'
FIXTURE = ROOT / 'docs' / 'MVP1' / 'fixtures' / 'daisy_golden_frames.json'


class TestTSTDEEP13_1(unittest.TestCase):
    def test_daisy_golden_frames_codec(self):
        self.assertTrue(FIXTURE.exists(), f'fixture missing: {FIXTURE}')
        test_src = IOT / 'tests' / 'DaisyGoldenFramesTest.cpp'
        self.assertTrue(test_src.exists(), f'test source missing: {test_src}')

        with tempfile.TemporaryDirectory() as tmp:
            binary = os.path.join(tmp, 'DaisyGoldenFramesTest')
            compile_cmd = [
                'c++', '-std=c++17', '-Wall', '-Wextra', '-Werror',
                '-I', str(IOT / 'include'),
                str(IOT / 'src' / 'FrameCodec.cpp'),
                str(test_src),
                '-o', binary,
            ]
            compile = subprocess.run(
                compile_cmd,
                cwd=str(ROOT),
                capture_output=True,
                text=True,
                timeout=60,
            )
            self.assertEqual(
                compile.returncode, 0,
                'compile failed:\n' + compile.stdout + '\n' + compile.stderr,
            )

            run = subprocess.run(
                [binary, str(FIXTURE)],
                cwd=str(ROOT),
                capture_output=True,
                text=True,
                timeout=30,
            )
            output = run.stdout + run.stderr
            self.assertEqual(
                run.returncode, 0,
                f'DaisyGoldenFramesTest failed (rc={run.returncode}):\n{output}',
            )
            self.assertIn('daisy_golden_frames_test: OK', run.stdout,
                          'success marker missing:\n' + output)
            self.assertIn('daisy_golden_frames_positive=', run.stdout,
                          'positive count marker missing')
            self.assertIn('daisy_golden_frames_negative=', run.stdout,
                          'negative count marker missing')

            artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
            if artifact_dir:
                out_dir = Path(artifact_dir) / 'native-output'
                out_dir.mkdir(parents=True, exist_ok=True)
                (out_dir / 'daisy_golden_frames.stdout.log').write_text(run.stdout)


if __name__ == '__main__':
    unittest.main()
