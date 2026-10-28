#!/usr/bin/env bash
# mvp1-baseline.sh — Run all MVP1 module test suites and produce report.json
# Usage:
#   Normal run:  bash scripts/mvp1-baseline.sh --output-dir DIR
#   Fixture run: bash scripts/mvp1-baseline.sh --output-dir DIR --fixture JSON_FILE [--timeout SECONDS]
set -euo pipefail

usage() {
    echo "Usage: $0 --output-dir DIR [--fixture FILE] [--timeout SECONDS]" >&2
    exit 3
}

OUTPUT_DIR=""
FIXTURE_FILE=""
TIMEOUT_SECONDS=300

while [[ $# -gt 0 ]]; do
    case "$1" in
        --output-dir) OUTPUT_DIR="$2"; shift 2 ;;
        --fixture)    FIXTURE_FILE="$2"; shift 2 ;;
        --timeout)    TIMEOUT_SECONDS="$2"; shift 2 ;;
        *) usage ;;
    esac
done

[[ -z "$OUTPUT_DIR" ]] && usage
mkdir -p "$OUTPUT_DIR"

REPORT="$OUTPUT_DIR/report.json"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# --- Python helper: run_modules.py ---
HELPER="$(mktemp /tmp/mvp1_baseline_XXXXXX.py)"
trap "rm -f '$HELPER'" EXIT

cat > "$HELPER" << 'PYEOF'
import json, os, subprocess, sys, time, pathlib

def get_tools():
    tools = {}
    for name, cmd in [
        ("python",  ["python3", "--version"]),
        ("git",     ["git", "--version"]),
        ("go",      ["go", "version"]),
        ("c++",     ["c++", "--version"]),
        ("node",    ["node", "--version"]),
        ("npm",     ["npm", "--version"]),
        ("java",    ["java", "-version"]),
        ("docker",  ["docker", "--version"]),
    ]:
        try:
            r = subprocess.run(cmd, capture_output=True, text=True, timeout=10)
            out = (r.stdout + r.stderr).strip().split("\n")[0]
            tools[name] = out if r.returncode == 0 else f"unavailable: rc={r.returncode}"
        except FileNotFoundError:
            tools[name] = f"unavailable: not found"
        except Exception as e:
            tools[name] = f"unavailable: {e}"
    return tools

def get_commit(root):
    try:
        r = subprocess.run(["git", "rev-parse", "HEAD"],
                           capture_output=True, text=True, cwd=root, timeout=10)
        if r.returncode == 0:
            return r.stdout.strip(), None
        return None, f"git rev-parse failed: {r.stderr.strip()}"
    except Exception as e:
        return None, str(e)

def run_one(mod, timeout_s, root):
    mid = mod["id"]
    cwd_path = mod["cwd"]
    argv = mod["argv"]

    # Resolve cwd relative to root if not absolute
    if not os.path.isabs(cwd_path):
        cwd_path = os.path.join(root, cwd_path)
    cwd_path = os.path.realpath(cwd_path)

    if not os.path.isdir(cwd_path):
        return {
            "id": mid, "cwd": cwd_path, "argv": argv,
            "rc": None, "status": "BLOCKED",
            "reason": f"cwd not found: {cwd_path}",
            "duration_seconds": 0.0, "stdout": "", "stderr": ""
        }

    env = os.environ.copy()
    env["PWD"] = cwd_path
    env.pop("OLDPWD", None)

    t0 = time.monotonic()
    try:
        result = subprocess.run(
            argv, cwd=cwd_path, env=env,
            capture_output=True, text=True,
            timeout=timeout_s
        )
        duration = time.monotonic() - t0
        stdout = result.stdout[-8192:] if len(result.stdout) > 8192 else result.stdout
        stderr = result.stderr[-8192:] if len(result.stderr) > 8192 else result.stderr
        status = "PASS" if result.returncode == 0 else "FAIL"
        return {
            "id": mid, "cwd": cwd_path, "argv": argv,
            "rc": result.returncode, "status": status, "reason": "",
            "duration_seconds": round(duration, 3),
            "stdout": stdout, "stderr": stderr
        }
    except subprocess.TimeoutExpired:
        duration = time.monotonic() - t0
        return {
            "id": mid, "cwd": cwd_path, "argv": argv,
            "rc": None, "status": "BLOCKED",
            "reason": f"timeout after {timeout_s}s",
            "duration_seconds": round(duration, 3),
            "stdout": "", "stderr": ""
        }
    except FileNotFoundError as e:
        duration = time.monotonic() - t0
        return {
            "id": mid, "cwd": cwd_path, "argv": argv,
            "rc": None, "status": "BLOCKED",
            "reason": f"executable not found: {e}",
            "duration_seconds": round(duration, 3),
            "stdout": "", "stderr": ""
        }

def main():
    root      = sys.argv[1]
    output    = sys.argv[2]
    fixture   = sys.argv[3] if sys.argv[3] != "" else None
    timeout_s = int(sys.argv[4])

    if fixture:
        with open(fixture) as f:
            modules = json.load(f)
        if isinstance(modules, dict) and "commands" in modules:
            modules = modules["commands"]
    else:
        # Canonical real modules from ARCH_21 §6
        modules = [
            {"id": "fiscal-backend", "cwd": "fiscal-backend", "argv": ["go", "test", "./..."]},
            {"id": "s3",        "cwd": ".", "argv": ["bash", "IoT/firmware/edge-agent-s3/idf/tests/run_native_serial.sh"]},
            {"id": "protocols", "cwd": ".", "argv": ["bash", "IoT/protocol-abstraction/run-tests.sh"]},
            {"id": "smartdevices", "cwd": "SmartDevices", "argv": ["./gradlew",
                ":bluecash-app:testProdDebugUnitTest", ":bluecash-app:testDemoDebugUnitTest",
                ":daisy-smart-app:testProdDebugUnitTest", ":daisy-smart-app:testDemoDebugUnitTest",
                "--no-daemon", "--rerun-tasks"]},
            {"id": "pos",      "cwd": "../POS/pos-app", "argv": ["npx", "--no-install", "jest", "--runInBand"]},
            {"id": "minipos",  "cwd": "minipos/miniposweb", "argv": ["npx", "--no-install", "vitest", "run"]},
            {"id": "e2e",      "cwd": ".", "argv": ["make", "compose-e2e"]},
        ]

    commit, commit_reason = get_commit(root)
    tools = get_tools()

    records = []
    for mod in modules:
        rec = run_one(mod, timeout_s, root)
        records.append(rec)

    statuses = [r["status"] for r in records]
    if all(s == "PASS" for s in statuses):
        overall = "PASS"
    elif any(s == "BLOCKED" for s in statuses):
        overall = "BLOCKED"
    else:
        overall = "FAIL"

    report = {
        "schema": 1,
        "commit": commit,
        "commit_reason": commit_reason,
        "tools": tools,
        "modules": records,
        "overall_status": overall,
    }

    os.makedirs(output, exist_ok=True)
    with open(os.path.join(output, "report.json"), "w") as f:
        json.dump(report, f, indent=2, ensure_ascii=False)

    if overall == "PASS":
        sys.exit(0)
    elif overall == "BLOCKED":
        sys.exit(3)
    else:
        sys.exit(1)

main()
PYEOF

python3 -I "$HELPER" "$ROOT" "$OUTPUT_DIR" "$FIXTURE_FILE" "$TIMEOUT_SECONDS"
