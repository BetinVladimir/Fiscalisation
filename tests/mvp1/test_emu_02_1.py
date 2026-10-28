"""EMU-02.1 — FakeDaisyDevice: 15 BCC-valid frames processed, 1 bad-BCC frame rejected."""
import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
EMU_DIR = ROOT / 'IoT/protocol-abstraction/tests/emu'
FIXTURES = ROOT / 'docs/MVP1/fixtures/daisy_golden_frames.json'


def _build_cli(tmp_dir: Path) -> Path:
    out = tmp_dir / 'daisy_cli'
    result = subprocess.run(
        ['c++', '-std=c++17', '-Wall', '-Wextra', '-Werror',
         '-I', str(EMU_DIR),
         str(EMU_DIR / 'FakeDaisyDeviceCLI.cpp'),
         '-o', str(out)],
        capture_output=True, text=True, timeout=120,
    )
    if result.returncode != 0:
        raise RuntimeError('Compile failed:\n' + result.stderr)
    return out


def _encode_daisy_request(seq: int, cmd: int, data: bytes = b'') -> bytes:
    """Encode a Daisy REQUEST frame (per DaisyCodec::encode logic)."""
    def nibble4(x):
        return bytes([0x30 + ((x >> s) & 15) for s in (12, 8, 4, 0)])

    frame = bytes([0x01, len(data) + 0x24, seq, cmd]) + data + b'\x05'
    bcc = sum(frame[1:]) & 0xFFFF
    return frame + nibble4(bcc) + b'\x03'


def _run_session(binary: Path, commands: list) -> list:
    stdin_data = '\n'.join(commands) + '\n'
    result = subprocess.run(
        [str(binary)],
        input=stdin_data, capture_output=True, text=True, timeout=60,
    )
    if result.returncode != 0:
        raise RuntimeError('CLI error: ' + result.stderr)
    return result.stdout.splitlines()


class TestEMU02_1(unittest.TestCase):
    def test_15_valid_1_rejected(self):
        golden = json.loads(FIXTURES.read_text())
        requests = [f for f in golden if f.get('direction') == 'request']
        answers  = {(f['cmd'], f['seq']): f['frame_hex']
                    for f in golden if f.get('direction') == 'answer'}

        # Build golden pairs
        pairs = {(f['cmd'], f['seq']): (f['frame_hex'], answers[(f['cmd'], f['seq'])])
                 for f in requests if (f['cmd'], f['seq']) in answers}

        # Construct 15 valid frames: use golden requests (8) + extra valid frames
        valid_frames = [bytes.fromhex(f['frame_hex']) for f in requests]
        seq = 0x21
        while len(valid_frames) < 15:
            # Additional valid requests: cmd=74 (status), no data
            frame = _encode_daisy_request(seq, 74)
            valid_frames.append(frame)
            seq = (seq + 1) if seq < 0xFF else 0x20

        valid_frames = valid_frames[:15]

        # Build one bad-BCC frame: take first golden request, flip a BCC byte
        bad = bytearray(bytes.fromhex(requests[0]['frame_hex']))
        bad[-2] ^= 0xFF  # corrupt BCC nibble
        bad_frame = bytes(bad)

        with tempfile.TemporaryDirectory() as tmpdir:
            binary = _build_cli(Path(tmpdir))
            cmds = []
            # Register golden pairs
            for (cmd, seq), (req_hex, ans_hex) in pairs.items():
                cmds.append(f'G {req_hex} {ans_hex}')
            # Handle 15 valid frames
            for f in valid_frames:
                cmds.append(f'H {f.hex()}')
            # Handle 1 bad frame
            cmds.append(f'H {bad_frame.hex()}')
            responses = _run_session(binary, cmds)

        self.assertEqual(len(responses), 16,
                         f'Expected 16 responses (15+1), got {len(responses)}')

        nak = bytes([0x15]).hex()  # '15'

        # First 15: not NAK
        for i in range(15):
            self.assertNotEqual(responses[i], nak,
                                f'Valid frame {i} got unexpected NAK')
            self.assertGreater(len(responses[i]), 0,
                               f'Valid frame {i} got empty response')

        # 16th: must be NAK
        self.assertEqual(responses[15], nak,
                         f'Bad-BCC frame should be NAK (15), got {responses[15]}')

        # Check golden pair matches
        req_hexes = {f['frame_hex'] for f in requests}
        for i, f in enumerate(valid_frames):
            key = None
            fhex = f.hex()
            for rq in requests:
                if rq['frame_hex'] == fhex:
                    key = (rq['cmd'], rq['seq'])
                    break
            if key and key in pairs:
                expected = pairs[key][1]
                self.assertEqual(responses[i], expected,
                                 f'Golden pair mismatch for frame {i}')

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out = Path(artifact_dir) / 'emu-output'
            out.mkdir(parents=True, exist_ok=True)
            (out / 'emu_02_1.log').write_text(f'valid=15 rejected=1\n' + '\n'.join(responses))


if __name__ == '__main__':
    unittest.main()
