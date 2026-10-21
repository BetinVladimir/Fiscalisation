#!/usr/bin/env python3
"""Mandatory native crypto gate; temporary dependencies/fixtures, allowlisted evidence."""
from __future__ import annotations
import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import shutil
import subprocess
import sys
import tempfile
import time
import uuid

MBEDTLS_COMMIT = "068ff080b369adfac81509f9b57b2afabaf82dc5"
FRAMEWORK_COMMIT = "dde0c4a0e448a0552f18817dcea633bb851fd288"
CJSON = {
    "cJSON.c": "75c51de8fa40ac9d7a99319c6330719bd692eb81c0a869265f3d4c682533f9b9",
    "cJSON.h": "0578cc29132912edbc88f83207a8fc76e5db3db0605497e909a9384ef3cc474b",
}
PHASES = ("preflight", "mbedtls-source", "framework-source", "cjson-source", "configure",
          "build", "profile", "proof", "time-anchor", "source-recheck")
PROFILE_MARKERS = (
    "=== RUN   TestBLEServerProfileIsSeparatePurposeAndEnrolledKeyBound",
    "--- PASS: TestBLEServerProfileIsSeparatePurposeAndEnrolledKeyBound",
    "PASS actual mbedTLS profile case server.pem expected=accept",
    "PASS actual mbedTLS profile case client-only.pem expected=reject",
    "PASS actual mbedTLS profile case server.pem expected=reject",
    "PASS actual mbedTLS profile case wrong-issuer.pem expected=reject",
    "PASS actual mbedTLS profile case wrong-key.pem expected=reject",
)
PROOF_MARKERS = (
    "=== RUN   TestBLECertificateActualFirmwareProofInterop",
    "--- PASS: TestBLECertificateActualFirmwareProofInterop",
    "PASS actual DeviceIdentity/NVS singlehash proof, strict root/context/int64/time/cache/fault/restart and both concurrent installers",
)

TIME_ANCHOR_TEST = "TestTimeAnchorActualCryptoInterop"
TIME_ANCHOR_MARKERS = (
    "=== RUN   " + TIME_ANCHOR_TEST,
    "--- PASS: " + TIME_ANCHOR_TEST,
    "PASS actual DeviceIdentity purpose-bound request singlehash signing",
    "PASS actual root DER low/highS, signed hostile context/integer vectors, unsigned preservation, returned-packet replacement, replay/expiry/boot/binding, default-no-root/no-autoboot",
)

class GateError(RuntimeError):
    pass

