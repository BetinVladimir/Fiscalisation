#!/usr/bin/env python3
import copy
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import sys
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / "verify-integration-ci-topology.py"
spec = importlib.util.spec_from_file_location("topology", SCRIPT)
topology = importlib.util.module_from_spec(spec)
spec.loader.exec_module(topology)


def fixture():
    socket = {"type": "volume", "source": "fiscal_db_socket", "target": "/var/run/postgresql"}
    return {
        "networks": {"dbroute": {"internal": True}, "private": {}, "ingress": {}},
        "services": {
            "postgres": {
                "networks": {"dbroute": {}, "private": {}},
                "ports": [{"host_ip": "127.0.0.1", "target": 5432, "published": "5433", "protocol": "tcp"}],
                "volumes": [copy.deepcopy(socket), {"type": "bind", "target": "/docker-entrypoint-initdb.d", "read_only": True}],
            },
            "caddy": {
                "networks": {"dbroute": {}, "ingress": {}},
                "ports": [{"host_ip": "127.0.0.1", "target": 80, "published": "8080", "protocol": "tcp"}],
            },
            "fiscal-backend": {
                "networks": {"dbroute": {}}, "volumes": [copy.deepcopy(socket)],
                "environment": {"DATABASE_URL": "postgres://fixture-secret@/fiscal?host=/var/run/postgresql&sslmode=disable", "RLS_DATABASE_URL": "postgres://fixture-reader-secret@/fiscal?host=/var/run/postgresql&sslmode=disable"},
            },
            **{name: {"networks": {"dbroute": {}}} for name in ("redis", "emqx", "rabbitmq")},
        },
    }


class IntegrationTopologyTest(unittest.TestCase):
    def assert_rejected(self, mutate):
        model = fixture()
        mutate(model)
        with self.assertRaises(topology.TopologyError):
            topology.validate(model)

    def test_expected_host_entrypoints_keep_internal_backend(self):
        topology.validate(fixture())

    def test_original_internal_only_topology_rejected_despite_ports(self):
        def original(m):
            m["services"]["caddy"]["networks"] = {"dbroute": {}}
            m["services"]["postgres"]["networks"] = {"dbroute": {}}
        self.assert_rejected(original)

    def test_backend_must_not_join_host_routable_network(self):
        self.assert_rejected(lambda m: m["services"]["fiscal-backend"]["networks"].update({"private": {}}))

    def test_dbroute_cannot_be_made_external_to_fix_ingress(self):
        self.assert_rejected(lambda m: m["networks"]["dbroute"].update({"internal": False}))

    def test_host_bridge_must_not_be_internal(self):
        for name in ("private", "ingress"):
            with self.subTest(name=name):
                self.assert_rejected(lambda m: m["networks"][name].update({"internal": True}))

    def test_host_ports_cannot_bind_all_interfaces(self):
        for name in ("postgres", "caddy"):
            with self.subTest(name=name):
                self.assert_rejected(lambda m: m["services"][name]["ports"][0].update({"host_ip": "0.0.0.0"}))

    def test_unexpected_extra_or_wrong_port_rejected(self):
        self.assert_rejected(lambda m: m["services"]["caddy"]["ports"].append({"target": 443, "published": "8443"}))
        self.assert_rejected(lambda m: m["services"]["postgres"]["ports"][0].update({"published": "5432"}))
        self.assert_rejected(lambda m: m["services"]["fiscal-backend"].update({"ports": [{"target": 8080}]}))

    def test_private_services_cannot_join_ingress(self):
        for name in ("postgres", "redis", "emqx", "rabbitmq"):
            with self.subTest(name=name):
                self.assert_rejected(lambda m: m["services"][name]["networks"].update({"ingress": {}}))

    def test_tcp_db_fallback_or_missing_socket_rejected(self):
        self.assert_rejected(lambda m: m["services"]["fiscal-backend"]["environment"].update({"DATABASE_URL": "postgres://postgres/fiscal"}))
        self.assert_rejected(lambda m: m["services"]["fiscal-backend"].update({"volumes": []}))

    def test_canonical_fixture_cannot_be_removed_or_writable(self):
        self.assert_rejected(lambda m: m["services"]["postgres"]["volumes"][-1].update({"read_only": False}))

    def test_cli_rejects_without_echoing_credentials(self):
        model = fixture()
        model["services"]["caddy"]["networks"] = {"dbroute": {}}
        process = subprocess.run([sys.executable, str(SCRIPT)], input=json.dumps(model), capture_output=True, text=True, timeout=10)
        self.assertEqual(process.returncode, 1)
        self.assertNotIn("fixture-secret", process.stdout + process.stderr)
        self.assertNotIn("fixture-reader-secret", process.stdout + process.stderr)
        self.assertNotIn("postgres://", process.stdout + process.stderr)


if __name__ == "__main__":
    unittest.main()
