from __future__ import annotations
import argparse, json, os, subprocess
from pathlib import Path
from .evidence import Evidence, now, sha256_file
from .registration_client import RegistrationClient

def flash(args: argparse.Namespace) -> None:
    firmware = Path(args.firmware).resolve()
    digest = sha256_file(firmware)
    if args.expected_sha256.lower() != digest:
        raise SystemExit("firmware SHA-256 mismatch")
    command = ["esptool.py", "--chip", "esp32s3", "--port", args.port, "write_flash", "0x0", str(firmware)]
    subprocess.run(command, check=True)

def register(args: argparse.Namespace) -> None:
    payload = json.loads(Path(args.evidence).read_text(encoding="utf-8"))
    payload["proof"] = args.proof
    result = RegistrationClient(args.backend, os.environ.get("BEEFISCAL_FACTORY_OIDC_TOKEN", "")).register(payload)
    print(json.dumps(result, indent=2))

def main() -> None:
    parser=argparse.ArgumentParser(prog="beefiscal-factory")
    sub=parser.add_subparsers(required=True)
    f=sub.add_parser("flash");f.add_argument("--port",required=True);f.add_argument("--firmware",required=True);f.add_argument("--expected-sha256",required=True);f.set_defaults(run=flash)
    r=sub.add_parser("register");r.add_argument("--backend",required=True);r.add_argument("--evidence",required=True);r.add_argument("--proof",required=True);r.set_defaults(run=register)
    args=parser.parse_args();args.run(args)

if __name__ == "__main__": main()
