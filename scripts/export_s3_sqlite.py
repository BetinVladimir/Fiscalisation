#!/usr/bin/env python3
"""Read-only, offline S3 SQLite extraction. No device/network access or keys.

Only a quiescent acquired database copy is supported. This is not live backup,
cryptographic provenance, a cloud export, or a legal compliance determination.
"""
import argparse
import base64
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import sqlite3
import stat
import tempfile

ROOT = Path(__file__).resolve().parent.parent
SCHEMA_RELATIVE = 'IoT/firmware/edge-agent-s3/idf/main/durable_storage.cpp'
PURPOSES = {
 'schema_version': 'Applied firmware schema markers; version 3 alone does not identify every later table addition.',
 'operations': 'Durable operation identity, payload digest, first transport, route snapshot, state and result.',
 'receipts': 'Local receipt projection and canonical intent payload; not independent physical fiscal proof.',
 'payments': 'Per-tender durable state and terminal/reversal correlation metadata; amounts are minor units.',
 'payment_saga': 'Operation-level durable payment execution/recovery phase.',
 'payment_saga_members': 'Per-tender membership and compensation references.',
 'event_outbox': 'Durable sequenced synchronization events, delivery attempts and acknowledgement metadata; acknowledged history may be pruned.',
 'sync_cursor': 'Singleton durable backend acknowledgement cursor and hash.',
 'frozen_sync_batch': 'Exact pending serialized synchronization batch retained for replay.',
 'frozen_sync_members': 'Exact sequenced membership of the frozen batch.',
 'local_sales': 'Local sale surrogate projection, version, state and payload.',
 'authority_state': 'Active local UNP lease and next allocation counters.',
 'authority_ranges': 'Historical provisioned UNP lease generations.',
 'receipt_authority': 'Receipt allocation/execution UNP and authority snapshot.',
 'sale_unp_claims': 'Unique local sale UNP claims and operation association.',
 'terminal_payment_guard': 'Persistent terminal-scoped operation ownership; unresolved payment quarantine survives restart.',
 'payment_reconciliations': 'Audited exact payment CAS decisions and evidence/authorization references; not independent bank proof.',
 'sqlite_sequence': 'SQLite AUTOINCREMENT high-water marks; included for complete extraction.',
}


def sha(data):
    return hashlib.sha256(data).hexdigest()


def encode_json(value):
    return (json.dumps(value, ensure_ascii=True, sort_keys=True, separators=(',', ':'), allow_nan=False) + '\n').encode('utf-8')


def ident(name):
    return '"' + name.replace('"', '""') + '"'


def schema(db):
    objects = [dict(zip(('type', 'name', 'table', 'sql'), row)) for row in db.execute(
        "SELECT type,name,tbl_name,sql FROM sqlite_master ORDER BY type,name")]
    tables = []
    for item in objects:
        if item['type'] != 'table':
            continue
        name = item['name']
        cols = [dict(zip(('cid', 'name', 'declared_type', 'not_null', 'default_sql', 'primary_key_position'), row))
                for row in db.execute('PRAGMA table_info(' + ident(name) + ')')]
        fk = [dict(zip(('id', 'sequence', 'target_table', 'from_column', 'to_column', 'on_update', 'on_delete', 'match'), row))
              for row in db.execute('PRAGMA foreign_key_list(' + ident(name) + ')')]
        indexes = [dict(zip(('sequence', 'name', 'unique', 'origin', 'partial'), row))
                   for row in db.execute('PRAGMA index_list(' + ident(name) + ')')]
        indexes.sort(key=lambda i: i['name'])
        for index in indexes:
            index['columns'] = [dict(zip(('sequence', 'cid', 'name'), row)) for row in db.execute('PRAGMA index_info(' + ident(index['name']) + ')')]
            index.pop('sequence')  # sqlite's enumeration order is not semantic.
        tables.append({'name': name, 'purpose': PURPOSES.get(name, 'UNCLASSIFIED: update dictionary review'),
                       'columns': cols, 'foreign_keys': fk, 'indexes': indexes})
    return {'objects': objects, 'tables': tables}


