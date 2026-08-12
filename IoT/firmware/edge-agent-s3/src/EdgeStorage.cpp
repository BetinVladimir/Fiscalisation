#include "EdgeStorage.h"

#ifndef EDGE_SD_CLK
#define EDGE_SD_CLK -1
#endif
#ifndef EDGE_SD_CMD
#define EDGE_SD_CMD -1
#endif
#ifndef EDGE_SD_D0
#define EDGE_SD_D0 -1
#endif

namespace beefiscal::edge {
namespace {
class Statement final {
public:
    explicit Statement(sqlite3_stmt* statement) : statement_(statement) {}
    ~Statement() { if (statement_) sqlite3_finalize(statement_); }
    sqlite3_stmt* get() const { return statement_; }
private:
    sqlite3_stmt* statement_;
};

constexpr const char* kSchema = R"sql(
CREATE TABLE IF NOT EXISTS schema_version (
  version INTEGER PRIMARY KEY,
  applied_at_unix INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS transaction_journal (
  sequence INTEGER PRIMARY KEY AUTOINCREMENT,
  transaction_id TEXT NOT NULL UNIQUE,
  kind TEXT NOT NULL,
  canonical_payload TEXT NOT NULL,
  signature TEXT NOT NULL,
  created_at_unix INTEGER NOT NULL,
  synced_at_unix INTEGER,
  attempt_count INTEGER NOT NULL DEFAULT 0,
  last_attempt_at_unix INTEGER,
  last_error TEXT,
  CHECK(length(transaction_id) BETWEEN 1 AND 128),
  CHECK(length(kind) BETWEEN 1 AND 64),
  CHECK(length(signature) > 0)
);
CREATE INDEX IF NOT EXISTS idx_transaction_journal_pending
  ON transaction_journal(synced_at_unix, sequence);
CREATE INDEX IF NOT EXISTS idx_transaction_journal_retention
  ON transaction_journal(synced_at_unix, created_at_unix);
INSERT OR IGNORE INTO schema_version(version, applied_at_unix)
VALUES (1, CAST(strftime('%s','now') AS INTEGER));
CREATE TABLE IF NOT EXISTS commands (
  command_id TEXT PRIMARY KEY,
  sender_id TEXT NOT NULL,
  sequence INTEGER NOT NULL,
  payload_digest TEXT NOT NULL,
  capability_id TEXT NOT NULL,
  transport TEXT NOT NULL,
  state TEXT NOT NULL DEFAULT 'RECEIVED',
  result_code TEXT,
  device_signature TEXT,
  received_at_unix INTEGER NOT NULL,
  completed_at_unix INTEGER,
  UNIQUE(sender_id, sequence)
);
CREATE TABLE IF NOT EXISTS outbox (
  event_id TEXT PRIMARY KEY,
  payload TEXT NOT NULL,
  signature TEXT NOT NULL,
  attempt_count INTEGER NOT NULL DEFAULT 0,
  next_attempt_at_unix INTEGER,
  created_at_unix INTEGER NOT NULL,
  acknowledged_at_unix INTEGER
);
CREATE INDEX IF NOT EXISTS idx_outbox_pending ON outbox(acknowledged_at_unix, created_at_unix);
CREATE TABLE IF NOT EXISTS replay_window (
  sender_id TEXT PRIMARY KEY,
  highest_sequence INTEGER NOT NULL CHECK(highest_sequence >= 0)
);
CREATE TABLE IF NOT EXISTS capability_cache (
  capability_id TEXT PRIMARY KEY,
  signed_payload TEXT NOT NULL,
  digest TEXT NOT NULL,
  expires_at_unix INTEGER NOT NULL,
  binding_version INTEGER NOT NULL,
  revoked INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS revocation_cache (
  revision INTEGER PRIMARY KEY,
  signed_snapshot TEXT NOT NULL,
  applied_at_unix INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS trusted_time_anchor (
  singleton INTEGER PRIMARY KEY CHECK(singleton=1),
  server_time_unix INTEGER NOT NULL,
  monotonic_millis INTEGER NOT NULL,
  signature_digest TEXT NOT NULL
);
INSERT OR IGNORE INTO schema_version(version, applied_at_unix)
VALUES (2, CAST(strftime('%s','now') AS INTEGER));
)sql";
}

StorageResult StorageResult::ok() { return {true, StorageError::None, SQLITE_OK, {}}; }
StorageResult StorageResult::fail(StorageError error, int code, const String& message) {
    return {false, error, code, message};
}

EdgeStorage::~EdgeStorage() { close(); }

StorageResult EdgeStorage::begin(const char* mountPoint,
                                 const char* databaseRelativePath,
                                 bool oneBitMode) {
    if (db_) return StorageResult::fail(StorageError::AlreadyOpen, SQLITE_MISUSE, "database already open");
    if (!mountPoint || !databaseRelativePath || databaseRelativePath[0] != '/')
        return StorageResult::fail(StorageError::InvalidArgument, SQLITE_MISUSE, "invalid database path");

    StorageResult mounted = mountSd(mountPoint, oneBitMode);
    if (!mounted) return mounted;

    const int slash = String(databaseRelativePath).lastIndexOf('/');
    if (slash > 0) {
        String directory = String(databaseRelativePath).substring(0, slash);
        if (!SD_MMC.exists(directory) && !SD_MMC.mkdir(directory)) {
            close();
            return StorageResult::fail(StorageError::DirectoryCreateFailed, SQLITE_CANTOPEN,
                                       "cannot create " + directory);
        }
    }

    databasePath_ = mountPoint_ + databaseRelativePath;
    int rc = sqlite3_initialize();
    if (rc != SQLITE_OK) { close(); return sqliteFailure(StorageError::DatabaseOpenFailed, rc, "sqlite init"); }
    rc = sqlite3_open_v2(databasePath_.c_str(), &db_,
                         SQLITE_OPEN_READWRITE | SQLITE_OPEN_CREATE | SQLITE_OPEN_FULLMUTEX, nullptr);
    if (rc != SQLITE_OK) {
        StorageResult failure = sqliteFailure(StorageError::DatabaseOpenFailed, rc, "database open");
        close(); return failure;
    }
    sqlite3_busy_timeout(db_, 5000);
    StorageResult migrated = migrate();
    if (!migrated) { close(); return migrated; }
    return StorageResult::ok();
}

StorageResult EdgeStorage::mountSd(const char* mountPoint, bool oneBitMode) {
    mountPoint_ = mountPoint;
    const bool customPins = EDGE_SD_CLK >= 0 && EDGE_SD_CMD >= 0 && EDGE_SD_D0 >= 0;
    if (customPins && !SD_MMC.setPins(EDGE_SD_CLK, EDGE_SD_CMD, EDGE_SD_D0))
        return StorageResult::fail(StorageError::SdMountFailed, SQLITE_CANTOPEN, "invalid SD_MMC pin mapping");
    if (!SD_MMC.begin(mountPoint, oneBitMode))
        return StorageResult::fail(StorageError::SdMountFailed, SQLITE_CANTOPEN, "SD_MMC mount failed");
    sdMounted_ = true;
    if (SD_MMC.cardType() == CARD_NONE) {
        close();
        return StorageResult::fail(StorageError::SdUnavailable, SQLITE_CANTOPEN, "SD card not present");
    }
    return StorageResult::ok();
}

StorageResult EdgeStorage::migrate() {
    StorageResult r = execute("PRAGMA journal_mode=DELETE; PRAGMA synchronous=FULL; PRAGMA foreign_keys=ON; BEGIN IMMEDIATE;");
    if (!r) return StorageResult::fail(StorageError::MigrationFailed, r.sqliteCode, r.message);
    r = execute(kSchema);
    if (!r) {
        execute("ROLLBACK;");
        return StorageResult::fail(StorageError::MigrationFailed, r.sqliteCode, r.message);
    }
    r = execute("COMMIT;");
    return r ? r : StorageResult::fail(StorageError::MigrationFailed, r.sqliteCode, r.message);
}

void EdgeStorage::close() {
    if (db_) { sqlite3_close_v2(db_); db_ = nullptr; }
    sqlite3_shutdown();
    if (sdMounted_) { SD_MMC.end(); sdMounted_ = false; }
    mountPoint_ = ""; databasePath_ = "";
}

uint64_t EdgeStorage::cardSizeBytes() const { return sdMounted_ ? SD_MMC.cardSize() : 0; }

StorageResult EdgeStorage::enqueue(const char* id, const char* kind, const char* payload,
                                   const char* signature, int64_t createdAt) {
    if (!db_ || !id || !*id || !kind || !*kind || !payload || !signature || !*signature || createdAt <= 0)
        return StorageResult::fail(StorageError::InvalidArgument, SQLITE_MISUSE, "invalid journal record");
    sqlite3_stmt* raw = nullptr;
    StorageResult r = prepare("INSERT INTO transaction_journal(transaction_id,kind,canonical_payload,signature,created_at_unix) VALUES(?,?,?,?,?);", &raw);
    if (!r) return r; Statement statement(raw);
    sqlite3_bind_text(raw, 1, id, -1, SQLITE_TRANSIENT);
    sqlite3_bind_text(raw, 2, kind, -1, SQLITE_TRANSIENT);
    sqlite3_bind_text(raw, 3, payload, -1, SQLITE_TRANSIENT);
    sqlite3_bind_text(raw, 4, signature, -1, SQLITE_TRANSIENT);
    sqlite3_bind_int64(raw, 5, createdAt);
    int rc = sqlite3_step(raw);
    return rc == SQLITE_DONE ? StorageResult::ok() : sqliteFailure(StorageError::SqlError, rc, "enqueue");
}

StorageResult EdgeStorage::forEachPending(size_t limit, const PendingVisitor& visitor) {
    if (!db_ || !visitor || limit == 0) return StorageResult::fail(StorageError::InvalidArgument, SQLITE_MISUSE, "invalid pending query");
    sqlite3_stmt* raw = nullptr;
    StorageResult r = prepare("SELECT sequence,transaction_id,kind,canonical_payload,signature,created_at_unix,attempt_count FROM transaction_journal WHERE synced_at_unix IS NULL ORDER BY sequence LIMIT ?;", &raw);
    if (!r) return r; Statement statement(raw);
    sqlite3_bind_int64(raw, 1, static_cast<sqlite3_int64>(limit));
    int rc;
    while ((rc = sqlite3_step(raw)) == SQLITE_ROW) {
        PendingTransaction tx{
            sqlite3_column_int64(raw, 0),
            reinterpret_cast<const char*>(sqlite3_column_text(raw, 1)),
            reinterpret_cast<const char*>(sqlite3_column_text(raw, 2)),
            reinterpret_cast<const char*>(sqlite3_column_text(raw, 3)),
            reinterpret_cast<const char*>(sqlite3_column_text(raw, 4)),
            sqlite3_column_int64(raw, 5),
            static_cast<uint32_t>(sqlite3_column_int64(raw, 6))
        };
        if (!visitor(tx)) break;
    }
    return rc == SQLITE_DONE || rc == SQLITE_ROW ? StorageResult::ok()
                                                   : sqliteFailure(StorageError::SqlError, rc, "pending query");
}

StorageResult EdgeStorage::markSynced(const char* id, int64_t at) {
    if (!db_ || !id || !*id || at <= 0) return StorageResult::fail(StorageError::InvalidArgument, SQLITE_MISUSE, "invalid sync marker");
    sqlite3_stmt* raw = nullptr; StorageResult r = prepare("UPDATE transaction_journal SET synced_at_unix=?,last_error=NULL WHERE transaction_id=? AND synced_at_unix IS NULL;", &raw);
    if (!r) return r; Statement statement(raw);
    sqlite3_bind_int64(raw, 1, at); sqlite3_bind_text(raw, 2, id, -1, SQLITE_TRANSIENT);
    int rc = sqlite3_step(raw);
    return rc == SQLITE_DONE ? StorageResult::ok() : sqliteFailure(StorageError::SqlError, rc, "mark synced");
}

StorageResult EdgeStorage::recordAttempt(const char* id, const char* error) {
    if (!db_ || !id || !*id) return StorageResult::fail(StorageError::InvalidArgument, SQLITE_MISUSE, "invalid attempt");
    sqlite3_stmt* raw = nullptr; StorageResult r = prepare("UPDATE transaction_journal SET attempt_count=attempt_count+1,last_attempt_at_unix=CAST(strftime('%s','now') AS INTEGER),last_error=? WHERE transaction_id=? AND synced_at_unix IS NULL;", &raw);
    if (!r) return r; Statement statement(raw);
    if (error) sqlite3_bind_text(raw, 1, error, -1, SQLITE_TRANSIENT); else sqlite3_bind_null(raw, 1);
    sqlite3_bind_text(raw, 2, id, -1, SQLITE_TRANSIENT);
    int rc = sqlite3_step(raw);
    return rc == SQLITE_DONE ? StorageResult::ok() : sqliteFailure(StorageError::SqlError, rc, "record attempt");
}

StorageResult EdgeStorage::pruneSynced(int64_t now, uint32_t days) {
    if (!db_ || now <= 0 || days < 90) return StorageResult::fail(StorageError::InvalidArgument, SQLITE_MISUSE, "retention must be at least 90 days");
    sqlite3_stmt* raw = nullptr; StorageResult r = prepare("DELETE FROM transaction_journal WHERE synced_at_unix IS NOT NULL AND created_at_unix < ?;", &raw);
    if (!r) return r; Statement statement(raw);
    sqlite3_bind_int64(raw, 1, now - static_cast<int64_t>(days) * 86400);
    int rc = sqlite3_step(raw);
    return rc == SQLITE_DONE ? StorageResult::ok() : sqliteFailure(StorageError::SqlError, rc, "prune synced");
}

StorageResult EdgeStorage::checkpoint() { return execute("PRAGMA optimize;"); }

StorageResult EdgeStorage::reserveCommand(const char* id, const char* sender,
                                          uint64_t sequence, const char* digest,
                                          const char* capability, const char* transport,
                                          int64_t at) {
    if (!db_ || !id || !*id || !sender || !*sender || !digest || !*digest ||
        !capability || !*capability || !transport || !*transport || at <= 0)
        return StorageResult::fail(StorageError::InvalidArgument, SQLITE_MISUSE, "invalid command reservation");
    sqlite3_stmt* raw = nullptr;
    StorageResult r = prepare("INSERT INTO commands(command_id,sender_id,sequence,payload_digest,capability_id,transport,received_at_unix) VALUES(?,?,?,?,?,?,?);", &raw);
    if (!r) return r;
    Statement statement(raw);
    sqlite3_bind_text(raw, 1, id, -1, SQLITE_TRANSIENT);
    sqlite3_bind_text(raw, 2, sender, -1, SQLITE_TRANSIENT);
    sqlite3_bind_int64(raw, 3, static_cast<sqlite3_int64>(sequence));
    sqlite3_bind_text(raw, 4, digest, -1, SQLITE_TRANSIENT);
    sqlite3_bind_text(raw, 5, capability, -1, SQLITE_TRANSIENT);
    sqlite3_bind_text(raw, 6, transport, -1, SQLITE_TRANSIENT);
    sqlite3_bind_int64(raw, 7, at);
    int rc = sqlite3_step(raw);
    return rc == SQLITE_DONE ? StorageResult::ok() : sqliteFailure(StorageError::SqlError, rc, "reserve command");
}

StorageResult EdgeStorage::completeCommand(const char* id, const char* result,
                                           const char* signature, int64_t at) {
    if (!db_ || !id || !result || !signature || at <= 0)
        return StorageResult::fail(StorageError::InvalidArgument, SQLITE_MISUSE, "invalid command completion");
    sqlite3_stmt* raw = nullptr;
    StorageResult r = prepare("UPDATE commands SET state='COMPLETED',result_code=?,device_signature=?,completed_at_unix=? WHERE command_id=? AND state='RECEIVED';", &raw);
    if (!r) return r;
    Statement statement(raw);
    sqlite3_bind_text(raw, 1, result, -1, SQLITE_TRANSIENT);
    sqlite3_bind_text(raw, 2, signature, -1, SQLITE_TRANSIENT);
    sqlite3_bind_int64(raw, 3, at);
    sqlite3_bind_text(raw, 4, id, -1, SQLITE_TRANSIENT);
    int rc = sqlite3_step(raw);
    return rc == SQLITE_DONE && sqlite3_changes(db_) == 1 ? StorageResult::ok()
        : StorageResult::fail(StorageError::SqlError, rc, "command not reservable");
}

StorageResult EdgeStorage::rememberReplaySequence(const char* sender, uint64_t sequence) {
    if (!db_ || !sender || !*sender)
        return StorageResult::fail(StorageError::InvalidArgument, SQLITE_MISUSE, "invalid replay sequence");
    sqlite3_stmt* raw = nullptr;
    StorageResult r = prepare("INSERT INTO replay_window(sender_id,highest_sequence) VALUES(?,?) ON CONFLICT(sender_id) DO UPDATE SET highest_sequence=excluded.highest_sequence WHERE excluded.highest_sequence > replay_window.highest_sequence;", &raw);
    if (!r) return r;
    Statement statement(raw);
    sqlite3_bind_text(raw, 1, sender, -1, SQLITE_TRANSIENT);
    sqlite3_bind_int64(raw, 2, static_cast<sqlite3_int64>(sequence));
    int rc = sqlite3_step(raw);
    return rc == SQLITE_DONE && sqlite3_changes(db_) == 1 ? StorageResult::ok()
        : StorageResult::fail(StorageError::SqlError, SQLITE_CONSTRAINT, "replayed sequence");
}

StorageResult EdgeStorage::execute(const char* sql) {
    if (!db_) return StorageResult::fail(StorageError::InvalidArgument, SQLITE_MISUSE, "database not open");
    char* message = nullptr; int rc = sqlite3_exec(db_, sql, nullptr, nullptr, &message);
    String detail = message ? message : ""; if (message) sqlite3_free(message);
    return rc == SQLITE_OK ? StorageResult::ok() : StorageResult::fail(StorageError::SqlError, rc, detail);
}

StorageResult EdgeStorage::prepare(const char* sql, sqlite3_stmt** statement) {
    if (!db_ || !statement) return StorageResult::fail(StorageError::InvalidArgument, SQLITE_MISUSE, "database not open");
    int rc = sqlite3_prepare_v2(db_, sql, -1, statement, nullptr);
    return rc == SQLITE_OK ? StorageResult::ok() : sqliteFailure(StorageError::SqlError, rc, "prepare");
}

StorageResult EdgeStorage::sqliteFailure(StorageError error, int code, const char* context) const {
    String message(context); message += ": "; message += db_ ? sqlite3_errmsg(db_) : sqlite3_errstr(code);
    return StorageResult::fail(error, code, message);
}

} // namespace beefiscal::edge
