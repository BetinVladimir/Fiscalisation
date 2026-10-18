# Generated BlueCash Android SQLite dictionary

Generated from actual firmware SQL. Field names, SQLite declared types, defaults, keys, relations and executable DDL are authoritative to this source hash; business descriptions require review.

Source SHA256: `4d75ca42c2e22281e75f2067d2f1d42cc598aa1680e163e3deb2ce347501593b`

Schema SQL SHA256: `0d4653db88cb2bde3cd4b9d8d8c148d98b54c3c015901fdd46e9660a63c54557`

Recognized forward migrations: `[]`

No tenant filtering is applied: this database is the complete acquired adapter store, including historical route generations. Text JSON payloads remain exact stored text. Local rows do not prove complete cloud or physical history.

## android_metadata

Android SQLite platform locale metadata; not application business data.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| locale | TEXT | False | 0 | None |

Foreign keys: `[]`

Indexes: `[]`

## ble_local_sale

Local BLE sale surrogate and exact serialized payload.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| surrogate_id | TEXT | False | 1 | None |
| payload | TEXT | True | 0 | None |
| updated_at | TEXT | True | 0 | None |

Foreign keys: `[]`

Indexes: `[{"columns": [{"cid": 0, "name": "surrogate_id", "sequence": 0}], "name": "sqlite_autoindex_ble_local_sale_1", "origin": "pk", "partial": 0, "unique": 1}]`

## ble_unp_range

Local UNP lease prefix and next/end sequence.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| singleton | INTEGER | False | 1 | None |
| prefix | TEXT | True | 0 | None |
| next_sequence | INTEGER | True | 0 | None |
| end_sequence | INTEGER | True | 0 | None |

Foreign keys: `[]`

Indexes: `[]`

## journal_event

Retained signed journal events; operation correlation, exact payload, hash chain and ACK metadata. Signatures are exported, not verified.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| journal_seq | INTEGER | False | 1 | None |
| event_id | TEXT | True | 0 | None |
| operation_id | TEXT | True | 0 | None |
| event_type | TEXT | True | 0 | None |
| occurred_at | TEXT | True | 0 | None |
| payload | TEXT | True | 0 | None |
| prev_hash | TEXT | False | 0 | None |
| event_hash | TEXT | True | 0 | None |
| signing_kid | TEXT | True | 0 | None |
| signature | TEXT | True | 0 | None |
| backend_ack_id | TEXT | False | 0 | None |
| acknowledged_at | TEXT | False | 0 | None |

Foreign keys: `[]`

Indexes: `[{"columns": [{"cid": 2, "name": "operation_id", "sequence": 0}, {"cid": 0, "name": "journal_seq", "sequence": 1}], "name": "journal_operation_idx", "origin": "c", "partial": 0, "unique": 0}, {"columns": [{"cid": 1, "name": "event_id", "sequence": 0}], "name": "sqlite_autoindex_journal_event_1", "origin": "u", "partial": 0, "unique": 1}, {"columns": [{"cid": 7, "name": "event_hash", "sequence": 0}], "name": "sqlite_autoindex_journal_event_2", "origin": "u", "partial": 0, "unique": 1}]`

## payment_transaction

Local card transaction amount in minor units, state, RRN and authorization reference.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| operation_id | TEXT | False | 1 | None |
| amount_minor | INTEGER | True | 0 | None |
| rrn | TEXT | False | 0 | None |
| authorization_code | TEXT | False | 0 | None |
| state | TEXT | True | 0 | None |
| updated_at | TEXT | True | 0 | None |

Foreign keys: `[]`

Indexes: `[{"columns": [{"cid": 0, "name": "operation_id", "sequence": 0}], "name": "sqlite_autoindex_payment_transaction_1", "origin": "pk", "partial": 0, "unique": 1}]`

## sqlite_sequence

SQLite AUTOINCREMENT high-water mark.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| name |  | False | 0 | None |
| seq |  | False | 0 | None |

Foreign keys: `[]`

Indexes: `[]`

## sync_checkpoint

Durable acknowledged journal prefix. Earlier acknowledged history may have been pruned.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| singleton | INTEGER | False | 1 | None |
| committed_through_seq | INTEGER | True | 0 | None |
| committed_event_hash | TEXT | False | 0 | None |
| ack_id | TEXT | False | 0 | None |
| acknowledged_at | TEXT | False | 0 | None |

Foreign keys: `[]`

Indexes: `[]`

## SQLite schema definitions (reference, not a restore script)

SQLite-owned objects such as sqlite_sequence are engine metadata; do not execute their definitions manually. The extractor does not restore or migrate a production database.

```sql
CREATE INDEX journal_operation_idx ON journal_event(operation_id,journal_seq);
```

```sql
CREATE TABLE android_metadata (locale TEXT);
```

```sql
CREATE TABLE ble_local_sale(surrogate_id TEXT PRIMARY KEY,payload TEXT NOT NULL,updated_at TEXT NOT NULL);
```

```sql
CREATE TABLE ble_unp_range(singleton INTEGER PRIMARY KEY CHECK(singleton=1),prefix TEXT NOT NULL,next_sequence INTEGER NOT NULL,end_sequence INTEGER NOT NULL);
```

```sql
CREATE TABLE journal_event(journal_seq INTEGER PRIMARY KEY AUTOINCREMENT,event_id TEXT NOT NULL UNIQUE,operation_id TEXT NOT NULL,event_type TEXT NOT NULL,occurred_at TEXT NOT NULL,payload TEXT NOT NULL,prev_hash TEXT,event_hash TEXT NOT NULL UNIQUE,signing_kid TEXT NOT NULL,signature TEXT NOT NULL,backend_ack_id TEXT,acknowledged_at TEXT);
```

```sql
CREATE TABLE payment_transaction(operation_id TEXT PRIMARY KEY,amount_minor INTEGER NOT NULL,rrn TEXT,authorization_code TEXT,state TEXT NOT NULL,updated_at TEXT NOT NULL);
```

```sql
CREATE TABLE sqlite_sequence(name,seq);
```

```sql
CREATE TABLE sync_checkpoint(singleton INTEGER PRIMARY KEY CHECK(singleton=1),committed_through_seq INTEGER NOT NULL,committed_event_hash TEXT,ack_id TEXT,acknowledged_at TEXT);
```