def expected_schema(source):
    raw = Path(source).read_bytes()
    text = raw.decode('utf-8')
    match = re.search(r'constexpr\s+const\s+char\s*\*\s*kSchema\s*=\s*R"sql\((.*?)\)sql"', text, re.S)
    if not match:
        raise ValueError('Firmware kSchema definition not found; explicit tool update required')
    db = sqlite3.connect(':memory:')
    # Even a wrong selected source file cannot attach/create external databases.
    forbidden = {sqlite3.SQLITE_ATTACH, sqlite3.SQLITE_DETACH, sqlite3.SQLITE_CREATE_VTABLE}
    db.set_authorizer(lambda action, a, b, database, trigger: sqlite3.SQLITE_DENY
                      if action in forbidden or (action == sqlite3.SQLITE_FUNCTION and b == 'load_extension')
                      else sqlite3.SQLITE_OK)
    try:
        db.executescript(match.group(1))
        # Runtime's only existing ALTER is idempotent for the current schema.
        migrations = re.findall(r'sqlite3_exec\(impl_->db,"(ALTER TABLE [^"\\]+)"', text)
        for sql in migrations:
            try:
                db.execute(sql)
            except sqlite3.OperationalError as error:
                if 'duplicate column name' not in str(error):
                    raise
        result = schema(db)
    finally:
        db.close()
    result['source'] = {'path': SCHEMA_RELATIVE, 'sha256': sha(raw),
                        'schema_sql_sha256': sha(match.group(1).encode()),
                        'migration_sql': migrations}
    return result


class SQLiteTextBytes(bytes):
    """A TEXT storage-class value with invalid UTF-8, preserved without repair."""


def sqlite_text(raw):
    try:
        return raw.decode('utf-8')
    except UnicodeDecodeError:
        return SQLiteTextBytes(raw)


def typed(value):
    if value is None:
        return {'type': 'null'}
    if isinstance(value, SQLiteTextBytes):
        return {'type': 'text_bytes', 'base64': base64.b64encode(value).decode('ascii')}
    if isinstance(value, bytes):
        return {'type': 'blob', 'base64': base64.b64encode(value).decode('ascii')}
    if isinstance(value, str):
        return {'type': 'text', 'value': value}
    if isinstance(value, int):
        return {'type': 'integer', 'decimal': str(value)}
    if isinstance(value, float):
        return {'type': 'real', 'hex': value.hex()}
    raise ValueError('Unsupported SQLite value type')


def file_hash(path):
    digest = hashlib.sha256()
    with path.open('rb') as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b''):
            digest.update(block)
    return digest.hexdigest()


def sidecars(path):
    return [Path(str(path) + suffix) for suffix in ('-wal', '-shm', '-journal') if Path(str(path) + suffix).exists()]


def markdown(dictionary):
    out = ['# Generated ' + dictionary['source'].get('store_label', 'S3 SQLite') + ' dictionary', '',
           'Generated from actual firmware SQL. Field names, SQLite declared types, defaults, keys, relations and executable DDL are authoritative to this source hash; business descriptions require review.', '',
           'Source SHA256: `' + dictionary['source']['sha256'] + '`', '',
           'Schema SQL SHA256: `' + dictionary['source']['schema_sql_sha256'] + '`', '',
           'Recognized forward migrations: `' + json.dumps(dictionary['source']['migration_sql']) + '`', '',
           'No tenant filtering is applied: this database is the complete acquired adapter store, including historical route generations. Text JSON payloads remain exact stored text. Local rows do not prove complete cloud or physical history.', '']
    for table in dictionary['tables']:
        out += ['## ' + table['name'], '', table['purpose'], '', '| Field | SQLite declared type | NOT NULL | PK position | Default SQL |', '|---|---|---|---|---|']
        for col in table['columns']:
            cells = [col['name'], col['declared_type'], str(bool(col['not_null'])), str(col['primary_key_position']), str(col['default_sql'])]
            out.append('| ' + ' | '.join(c.replace('|', '\\|').replace('\n', ' ') for c in cells) + ' |')
        out += ['', 'Foreign keys: `' + json.dumps(table['foreign_keys'], sort_keys=True) + '`', '',
                'Indexes: `' + json.dumps(table['indexes'], sort_keys=True) + '`', '']
    out += ['## SQLite schema definitions (reference, not a restore script)', '', 'SQLite-owned objects such as sqlite_sequence are engine metadata; do not execute their definitions manually. The extractor does not restore or migrate a production database.', '']
    for item in dictionary['objects']:
        if item['sql']:
            out += ['```sql', item['sql'] + ';', '```', '']
    return '\n'.join(out).encode()


