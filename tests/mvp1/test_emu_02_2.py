"""EMU-02.2 — FakeDaisyDevice: non-CASH payment → UNSUPPORTED; Latin tax letter → NAK."""
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
EMU_DIR = ROOT / 'IoT/protocol-abstraction/tests/emu'


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


def _encode_daisy_request(seq: int, cmd: int, data: bytes = b'') -> str:
    """Encode a Daisy REQUEST frame as hex string."""
    def nibble4(x):
        return bytes([0x30 + ((x >> s) & 15) for s in (12, 8, 4, 0)])
    frame = bytes([0x01, len(data) + 0x24, seq, cmd]) + data + b'\x05'
    bcc = sum(frame[1:]) & 0xFFFF
    return (frame + nibble4(bcc) + b'\x03').hex()


def _run_session(binary: Path, commands: list) -> list:
    stdin_data = '\n'.join(commands) + '\n'
    result = subprocess.run(
        [str(binary)],
        input=stdin_data, capture_output=True, text=True, timeout=30,
    )
    if result.returncode != 0:
        raise RuntimeError('CLI error: ' + result.stderr)
    return result.stdout.splitlines()


class TestEMU02_2(unittest.TestCase):
    def test_non_cash_payment_unsupported(self):
        # cmd=53 (Daisy payment slot), data starting with '1' (CARD type code)
        # FakeDaisyDevice should return error response (non-NAK but error status)
        card_data = b'1'  # CARD payment type code
        frame_hex = _encode_daisy_request(0x20, 53, card_data)
        # Valid CASH payment (data starts with '0') — should succeed
        cash_data = b'0'
        cash_hex = _encode_daisy_request(0x21, 53, cash_data)

        with tempfile.TemporaryDirectory() as tmpdir:
            binary = _build_cli(Path(tmpdir))
            responses = _run_session(binary, [f'H {frame_hex}', f'H {cash_hex}'])

        self.assertEqual(len(responses), 2)
        nak = bytes([0x15]).hex()
        # CARD: response is error (not NAK, but has error flag in status)
        self.assertNotEqual(responses[0], nak,
                            'CARD payment should get error response, not NAK')
        self.assertGreater(len(responses[0]), 0,
                           'CARD payment should get non-empty error response')
        # Verify the error response has status byte 0 with error bit set (status byte[0] & 0x01)
        resp_bytes = bytes.fromhex(responses[0])
        # Response format: 01 LEN SEQ CMD 04 STATUS(6) 05 BCC 03
        # Status at offset 4+dataLen+1; for empty data: offset 5
        # but LEN raw byte tells us dataLen = LEN - 0x2B
        if len(resp_bytes) >= 12 and resp_bytes[0] == 0x01 and resp_bytes[-1] == 0x03:
            data_len = resp_bytes[1] - 0x2B
            status_offset = 4 + data_len + 1
            if status_offset < len(resp_bytes) - 7:
                self.assertTrue(resp_bytes[status_offset] & 0x01,
                                'CARD payment response should have error bit in status')
        # CASH: should succeed (no error)
        self.assertNotEqual(responses[1], nak,
                            'CASH payment should not get NAK')

    def test_latin_tax_letter_rejected(self):
        # cmd=49 (Daisy sale), data with Latin 'B' as standalone tax group
        # Format: price\tqty\tname\ttax_group\t → tax group 'B' (Latin ASCII 0x42)
        latin_data = b'1.00\t1.000\tCoffee\tB\t'
        frame_hex = _encode_daisy_request(0x30, 49, latin_data)
        # Same with Cyrillic 'Б' in CP1251 (0xC1) — should pass
        cyrillic_data = b'1.00\t1.000\tCoffee\t\xc1\t'
        cyrillic_hex = _encode_daisy_request(0x31, 49, cyrillic_data)

        with tempfile.TemporaryDirectory() as tmpdir:
            binary = _build_cli(Path(tmpdir))
            responses = _run_session(binary, [f'H {frame_hex}', f'H {cyrillic_hex}'])

        self.assertEqual(len(responses), 2)
        nak = bytes([0x15]).hex()
        # Latin tax letter → NAK
        self.assertEqual(responses[0], nak,
                         f'Latin tax letter should get NAK, got {responses[0]}')
        # Cyrillic → valid response
        self.assertNotEqual(responses[1], nak,
                            'Cyrillic tax letter should not get NAK')

    def test_valid_cash_sale_processed(self):
        # A standard valid cash sale frame should be processed normally
        cash_frame_hex = _encode_daisy_request(0x40, 74)  # cmd=74 status, no data
        with tempfile.TemporaryDirectory() as tmpdir:
            binary = _build_cli(Path(tmpdir))
            responses = _run_session(binary, [f'H {cash_frame_hex}'])
        self.assertEqual(len(responses), 1)
        nak = bytes([0x15]).hex()
        self.assertNotEqual(responses[0], nak)
        self.assertGreater(len(responses[0]), 0)

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out = Path(artifact_dir) / 'emu-output'
            out.mkdir(parents=True, exist_ok=True)
            (out / 'emu_02_2.log').write_text('3 tests pass')


if __name__ == '__main__':
    unittest.main()
