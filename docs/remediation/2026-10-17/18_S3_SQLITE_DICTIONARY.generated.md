# Generated S3 SQLite dictionary

Generated from actual firmware SQL. Field names, SQLite declared types, defaults, keys, relations and executable DDL are authoritative to this source hash; business descriptions require review.

Source SHA256: `b0cb32ff113a83cc1d72ef5d6c55ce6d5e4ba3f6e26ea407ec6c0598ca05065d`

Schema SQL SHA256: `80e0b9539a16897733296a8cb28c3fee250013c06601e41cbe09317ec372136b`

Recognized forward migrations: `["ALTER TABLE sync_cursor ADD COLUMN committed_event_hash TEXT"]`

No tenant filtering is applied: this database is the complete acquired adapter store, including historical route generations. Text JSON payloads remain exact stored text. Local rows do not prove complete cloud or physical history.

## authority_ranges

Historical provisioned UNP lease generations.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| generation | INTEGER | False | 1 | None |
| prefix | TEXT | True | 0 | None |
| range_start | INTEGER | True | 0 | None |
| range_end | INTEGER | True | 0 | None |

Foreign keys: `[]`

Indexes: `[]`

## authority_state

Active local UNP lease and next allocation counters.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| singleton | INTEGER | False | 1 | None |
| prefix | TEXT | True | 0 | None |
| range_start | INTEGER | True | 0 | None |
| range_end | INTEGER | True | 0 | None |
| next_unp | INTEGER | True | 0 | None |
| next_operation | INTEGER | True | 0 | None |
| generation | INTEGER | True | 0 | None |

Foreign keys: `[]`

Indexes: `[]`

## event_outbox

Durable sequenced synchronization events, delivery attempts and acknowledgement metadata; acknowledged history may be pruned.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| sequence | INTEGER | False | 1 | None |
| event_id | TEXT | True | 0 | None |
| operation_id | TEXT | True | 0 | None |
| kind | TEXT | True | 0 | None |
| payload | TEXT | True | 0 | None |
| payload_digest | TEXT | True | 0 | None |
| created_at | INTEGER | True | 0 | None |
| attempt_count | INTEGER | True | 0 | 0 |
| last_attempt_at | INTEGER | False | 0 | None |
| last_error | TEXT | False | 0 | None |
| acknowledged_at | INTEGER | False | 0 | None |
| ack_id | TEXT | False | 0 | None |

Foreign keys: `[{"from_column": "operation_id", "id": 0, "match": "NONE", "on_delete": "NO ACTION", "on_update": "NO ACTION", "sequence": 0, "target_table": "operations", "to_column": "operation_id"}]`

Indexes: `[{"columns": [{"cid": 10, "name": "acknowledged_at", "sequence": 0}, {"cid": 0, "name": "sequence", "sequence": 1}], "name": "idx_event_pending", "origin": "c", "partial": 0, "unique": 0}, {"columns": [{"cid": 10, "name": "acknowledged_at", "sequence": 0}, {"cid": 6, "name": "created_at", "sequence": 1}], "name": "idx_event_retention", "origin": "c", "partial": 0, "unique": 0}, {"columns": [{"cid": 1, "name": "event_id", "sequence": 0}], "name": "sqlite_autoindex_event_outbox_1", "origin": "u", "partial": 0, "unique": 1}]`

## frozen_sync_batch

Exact pending serialized synchronization batch retained for replay.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| singleton | INTEGER | False | 1 | None |
| batch_id | TEXT | True | 0 | None |
| body | TEXT | True | 0 | None |
| first_sequence | INTEGER | True | 0 | None |
| last_sequence | INTEGER | True | 0 | None |
| last_event_hash | TEXT | True | 0 | None |
| previous_hash | TEXT | True | 0 | None |

Foreign keys: `[]`

Indexes: `[]`

## frozen_sync_members

Exact sequenced membership of the frozen batch.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| sequence | INTEGER | False | 1 | None |
| event_id | TEXT | True | 0 | None |
| operation_id | TEXT | True | 0 | None |
| kind | TEXT | True | 0 | None |
| payload | TEXT | True | 0 | None |
| payload_digest | TEXT | True | 0 | None |
| created_at | INTEGER | True | 0 | None |

Foreign keys: `[]`

Indexes: `[]`

## local_sales