def sha(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()

def require_hash(path: Path, expected: str) -> None:
    if sha(path) != expected:
        raise GateError(f"reviewed source hash mismatch: {path.name}")

def require_native_cases(log: str, markers: tuple[str, ...]) -> None:
    if "[no tests to run]" in log or "--- SKIP:" in log or any(marker not in log for marker in markers):
        raise GateError("native test absent, skipped or required case evidence missing")

def require_time_anchor_cases(log: str) -> None:
    require_native_cases(log, TIME_ANCHOR_MARKERS)
    # Exit zero alone also covers missing Go -run matches and optional-env skips.
    # Require the actual top-level test start/completion, rather than substrings
    # quoted in diagnostics or output from an unrelated test.
    for pattern in (r"^=== RUN   " + TIME_ANCHOR_TEST + r"$",
                    r"^--- PASS: " + TIME_ANCHOR_TEST + r" \(.*\)$"):
        if len(re.findall(pattern, log, re.MULTILINE)) != 1:
            raise GateError("actual time-anchor Go test start/completion absent or ambiguous")

def require_native_manifest(produced: dict, libraries: dict, *, time_anchor=False) -> None:
    if produced.get("libraries_sha256") != libraries:
        raise GateError("native binary library linkage evidence mismatch")
    if not re.fullmatch(r"[0-9a-f]{64}", str(produced.get("binary_sha256", ""))):
        raise GateError("native binary build evidence absent")
    if time_anchor:
        flags = produced.get("compile_flags", [])
        if (produced.get("synthetic_core_permit_factory") is not False
                or produced.get("host_test_root_injection") is not True
                or "-DBEEFISCAL_TIME_ANCHOR_CRYPTO_HOST_TEST" not in flags
                or any("BEEFISCAL_TIME_ANCHOR_HOST_TEST" in flag for flag in flags)
                or produced.get("cjson_sources_sha256") != CJSON):
            raise GateError("time-anchor actual crypto build contract mismatch")

def native_command(repo: Path, script: str, *, time_anchor=False) -> list[str]:
    command = [str(repo / script)]
    # Newly added checked-in shell fixture may have mode 0644. Execute its
    # reviewed POSIX shell contents without depending on an executable bit.
    return ["/bin/sh", *command] if time_anchor else command

def source_files(repo: Path) -> list[Path]:
    idf = repo / "IoT/firmware/edge-agent-s3/idf"
    paths = list((idf / "main").glob("*.cpp")) + list((idf / "main").glob("*.h"))
    paths += [idf / "main/CMakeLists.txt"]
    for folder in ("ble_proof_host", "entropy_host", "provisioning_host", "money_host", "ble_server_certificate", "time_anchor_crypto_host"):
        paths += [p for p in (idf / "tests" / folder).rglob("*") if p.is_file()]
    paths += list((repo / "fiscal-backend").rglob("*.go"))
    paths += [repo / name for name in (
        "fiscal-backend/go.mod", "fiscal-backend/go.sum", "Makefile",
        "scripts/run-ble-certificate-crypto.py", "scripts/tests/test_ble_certificate_crypto_gate.py",
        "scripts/verify-ble-server-certificate-profile.sh", "scripts/verify-ble-certificate-proof-host.sh",
        "scripts/verify-device-time-anchor-crypto-host.sh",
        ".github/workflows/ble-certificate-crypto.yml")]
    return sorted(set(paths))

class Gate:
    def __init__(self, repo: Path, evidence: Path):
        self.repo = repo.resolve()
        self.evidence = evidence.resolve()
        self.run_id = dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ-") + uuid.uuid4().hex[:12]
        self.output = self.evidence / self.run_id
        self.output.mkdir(parents=True, exist_ok=False)
        self.env = {k: os.environ[k] for k in ("PATH", "HOME", "TMPDIR") if k in os.environ}
        self.env.update({"GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.devnull,
                         "GIT_TERMINAL_PROMPT": "0", "GOTOOLCHAIN": "local"})
        self.data = {"gate": "ble-certificate-native-crypto-v1", "run_id": self.run_id,
                     "status": "FAILED", "host": platform.platform(),
                     "scope": "host native proof/profile/time-anchor crypto only; production authority clock/provider, full IDF target and hardware remain OPEN",
                     "phases": {name: {"status": "NOT_RUN", "exit_code": None, "commands": []} for name in PHASES},
                     "dependency_sources": {}, "source_sha256": {}, "tools": {}, "build": {}}
        self.current = None
        self.log = None
        self.tools = {}

    def phase(self, name, function):
        record = self.data["phases"][name]
        record.update(status="RUNNING", started_utc=dt.datetime.now(dt.timezone.utc).isoformat())
        self.current = name
        start = time.monotonic()
        try:
            with (self.output / (name + ".log")).open("w") as self.log:
                function()
            record.update(status="PASS", exit_code=0)
        except BaseException as error:
            record.update(status="FAILED", exit_code=record.get("failed_command_exit", 1),
                          error=f"{type(error).__name__}: {error}")
            raise
        finally:
            record["elapsed_seconds"] = round(time.monotonic() - start, 3)
            self.log = None
            self.current = None

    def run(self, command, cwd=None, timeout=180, capture=False):
        argv = [str(x) for x in command]
        record = {"argv": argv}
        self.data["phases"][self.current]["commands"].append(record)
        self.log.write("COMMAND " + json.dumps(argv) + "\n")
        self.log.flush()
        try:
            if capture:
                result = subprocess.run(argv, cwd=cwd, env=self.env, text=True, stdout=subprocess.PIPE,
                                        stderr=subprocess.STDOUT, timeout=timeout)
                self.log.write(result.stdout)
            else:
                result = subprocess.run(argv, cwd=cwd, env=self.env, stdout=self.log,
                                        stderr=subprocess.STDOUT, timeout=timeout)
            record["exit_code"] = result.returncode
        except subprocess.TimeoutExpired:
            record["exit_code"] = 124
            self.data["phases"][self.current]["failed_command_exit"] = 124
            raise GateError("command deadline exceeded")
        if result.returncode:
            self.data["phases"][self.current]["failed_command_exit"] = result.returncode
            raise GateError(f"command failed ({result.returncode}): {Path(argv[0]).name}")
        return result.stdout.strip() if capture else ""

    def preflight(self):
        names = {"git": "git", "cmake": "cmake", "curl": "curl", "go": "go",
                 "cc": os.environ.get("CC", "cc"), "cxx": os.environ.get("CXX", "c++")}
        for name, executable in names.items():
            path = shutil.which(executable)
            if not path:
                raise GateError("required tool unavailable: " + name)
            self.tools[name] = path
            version = self.run([path, "version" if name == "go" else "--version"], capture=True)
            self.data["tools"][name] = {"path": str(Path(path).resolve()), "version": version,
                                        "binary_sha256": sha(Path(path).resolve())}
        self.data["tools"]["python"] = {"version": sys.version, "path": sys.executable,
                                          "binary_sha256": sha(Path(sys.executable).resolve())}
        self.env.update(CC=self.tools["cc"], CXX=self.tools["cxx"])
        self.data["source_sha256"] = {str(p.relative_to(self.repo)): sha(p) for p in source_files(self.repo)}
        if not self.data["source_sha256"]:
            raise GateError("empty source inventory")

    def fetch_git(self, directory: Path, url: str, commit: str):
        self.run([self.tools["git"], "init", "--quiet", directory])
        self.run([self.tools["git"], "-C", directory, "remote", "add", "origin", url])
        self.run([self.tools["git"], "-C", directory, "fetch", "--depth=1", "origin", commit], timeout=240)
        self.run([self.tools["git"], "-C", directory, "checkout", "--detach", commit])
        self.clean_git(directory, commit)

    def clean_git(self, directory: Path, commit: str):
        actual = self.run([self.tools["git"], "-C", directory, "rev-parse", "HEAD"], capture=True)
        dirty = self.run([self.tools["git"], "-C", directory, "status", "--porcelain", "--untracked-files=all"], capture=True)
        if actual != commit or dirty:
            raise GateError("dependency revision or tracked/untracked source state changed")

    def execute(self):
        try:
            self.phase("preflight", self.preflight)
            with tempfile.TemporaryDirectory(prefix="fiscal-ble-native-gate-") as temp:
                work = Path(temp)
                source, framework, cjson, build = work / "mbedtls", work / "mbedtls/framework", work / "cjson", work / "build"
                self.env.update(TMPDIR=str(work), GOCACHE=str(work / "go-cache"),
                                GOMODCACHE=str(work / "go-mod-cache"), GOPATH=str(work / "go"))
                self.phase("mbedtls-source", lambda: self.fetch_git(source, "https://github.com/Mbed-TLS/mbedtls.git", MBEDTLS_COMMIT))
                def fetch_framework():
                    expected = self.run([self.tools["git"], "-C", source, "ls-tree", "HEAD", "framework"], capture=True)
                    if expected.split()[:3] != ["160000", "commit", FRAMEWORK_COMMIT]:
                        raise GateError("reviewed framework gitlink changed")
                    self.fetch_git(framework, "https://github.com/Mbed-TLS/mbedtls-framework", FRAMEWORK_COMMIT)
                    self.clean_git(source, MBEDTLS_COMMIT)
                self.phase("framework-source", fetch_framework)
                self.data["dependency_sources"].update(mbedtls={"url": "https://github.com/Mbed-TLS/mbedtls.git", "commit": MBEDTLS_COMMIT},
                                                       framework={"url": "https://github.com/Mbed-TLS/mbedtls-framework", "commit": FRAMEWORK_COMMIT})
                def fetch_cjson():
                    cjson.mkdir()
                    for name, digest in CJSON.items():
                        url = "https://raw.githubusercontent.com/DaveGamble/cJSON/v1.7.18/" + name
                        self.run([self.tools["curl"], "--fail", "--location", "--retry", "2", "--connect-timeout", "15", "--max-time", "90", url, "-o", cjson / name], timeout=240)
                        require_hash(cjson / name, digest)
                    self.data["dependency_sources"]["cjson"] = {"tag": "v1.7.18", "url": "https://github.com/DaveGamble/cJSON", "source_sha256": CJSON,
                                                               "provenance": "official upstream bytes pinned by hash; no PlatformIO/git-clean claim"}
                self.phase("cjson-source", fetch_cjson)
                self.phase("configure", lambda: self.run([self.tools["cmake"], "-S", source, "-B", build,
                    "-DENABLE_TESTING=OFF", "-DENABLE_PROGRAMS=OFF", "-DCMAKE_C_COMPILER=" + self.tools["cc"]]))
                self.phase("build", lambda: self.run([self.tools["cmake"], "--build", build, "--parallel", "4"], timeout=360))
                self.data["build"] = {"cmake_cache_sha256": sha(build / "CMakeCache.txt"),
                    "libraries_sha256": {name: sha(build / "library" / name) for name in ("libmbedtls.a", "libmbedx509.a", "libmbedcrypto.a")}}
                self.env.update(MBEDTLS_SOURCE_DIR=str(source), MBEDTLS_BUILD_DIR=str(build), CJSON_SOURCE_DIR=str(cjson))
                def native(name, script, markers, *, time_anchor=False):
                    manifest_env = "TIME_ANCHOR_CRYPTO_BUILD_MANIFEST" if time_anchor else "BLE_CRYPTO_BUILD_MANIFEST"
                    self.env.pop("TIME_ANCHOR_CRYPTO_BUILD_MANIFEST", None)
                    self.env.pop("BLE_CRYPTO_BUILD_MANIFEST", None)
                    self.env[manifest_env] = str(self.output / (name + "-build-manifest.json"))
                    command = native_command(self.repo, script, time_anchor=time_anchor)
                    self.run(command, cwd=self.repo, timeout=240)
                    self.log.flush()
                    native_log = (self.output / (name + ".log")).read_text()
                    if time_anchor: require_time_anchor_cases(native_log)
                    else: require_native_cases(native_log, markers)
                    manifest_path = self.output / (name + "-build-manifest.json")
                    produced = json.loads(manifest_path.read_text())
                    require_native_manifest(produced, self.data["build"]["libraries_sha256"], time_anchor=time_anchor)
                    self.data["phases"][name]["build_manifest_sha256"] = sha(manifest_path)
                self.phase("profile", lambda: native("profile", "scripts/verify-ble-server-certificate-profile.sh", PROFILE_MARKERS))
                self.phase("proof", lambda: native("proof", "scripts/verify-ble-certificate-proof-host.sh", PROOF_MARKERS))
                self.phase("time-anchor", lambda: native("time-anchor", "scripts/verify-device-time-anchor-crypto-host.sh", TIME_ANCHOR_MARKERS, time_anchor=True))
                def recheck():
                    self.clean_git(source, MBEDTLS_COMMIT)
                    self.clean_git(framework, FRAMEWORK_COMMIT)
                    for name, digest in CJSON.items(): require_hash(cjson / name, digest)
                    current = {str(p.relative_to(self.repo)): sha(p) for p in source_files(self.repo)}
                    if current != self.data["source_sha256"]: raise GateError("tested repository inputs changed during the gate")
                self.phase("source-recheck", recheck)
                self.data["status"] = "PASS"
            return 0
        except BaseException as error:
            self.data["failure"] = f"{type(error).__name__}: {error}"
            return 1
        finally:
            self.data["finished_utc"] = dt.datetime.now(dt.timezone.utc).isoformat()
            self.data["artifacts_sha256"] = {p.name: sha(p) for p in sorted(self.output.iterdir())
                if p.is_file() and (p.suffix == ".log" or p.name.endswith("-build-manifest.json"))}
            text = json.dumps(self.data, indent=2) + "\n"
            (self.output / "run-manifest.json").write_text(text)
            (self.evidence / "manifest.json").write_text(text)

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument("--evidence", type=Path, required=True)
    args = parser.parse_args()
    gate = Gate(args.repo, args.evidence)
    result = gate.execute()
    print(gate.data["status"] + " native BLE certificate gate; evidence " + str(gate.evidence / "manifest.json"))
    if result: print(gate.data.get("failure", "gate failed"), file=sys.stderr)
    return result

if __name__ == "__main__":
    raise SystemExit(main())
