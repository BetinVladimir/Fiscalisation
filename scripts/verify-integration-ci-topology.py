#!/usr/bin/env python3
"""Validate rendered disposable integration Compose JSON; never print its env."""
import json
import sys


class TopologyError(ValueError):
    pass


def validate(config):
    services = config.get("services", {})
    networks = config.get("networks", {})
    if networks.get("dbroute", {}).get("internal") is not True:
        raise TopologyError("dbroute must remain internal")
    for name in ("private", "ingress"):
        if name not in networks or networks[name].get("internal", False):
            raise TopologyError(name + " must be a non-internal host-entrypoint network")
    expected = {
        "postgres": {"dbroute", "private"},
        "caddy": {"dbroute", "ingress"},
        "fiscal-backend": {"dbroute"},
        "redis": {"dbroute"},
        "emqx": {"dbroute"},
        "rabbitmq": {"dbroute"},
    }
    for name, allowed in expected.items():
        if set(services.get(name, {}).get("networks", {})) != allowed:
            raise TopologyError(name + " network boundary drift")
    for service, target, published in (("caddy", 80, "8080"), ("postgres", 5432, "5433")):
        ports = services[service].get("ports", [])
        if len(ports) != 1 or any(
            port.get("host_ip") != "127.0.0.1"
            or port.get("target") != target
            or str(port.get("published")) != published
            or port.get("protocol", "tcp") != "tcp"
            for port in ports
        ):
            raise TopologyError(service + " must publish only its exact loopback CI endpoint")
    backend = services["fiscal-backend"]
    if backend.get("ports"):
        raise TopologyError("backend must not publish a host port")
    env = backend.get("environment", {})
    for key in ("DATABASE_URL", "RLS_DATABASE_URL"):
        if "host=/var/run/postgresql" not in env.get(key, ""):
            raise TopologyError("backend must keep the isolated PostgreSQL Unix socket")
    for service in ("postgres", "fiscal-backend"):
        if not any(
            volume.get("type") == "volume"
            and volume.get("source") == "fiscal_db_socket"
            and volume.get("target") == "/var/run/postgresql"
            for volume in services[service].get("volumes", [])
        ):
            raise TopologyError(service + " must share the disposable DB socket volume")
    if not any(
        volume.get("type") == "bind"
        and volume.get("target") == "/docker-entrypoint-initdb.d"
        and volume.get("read_only") is True
        for volume in services["postgres"].get("volumes", [])
    ):
        raise TopologyError("postgres must retain read-only canonical fixture initialization")


def main():
    try:
        validate(json.load(sys.stdin))
    except (ValueError, TypeError, AttributeError, KeyError):
        # Do not echo the rendered model: it contains environment credentials.
        print("integration CI topology invalid; check loopback entrypoints, network boundaries and DB socket fixture", file=sys.stderr)
        return 1
    print("integration CI topology OK: loopback HTTP/PG entrypoints; internal backend and DB socket retained")
    return 0


if __name__ == "__main__":
    sys.exit(main())