Local sale surrogate projection, version, state and payload.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| surrogate_id | TEXT | False | 1 | None |
| version | INTEGER | True | 0 | None |
| state | TEXT | True | 0 | None |
| payload | TEXT | True | 0 | None |
| created_at | INTEGER | True | 0 | None |
| updated_at | INTEGER | True | 0 | None |

Foreign keys: `[]`

Indexes: `[{"columns": [{"cid": 0, "name": "surrogate_id", "sequence": 0}], "name": "sqlite_autoindex_local_sales_1", "origin": "pk", "partial": 0, "unique": 1}]`

## operations

Durable operation identity, payload digest, first transport, route snapshot, state and result.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| operation_id | TEXT | False | 1 | None |
| payload_digest | TEXT | True | 0 | None |
| first_transport | TEXT | True | 0 | None |
| route_snapshot | TEXT | True | 0 | None |
| state | TEXT | True | 0 | 'RESERVED' |
| result_payload | TEXT | False | 0 | None |
| created_at | INTEGER | True | 0 | None |
| updated_at | INTEGER | True | 0 | None |

Foreign keys: `[]`

Indexes: `[{"columns": [{"cid": 0, "name": "operation_id", "sequence": 0}], "name": "sqlite_autoindex_operations_1", "origin": "pk", "partial": 0, "unique": 1}]`

## payment_reconciliations

Audited exact payment CAS decisions and evidence/authorization references; not independent bank proof.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| reconciliation_id | TEXT | False | 1 | None |
| terminal_id | TEXT | True | 0 | None |
| payment_id | TEXT | True | 0 | None |
| operation_id | TEXT | True | 0 | None |
| actor_id | TEXT | True | 0 | None |
| evidence_reference | TEXT | True | 0 | None |
| evidence_sha256 | TEXT | True | 0 | None |
| authorization_sha256 | TEXT | True | 0 | None |
| old_state | TEXT | True | 0 | None |
| new_state | TEXT | True | 0 | None |
| created_at | INTEGER | True | 0 | None |

Foreign keys: `[{"from_column": "operation_id", "id": 0, "match": "NONE", "on_delete": "NO ACTION", "on_update": "NO ACTION", "sequence": 0, "target_table": "operations", "to_column": "operation_id"}, {"from_column": "payment_id", "id": 1, "match": "NONE", "on_delete": "NO ACTION", "on_update": "NO ACTION", "sequence": 0, "target_table": "payments", "to_column": "payment_id"}]`

Indexes: `[{"columns": [{"cid": 0, "name": "reconciliation_id", "sequence": 0}], "name": "sqlite_autoindex_payment_reconciliations_1", "origin": "pk", "partial": 0, "unique": 1}]`

## payment_saga

Operation-level durable payment execution/recovery phase.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| operation_id | TEXT | False | 1 | None |
| phase | TEXT | True | 0 | None |
| updated_at | INTEGER | True | 0 | None |

Foreign keys: `[{"from_column": "operation_id", "id": 0, "match": "NONE", "on_delete": "NO ACTION", "on_update": "NO ACTION", "sequence": 0, "target_table": "operations", "to_column": "operation_id"}]`

Indexes: `[{"columns": [{"cid": 0, "name": "operation_id", "sequence": 0}], "name": "sqlite_autoindex_payment_saga_1", "origin": "pk", "partial": 0, "unique": 1}]`

## payment_saga_members

Per-tender membership and compensation references.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| operation_id | TEXT | True | 0 | None |
| payment_id | TEXT | False | 1 | None |
| reversal_reference | TEXT | False | 0 | None |
| reversal_rrn | TEXT | False | 0 | None |
| reversal_auth_code | TEXT | False | 0 | None |

Foreign keys: `[{"from_column": "payment_id", "id": 0, "match": "NONE", "on_delete": "NO ACTION", "on_update": "NO ACTION", "sequence": 0, "target_table": "payments", "to_column": "payment_id"}, {"from_column": "operation_id", "id": 1, "match": "NONE", "on_delete": "NO ACTION", "on_update": "NO ACTION", "sequence": 0, "target_table": "payment_saga", "to_column": "operation_id"}]`

Indexes: `[{"columns": [{"cid": 1, "name": "payment_id", "sequence": 0}], "name": "sqlite_autoindex_payment_saga_members_1", "origin": "pk", "partial": 0, "unique": 1}]`

