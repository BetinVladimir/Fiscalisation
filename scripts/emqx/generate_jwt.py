#!/usr/bin/env python3
"""Generate EMQX-compatible HS256 JWT with ACL claim.

Usage:
  python scripts/emqx/generate_jwt.py \
    --secret "$EMQX_JWT_SECRET" \
    --username device-001 \
    --pub devices/device-001/telemetry \
    --sub devices/device-001/commands/#
"""

from __future__ import annotations

import argparse
import base64
import hashlib
import hmac
import json
import time


def b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def sign_hs256(message: bytes, secret: str) -> str:
    digest = hmac.new(secret.encode("utf-8"), message, hashlib.sha256).digest()
    return b64url(digest)


def build_token(secret: str, username: str, pub_topics: list[str], sub_topics: list[str], ttl_sec: int) -> str:
    now = int(time.time())
    header = {"alg": "HS256", "typ": "JWT"}

    acl: list[dict[str, str]] = []
    for topic in pub_topics:
        acl.append({"permission": "allow", "action": "publish", "topic": topic})
    for topic in sub_topics:
        acl.append({"permission": "allow", "action": "subscribe", "topic": topic})

    payload = {
        "username": username,
        "iat": now,
        "exp": now + ttl_sec,
        "acl": acl,
    }

    encoded_header = b64url(json.dumps(header, separators=(",", ":")).encode("utf-8"))
    encoded_payload = b64url(json.dumps(payload, separators=(",", ":")).encode("utf-8"))

    signing_input = f"{encoded_header}.{encoded_payload}".encode("ascii")
    signature = sign_hs256(signing_input, secret)
    return f"{encoded_header}.{encoded_payload}.{signature}"


def main() -> None:
    parser = argparse.ArgumentParser(description="Generate JWT for EMQX auth + ACL")
    parser.add_argument("--secret", required=True, help="HS256 secret, must match EMQX_JWT_SECRET")
    parser.add_argument("--username", required=True, help="MQTT client username")
    parser.add_argument("--pub", action="append", default=[], help="Publish topic (repeatable)")
    parser.add_argument("--sub", action="append", default=[], help="Subscribe topic (repeatable)")
    parser.add_argument("--ttl", type=int, default=3600, help="Token TTL in seconds")
    args = parser.parse_args()

    token = build_token(args.secret, args.username, args.pub, args.sub, args.ttl)
    print(token)


if __name__ == "__main__":
    main()
