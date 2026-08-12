#pragma once

#include <Arduino.h>
#include <FS.h>
#include <SD_MMC.h>
#include <sqlite3.h>
#include <functional>

namespace beefiscal::edge {

enum class StorageError : uint8_t {
    None,
    AlreadyOpen,
    SdMountFailed,
    SdUnavailable,
    DirectoryCreateFailed,
    DatabaseOpenFailed,
    MigrationFailed,
    InvalidArgument,
    SqlError
};

struct StorageResult {
    bool success;
    StorageError error;
    int sqliteCode;
    String message;

    static StorageResult ok();
    static StorageResult fail(StorageError error, int sqliteCode, const String& message);
    explicit operator bool() const { return success; }
};

struct PendingTransaction {
    int64_t sequence;
    String transactionId;
    String kind;
    String payload;
    String signature;
    int64_t createdAtUnix;
    uint32_t attemptCount;
};

class EdgeStorage final {
public:
    using PendingVisitor = std::function<bool(const PendingTransaction&)>;

    EdgeStorage() = default;
    ~EdgeStorage();
    EdgeStorage(const EdgeStorage&) = delete;
    EdgeStorage& operator=(const EdgeStorage&) = delete;

    StorageResult begin(const char* mountPoint = "/sdcard",
                        const char* databaseRelativePath = "/beefiscal/edge-agent.db",
                        bool oneBitMode = true);
    void close();
    bool isOpen() const { return db_ != nullptr; }
    uint64_t cardSizeBytes() const;

    StorageResult enqueue(const char* transactionId,
                          const char* kind,
                          const char* canonicalPayload,
                          const char* signature,
                          int64_t createdAtUnix);
    StorageResult forEachPending(size_t limit, const PendingVisitor& visitor);
    StorageResult markSynced(const char* transactionId, int64_t syncedAtUnix);
    StorageResult recordAttempt(const char* transactionId, const char* lastError);
    StorageResult pruneSynced(int64_t nowUnix, uint32_t retentionDays = 93);
    StorageResult checkpoint();
    StorageResult reserveCommand(const char* commandId, const char* senderId,
                                 uint64_t sequence, const char* payloadDigest,
                                 const char* capabilityId, const char* transport,
                                 int64_t receivedAtUnix);
    StorageResult completeCommand(const char* commandId, const char* resultCode,
                                  const char* deviceSignature, int64_t completedAtUnix);
    StorageResult rememberReplaySequence(const char* senderId, uint64_t sequence);

private:
    StorageResult mountSd(const char* mountPoint, bool oneBitMode);
    StorageResult migrate();
    StorageResult execute(const char* sql);
    StorageResult prepare(const char* sql, sqlite3_stmt** statement);
    StorageResult sqliteFailure(StorageError error, int code, const char* context) const;

    sqlite3* db_ = nullptr;
    String mountPoint_;
    String databasePath_;
    bool sdMounted_ = false;
};

} // namespace beefiscal::edge
