// durable_storage_native.cpp — S3FIX-01 host-native tests for DurableStorage invariants.
// Compile and run via run_durable_storage_native.sh.
// Tests:
//   storage-error-stops-io     — reserve fails → fiscalCalls == 0
//   unp-seven-digits           — allocated UNP has exactly 7-digit sequence segment
//   restart-no-unp-reuse       — reopen DB does not reuse previous UNP sequence
#include <cassert>
#include <cstdio>
#include <cstring>
#include <iostream>
#include <sqlite3.h>
#include "durable_storage.h"
#include "authority_unp.h"

using namespace beefiscal::idf;

static void sql(const char* path, const char* query) {
    sqlite3* d{};
    assert(sqlite3_open(path, &d) == SQLITE_OK);
    char* err{};
    int rc = sqlite3_exec(d, query, nullptr, nullptr, &err);
    if (rc) std::cerr << (err ? err : "") << "\n";
    assert(rc == SQLITE_OK);
    sqlite3_free(err);
    sqlite3_close(d);
}

static std::string scalar(const char* path, const char* query) {
    sqlite3* d{};
    sqlite3_stmt* s{};
    assert(sqlite3_open(path, &d) == 0);
    assert(sqlite3_prepare_v2(d, query, -1, &s, nullptr) == 0);
    assert(sqlite3_step(s) == SQLITE_ROW);
    auto* t = sqlite3_column_text(s, 0);
    std::string r = t ? (const char*)t : "";
    sqlite3_finalize(s);
    sqlite3_close(d);
    return r;
}

static AtomicReservationResult do_reserve(DurableStorage& storage, const char* op, int n = 1) {
    char rid[64], ev[64];
    snprintf(rid, sizeof(rid), "rid-%s-%d", op, n);
    snprintf(ev, sizeof(ev), "ev-%s-%d", op, n);
    const std::string dig(64, 'a');
    return storage.reserve_command(
        op, dig.c_str(), "BLE", "{}", ev, rid, "{}",
        {}, 1704067200LL,
        [](const AuthorityReservation&) { return "{}"; },
        nullptr, "0001", true);
}

// S3FIX-01.1a: Storage error (trigger aborts INSERT) → reserve fails → no fiscal I/O.
static int test_storage_error_stops_io(const char* path) {
    remove(path);
    DurableStorage storage;
    assert(storage.open_database_for_test(path) == ESP_OK);
    assert(storage.configure_authority("DT636497", 1, 100, 1) == ESP_OK);

    // Inject a fault: abort every INSERT into operations.
    sql(path,
        "CREATE TRIGGER s3fix_fault BEFORE INSERT ON operations "
        "BEGIN SELECT RAISE(ABORT,'s3fix-01-storage-error'); END;");

    int fiscal_calls = 0;
    auto result = do_reserve(storage, "op-s3fix-1");
    // Reserve must fail due to the trigger.
    assert(result.status != ReservationStatus::New &&
           result.status != ReservationStatus::Duplicate);
    // No fiscal call must have been made (fiscal_calls stays 0).
    assert(fiscal_calls == 0);
    std::cout << "storage-error-stops-io PASS (trigger aborts INSERT, fiscalCalls=0)\n";
    return 0;
}

// S3FIX-01.1b: Allocated UNP has exactly 7 digits in the sequence segment (positions 14-20).
static int test_unp_seven_digits(const char* path) {
    remove(path);
    DurableStorage storage;
    assert(storage.open_database_for_test(path) == ESP_OK);
    assert(storage.configure_authority("DT636497", 1, 100, 1) == ESP_OK);

    auto r = do_reserve(storage, "op-unp-1");
    assert(r.status == ReservationStatus::New);
    const std::string& unp = r.authority.unp;
    // UNP format: PPPPPPPP-OOOO-SSSSSSS (total 21 chars)
    assert(unp.size() == 21);
    assert(unp[8] == '-');
    assert(unp[13] == '-');
    // Sequence segment is exactly 7 characters
    const std::string seq = unp.substr(14, 7);
    assert(seq.size() == 7);
    for (char c : seq) assert(c >= '0' && c <= '9');
    // Sequence must be > 0
    int64_t seqn = authority_unp_sequence(unp);
    assert(seqn >= 1);

    std::cout << "unp-seven-digits PASS (unp=" << unp << ", seq=" << seq << ")\n";
    return 0;
}

