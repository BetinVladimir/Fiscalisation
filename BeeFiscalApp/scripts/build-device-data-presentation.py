#!/usr/bin/env python3
"""Export the synthetic presentation host without changing the application entrypoint."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import uuid

KIND = 'SYNTHETIC_DEVICE_DATA_PRESENTATION'
ENTRYPOINT = './e2e/device-data-presentation-host.tsx'
MANIFEST = 'presentation-build-manifest.json'
FILES = ('package.json', 'package-lock.json', 'app.json', 'app.config.js', 'metro.config.cjs', 'babel.config.js', 'tsconfig.json', 'e2e/device-data-presentation-host.tsx', 'e2e/device-data-fixtures.ts', 'store/config.cjs', 'store/app.json', 'store/icon.png', 'store/adaptive-icon.png', 'store/graphics/thumbnail-128.png')
EXTENSIONS = {'.ts', '.tsx', '.js', '.cjs', '.mjs', '.json', '.png', '.jpg', '.jpeg', '.svg', '.ttf', '.otf', '.webp'}

def hashes(root):
    result = {}
    for file in sorted(root.rglob('*')):
        if file.is_symlink():
            raise ValueError('Symlink in exported output: ' + str(file))
        if file.is_file():
            result[file.relative_to(root).as_posix()] = hashlib.sha256(file.read_bytes()).hexdigest()
    return result

def source_files(root):
    files = [root / name for name in FILES]
    for name in ('src', 'assets', 'plugins'):
        for file in (root / name).rglob('*'):
            if file.is_file() and file.suffix in EXTENSIONS and not any(part in ('node_modules', '__tests__', '__fixtures__') for part in file.relative_to(root).parts) and '.test.' not in file.name and '.spec.' not in file.name:
                files.append(file)
    return files

def shared_snapshot(backend):
    result = {}
    for name in ('design-tokens', 'shared-ui'):
        directory = backend / 'mobile' / name
        if not directory.is_dir():
            raise ValueError('Missing shared dependency: ' + name)
        for file in sorted(directory.rglob('*')):
            relative = file.relative_to(backend)
            if file.is_file() and file.suffix in EXTENSIONS and 'node_modules' not in relative.parts:
                result[relative.as_posix()] = hashlib.sha256(file.read_bytes()).hexdigest()
    return result

def build(root, output, node='node', timeout=600):
    root = Path(root).resolve()
    output = Path(output)
    if not output.is_absolute() or output.is_symlink():
        raise ValueError('--output must be an absolute path, not a symlink')
    output = output.resolve()
    if output == root or output in root.parents or root in output.parents:
        raise ValueError('Output must be outside the application source tree')
    if output.exists() and not output.is_dir():
        raise ValueError('Output must be a directory')
    output.parent.mkdir(parents=True, exist_ok=True)
    # Staging is on the output filesystem, so publication uses atomic renames.
    with tempfile.TemporaryDirectory(prefix='.presentation-build-', dir=output.parent) as workspace:
        workspace = Path(workspace)
        app = workspace / 'Fiscalisation' / 'BeeFiscalApp'
        app.mkdir(parents=True)
        source_hashes = {}
        for file in source_files(root):
            if file.is_symlink() or not file.is_file():
                raise ValueError('Missing or symlinked input: ' + str(file))
            relative = file.relative_to(root)
            target = app / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            data = file.read_bytes()
            target.write_bytes(data)
            source_hashes[relative.as_posix()] = hashlib.sha256(data).hexdigest()
        dependencies = root / 'node_modules'
        backend = root.parent.parent / 'BeeloyBackend'
        if not dependencies.is_dir() or not backend.is_dir():
            raise ValueError('Installed node_modules and sibling BeeloyBackend are required')
        (app / 'node_modules').symlink_to(dependencies, target_is_directory=True)
        (workspace / 'BeeloyBackend').symlink_to(backend, target_is_directory=True)
        shared_hashes = shared_snapshot(backend)
        package = json.loads((app / 'package.json').read_text())
        package['main'] = ENTRYPOINT
        (app / 'package.json').write_text(json.dumps(package, indent=2) + '\n')
        staged = workspace / 'export'
        # Do not forward arbitrary EXPO_PUBLIC values or credentials into a bundle.
        env = {key: value for key, value in os.environ.items() if key in ('PATH', 'HOME', 'TMPDIR', 'SYSTEMROOT')}
        env.update(CI='1', EXPO_NO_TELEMETRY='1', EXPO_NO_DOTENV='1', APP_VARIANT='demo', EXPO_PUBLIC_APP_ENV='demo', EXPO_PUBLIC_FISCAL_API_URL='http://fiscal-admin.test/public/v1')
        subprocess.run([node, str(dependencies / 'expo/bin/cli'), 'export', '--clear', '--platform', 'web', '--output-dir', str(staged)], cwd=app, env=env, check=True, timeout=timeout)
        if shared_snapshot(backend) != shared_hashes:
            raise ValueError('Shared source changed during export')
        if not (staged / 'index.html').is_file():
            raise ValueError('Expo export has no index.html')
        artifact_hashes = hashes(staged)
        manifest = {'schema_version': 1, 'kind': KIND, 'entrypoint': ENTRYPOINT, 'source_sha256': source_hashes, 'shared_source_sha256': shared_hashes, 'export_sha256': artifact_hashes, 'production_entrypoint_changed': False}
        (staged / MANIFEST).write_text(json.dumps(manifest, sort_keys=True, indent=2) + '\n')
        backup = output.parent / ('.presentation-backup-' + uuid.uuid4().hex)
        existed = output.exists()
        if existed:
            output.rename(backup)
        try:
            staged.rename(output)
        except BaseException:
            if existed:
                backup.rename(output)
            raise
        if existed:
            shutil.rmtree(backup)
        return manifest

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--timeout', type=int, default=600)
    args = parser.parse_args()
    if args.timeout <= 0:
        parser.error('--timeout must be positive')
    build(Path(__file__).resolve().parents[1], args.output, timeout=args.timeout)
