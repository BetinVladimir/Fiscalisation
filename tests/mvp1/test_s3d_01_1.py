"""S3D-01.1 — Datecs golden 116 request frames: C++ DatecsCodec and Kotlin DatecsFrameCodec agree."""
import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path
import xml.etree.ElementTree as ET

ROOT = Path(__file__).parent.parent.parent
FIXTURE = ROOT / 'docs' / 'MVP1' / 'fixtures' / 'datecs_golden_frames.json'
FRAME_CODEC_CPP = ROOT / 'IoT' / 'protocol-abstraction' / 'src' / 'FrameCodec.cpp'
FRAME_CODEC_HDR = ROOT / 'IoT' / 'protocol-abstraction' / 'include'
BLUECASH_DIR = ROOT / 'SmartDevices' / 'bluecash-app'
SMARTDEVICES_DIR = ROOT / 'SmartDevices'


def _load_request_frames():
    with open(FIXTURE) as f:
        data = json.load(f)
    return [r for r in data if r['direction'] == 'request']


def _hex_bytes_literal(hex_str: str) -> str:
    """Return C++ initialiser list for a hex string, e.g. '01ab' → '0x01,0xab'."""
    if not hex_str:
        return ''
    return ','.join(f'0x{hex_str[i:i+2]}' for i in range(0, len(hex_str), 2))


def _build_golden_cpp(frames: list) -> str:
    lines = [
        '#include "FrameCodec.hpp"',
        '#include <cstdio>',
        '#include <vector>',
        'using Bytes = std::vector<uint8_t>;',
        'static int failures = 0;',
        'static void check(bool ok, const char* label) {',
        '    if (!ok) { std::fprintf(stderr, "FAIL %s\\n", label); ++failures; }',
        '}',
        'int main() {',
    ]
    for i, fr in enumerate(frames):
        seq = fr['seq']
        cmd = fr['cmd']
        data_lit = _hex_bytes_literal(fr['data_hex'])
        frame_lit = _hex_bytes_literal(fr['frame_hex'])
        data_init = f'{{{data_lit}}}' if data_lit else 'Bytes{}'
        label = f'frame_{i+1}_cmd{cmd}_seq{seq}'
        lines.append(
            f'    check(bee::DatecsCodec::encode(0x{seq:02x}, 0x{cmd:04x},'
            f' Bytes{{{data_lit}}}) == Bytes{{{frame_lit}}}, "{label}");'
        )
    lines += [
        f'    int total = {len(frames)};',
        '    int passed = total - failures;',
        '    std::printf("%d/%d datecs_golden_request_frames=pass\\n", passed, total);',
        '    return failures ? 1 : 0;',
        '}',
    ]
    return '\n'.join(lines)


class TestS3D01GoldenFrames(unittest.TestCase):

    def test_fixture_has_116_request_frames(self):
        frames = _load_request_frames()
        self.assertEqual(
            len(frames), 116,
            f'Expected 116 request frames in fixture, got {len(frames)}',
        )

    def test_cpp_datecs_codec_matches_all_116_golden_frames(self):
        frames = _load_request_frames()
        self.assertEqual(len(frames), 116, 'fixture must have 116 request frames')

        cpp_src = _build_golden_cpp(frames)
        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR', tempfile.mkdtemp())

        with tempfile.TemporaryDirectory() as tmpdir:
            src_file = os.path.join(tmpdir, 'datecs_golden_test.cpp')
            exe_file = os.path.join(tmpdir, 'datecs_golden_test')
            with open(src_file, 'w') as f:
                f.write(cpp_src)

            compile_result = subprocess.run(
                [
                    'c++', '-std=c++17', '-Wall', '-Wextra', '-Werror',
                    f'-I{FRAME_CODEC_HDR}',
                    str(FRAME_CODEC_CPP),
                    src_file,
                    '-o', exe_file,
                ],
                cwd=str(ROOT),
                capture_output=True,
                text=True,
                timeout=120,
            )
            self.assertEqual(
                compile_result.returncode, 0,
                'C++ golden frame test compilation failed:\n'
                + compile_result.stdout + compile_result.stderr,
            )

            run_result = subprocess.run(
                [exe_file],
                capture_output=True,
                text=True,
                timeout=60,
            )
            # Save evidence
            evidence_path = os.path.join(artifact_dir, 'cpp_datecs_golden.txt')
            with open(evidence_path, 'w') as f:
                f.write(run_result.stdout + run_result.stderr)

            self.assertEqual(
                run_result.returncode, 0,
                'C++ golden frame test failed:\n'
                + run_result.stdout[-3000:] + run_result.stderr[-1000:],
            )
            self.assertIn(
                '116/116 datecs_golden_request_frames=pass', run_result.stdout,
                'Expected 116/116 marker not found in output:\n' + run_result.stdout,
            )

    def test_kotlin_datecs_protocol_tests_pass(self):
        self.assertTrue(BLUECASH_DIR.exists(), f'bluecash-app dir not found: {BLUECASH_DIR}')

        gradlew = str(SMARTDEVICES_DIR / 'gradlew')
        result = subprocess.run(
            [
                gradlew, 'testProdDebugUnitTest',
                '--tests', 'com.beeloy.fiscal.bluecash.DatecsProtocolTest',
                '-q', '--no-daemon',
            ],
            cwd=str(BLUECASH_DIR),
            capture_output=True,
            text=True,
            timeout=300,
        )

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR', tempfile.mkdtemp())
        evidence_path = os.path.join(artifact_dir, 'kotlin_datecs_protocol.txt')
        with open(evidence_path, 'w') as f:
            f.write(result.stdout[-4000:] + result.stderr[-2000:])

        self.assertEqual(
            result.returncode, 0,
            'Kotlin DatecsProtocolTest failed:\n'
            + result.stdout[-3000:] + result.stderr[-1000:],
        )

        # Parse JUnit XML for test counts
        xml_path = (
            BLUECASH_DIR / 'build' / 'test-results' / 'testProdDebugUnitTest'
            / 'TEST-com.beeloy.fiscal.bluecash.DatecsProtocolTest.xml'
        )
        if xml_path.exists():
            tree = ET.parse(xml_path)
            suite = tree.getroot()
            test_count = int(suite.get('tests', 0))
            failures = int(suite.get('failures', 0))
            errors = int(suite.get('errors', 0))
            self.assertGreater(test_count, 0, 'No tests found in DatecsProtocolTest')
            self.assertEqual(failures + errors, 0,
                             f'{failures} failures, {errors} errors in DatecsProtocolTest')


if __name__ == '__main__':
    unittest.main()