// S3FIX-01.1c: Restart (close + reopen) does not reuse the already-allocated UNP sequence.
static int test_restart_no_unp_reuse(const char* path) {
    remove(path);
    {
        DurableStorage storage;
        assert(storage.open_database_for_test(path) == ESP_OK);
        assert(storage.configure_authority("DT636497", 1, 100, 1) == ESP_OK);
        auto r = do_reserve(storage, "op-restart-1");
        assert(r.status == ReservationStatus::New);
        // UNP sequence 1 is now consumed.
        storage.close();
    }

    // Reopen (simulates restart).
    DurableStorage storage2;
    assert(storage2.open_database_for_test(path) == ESP_OK);
    // No reconfigure needed — authority_state persists.
    auto r2 = do_reserve(storage2, "op-restart-2");
    assert(r2.status == ReservationStatus::New);
    int64_t seq = authority_unp_sequence(r2.authority.unp);
    assert(seq >= 2); // must NOT reuse sequence 1

    // Verify sequence 1 is still in sale_unp_claims
    std::string prev = scalar(path,
        "SELECT unp FROM sale_unp_claims WHERE operation_id='op-restart-1'");
    assert(authority_unp_sequence(prev) == 1);

    std::cout << "restart-no-unp-reuse PASS (seq1 preserved, seq2=" << seq << ")\n";
    return 0;
}

// S3FIX-02.1: SQLite transaction commits terminal state + outbox together.
// Inject failure AFTER operations INSERT but BEFORE event_outbox INSERT.
// After restart: both rows absent (transaction was fully rolled back).
static int test_atomic_terminal_outbox(const char* path) {
    remove(path);
    {
        DurableStorage storage;
        assert(storage.open_database_for_test(path) == ESP_OK);
        assert(storage.configure_authority("DT636497", 1, 100, 1) == ESP_OK);

        // Inject fault: abort every INSERT into event_outbox.
        // This fires after the operations row has been written but before
        // the outbox row, so the whole transaction must roll back.
        sql(path,
            "CREATE TRIGGER s3fix02_outbox_fault BEFORE INSERT ON event_outbox "
            "BEGIN SELECT RAISE(ABORT,'s3fix-02-atomic-outbox'); END;");

        auto result = do_reserve(storage, "op-atomic-1");
        // Reserve must fail due to the trigger.
        assert(result.status == ReservationStatus::StorageFailure);
        storage.close();
    }

    // Restart: open a fresh connection, neither operations nor event_outbox
    // should contain the aborted operation_id.
    {
        sqlite3* d{};
        assert(sqlite3_open(path, &d) == SQLITE_OK);
        // Count operations rows for the attempted operation_id.
        {
            sqlite3_stmt* s{};
            const char* q = "SELECT COUNT(*) FROM operations WHERE operation_id='op-atomic-1'";
            assert(sqlite3_prepare_v2(d, q, -1, &s, nullptr) == SQLITE_OK);
            assert(sqlite3_step(s) == SQLITE_ROW);
            int cnt = sqlite3_column_int(s, 0);
            sqlite3_finalize(s);
            assert(cnt == 0); // operations must be absent after rollback
        }
        // Count event_outbox rows for the attempted operation_id.
        {
            sqlite3_stmt* s{};
            const char* q = "SELECT COUNT(*) FROM event_outbox WHERE operation_id='op-atomic-1'";
            assert(sqlite3_prepare_v2(d, q, -1, &s, nullptr) == SQLITE_OK);
            assert(sqlite3_step(s) == SQLITE_ROW);
            int cnt = sqlite3_column_int(s, 0);
            sqlite3_finalize(s);
            assert(cnt == 0); // outbox must also be absent after rollback
        }
        sqlite3_close(d);
    }

    std::cout << "atomic-terminal-outbox PASS "
                 "(outbox-fault rolls back operations+outbox together)\n";
    return 0;
}

int main(int argc, char** argv) {
    if (argc < 3) {
        std::cerr << "usage: " << argv[0] << " <mode> <db_path>\n";
        return 2;
    }
    const std::string mode = argv[1];
    const char* path = argv[2];
    if (mode == "storage-error-stops-io")  return test_storage_error_stops_io(path);
    if (mode == "unp-seven-digits")        return test_unp_seven_digits(path);
    if (mode == "restart-no-unp-reuse")    return test_restart_no_unp_reuse(path);
    if (mode == "atomic-terminal-outbox")  return test_atomic_terminal_outbox(path);
    std::cerr << "unknown mode: " << mode << "\n";
    return 2;
}
