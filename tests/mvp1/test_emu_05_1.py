"""EMU-05.1 — FakeBroker: ARCH_19 transport S1,S4,S5,S6,S7 green."""
import os
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BACKEND = ROOT / 'fiscal-backend'


from native_harness import check_native_tests


class TestEMU05_1(unittest.TestCase):
    def test_fakebroker_transport_scenarios(self):
        check_native_tests(
            self,
            argv=['go', 'test', '-json', '-run', 'TestFakeBroker',
                  './internal/mqttclient/...'],
            root=BACKEND,
            cwd='.',
            required_tests=[
                'TestFakeBrokerS1_BasicPubSub',
                'TestFakeBrokerS4_OfflineQueue',
                'TestFakeBrokerS5_QoS1DupDelivery',
                'TestFakeBrokerS6_LWT',
                'TestFakeBrokerS7_TopicACL',
            ],
            minimum=5,
            timeout=120,
        )


if __name__ == '__main__':
    unittest.main()