def extract(database, output, source, device_id, source_commit, acquisition_note, quiescent_copy=False, *, schema_provider=None, storage_profile=None):
    if not quiescent_copy:
        raise ValueError('Require --quiescent-copy: acquire an offline consistent copy with all writers stopped first')
    original = Path(database).absolute()
    if original.is_symlink() or not stat.S_ISREG(original.stat().st_mode):
        raise ValueError('Input must be a regular acquired database file, not a symlink/device')
    database = original.resolve()
    output = Path(output).absolute()
    if output.exists() or output.is_symlink():
        raise ValueError('Output must not exist; refusing to overwrite evidence')
    output_parent = output.parent.resolve(strict=True)
    output = output_parent / output.name
    if output == database or output in database.parents:
        raise ValueError('Output conflicts with the acquired input')
    if sidecars(database):
        raise ValueError('Input has WAL/SHM/journal sidecars; acquire a quiescent consolidated copy, do not delete sidecars')
    if not re.fullmatch('[0-9a-f]{40}|[0-9a-f]{64}', source_commit):
        raise ValueError('source-commit must be a full lowercase source revision identifier')
    if not device_id.strip() or not acquisition_note.strip():
        raise ValueError('Device and acquisition description are required operator declarations')
    profile = storage_profile or {}
    expected = (schema_provider or expected_schema)(source)
    before = file_hash(database)
    stage = Path(tempfile.mkdtemp(prefix='.s3-extract-', dir=output_parent))
    os.chmod(stage, 0o700)
    db = None
    output_created = False
    files = {}
    def write(name, body):
        path = stage / name
        with path.open('xb') as handle:
            os.chmod(path, 0o600)
            handle.write(body)
        files[name] = {'sha256': sha(body), 'bytes': len(body)}
    try:
        # immutable is safe ONLY for the explicitly declared offline acquired copy.
        # It avoids journal recovery/creation and never opens the source for writes.
        db = sqlite3.connect(database.as_uri() + '?mode=ro&immutable=1', uri=True)
        if hasattr(db, "enable_load_extension"):
            db.enable_load_extension(False)
        db.text_factory = sqlite_text
        db.execute('PRAGMA query_only=ON')
        db.execute('PRAGMA trusted_schema=OFF')
        db.execute('BEGIN')
        actual = schema(db)
        for table in actual["tables"]:
            table["purpose"] = profile.get("table_purposes", {}).get(table["name"], table["purpose"])
        user_version = db.execute('PRAGMA user_version').fetchone()[0]
        if 'user_version' in profile and user_version != profile['user_version']:
            raise ValueError('Database user_version differs from selected storage profile')
        if actual != {k: v for k, v in expected.items() if k != 'source'}:
            raise ValueError('Database schema differs from selected actual firmware schema; no silent omission/migration permitted')
        integrity = [row[0] for row in db.execute('PRAGMA integrity_check')]
        foreign_keys = [list(row) for row in db.execute('PRAGMA foreign_key_check')]
        if integrity != ['ok']:
            raise ValueError('Database integrity_check failed; preserve source and use specialist acquisition/recovery')
        write('dictionary.json', encode_json(expected))
        write('dictionary.md', markdown(expected))
        counts = {}
        for index, table in enumerate(actual['tables']):
            name = table['name']
            columns = [col['name'] for col in table['columns']]
            order = ','.join(ident(col) for col in columns)
            path = stage / ('table-%02d.jsonl' % index)
            digest = hashlib.sha256()
            count = 0
            with path.open('xb') as handle:
                os.chmod(path, 0o600)
                for row in db.execute('SELECT ' + order + ' FROM ' + ident(name) + ' ORDER BY ' + order):
                    line = encode_json([typed(value) for value in row])
                    handle.write(line)
                    digest.update(line)
                    count += 1
            count_check = db.execute('SELECT count(*) FROM ' + ident(name)).fetchone()[0]
            if count_check != count:
                raise ValueError('Export completeness mismatch')
            counts[name] = {'rows': count, 'file': path.name, 'columns': columns}
            files[path.name] = {'sha256': digest.hexdigest(), 'bytes': path.stat().st_size}
        db.rollback()
        db.close()
        db = None
        after = file_hash(database)
        if before != after or sidecars(database):
            raise ValueError('Source changed during extraction; acquired copy was not quiescent')
        manifest = {'format': profile.get('format', 'beefiscal-s3-sqlite-extract-v1'), 'exported_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
                    'scope': profile.get('scope', 'complete acquired S3 SQLite database only'),
                    'status': 'EXPORTED_WITH_FOREIGN_KEY_ISSUES' if foreign_keys else 'EXPORTED',
                    'acquisition': {'device_id_declared': device_id, 'note_declared': acquisition_note,
                                    'quiescent_copy_declared': True, 'source_commit_declared': source_commit,
                                    'provenance_authenticated': False},
                    'input': {'sha256': before, 'bytes': database.stat().st_size},
                    'tool': {'source_sha256': sha(Path(__file__).read_bytes()), 'python_sqlite_version': sqlite3.sqlite_version,
                             **profile.get('tool_metadata', {})},
                    'firmware_schema_source': expected['source'], 'schema_versions': [],
                    'sqlite_user_version': user_version,
                    'entrypoint': profile.get('entrypoint', 'scripts/export_s3_sqlite.py'),
                    'integrity_check': integrity, 'foreign_key_issues': foreign_keys,
                    'tables': counts, 'files': files,
                    'not_included': ['cloud Fiscal/MiniPOS PostgreSQL and audit/event/object stores', 'Android BlueCash and native app databases/preferences',
                                     'MiniPOSWeb IndexedDB/localStorage', 'ESP32-S3 NVS identities/configuration/secrets', 'physical fiscal/POS device memory',
                                     'pruned local records and already lost data', 'authenticated firmware/device provenance or legal determination']}
        # schema_version rows already included losslessly; manifest is a convenience index.
        if 'schema_version' in counts:
            version_file = stage / counts['schema_version']['file']
            manifest['schema_versions'] = [json.loads(line) for line in version_file.read_text().splitlines()]
        if 'not_included' in profile:
            manifest['not_included'] = profile['not_included']
        with (stage / 'manifest.json').open('xb') as handle:
            os.chmod(stage / 'manifest.json', 0o600)
            handle.write(encode_json(manifest))
        # Exclusive mkdir avoids POSIX rename replacing an empty destination.
        # manifest.json is published last and is the completion marker.
        output.mkdir(mode=0o700)
        output_created = True
        for path in stage.iterdir():
            if path.name != 'manifest.json':
                path.rename(output / path.name)
        (stage / 'manifest.json').rename(output / 'manifest.json')
        stage.rmdir()
        return manifest
    except BaseException:
        if db is not None:
            db.close()
        shutil.rmtree(stage, ignore_errors=True)
        if output_created:
            shutil.rmtree(output, ignore_errors=True)
        raise


