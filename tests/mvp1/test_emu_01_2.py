"""EMU-01.2 — FakeDatecsDevice: SEQ cache + FaultPlan DROP_AFTER=[56] → doc_counter==1."""
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
EMU_DIR = ROOT / 'IoT/protocol-abstraction/tests/emu'


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


def _encode_datecs_request(seq: int, cmd: int, data: bytes = b'') -> str:
    """Encode a Datecs REQUEST frame as hex string (per DatecsCodec::encode logic)."""
    def nibble4(x):
        return bytes([0x30 + ((x >> s) & 15) for s in (12, 8, 4, 0)])

    frame = b'\x01'
    frame += nibble4(len(data) + 0x2A)
    frame += bytes([seq])
    frame += nibble4(cmd)
    frame += data
    frame += b'\x05'
    bcc = sum(frame[1:]) & 0xFFFF
    frame += nibble4(bcc)
    frame += b'\x03'
    return frame.hex()


def _run_session(binary: Path, commands: list) -> list:
    stdin_data = '\n'.join(commands) + '\n'
    result = subprocess.run(
        [str(binary)],
        input=stdin_data, capture_output=True, text=True, timeout=30,
    )
    if result.returncode != 0:
        raise RuntimeError('CLI error: ' + result.stderr)
    return result.stdout.splitlines()


class TestEMU01_2(unittest.TestCase):
    def test_drop_after_and_seq_cache(self):
        # Build cmd-56 frame with SEQ=0x20
        frame_hex = _encode_datecs_request(0x20, 56)

        with tempfile.TemporaryDirectory() as tmpdir:
            binary = _build_cli(Path(tmpdir))
            cmds = [
                'DROP 56',         # set DROP_AFTER=[56]
                f'H {frame_hex}',  # first call: cmd 56 executed, no response
                f'H {frame_hex}',  # second call: same SEQ → cached (no response), counter NOT re-incremented
                'C',               # get doc_counter
            ]
            responses = _run_session(binary, cmds)

        # Two H commands + one C command = 3 output lines
        self.assertEqual(len(responses), 3,
                         f'Expected 3 output lines, got {len(responses)}: {responses}')

        # Both H commands got DROP (empty response)
        self.assertEqual(responses[0], '',
                         'First send of cmd 56 should be dropped (empty response)')
        self.assertEqual(responses[1], '',
                         'Second send (same SEQ) should use cached empty response')

        # doc_counter must be exactly 1 — second call reused cache, did NOT re-execute
        self.assertEqual(responses[2], '1',
                         f'doc_counter should be 1 after two sends with same SEQ, got {responses[2]}')

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out = Path(artifact_dir) / 'emu-output'
            out.mkdir(parents=True, exist_ok=True)
            (out / 'emu_01_2.log').write_text('\n'.join(responses))


if __name__ == '__main__':
    unittest.main()
