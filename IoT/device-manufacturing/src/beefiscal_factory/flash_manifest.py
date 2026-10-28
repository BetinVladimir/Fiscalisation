"""Fail-closed ESP32-S3 flash layout validation; no hardware operations."""
from __future__ import annotations
import hashlib
import json
import struct
from pathlib import Path


def require(condition, message):
    if not condition:
        raise ValueError(message)


def integer(value, name):
    require(type(value) is int and value >= 0, f"invalid {name}")
    return value


def verified_file(base, record):
    require(isinstance(record, dict), "invalid artifact record")
    name = record.get("path")
    require(isinstance(name, str) and bool(name), "artifact path required")
    path = (base / name).resolve()
    data = path.read_bytes()
    require(bool(data), "empty artifact")
    require(record.get("sha256") == hashlib.sha256(data).hexdigest(), "artifact SHA-256 mismatch")
    return data


def app_partition(table, label, flash_size, table_offset):
    require(isinstance(label, str) and bool(label), "app_label required")
    require(32 <= len(table) <= 0xC00 and len(table) % 32 == 0, "invalid partition table size")
    apps = []
    ranges = []
    names = set()
    otadata_count = 0
    for pos in range(0, len(table), 32):
        entry = table[pos:pos + 32]
        require(len(entry) == 32, "truncated partition table")
        if entry == b"\xff" * 32:
            require(all(b == 255 for b in table[pos:]), "data after partition terminator")
            break
        if entry[:2] == b"\xeb\xeb":
            require(entry[2:16] == b"\xff" * 14, "invalid partition checksum record")
            require(entry[16:32] == hashlib.md5(table[:pos]).digest(), "partition table MD5 mismatch")
            require(all(b == 255 for b in table[pos + 32:]), "data after partition checksum")
            break
        magic, kind, subtype, offset, size, raw_label, flags = struct.unpack("<HBBII16sI", entry)
        require(magic == 0x50AA and size > 0, "invalid partition entry")
        require(offset + size <= flash_size, "partition exceeds flash")
        require(offset >= table_offset + 0x1000, "partition overlaps reserved table sector/bootloader area")
        require(kind in (0, 1, 2, 3) or 0x40 <= kind <= 0xFE, "unsupported partition type")
        require(flags & ~0x3 == 0, "unsupported partition flags")
        require(offset % (0x10000 if kind == 0 else 0x1000) == 0, "unaligned partition offset")
        if kind == 0:
            require(subtype in (0, 0x20) or 0x10 <= subtype <= 0x1F, "unsupported application subtype")
            require(size % 0x1000 == 0, "unaligned application partition size")
        elif kind == 1:
            require(subtype in (0, 1, 2, 3, 4, 5, 6, 0x80, 0x81, 0x82, 0x83), "unsupported data subtype")
            require(not (subtype in (0, 3) and flags & 2), "otadata/coredump cannot be readonly")
            if subtype == 0:
                otadata_count += 1
                require(otadata_count == 1 and size == 0x2000, "invalid or duplicate otadata partition")
        elif kind in (2, 3):
            require(subtype == 1, "unsupported bootloader/table subtype")
        name = raw_label.split(b"\0", 1)[0].decode("utf-8")
        require(bool(name) and name not in names, "empty or duplicate partition label")
        names.add(name)
        require(all(offset + size <= a or offset >= b for a, b in ranges), "overlapping partitions")
        ranges.append((offset, offset + size))
        if kind == 0 and name == label:
            apps.append((offset, size))
    require(len(apps) == 1, "selected application partition missing or ambiguous")
    return apps[0]


def load_manifest(path, expected_sha256):
    path = Path(path).resolve()
    raw = path.read_bytes()
    require(hashlib.sha256(raw).hexdigest() == expected_sha256.lower(), "manifest SHA-256 mismatch")
    manifest = json.loads(raw)
    require(isinstance(manifest, dict) and manifest.get("schema_version") == 1, "unsupported manifest schema")
    require(manifest.get("chip") == "esp32s3", "unsupported manifest chip")
    mode = manifest.get("mode")
    require(mode in ("segments", "app_only", "merged"), "explicit artifact mode required")
    flash_size = integer(manifest.get("flash_size_bytes"), "flash_size_bytes")
    require(1024 * 1024 <= flash_size <= 32 * 1024 * 1024 and flash_size & (flash_size - 1) == 0, "unsupported flash size")
    records = manifest.get("segments")
    require(isinstance(records, list) and bool(records), "segments required")
    segments = []
    kinds = set()
    for record in records:
        require(isinstance(record, dict), "invalid segment")
        kind = record.get("kind")
        require(kind in ("bootloader", "partition_table", "application", "merged") and kind not in kinds, "invalid or duplicate artifact kind")
        kinds.add(kind)
        offset = integer(record.get("offset"), "segment offset")
        data = verified_file(path.parent, record)
        require(offset + len(data) <= flash_size, "segment exceeds flash")
        require(all(offset + len(data) <= a or offset >= a + len(b) for _, a, b in segments), "overlapping segments")
        segments.append((kind, offset, data))
    expected_kinds = {"segments": {"bootloader", "partition_table", "application"}, "app_only": {"application"}, "merged": {"merged"}}[mode]
    require(kinds == expected_kinds, "artifact set does not match mode")
    layout = manifest.get("partition_layout")
    table = verified_file(path.parent, layout)
    table_offset = integer(layout.get("table_offset"), "partition table offset")
    require(table_offset > 0 and table_offset % 4096 == 0 and table_offset + 0x1000 <= flash_size, "invalid partition table location")
    app_offset, capacity = app_partition(table, layout.get("app_label"), flash_size, table_offset)
    require(app_offset >= table_offset + 0x1000, "application overlaps partition table")
    if mode == "merged":
        _, offset, data = segments[0]
        require(offset == 0, "merged image requires explicit offset 0")
        require(data[:1] == b"\xe9", "merged image bootloader header missing")
        require(data[table_offset:table_offset + len(table)] == table, "merged partition table mismatch")
        require(data[app_offset:app_offset + 1] == b"\xe9", "merged application header missing")
        require(len(data) <= app_offset + capacity, "merged image exceeds selected application partition")
    else:
        app = next(x for x in segments if x[0] == "application")
        require(app[1] == app_offset and len(app[2]) <= capacity and app[2][:1] == b"\xe9", "application does not fit verified partition layout")
        if mode == "segments":
            boot = next(x for x in segments if x[0] == "bootloader")
            partition = next(x for x in segments if x[0] == "partition_table")
            require(boot[1] == 0 and boot[2][:1] == b"\xe9", "ESP32-S3 bootloader requires offset 0")
            require(partition[1] == table_offset and partition[2] == table, "partition segment differs from verified layout")
    return sorted(segments, key=lambda x: x[1])
