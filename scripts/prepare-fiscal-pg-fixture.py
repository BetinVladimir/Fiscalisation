#!/usr/bin/env python3
"""Prepare disposable SQL fixtures; never apply or edit canonical migrations."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import stat


def prepare(source, output, namespace):
    source, output = Path(source).resolve(), Path(output)
    if namespace not in ('minipos', 'fiscal'):
        raise ValueError('Unsupported fixture namespace')
    if output.exists():
        raise ValueError('Fixture output must not exist')
    migrations = sorted(source.glob('[0-9][0-9][0-9]_*.sql'))
    if not migrations:
        raise ValueError('No SQL migrations found')
    shell_files = sorted(source.glob('*.sh'))
    if any(file.name != '004_runtime_login.sh' for file in shell_files):
        raise ValueError('Unreviewed fixture shell script')
    login = source / '004_runtime_login.sh'
    if not login.is_file() or login.is_symlink():
        raise ValueError('Required runtime login fixture is missing or symlinked')
    login_bytes = login.read_bytes()
    prepared = {}
    manifest = {'kind': 'DISPOSABLE_LOCAL_FISCAL_SQL_FIXTURE', 'namespace': namespace, 'production_migration': False, 'migrations': {}, 'runtime_scripts': {'004_runtime_login.sh': {'source_sha256': hashlib.sha256(login_bytes).hexdigest(), 'fixture_sha256': hashlib.sha256(login_bytes).hexdigest()}}}
    for migration in migrations:
        original = migration.read_text()
        number = int(migration.name[:3])
        if namespace == 'minipos' or number <= 21:
            transformed = original
            transformed = re.sub(r'SET search_path\s*=\s*public,\s*pg_temp', 'SET search_path = ' + namespace + ', pg_temp', transformed)
            transformed = 'SET search_path=' + namespace + ',public;\n' + transformed
        elif number == 22:
            if 'fiscal.fiscal_runtime_audit' not in original:
                raise ValueError('Unexpected migration022 namespace; review fixture adapter')
            transformed = original
        else:
            raise ValueError('Unreviewed migration requires explicit fixture adapter: ' + migration.name)
        prepared[migration.name] = transformed
        manifest['migrations'][migration.name] = {'source_sha256': hashlib.sha256(original.encode()).hexdigest(), 'fixture_sha256': hashlib.sha256(transformed.encode()).hexdigest()}
    role = 'beefiscal_tenant' if namespace == 'fiscal' else 'beeminipos_tenant'
    prepared['000_fixture_namespace.sql'] = 'CREATE SCHEMA ' + namespace + ';\n'
    prepared['999_fixture_namespace.sql'] = 'GRANT USAGE ON SCHEMA ' + namespace + ' TO ' + role + ';\n' + "DO $$ BEGIN EXECUTE format('ALTER DATABASE %I SET search_path=" + namespace + ",public',current_database()); END $$;\n"
    output.mkdir(parents=True)
    for name, data in prepared.items():
        (output / name).write_text(data)
    (output / login.name).write_bytes(login_bytes)
    (output / login.name).chmod(stat.S_IMODE(login.stat().st_mode))
    (output / 'fixture-manifest.json').write_text(json.dumps(manifest, indent=2, sort_keys=True) + '\n')
    return manifest

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, default=Path(__file__).resolve().parents[1] / 'database/fiscal')
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--namespace', choices=('minipos', 'fiscal'), required=True)
    args = parser.parse_args()
    prepare(args.source, args.output, args.namespace)
