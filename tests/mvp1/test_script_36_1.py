"""SCRIPT-36.1: make contract-lock target calls verify-contract-lock.sh"""
import subprocess, os, unittest, re
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript361(unittest.TestCase):
    def test_contract_lock_target_defined(self):
        makefile = open(os.path.join(ROOT, "Makefile")).read()
        self.assertIsNotNone(re.search(r'^contract-lock:', makefile, re.MULTILINE),
                             "Makefile must define 'contract-lock:' target")

    def test_contract_lock_calls_script(self):
        makefile = open(os.path.join(ROOT, "Makefile")).read()
        self.assertIn("verify-contract-lock.sh", makefile,
                      "contract-lock must call verify-contract-lock.sh")

    def test_phony_includes_contract_lock(self):
        makefile = open(os.path.join(ROOT, "Makefile")).read()
        self.assertIsNotNone(re.search(r'\.PHONY:.*\bcontract-lock\b', makefile),
                             "contract-lock must be in .PHONY")

    def test_verify_contract_lock_script_exists(self):
        self.assertTrue(
            os.path.exists(os.path.join(ROOT, "scripts", "verify-contract-lock.sh")),
            "verify-contract-lock.sh must exist"
        )

if __name__ == "__main__": unittest.main()
