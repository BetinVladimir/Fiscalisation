from __future__ import annotations
from dataclasses import asdict, dataclass
from hashlib import sha256
import json
from pathlib import Path
from datetime import datetime, timezone

@dataclass(frozen=True)
class Evidence:
    serial: str
    batch: str
    hardware_revision: str
    firmware_version: str
    firmware_sha256: str
    device_public_key_jwk: dict
    manufacturing_station_id: str
    captured_at: str

    def canonical(self) -> bytes:
        return json.dumps(asdict(self), sort_keys=True, separators=(",", ":")).encode()

    @property
    def digest(self) -> str:
        return sha256(self.canonical()).hexdigest()

    def save(self, path: Path) -> None:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps({**asdict(self), "registration_evidence_sha256": self.digest}, indent=2), encoding="utf-8")

def sha256_file(path: Path) -> str:
    digest = sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()

def now() -> str:
    return datetime.now(timezone.utc).isoformat()
