"""native_intent.py — compile and run intent_processor_native.cpp host tests.

Provides compile_and_run() which builds the binary once per process (cached)
and runs it against a temporary directory, returning (stdout, stderr, rc).
Used by test_tst_s3_01_1.py … test_tst_s3_05_1.py.
"""
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
IDF_ROOT = ROOT / 'IoT/firmware/edge-agent-s3/idf'
TESTS_DIR = IDF_ROOT / 'tests'
SOURCE_ROOT = IDF_ROOT

_binary_cache: 'dict[str, str]' = {}
_build_dir: 'list[str]' = []  # persistent build directory, not cleaned up


def _find_openssl() -> str:
    """Return OpenSSL prefix, or raise RuntimeError."""
    prefix = os.environ.get('OPENSSL_PREFIX', '')
    if prefix and os.path.isfile(os.path.join(prefix, 'include', 'openssl', 'sha.h')):
        return prefix
    try:
        result = subprocess.run(
            ['pkg-config', '--variable=prefix', 'openssl'],
            capture_output=True, text=True, timeout=10,
        )
        if result.returncode == 0:
            p = result.stdout.strip()
            if p and os.path.isfile(os.path.join(p, 'include', 'openssl', 'sha.h')):
                return p
    except (FileNotFoundError, subprocess.TimeoutExpired):
        pass
    try:
        result = subprocess.run(
            ['brew', '--prefix', 'openssl@3'],
            capture_output=True, text=True, timeout=10,
        )
        if result.returncode == 0:
            p = result.stdout.strip()
            if p and os.path.isfile(os.path.join(p, 'include', 'openssl', 'sha.h')):
                return p
    except (FileNotFoundError, subprocess.TimeoutExpired):
        pass
    raise RuntimeError('OpenSSL headers not found; set OPENSSL_PREFIX')


def _find_cjson() -> str:
    """Return cJSON source directory, or raise RuntimeError."""
    d = os.environ.get('CJSON_SOURCE_DIR', '')
    if d and os.path.isfile(os.path.join(d, 'cJSON.c')):
        return d
    home = os.path.expanduser('~')
    candidate = os.path.join(
        home, '.platformio', 'packages',
        'framework-espidf', 'components', 'json', 'cJSON',
    )
    if os.path.isfile(os.path.join(candidate, 'cJSON.c')):
        return candidate
    raise RuntimeError('cJSON source not found; set CJSON_SOURCE_DIR')


def _find_mbedtls(out_dir: str) -> tuple[str, str]:
    """Run mbedtls_host.py and return (include, library) paths."""
    script = TESTS_DIR / 'mbedtls_host.py'
    result = subprocess.run(
        [sys.executable, str(script), out_dir],
        capture_output=True, text=True, timeout=30,
    )
    if result.returncode != 0:
        raise RuntimeError(f'mbedtls_host.py failed: {result.stderr}')
    lines = result.stdout.strip().splitlines()
    if len(lines) < 2:
        raise RuntimeError(f'mbedtls_host.py bad output: {result.stdout!r}')
    return lines[0], lines[1]


def _persistent_build_dir() -> str:
    """Return a persistent build directory for the binary (created once per process)."""
    if _build_dir:
        return _build_dir[0]
    d = tempfile.mkdtemp(
        dir=os.environ.get('TMPDIR', '/tmp'),
        prefix='beefiscal-intent-build.',
    )
    import atexit
    atexit.register(shutil.rmtree, d, True)
    _build_dir.append(d)
    return d


def build_binary(out_dir: str) -> str:
    """Compile intent_processor_native.cpp and return binary path."""
    if 'binary' in _binary_cache:
        return _binary_cache['binary']

    build_out = _persistent_build_dir()
    openssl = _find_openssl()
    cjson = _find_cjson()
    mbedtls_include, mbedtls_library = _find_mbedtls(build_out)

    cxx = os.environ.get('CXX', 'c++')
    cc = os.environ.get('CC', 'cc')

    # Compile cJSON into persistent build dir
    cjson_obj = os.path.join(build_out, 'cJSON.o')
    subprocess.run(
        [cc, '-I', cjson, '-c', os.path.join(cjson, 'cJSON.c'), '-o', cjson_obj],
        check=True, capture_output=True,
    )

    binary = os.path.join(build_out, 'intent_processor_native')
    src = SOURCE_ROOT / 'main'
    includes = [
        '-I', mbedtls_include,
        '-I', str(TESTS_DIR / 'storage_host'),
        '-I', str(TESTS_DIR / 'intent_host'),
        '-I', str(src),
        '-I', cjson,
        '-I', os.path.join(openssl, 'include'),
    ]
    sources = [
        str(src / 'durable_storage.cpp'),
        str(src / 'intent_processor.cpp'),
        str(src / 'execution_authorization.cpp'),
        str(src / 'payment_reconciliation.cpp'),
        str(src / 'go_json.cpp'),
        str(src / 'go_wire.cpp'),
        str(src / 'canonical_cbor.cpp'),
        str(TESTS_DIR / 'intent_processor_native.cpp'),
        cjson_obj,
        mbedtls_library,
    ]
    libs = ['-L', os.path.join(openssl, 'lib'), '-lcrypto', '-lsqlite3']

    result = subprocess.run(
        [cxx, '-std=c++17'] + includes + sources + libs + ['-o', binary],
        capture_output=True, text=True,
    )
    if result.returncode != 0:
        raise RuntimeError(
            f'intent_processor_native compilation failed:\n{result.stderr}'
        )
    _binary_cache['binary'] = binary
    return binary


def compile_and_run(tmp_parent: str = '') -> tuple[str, str, int]:
    """Build the binary (cached) and run it; return (stdout, stderr, rc).

    Callers use their own tempfile logic for the tmp_parent directory.
    """
    out_dir = tempfile.mkdtemp(
        dir=tmp_parent or os.environ.get('TMPDIR', '/tmp'),
        prefix='beefiscal-intent-native.',
    )
    try:
        binary = build_binary(out_dir)
        result = subprocess.run(
            [binary, out_dir],
            capture_output=True, text=True, timeout=120,
        )
        return result.stdout, result.stderr, result.returncode
    finally:
        shutil.rmtree(out_dir, ignore_errors=True)


def parse_tap(stdout: str) -> dict:
    """Parse TAP output; return {name: bool} for each test line."""
    tests: dict[str, bool] = {}
    for line in stdout.splitlines():
        line = line.strip()
        if line.startswith('ok ') or line.startswith('not ok '):
            ok = line.startswith('ok ')
            rest = line[len('ok '):] if ok else line[len('not ok '):]
            # rest = "N - test-name" or "N - test-name # directive"
            parts = rest.split(' - ', 1)
            if len(parts) == 2:
                name = parts[1].split(' # ')[0].strip()
                tests[name] = ok
    return tests
