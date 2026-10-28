"""EMU-01.1 — FakeDatecsDevice: 116/116 requests processed, 45 paired answers match golden."""
import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
EMU_DIR = ROOT / 'IoT/protocol-abstraction/tests/emu'
FIXTURES = ROOT / 'docs/MVP1/fixtures/datecs_golden_frames.json'


def _build_cli(tmp_dir: Path) -> Path:
    out = tmp_dir / 'datecs_cli'
    result = subprocess.run(
        ['c++', '-std=c++17', '-Wall', '-Wextra', '-Werror',
         '-I', str(EMU_DIR),
         str(EMU_DIR / 'FakeDatecsDeviceCLI.cpp'),
         '-o', str(out)],
        capture_output=True, text=True, timeout=120,
    )
    if result.returncode != 0:
        raise RuntimeError('Compile failed:\n' + result.stderr)
    return out


def _run_device_session(binary: Path, commands: list) -> list:
    """Send lines to the device CLI and collect responses."""
    stdin_data = '\n'.join(commands) + '\n'
    result = subprocess.run(
        [str(binary)],
        input=stdin_data, capture_output=True, text=True, timeout=60,
    )
    if result.returncode != 0:
        raise RuntimeError('CLI failed: ' + result.stderr)
    return result.stdout.splitlines()


class TestEMU01_1(unittest.TestCase):
    def test_golden_frames_processed(self):
        frames = json.loads(FIXTURES.read_text())
        requests = [f for f in frames if f.get('direction') == 'request']
        answers  = [f for f in frames if f.get('direction') == 'answer']
        self.assertEqual(len(requests), 116, f'Expected 116 requests, got {len(requests)}')

        # Build (cmd, seq) → answer frame hex map for the 45 paired answers
        req_index = {(f['cmd'], f['seq']): f['frame_hex'] for f in requests}
        ans_index  = {(f['cmd'], f['seq']): f['frame_hex'] for f in answers}
        pairs = {key: (req_index[key], ans_index[key])
                 for key in req_index if key in ans_index}
        self.assertGreaterEqual(len(pairs), 40, f'Expected >=40 golden pairs, got {len(pairs)}')

        with tempfile.TemporaryDirectory() as tmpdir:
            binary = _build_cli(Path(tmpdir))
            cmds = []
            # Register all golden pairs
            for (cmd, seq), (req_hex, ans_hex) in pairs.items():
                cmds.append(f'G {req_hex} {ans_hex}')
            # Handle all 116 requests
            for f in requests:
                cmds.append(f'H {f["frame_hex"]}')
            responses = _run_device_session(binary, cmds)

        # responses[0..len(pairs)-1] are from G commands (no output), but G doesn't emit output
        # responses list should have len(requests) entries (one per H command)
        self.assertEqual(len(responses), len(requests),
                         f'Expected {len(requests)} responses, got {len(responses)}: {responses[:5]}')

        nak = bytes([0x15]).hex()  # '15'
        paired_match = 0
        for i, f in enumerate(requests):
            resp = responses[i]
            self.assertNotEqual(resp, nak,
                                f'Request {i} (cmd={f["cmd"]},seq={f["seq"]}) got NAK')
            key = (f['cmd'], f['seq'])
            if key in pairs:
                expected = pairs[key][1]  # golden answer frame hex
                self.assertEqual(resp, expected,
                                 f'Paired answer mismatch for cmd={f["cmd"]},seq={f["seq"]}')
                paired_match += 1

        self.assertEqual(paired_match, len(pairs),
                         f'Not all paired answers matched: {paired_match} vs {len(pairs)}')

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out = Path(artifact_dir) / 'emu-output'
            out.mkdir(parents=True, exist_ok=True)
            (out / 'emu_01_1.log').write_text(
                f'processed={len(requests)} paired_match={paired_match}')


if __name__ == '__main__':
    unittest.main()
