"""VG-17.2: Latin A (0x41) ≠ Cyrillic А (0xC0) — TAX_GROUP_LATIN rejection.

buildDaisySaleItem with taxGroup='A' must produce byte 0xC0 (CP1251 А), NOT 0x41 (Latin A).
FakeDaisyDevice rejects sale frames containing a standalone Latin ASCII uppercase letter
as the tax group byte (TAX_GROUP_LATIN → NAK 0x15).
"""
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
HERE = ROOT / 'IoT/protocol-abstraction'
COMMON = ROOT / 'IoT/common-modules'

ARTIFACT_DIR = os.environ.get('MVP1_ARTIFACT_DIR')

# Small C++ program that confirms the Latin→CP1251 conversion and rejects raw Latin bytes.
_CPP_ORACLE = r"""
#include "CommandPayload.hpp"
#include "FrameCodec.hpp"
#include <cassert>
#include <iostream>
#include <vector>
using namespace bee;

int main() {
    SaleItemPayload item{};
    item.name = "Test";
    item.unitPrice = "1.00";
    item.quantity = "1.000";
    std::vector<uint8_t> payload;
    std::string error;

    // Tax group 'A' must become CP1251 0xC0, not ASCII 0x41
    item.taxGroup = 'A';
    assert(buildDaisySaleItem(item, payload, error));
    // "Test\t" = 5 bytes; tax byte at index 5
    assert(payload.size() > 5);
    assert(payload[5] == 0xC0); // CP1251 Cyrillic А
    assert(payload[5] != 0x41); // must NOT be Latin A
    std::cout << "TAX_GROUP_LATIN_REJECTED: tax_A_encodes_as_0xC0=pass\n";

    // Tax group 'H' must become CP1251 0xC7, not ASCII 0x48
    item.taxGroup = 'H';
    assert(buildDaisySaleItem(item, payload, error));
    assert(payload.size() > 5);
    assert(payload[5] == 0xC7); // CP1251 Cyrillic З
    assert(payload[5] != 0x48); // must NOT be Latin H
    std::cout << "TAX_GROUP_LATIN_REJECTED: tax_H_encodes_as_0xC7=pass\n";

    // Raw CP1251 byte 0xC0 passed as taxGroup must be REJECTED (outside 'A'..'H' range)
    item.taxGroup = (char)0xC0;
    assert(!buildDaisySaleItem(item, payload, error));
    assert(error == "DAISY_ITEM_FIELDS");
    std::cout << "TAX_GROUP_LATIN_REJECTED: raw_CP1251_0xC0_rejected_by_api=pass\n";

    std::cout << "TAX_GROUP_LATIN_REJECTED: all_pass\n";
    return 0;
}
"""


class TestVG17_2(unittest.TestCase):

    def _build_and_run(self, tmpdir: str) -> str:
        """Compile and run the oracle, returning combined stdout+stderr."""
        src = Path(tmpdir) / 'oracle.cpp'
        out = Path(tmpdir) / 'oracle'
        src.write_text(_CPP_ORACLE)
        compile_result = subprocess.run(
            ['c++', '-std=c++17', '-Wall', '-Wextra', '-Werror',
             f'-I{HERE / "tests"}',
             f'-I{HERE / "include"}',
             f'-I{HERE / "src"}',
             f'-I{COMMON / "daisy"}',
             str(HERE / 'src/CommandPayload.cpp'),
             str(HERE / 'src/FrameCodec.cpp'),
             str(src),
             '-o', str(out)],
            capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(compile_result.returncode, 0,
                         f'Compile failed:\n{compile_result.stderr}')
        run_result = subprocess.run(
            [str(out)],
            capture_output=True, text=True, timeout=30,
        )
        self.assertEqual(run_result.returncode, 0,
                         f'Oracle binary failed:\n{run_result.stderr}')
        return run_result.stdout + run_result.stderr

    def test_latin_a_encodes_as_cp1251_not_ascii(self):
        """taxGroup='A' must produce 0xC0 in the wire payload (not 0x41)."""
        with tempfile.TemporaryDirectory() as tmpdir:
            output = self._build_and_run(tmpdir)
        self.assertIn('tax_A_encodes_as_0xC0=pass', output,
                      f'CP1251 encoding check failed; output:\n{output}')

    def test_latin_h_encodes_as_cp1251_not_ascii(self):
        """taxGroup='H' must produce 0xC7 in the wire payload (not 0x48)."""
        with tempfile.TemporaryDirectory() as tmpdir:
            output = self._build_and_run(tmpdir)
        self.assertIn('tax_H_encodes_as_0xC7=pass', output,
                      f'CP1251 encoding check failed; output:\n{output}')

    def test_raw_cp1251_byte_rejected_by_api(self):
        """Passing raw 0xC0 as taxGroup must be rejected with DAISY_ITEM_FIELDS."""
        with tempfile.TemporaryDirectory() as tmpdir:
            output = self._build_and_run(tmpdir)
        self.assertIn('raw_CP1251_0xC0_rejected_by_api=pass', output,
                      f'API rejection check failed; output:\n{output}')

    def test_fake_daisy_device_has_tax_group_latin_check(self):
        """FakeDaisyDevice.hpp must implement isLatinTaxLetter guard (TAX_GROUP_LATIN)."""
        fake_device = HERE / 'tests/emu/FakeDaisyDevice.hpp'
        self.assertTrue(fake_device.exists(), f'FakeDaisyDevice.hpp not found: {fake_device}')
        content = fake_device.read_text()
        self.assertIn('isLatinTaxLetter', content,
                      'FakeDaisyDevice must have isLatinTaxLetter guard')
        self.assertIn('TAX_GROUP_LATIN', content,
                      'FakeDaisyDevice must reference TAX_GROUP_LATIN')
        # Latin ASCII uppercase range 0x41..0x5A
        self.assertIn('0x41', content, 'Guard must check byte >= 0x41')
        self.assertIn('0x5A', content, 'Guard must check byte <= 0x5A')


if __name__ == '__main__':
    import sys
    result = unittest.main(exit=False, verbosity=2)
    if ARTIFACT_DIR:
        out = Path(ARTIFACT_DIR)
        out.mkdir(parents=True, exist_ok=True)
        (out / 'vg_17_2.log').write_text(
            f'ran={result.result.testsRun} failures={len(result.result.failures)} '
            f'errors={len(result.result.errors)}'
        )
    sys.exit(0 if result.result.wasSuccessful() else 1)
