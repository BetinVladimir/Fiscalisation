#!/usr/bin/env python3
"""Pinned ESP-IDF build and artifact consistency gate. Never installs or flashes."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import subprocess
import sys

PROJECT = Path(__file__).resolve().parents[1]
REPO = PROJECT.parents[3]
LOCK_FILE = PROJECT / 'tools/build.lock.json'
ARTIFACTS = ('beefiscal_edge_agent_s3.bin', 'beefiscal_edge_agent_s3.elf',
             'bootloader/bootloader.bin', 'partition_table/partition-table.bin')
OVERRIDES = ('CC', 'CXX', 'AR', 'RANLIB', 'CFLAGS', 'CXXFLAGS', 'CPPFLAGS', 'LDFLAGS',
             'CMAKE_TOOLCHAIN_FILE', 'CMAKE_PROJECT_INCLUDE', 'CMAKE_PROJECT_INCLUDE_BEFORE',
             'EXTRA_COMPONENT_DIRS', 'SDKCONFIG', 'SDKCONFIG_DEFAULTS', 'IDF_EXTRA_ACTIONS_PATH')

class GateError(Exception):
    pass

def require(ok, message):
    if not ok:
        raise GateError(message)

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def run(args, **kwargs):
    try:
        return subprocess.check_output([str(a) for a in args], text=True,
                                       stderr=subprocess.PIPE, timeout=60, **kwargs).strip()
    except (subprocess.SubprocessError, OSError) as exc:
        raise GateError(f'Command failed: {args[0]}: {exc}') from exc

def physical_path(value, name):
    path = Path(value).absolute()
    require(not any(c.isspace() for c in str(path)), f'{name} must not contain whitespace')
    require(path == path.resolve(), f'{name} must be a physical path without symlink aliases')
    if sys.platform == 'darwin':
        require(not str(path).startswith(('/private/tmp/', '/tmp/', '/private/var/', '/var/')),
                f'{name}: use a physical non-temporary-root path on macOS (IDF path alias limitation)')
    return path

def load_lock():
    lock = json.loads(LOCK_FILE.read_text())
    require(lock['schema_version'] == 1, 'Unsupported build lock schema')
    for rel, expected in lock['pinned_inputs'].items():
        path = PROJECT / rel
        require(path.is_file() and digest(path) == expected, f'Pinned build input changed: {rel}')
    return lock

def source_hashes():
    # Include all application/private component and protocol sources, not generated build files.
    roots = (PROJECT / 'main', PROJECT / 'components', PROJECT / 'tools',
             REPO / 'IoT/protocol-abstraction/include', REPO / 'IoT/protocol-abstraction/src')
    files = {PROJECT / n for n in ('CMakeLists.txt', 'sdkconfig.defaults',
                                  'partitions_8mb.csv', 'dependencies.lock')}
    for root in roots:
        files.update(p for p in root.rglob('*') if p.is_file() and '__pycache__' not in p.parts)
    result = {}
    for path in sorted(files):
        require(not path.is_symlink() and path.resolve().is_relative_to(REPO),
                f'Build input escapes repository: {path}')
        result[str(path.relative_to(REPO))] = digest(path)
    return result

def git_exact(path, expected):
    require(Path(run(['git', '-C', path, 'rev-parse', '--show-toplevel'])).resolve() == path.resolve(),
            f'Expected independent SDK/submodule checkout: {path}')
    require(run(['git', '-C', path, 'rev-parse', 'HEAD']) == expected, f'Unsupported SDK/submodule: {path}')
    require(not run(['git', '-C', path, 'status', '--porcelain', '--untracked-files=normal']),
            f'SDK/submodule checkout must be clean: {path}')

def preflight(lock):
    for name in OVERRIDES:
        require(not os.environ.get(name), f'Unsupported build environment override: {name}')
    require(os.environ.get('IDF_PATH'), 'Set IDF_PATH to the pinned official checkout')
    sdk = physical_path(os.environ['IDF_PATH'], 'IDF_PATH')
    physical_path(str(PROJECT), 'project')
    git_exact(sdk, lock['sdk']['commit'])
    require(digest(sdk / 'tools/tools.json') == lock['sdk']['tools_json_sha256'], 'SDK tool manifest changed')
    for rel, commit in lock['sdk']['submodules'].items():
        git_exact(sdk / rel, commit)
    vendor = sdk / 'components/mbedtls/mbedtls'
    framework = run(['git', '-C', vendor, 'rev-parse', 'HEAD:framework'])
    require(framework == lock['sdk']['mbedtls_framework_tree'], 'Unsupported vendor framework tree')
    require(not any(line.startswith(('-', '+', 'U')) for line in
                    run(['git', '-C', sdk, 'submodule', 'status', '--recursive']).splitlines()),
            'Initialize exact SDK submodules recursively')
    python_env = Path(os.environ.get('IDF_PYTHON_ENV_PATH', ''))
    python = python_env / ('Scripts/python.exe' if os.name == 'nt' else 'bin/python')
    require(os.environ.get('IDF_PYTHON_ENV_PATH') and python.is_file(), 'Set IDF_PYTHON_ENV_PATH')
    tools = Path(os.environ.get('IDF_TOOLS_PATH', ''))
    require(os.environ.get('IDF_TOOLS_PATH'), 'Set isolated IDF_TOOLS_PATH (no automatic installation)')
    constraint = tools / 'espidf.constraints.v5.5.txt'
    require(constraint.is_file() and digest(constraint) == lock['python_constraints_sha256'],
            'Official Python constraints do not match the pinned snapshot')
    versions = json.loads(run([python, '-c',
        'import json,sys; from importlib.metadata import version; '
        'print(json.dumps({name:version(name) for name in sys.argv[1:]}))', *lock['python_tools']]))
    require(versions == lock['python_tools'], 'Unsupported Python build tools')
    compiler = shutil.which('xtensa-esp32s3-elf-gcc')
    require(compiler is not None, 'Supported Xtensa compiler is missing from PATH')
    compiler_version = run([compiler, '--version']).splitlines()[0]
    require(compiler_version == lock['compiler_version'], 'Unsupported Xtensa compiler version')
    cmake_version = run(['cmake', '--version']).splitlines()[0]
    require(cmake_version == 'cmake version ' + lock['cmake_version'], 'Unsupported CMake version')
    ninja_version = run(['ninja', '--version'])
    require(ninja_version == lock['ninja_version'], 'Unsupported Ninja version')
    return {'sdk': sdk, 'python': python, 'compiler': Path(compiler),
            'tools': {'compiler': compiler_version, 'cmake': cmake_version, 'ninja': ninja_version, 'python_tools': versions}}

def managed_component(lock, context, required):
    component = PROJECT / 'managed_components/espressif__usb_host_cdc_acm'
    if not component.exists() and not required:
        return  # IDF downloads the immutable component specified in dependencies.lock.
    require(component.is_dir(), 'Pinned USB component was not resolved')
    expected = lock['usb_component']['hash']
    marker = component / '.component_hash'
    require(marker.is_file() and marker.read_text().strip() == expected, 'Unsupported USB component marker')
    # Use the official registry algorithm, not only the mutable marker file.
    code = ('import sys; from idf_component_tools.hash_tools.validate import validate_hash_eq_hashdir; '
            'validate_hash_eq_hashdir(sys.argv[1], sys.argv[2])')
    run([context['python'], '-c', code, component, expected])

def read_config(path):
    require(path.is_file(), 'Generated sdkconfig is missing')
    return dict(line.split('=', 1) for line in path.read_text().splitlines()
                if line.startswith('CONFIG_') and '=' in line)

def inspect_outputs(build, lock, context):
    config = read_config(build / 'sdkconfig')
    for key, expected in lock['required_config'].items():
        require(config.get(key) == expected, f'Generated sdkconfig violates {key}')
    commands = json.loads((build / 'compile_commands.json').read_text())
    app = [item for item in commands if Path(item['file']).resolve() == PROJECT / 'main/app_main.cpp']
    require(len(app) == 1, 'Missing/ambiguous actual app compilation entry')
    args = shlex.split(app[0]['command'])
    require('-std=gnu++2b' in args and not any(a.startswith('-std=') and a != '-std=gnu++2b' for a in args),
            'Actual application must use the approved official C++23 dialect')
    expected_cxx = context['compiler'].with_name('xtensa-esp32s3-elf-g++').resolve()
    require(Path(args[0]).resolve() == expected_cxx, 'Actual app compiler differs from validated toolchain')
    require('v5.5.6' in app[0]['command'] and 'dirty' not in app[0]['command'], 'Dirty/wrong compiled SDK version')
    response = build / 'toolchain/cxxflags'
    require('@' + str(response) in args, 'Missing actual app compiler response file')
    flags = shlex.split(response.read_text())
    for name, gpio in lock['gpio_definitions'].items():
        require(flags.count(f'-D{name}={gpio}') == 1, f'Missing/ambiguous board flag {name}')
    cache = (build / 'CMakeCache.txt').read_text()
    require(str(context['sdk']) in cache, 'Generated build does not reference the validated SDK')
    files = {}
    for rel in ARTIFACTS + ('sdkconfig', 'compile_commands.json', 'toolchain/cxxflags'):
        file = build / rel
        require(file.is_file() and not file.is_symlink() and file.resolve().is_relative_to(build),
                f'Missing/unsafe artifact: {rel}')
        require(file.stat().st_size > 0, f'Empty artifact: {rel}')
        files[rel] = digest(file)
    return files

def verify(build, lock, context):
    proof_file = build / 'provenance.json'
    require(proof_file.is_file(), 'No successful build provenance')
    proof = json.loads(proof_file.read_text())
    require(proof.get('schema_version') == 1, 'Unsupported provenance schema')
    require(proof.get('lock_sha256') == digest(LOCK_FILE), 'Artifact build lock differs')
    require(proof.get('sources') == source_hashes(), 'Artifact sources differ from current inputs')
    require(proof.get('sdk_commit') == lock['sdk']['commit'] and proof.get('tools') == context['tools'],
            'Artifact SDK/toolchain provenance differs')
    require(proof.get('artifacts') == inspect_outputs(build, lock, context), 'Artifact/config hash mismatch')
    managed_component(lock, context, True)
    return proof

def build_fresh(build, lock, context):
    require(not build.exists() or not any(build.iterdir()), 'Build directory must be new/empty; use verify for artifacts')
    require(not build.is_relative_to(context['sdk']) and not PROJECT.is_relative_to(build),
            'Build output must not contain the source or modify the SDK')
    require(not build.is_relative_to(PROJECT), 'Build output must be outside the IDF source directory')
    before = source_hashes()
    managed_component(lock, context, False)
    build.mkdir(parents=True, exist_ok=True)
    flags = ' '.join(f'-D{k}={v}' for k, v in lock['gpio_definitions'].items())
    args = [str(context['python']), str(context['sdk'] / 'tools/idf.py'), '-B', str(build),
            '-DIDF_TARGET=esp32s3', '-DSDKCONFIG=' + str(build / 'sdkconfig'),
            '-DSDKCONFIG_DEFAULTS=' + str(PROJECT / 'sdkconfig.defaults'),
            '-DCMAKE_C_FLAGS=' + flags, '-DCMAKE_CXX_FLAGS=' + flags, 'build']
    print('Building pinned ESP-IDF; log:', build / 'build.log', flush=True)
    with (build / 'build.log').open('w') as log:
        result = subprocess.run(args, cwd=PROJECT, stdout=log, stderr=subprocess.STDOUT, timeout=1200)
    require(result.returncode == 0, f'Official idf.py failed ({result.returncode}); see build.log')
    after_context = preflight(lock)
    require(after_context['tools'] == context['tools'], 'Build environment changed during compilation')
    require(source_hashes() == before, 'Inputs changed during build (including dependency lock)')
    managed_component(lock, context, True)
    artifacts = inspect_outputs(build, lock, context)
    proof = {'schema_version': 1, 'lock_sha256': digest(LOCK_FILE), 'sources': before,
             'sdk_commit': lock['sdk']['commit'], 'tools': context['tools'], 'artifacts': artifacts}
    (build / 'provenance.json').write_text(json.dumps(proof, indent=2, sort_keys=True) + '\n')
    verify(build, lock, context)

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('check', 'build', 'verify'))
    parser.add_argument('--build-dir', type=Path)
    args = parser.parse_args()
    try:
        lock = load_lock()
        context = preflight(lock)  # Reject unsupported SDK/compiler before invoking idf.py.
        if args.action == 'check':
            managed_component(lock, context, False)
        else:
            require(args.build_dir is not None, '--build-dir is required')
            target = physical_path(str(args.build_dir), 'build directory')
            if args.action == 'build':
                build_fresh(target, lock, context)
            else:
                verify(target, lock, context)
        print('PASS', args.action, 'ESP-IDF', lock['sdk']['version'])
        return 0
    except (GateError, KeyError, ValueError, OSError, subprocess.SubprocessError) as exc:
        print('FAIL:', exc, file=sys.stderr)
        return 1

if __name__ == '__main__':
    sys.exit(main())
