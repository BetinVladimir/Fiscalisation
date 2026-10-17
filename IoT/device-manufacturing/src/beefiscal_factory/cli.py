from __future__ import annotations
import argparse, json, os, subprocess, tempfile
from pathlib import Path
from .evidence import Evidence, now, sha256_file
from .flash_manifest import load_manifest

def flash(args: argparse.Namespace) -> None:
    try:
        segments = load_manifest(args.manifest, args.expected_manifest_sha256)
    except (ValueError, OSError, TypeError, KeyError) as error:
        raise SystemExit(str(error)) from error
    # Execute against the exact verified bytes, not mutable original files.
    with tempfile.TemporaryDirectory(prefix="beefiscal-flash-") as directory:
        command = ["esptool.py", "--chip", "esp32s3", "--port", args.port, "write_flash"]
        for index, (kind, offset, data) in enumerate(segments):
            artifact = Path(directory) / f"{index}-{kind}.bin"
            artifact.write_bytes(data)
            command.extend([hex(offset), str(artifact)])
        subprocess.run(command, check=True)

def bundle(args: argparse.Namespace) -> None:
    from .bundle import create_bundle
    try:
        manifest, digest = create_bundle(args.output, args.bootloader, args.partition_table,
            args.application, args.flash_size_bytes, args.table_offset, args.app_label)
    except (ValueError, OSError, TypeError, KeyError) as error:
        raise SystemExit(str(error)) from error
    print(json.dumps({"manifest": str(manifest), "manifest_sha256": digest}, sort_keys=True))

def register(args: argparse.Namespace) -> None:
    from .registration_client import RegistrationClient
    payload = json.loads(Path(args.evidence).read_text(encoding="utf-8"))
    payload["proof"] = args.proof
    result = RegistrationClient(args.backend, os.environ.get("BEEFISCAL_FACTORY_OIDC_TOKEN", "")).register(payload)
    print(json.dumps(result, indent=2))

def main() -> None:
    parser=argparse.ArgumentParser(prog="beefiscal-factory")
    sub=parser.add_subparsers(required=True)
    f=sub.add_parser("flash");f.add_argument("--port",required=True);f.add_argument("--manifest",required=True);f.add_argument("--expected-manifest-sha256",required=True);f.set_defaults(run=flash)
    r=sub.add_parser("register");r.add_argument("--backend",required=True);r.add_argument("--evidence",required=True);r.add_argument("--proof",required=True);r.set_defaults(run=register)
    b=sub.add_parser("bundle");b.add_argument("--output",required=True);b.add_argument("--bootloader",required=True);b.add_argument("--partition-table",required=True);b.add_argument("--application",required=True);b.add_argument("--flash-size-bytes",required=True,type=int);b.add_argument("--table-offset",required=True,type=lambda value:int(value,0));b.add_argument("--app-label",required=True);b.set_defaults(run=bundle)
    args=parser.parse_args();args.run(args)

if __name__ == "__main__": main()
