"""SUPTO-01.3 — LoadSutpoProfiles rejects invalid fixture (missing id / empty audit_events)."""
import subprocess, json, tempfile, os, unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BACKEND = ROOT / 'fiscal-backend'

BAD_FIXTURE_MISSING_ID = {"profiles": [{"id": "", "name": "X", "disabled_operator": False,
    "read_only_auditor": True, "audit_events": ["storno"], "unsupported": ["x"], "unsupported_reason": "y"}]}

BAD_FIXTURE_EMPTY_EVENTS = {"profiles": [{"id": "T1", "name": "T", "disabled_operator": False,
    "read_only_auditor": True, "audit_events": [], "unsupported": ["x"], "unsupported_reason": "y"}]}


class TestSupto01_3(unittest.TestCase):
    def _write_fixture(self, data):
        f = tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False)
        json.dump(data, f)
        f.close()
        return f.name

    def _run_inline(self, fixture_path):
        prog = f"""
package main
import (
    "fmt"
    "os"
    "fiscalisation/fiscal-backend/internal/domain"
)
func main() {{
    profiles, err := domain.LoadSutpoProfiles("{fixture_path}")
    if err != nil {{
        fmt.Fprintln(os.Stderr, "ERROR:", err)
        os.Exit(1)
    }}
    fmt.Println("OK", len(profiles))
}}
"""
        tmp = tempfile.mkdtemp()
        main_go = os.path.join(tmp, 'main.go')
        with open(main_go, 'w') as f:
            f.write(prog)
        r = subprocess.run(['go', 'run', main_go], cwd=str(BACKEND),
            capture_output=True, text=True, timeout=30)
        return r

    def test_rejects_missing_id(self):
        path = self._write_fixture(BAD_FIXTURE_MISSING_ID)
        try:
            r = self._run_inline(path)
            self.assertNotEqual(r.returncode, 0,
                f'expected failure for missing id but got rc=0\n{r.stdout}')
        finally:
            os.unlink(path)

    def test_rejects_empty_audit_events(self):
        path = self._write_fixture(BAD_FIXTURE_EMPTY_EVENTS)
        try:
            r = self._run_inline(path)
            self.assertNotEqual(r.returncode, 0,
                f'expected failure for empty audit_events but got rc=0\n{r.stdout}')
        finally:
            os.unlink(path)


if __name__ == '__main__':
    unittest.main()
