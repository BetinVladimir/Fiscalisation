"""Create a self-contained, validated flash bundle without touching a device."""
from __future__ import annotations
import hashlib
import json
import tempfile
from pathlib import Path
from .flash_manifest import app_partition, load_manifest, require


def create_bundle(output, bootloader, partition_table, application, flash_size_bytes,
                  table_offset, app_label):
    output = Path(output).absolute()
    require(not output.exists(), "bundle output already exists")
    # Read once: validation, hashes and copies all refer to the same bytes.
    artifacts = [("bootloader", 0, Path(bootloader).read_bytes()),
                 ("partition_table", table_offset, Path(partition_table).read_bytes())]
    app = Path(application).read_bytes()
    app_offset, _ = app_partition(artifacts[1][2], app_label, flash_size_bytes, table_offset)
    artifacts.append(("application", app_offset, app))
    manifest = {"schema_version": 1, "chip": "esp32s3", "mode": "segments",
                "flash_size_bytes": flash_size_bytes, "segments": []}
    for kind, offset, data in artifacts:
        record = {"kind": kind, "offset": offset, "path": kind + ".bin",
                  "sha256": hashlib.sha256(data).hexdigest()}
        manifest["segments"].append(record)
        if kind == "partition_table":
            manifest["partition_layout"] = {"path": record["path"], "sha256": record["sha256"],
                                            "table_offset": table_offset, "app_label": app_label}
    raw = (json.dumps(manifest, sort_keys=True, indent=2) + "\n").encode()
    digest = hashlib.sha256(raw).hexdigest()
    # Validate the completed bundle with the exact same validator as flash.
    # Temporary bytes stay on the destination filesystem to avoid cross-filesystem moves.
    with tempfile.TemporaryDirectory(prefix=".beefiscal-bundle-", dir=output.parent) as temporary:
        staged = Path(temporary) / "bundle"
        staged.mkdir()
        for kind, _, data in artifacts:
            (staged / (kind + ".bin")).write_bytes(data)
        path = staged / "manifest.json"
        path.write_bytes(raw)
        load_manifest(path, digest)
        (staged / "manifest.sha256").write_text(digest + "  manifest.json\n", encoding="ascii")
        # mkdir is exclusive; an existing destination is never overwritten.
        output.mkdir()
        try:
            for artifact in staged.iterdir():
                artifact.rename(output / artifact.name)
        except BaseException:
            # Keep partial output for inspection; flash requires all hashes.
            raise
    return output / "manifest.json", digest
