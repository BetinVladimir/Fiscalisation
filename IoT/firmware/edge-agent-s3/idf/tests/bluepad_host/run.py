#!/usr/bin/env python3
"""Compile actual BluePad central against asynchronous fake NimBLE, then exercise vendor vectors."""
from pathlib import Path
import os
import subprocess
import tempfile
HERE = Path(__file__).resolve().parent
MODES = '''result-error result-retry coalesced fragmented missing-power power-failure wake-failure notification-overflow approved declined intermediate wake cccd-timeout cccd-failure stale-generation foreign-df03 missing-tags wrong-type wrong-stan duplicate-tag invalid-ber end-failure early-final final-missing bad-bcc oversize fragment-timeout queue-overflow intermediate-only ack-only invalid-stan immediate-failure partial-immediate completion-failure completion-held disconnect host-reset write-timeout'''.split()
with tempfile.TemporaryDirectory(prefix="bluepad-host-") as temporary:
    binary = str(Path(temporary) / "runtime")
    subprocess.run([os.environ.get("CXX", "c++"), "-std=c++17", "-Wall", "-Wextra", "-Werror", "-pthread", "-I" + str(HERE / "include"), "-I" + str(HERE.parent.parent / "main"), str(HERE / "runtime.cpp"), "-o", binary], check=True)
    for mode in MODES:
        subprocess.run([binary, mode], check=True, timeout=10)
print(f"BluePad actual runtime: {len(MODES)} scenarios PASS")
