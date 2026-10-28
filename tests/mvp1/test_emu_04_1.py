"""EMU-04.1 — FakeBluePad: 4 modes covered, purchase(ref) idempotent (realCharges==1)."""
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
EMU_DIR = ROOT / 'IoT/firmware/edge-agent-s3/idf/tests/bluepad_host'

_TEST_CPP = r"""
#include "FakeBluePad.hpp"
#include <cassert>
#include <iostream>
using namespace bee;

int main() {
    // APPROVE mode: result APPROVED, idempotent (realCharges==1 on repeat)
    {
        FakeBluePad pad(BluePadMode::APPROVE);
        assert(pad.purchase("ref001") == BluePadResult::APPROVED);
        assert(pad.purchase("ref001") == BluePadResult::APPROVED);
        assert(pad.real_charges() == 1);
        std::cout << "APPROVE mode: PASS (realCharges=" << pad.real_charges() << ")\n";
    }
    // DECLINE mode: result DECLINED, idempotent
    {
        FakeBluePad pad(BluePadMode::DECLINE);
        assert(pad.purchase("ref002") == BluePadResult::DECLINED);
        assert(pad.purchase("ref002") == BluePadResult::DECLINED);
        assert(pad.real_charges() == 1);
        std::cout << "DECLINE mode: PASS (realCharges=" << pad.real_charges() << ")\n";
    }
    // TIMEOUT_AFTER_SEND mode: result TIMEOUT, idempotent
    {
        FakeBluePad pad(BluePadMode::TIMEOUT_AFTER_SEND);
        assert(pad.purchase("ref003") == BluePadResult::TIMEOUT);
        assert(pad.purchase("ref003") == BluePadResult::TIMEOUT);
        assert(pad.real_charges() == 1);
        std::cout << "TIMEOUT_AFTER_SEND mode: PASS (realCharges=" << pad.real_charges() << ")\n";
    }
    // BLE_DISCONNECT mode: result DISCONNECTED, idempotent
    {
        FakeBluePad pad(BluePadMode::BLE_DISCONNECT);
        assert(pad.purchase("ref004") == BluePadResult::DISCONNECTED);
        assert(pad.purchase("ref004") == BluePadResult::DISCONNECTED);
        assert(pad.real_charges() == 1);
        std::cout << "BLE_DISCONNECT mode: PASS (realCharges=" << pad.real_charges() << ")\n";
    }
    // Different refs are independent charges
    {
        FakeBluePad pad(BluePadMode::APPROVE);
        pad.purchase("refA");
        pad.purchase("refB");
        assert(pad.real_charges() == 2);
        std::cout << "independent refs: PASS (realCharges=" << pad.real_charges() << ")\n";
    }
    std::cout << "FakeBluePad: 4 modes PASS\n";
    return 0;
}
"""


class TestEMU04_1(unittest.TestCase):
    def test_bluepad_four_modes_idempotent(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            tmp = Path(tmpdir)
            src = tmp / 'test_bluepad.cpp'
            src.write_text(_TEST_CPP)
            out = tmp / 'test_bluepad'
            compile_result = subprocess.run(
                ['c++', '-std=c++17', '-Wall', '-Wextra', '-Werror',
                 '-I', str(EMU_DIR),
                 str(src), '-o', str(out)],
                capture_output=True, text=True, timeout=60,
            )
            if compile_result.returncode != 0:
                self.fail('Compile failed:\n' + compile_result.stderr)

            run_result = subprocess.run(
                [str(out)], capture_output=True, text=True, timeout=30,
            )
            self.assertEqual(
                run_result.returncode, 0,
                'Test binary failed:\n' + run_result.stdout + run_result.stderr,
            )
            output = run_result.stdout
            self.assertIn('APPROVE mode: PASS', output)
            self.assertIn('DECLINE mode: PASS', output)
            self.assertIn('TIMEOUT_AFTER_SEND mode: PASS', output)
            self.assertIn('BLE_DISCONNECT mode: PASS', output)
            self.assertIn('realCharges=1', output)
            self.assertIn('FakeBluePad: 4 modes PASS', output)

            artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
            if artifact_dir:
                out_dir = Path(artifact_dir) / 'emu-04-evidence'
                out_dir.mkdir(parents=True, exist_ok=True)
                (out_dir / 'emu_04_1.log').write_text(output)


if __name__ == '__main__':
    unittest.main()
