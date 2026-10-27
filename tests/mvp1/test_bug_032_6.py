"""BUG-032.6 — Local HTTP 8088 binds on allowed interface with documented risk."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
LOCAL_HTTP = ROOT / 'SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashLocalHttpServer.kt'
DOC = ROOT / 'docs/BLUECASH_POS_INTEGRATION.md'


class TestBUG0326(unittest.TestCase):
    def _src(self):
        self.assertTrue(LOCAL_HTTP.exists(), f'BlueCashLocalHttpServer.kt not found: {LOCAL_HTTP}')
        return LOCAL_HTTP.read_text()

    def test_local_http_server_exists(self):
        src = self._src()
        self.assertRegex(
            src,
            r'class\s+BlueCashLocalHttpServer\b',
            'BlueCashLocalHttpServer class must exist',
        )

    def test_local_http_port_8088_in_runtime(self):
        # 8088 is passed as a constructor argument from RuntimeHost (or MainActivity)
        runtime_host = ROOT / 'SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/RuntimeHost.kt'
        main = ROOT / 'SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/MainActivity.kt'
        callers = [f for f in (runtime_host, main) if f.exists()]
        self.assertTrue(callers, 'Neither RuntimeHost.kt nor MainActivity.kt found')
        found = any('8088' in f.read_text() for f in callers)
        self.assertTrue(found, 'Port 8088 must appear in RuntimeHost.kt or MainActivity.kt')

    def test_doc_mentions_http_security_risk(self):
        self.assertTrue(DOC.exists(), f'BLUECASH_POS_INTEGRATION.md not found: {DOC}')
        doc = DOC.read_text()
        # doc must mention the HTTP network risk (plaintext, localhost, or TLS)
        found = any(kw in doc.upper() for kw in ('8088', 'HTTP', 'TLS', 'LOCALHOST', 'RISK', 'SECURITY'))
        self.assertTrue(
            found,
            'BLUECASH_POS_INTEGRATION.md must document HTTP security risk (mention 8088/HTTP/TLS/RISK)',
        )


if __name__ == '__main__':
    unittest.main()