## payments

Per-tender durable state and terminal/reversal correlation metadata; amounts are minor units.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| payment_id | TEXT | False | 1 | None |
| operation_id | TEXT | True | 0 | None |
| receipt_id | TEXT | False | 0 | None |
| state | TEXT | True | 0 | None |
| amount_minor | INTEGER | True | 0 | None |
| terminal_reference | TEXT | False | 0 | None |
| rrn | TEXT | False | 0 | None |
| auth_code | TEXT | False | 0 | None |
| created_at | INTEGER | True | 0 | None |
| updated_at | INTEGER | True | 0 | None |

Foreign keys: `[{"from_column": "receipt_id", "id": 0, "match": "NONE", "on_delete": "NO ACTION", "on_update": "NO ACTION", "sequence": 0, "target_table": "receipts", "to_column": "receipt_id"}, {"from_column": "operation_id", "id": 1, "match": "NONE", "on_delete": "NO ACTION", "on_update": "NO ACTION", "sequence": 0, "target_table": "operations", "to_column": "operation_id"}]`

Indexes: `[{"columns": [{"cid": 0, "name": "payment_id", "sequence": 0}], "name": "sqlite_autoindex_payments_1", "origin": "pk", "partial": 0, "unique": 1}]`

## receipt_authority

Receipt allocation/execution UNP and authority snapshot.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| receipt_id | TEXT | False | 1 | None |
| allocated_unp | TEXT | True | 0 | None |
| execution_unp | TEXT | True | 0 | None |
| operation_sequence | INTEGER | True | 0 | None |
| unp_sequence | INTEGER | True | 0 | None |
| prefix | TEXT | True | 0 | None |
| range_start | INTEGER | True | 0 | None |
| range_end | INTEGER | True | 0 | None |
| generation | INTEGER | True | 0 | None |

Foreign keys: `[{"from_column": "receipt_id", "id": 0, "match": "NONE", "on_delete": "NO ACTION", "on_update": "NO ACTION", "sequence": 0, "target_table": "receipts", "to_column": "receipt_id"}]`

Indexes: `[{"columns": [{"cid": 0, "name": "receipt_id", "sequence": 0}], "name": "sqlite_autoindex_receipt_authority_1", "origin": "pk", "partial": 0, "unique": 1}]`

## receipts

Local receipt projection and canonical intent payload; not independent physical fiscal proof.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| receipt_id | TEXT | False | 1 | None |
| operation_id | TEXT | True | 0 | None |
| state | TEXT | True | 0 | None |
| canonical_payload | TEXT | True | 0 | None |
| created_at | INTEGER | True | 0 | None |
| updated_at | INTEGER | True | 0 | None |

Foreign keys: `[{"from_column": "operation_id", "id": 0, "match": "NONE", "on_delete": "NO ACTION", "on_update": "NO ACTION", "sequence": 0, "target_table": "operations", "to_column": "operation_id"}]`

Indexes: `[{"columns": [{"cid": 0, "name": "receipt_id", "sequence": 0}], "name": "sqlite_autoindex_receipts_1", "origin": "pk", "partial": 0, "unique": 1}]`

## sale_unp_claims

Unique local sale UNP claims and operation association.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| unp | TEXT | False | 1 | None |
| operation_id | TEXT | True | 0 | None |
| serial | TEXT | True | 0 | None |
| sequence | INTEGER | True | 0 | None |

Foreign keys: `[{"from_column": "operation_id", "id": 0, "match": "NONE", "on_delete": "NO ACTION", "on_update": "NO ACTION", "sequence": 0, "target_table": "operations", "to_column": "operation_id"}]`

Indexes: `[{"columns": [{"cid": 0, "name": "unp", "sequence": 0}], "name": "sqlite_autoindex_sale_unp_claims_1", "origin": "pk", "partial": 0, "unique": 1}, {"columns": [{"cid": 1, "name": "operation_id", "sequence": 0}], "name": "sqlite_autoindex_sale_unp_claims_2", "origin": "u", "partial": 0, "unique": 1}, {"columns": [{"cid": 2, "name": "serial", "sequence": 0}, {"cid": 3, "name": "sequence", "sequence": 1}], "name": "sqlite_autoindex_sale_unp_claims_3", "origin": "u", "partial": 0, "unique": 1}]`