def decode_typed(cell):
    if not isinstance(cell, dict):
        raise ValueError('Invalid typed cell')
    kind = cell.get('type')
    if kind == 'null' and set(cell) == {'type'}:
        return None
    if kind == 'text' and set(cell) == {'type', 'value'} and isinstance(cell['value'], str):
        return cell['value']
    if kind == 'integer' and set(cell) == {'type', 'decimal'} and isinstance(cell['decimal'], str):
        n = int(cell['decimal'])
        if str(n) == cell['decimal'] and -(2**63) <= n < 2**63:
            return n
    if kind == 'real' and set(cell) == {'type', 'hex'} and isinstance(cell['hex'], str):
        n = float.fromhex(cell['hex'])
        if n.hex() == cell['hex']:
            return n
    if kind == 'text_bytes' and set(cell) == {'type', 'base64'} and isinstance(cell['base64'], str):
        return SQLiteTextBytes(base64.b64decode(cell['base64'], validate=True))
    if kind == 'blob' and set(cell) == {'type', 'base64'} and isinstance(cell['base64'], str):
        return base64.b64decode(cell['base64'], validate=True)
    raise ValueError('Invalid typed cell')


def verify(directory, *, allowed_formats=('beefiscal-s3-sqlite-extract-v1',)):
    """Check accidental corruption/completeness, NOT signatures or provenance."""
    directory = Path(directory).resolve(strict=True)
    manifest_path = directory / 'manifest.json'
    if manifest_path.is_symlink():
        raise ValueError('Symlink manifest rejected')
    manifest = json.loads(manifest_path.read_text())
    if manifest.get('format') not in allowed_formats:
        raise ValueError('Unsupported extraction format')
    files = manifest.get('files')
    if not isinstance(files, dict) or set(p.name for p in directory.iterdir()) != set(files) | {'manifest.json'}:
        raise ValueError('Unexpected or omitted extraction files')
    for name, metadata in files.items():
        if not re.fullmatch(r'(dictionary\.(json|md)|table-[0-9]+\.jsonl)', name):
            raise ValueError('Unsafe extraction filename')
        path = directory / name
        if path.is_symlink() or not path.is_file() or path.parent != directory:
            raise ValueError('Unsafe extraction file')
        if metadata['sha256'] != file_hash(path) or metadata['bytes'] != path.stat().st_size:
            raise ValueError('Extraction artifact hash/size mismatch')
    dictionary = json.loads((directory / 'dictionary.json').read_text())
    expected_tables = {table['name']: table for table in dictionary['tables']}
    if set(manifest['tables']) != set(expected_tables):
        raise ValueError('Table inventory incomplete')
    referenced = {'dictionary.json', 'dictionary.md'}
    for name, table in manifest['tables'].items():
        if table['file'] in referenced or table['file'] not in files:
            raise ValueError('Duplicate/missing table artifact')
        referenced.add(table['file'])
        if table['columns'] != [col['name'] for col in expected_tables[name]['columns']]:
            raise ValueError('Column inventory mismatch')
        count = 0
        with (directory / table['file']).open() as handle:
            for line in handle:
                row = json.loads(line)
                if not isinstance(row, list) or len(row) != len(table['columns']):
                    raise ValueError('Row column count mismatch')
                for cell in row:
                    decode_typed(cell)
                count += 1
        if type(table['rows']) is not int or table['rows'] != count:
            raise ValueError('Row inventory mismatch')
    if referenced != set(files):
        raise ValueError('Unreferenced extraction artifact')
    return {'status': 'ARCHIVE_INTEGRITY_VERIFIED', 'provenance_authenticated': False,
            'tables': len(expected_tables), 'source_status': manifest['status']}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--schema-source', type=Path, default=ROOT / SCHEMA_RELATIVE)
    sub = parser.add_subparsers(dest='command', required=True)
    dictionary = sub.add_parser('dictionary', help='Generate a schema dictionary from actual firmware source')
    dictionary.add_argument('--output', type=Path, required=True)
    verifier = sub.add_parser('verify', help='Check archive hashes/inventory; does not authenticate provenance')
    verifier.add_argument('--directory', type=Path, required=True)
    export = sub.add_parser('export', help='Extract one acquired quiescent S3 SQLite database, without filters')
    export.add_argument('--database', type=Path, required=True)
    export.add_argument('--output', type=Path, required=True)
    export.add_argument('--device-id', required=True)
    export.add_argument('--source-commit', required=True)
    export.add_argument('--acquisition-note', required=True)
    export.add_argument('--quiescent-copy', action='store_true')
    args = parser.parse_args()
    try:
        if args.command == 'verify':
            print(json.dumps(verify(args.directory)))
        elif args.command == 'dictionary':
            dictionary_value = expected_schema(args.schema_source)
            # Exclusive creation: an existing dossier file is never replaced.
            with args.output.open('xb') as handle:
                os.chmod(args.output, 0o600)
                handle.write(markdown(dictionary_value))
        else:
            result = extract(args.database, args.output, args.schema_source, args.device_id,
                             args.source_commit, args.acquisition_note, args.quiescent_copy)
            print(json.dumps({'status': result['status'], 'tables': len(result['tables']), 'input_sha256': result['input']['sha256']}))
            return 2 if result['foreign_key_issues'] else 0
        return 0
    except (ValueError, OSError, sqlite3.Error, KeyError, TypeError) as error:
        parser.exit(1, 'Extraction refused: ' + str(error) + '\n')


if __name__ == '__main__':
    raise SystemExit(main())