## schema_version

Applied firmware schema markers; version 3 alone does not identify every later table addition.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| version | INTEGER | False | 1 | None |
| applied_at | INTEGER | True | 0 | None |

Foreign keys: `[]`

Indexes: `[]`

## sqlite_sequence

SQLite AUTOINCREMENT high-water marks; included for complete extraction.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| name |  | False | 0 | None |
| seq |  | False | 0 | None |

Foreign keys: `[]`

Indexes: `[]`

## sync_cursor

Singleton durable backend acknowledgement cursor and hash.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| singleton | INTEGER | False | 1 | None |
| acknowledged_sequence | INTEGER | True | 0 | 0 |
| ack_id | TEXT | False | 0 | None |
| committed_event_hash | TEXT | False | 0 | None |
| updated_at | INTEGER | True | 0 | 0 |

Foreign keys: `[]`

Indexes: `[]`

## terminal_payment_guard

Persistent terminal-scoped operation ownership; unresolved payment quarantine survives restart.

| Field | SQLite declared type | NOT NULL | PK position | Default SQL |
|---|---|---|---|---|
| terminal_id | TEXT | False | 1 | None |
| operation_id | TEXT | True | 0 | None |
| created_at | INTEGER | True | 0 | None |

Foreign keys: `[{"from_column": "operation_id", "id": 0, "match": "NONE", "on_delete": "NO ACTION", "on_update": "NO ACTION", "sequence": 0, "target_table": "operations", "to_column": "operation_id"}]`

Indexes: `[{"columns": [{"cid": 0, "name": "terminal_id", "sequence": 0}], "name": "sqlite_autoindex_terminal_payment_guard_1", "origin": "pk", "partial": 0, "unique": 1}, {"columns": [{"cid": 1, "name": "operation_id", "sequence": 0}], "name": "sqlite_autoindex_terminal_payment_guard_2", "origin": "u", "partial": 0, "unique": 1}]`

## SQLite schema definitions (reference, not a restore script)

SQLite-owned objects such as sqlite_sequence are engine metadata; do not execute their definitions manually. The extractor does not restore or migrate a production database.

```sql
CREATE INDEX idx_event_pending ON event_outbox(acknowledged_at,sequence);
```

```sql
CREATE INDEX idx_event_retention ON event_outbox(acknowledged_at,created_at);
```

```sql
CREATE TABLE authority_ranges(generation INTEGER PRIMARY KEY,prefix TEXT NOT NULL,range_start INTEGER NOT NULL,range_end INTEGER NOT NULL);
```

```sql
CREATE TABLE authority_state(singleton INTEGER PRIMARY KEY CHECK(singleton=1),
 prefix TEXT NOT NULL,range_start INTEGER NOT NULL,range_end INTEGER NOT NULL,
 next_unp INTEGER NOT NULL,next_operation INTEGER NOT NULL,generation INTEGER NOT NULL,
 CHECK(range_start>0),CHECK(range_end>=range_start),CHECK(next_unp>=range_start));
```

```sql
CREATE TABLE event_outbox(
 sequence INTEGER PRIMARY KEY AUTOINCREMENT,event_id TEXT NOT NULL UNIQUE,
 operation_id TEXT NOT NULL REFERENCES operations(operation_id),kind TEXT NOT NULL,
 payload TEXT NOT NULL,payload_digest TEXT NOT NULL,created_at INTEGER NOT NULL,
 attempt_count INTEGER NOT NULL DEFAULT 0,last_attempt_at INTEGER,last_error TEXT,
 acknowledged_at INTEGER,ack_id TEXT,CHECK(length(payload_digest)=64));
```

```sql
CREATE TABLE frozen_sync_batch(singleton INTEGER PRIMARY KEY CHECK(singleton=1),
 batch_id TEXT NOT NULL,body TEXT NOT NULL,first_sequence INTEGER NOT NULL,last_sequence INTEGER NOT NULL,
 last_event_hash TEXT NOT NULL,previous_hash TEXT NOT NULL);
```

```sql
CREATE TABLE frozen_sync_members(sequence INTEGER PRIMARY KEY,event_id TEXT NOT NULL,
 operation_id TEXT NOT NULL,kind TEXT NOT NULL,payload TEXT NOT NULL,payload_digest TEXT NOT NULL,created_at INTEGER NOT NULL);
```

```sql
CREATE TABLE local_sales(
 surrogate_id TEXT PRIMARY KEY,version INTEGER NOT NULL,state TEXT NOT NULL,
 payload TEXT NOT NULL,created_at INTEGER NOT NULL,updated_at INTEGER NOT NULL);
```

```sql
CREATE TABLE operations(
 operation_id TEXT PRIMARY KEY,payload_digest TEXT NOT NULL,first_transport TEXT NOT NULL,
 route_snapshot TEXT NOT NULL,state TEXT NOT NULL DEFAULT 'RESERVED',result_payload TEXT,
 created_at INTEGER NOT NULL,updated_at INTEGER NOT NULL,
 CHECK(length(operation_id) BETWEEN 1 AND 128),CHECK(length(payload_digest)=64));
```

```sql
CREATE TABLE payment_reconciliations(reconciliation_id TEXT PRIMARY KEY,terminal_id TEXT NOT NULL,payment_id TEXT NOT NULL REFERENCES payments(payment_id),operation_id TEXT NOT NULL REFERENCES operations(operation_id),actor_id TEXT NOT NULL,evidence_reference TEXT NOT NULL,evidence_sha256 TEXT NOT NULL,authorization_sha256 TEXT NOT NULL,old_state TEXT NOT NULL,new_state TEXT NOT NULL,created_at INTEGER NOT NULL);
```

```sql
CREATE TABLE payment_saga(
 operation_id TEXT PRIMARY KEY REFERENCES operations(operation_id),phase TEXT NOT NULL,updated_at INTEGER NOT NULL);
```

```sql
CREATE TABLE payment_saga_members(
 operation_id TEXT NOT NULL REFERENCES payment_saga(operation_id),payment_id TEXT PRIMARY KEY REFERENCES payments(payment_id),reversal_reference TEXT,reversal_rrn TEXT,reversal_auth_code TEXT);
```

```sql
CREATE TABLE payments(
 payment_id TEXT PRIMARY KEY,operation_id TEXT NOT NULL REFERENCES operations(operation_id),
 receipt_id TEXT REFERENCES receipts(receipt_id),state TEXT NOT NULL,amount_minor INTEGER NOT NULL,
 terminal_reference TEXT,rrn TEXT,auth_code TEXT,created_at INTEGER NOT NULL,updated_at INTEGER NOT NULL);
```

```sql
CREATE TABLE receipt_authority(receipt_id TEXT PRIMARY KEY REFERENCES receipts(receipt_id),allocated_unp TEXT NOT NULL,execution_unp TEXT NOT NULL,operation_sequence INTEGER NOT NULL,unp_sequence INTEGER NOT NULL,prefix TEXT NOT NULL,range_start INTEGER NOT NULL,range_end INTEGER NOT NULL,generation INTEGER NOT NULL);
```

```sql
CREATE TABLE receipts(
 receipt_id TEXT PRIMARY KEY,operation_id TEXT NOT NULL REFERENCES operations(operation_id),
 state TEXT NOT NULL,canonical_payload TEXT NOT NULL,created_at INTEGER NOT NULL,updated_at INTEGER NOT NULL);
```

```sql
CREATE TABLE sale_unp_claims(unp TEXT PRIMARY KEY,operation_id TEXT NOT NULL UNIQUE REFERENCES operations(operation_id),serial TEXT NOT NULL CHECK(length(serial)=8),sequence INTEGER NOT NULL CHECK(sequence BETWEEN 1 AND 9999999),UNIQUE(serial,sequence),CHECK(length(unp)=21));
```

```sql
CREATE TABLE schema_version(version INTEGER PRIMARY KEY,applied_at INTEGER NOT NULL);
```

```sql
CREATE TABLE sqlite_sequence(name,seq);
```

```sql
CREATE TABLE sync_cursor(singleton INTEGER PRIMARY KEY CHECK(singleton=1),
 acknowledged_sequence INTEGER NOT NULL DEFAULT 0,ack_id TEXT,committed_event_hash TEXT,
 updated_at INTEGER NOT NULL DEFAULT 0);
```

```sql
CREATE TABLE terminal_payment_guard(terminal_id TEXT PRIMARY KEY,operation_id TEXT NOT NULL UNIQUE REFERENCES operations(operation_id),created_at INTEGER NOT NULL);
```
