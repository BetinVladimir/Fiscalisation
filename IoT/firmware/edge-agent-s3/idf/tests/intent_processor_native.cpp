// intent_processor_native.cpp — TAP native tests for IntentProcessor invariants.
// Tests: TST-S3-01..05 behaviors.
// Compile and run via native_intent.py or run_native_saga.sh.
// Usage: ./intent_processor_native <tmp-dir>
#include <cassert>
#include <cstdio>
#include <cstring>
#include <iostream>
#include <string>
#include <vector>
#include <sqlite3.h>
#include <openssl/sha.h>
#include <openssl/hmac.h>
#include <openssl/evp.h>
#include "intent_processor.h"
#include "canonical_cbor.h"
#include "cJSON.h"
#include "go_json.h"

using namespace beefiscal::idf;

// ---------------------------------------------------------------------------
// Utilities shared across tests
// ---------------------------------------------------------------------------
static std::string g_dir;

static std::string db_path(const char* name) { return g_dir + "/" + name + ".sqlite"; }

static std::string scalar(const char* path, const char* query) {
    sqlite3* d{}; sqlite3_stmt* s{};
    assert(sqlite3_open(path, &d) == 0);
    assert(sqlite3_prepare_v2(d, query, -1, &s, nullptr) == 0);
    assert(sqlite3_step(s) == SQLITE_ROW);
    auto* t = sqlite3_column_text(s, 0);
    std::string r = t ? (const char*)t : "";
    sqlite3_finalize(s); sqlite3_close(d);
    return r;
}

static std::string sha256_hex(const std::string& s) {
    unsigned char out[32];
    SHA256((const unsigned char*)s.data(), s.size(), out);
    char b[65]; for (int n = 0; n < 32; n++) snprintf(b + n*2, 3, "%02x", out[n]);
    return b;
}

// Normalise a JSON string via go_json (canonical key-sorted order).
static std::string normalise(const std::string& json) {
    auto* j = cJSON_Parse(json.c_str()); assert(j);
    beefiscal::go_json::Value v; std::string e, out;
    assert(beefiscal::go_json::from_cjson(j, v, e));
    assert(beefiscal::go_json::encode(v, out, e));
    cJSON_Delete(j); return out;
}

// Build signed MQTT SALE_FINALIZE with given items_json, payments_json, unp.
// The signing key is 32×'k' which decodes from the base64 key "a2tra2tr..."
static std::string mqtt_sale_finalize(
    const char* op_id, const char* session_id,
    const char* items_json, const char* payments_json,
    const char* unp, const char* issued = "2024-01-01T00:00:00Z",
    const char* expires = "2024-01-01T00:02:00Z")
{
    // Build canonical payload
    std::string ps = std::string("{\"items\":") + items_json
        + ",\"operator_code\":\"0001\","
        "\"payments\":" + payments_json
        + ",\"server_sale_id\":\"srv1\","
        "\"unp\":\"" + unp + "\","
        "\"unp_source\":\"SERVER_ASSIGNED\"}";
    ps = normalise(ps);
    std::string digest = sha256_hex(ps);
    // Build outer command JSON
    std::string cmd = std::string("{")
        + "\"client_operation_id\":\"" + op_id + "\","
        "\"command_type\":\"SALE_FINALIZE\","
        "\"device_id\":\"e\","
        "\"expires_at\":\"" + expires + "\","
        "\"fencing_token\":1,"
        "\"issued_at\":\"" + issued + "\","
        "\"payload\":" + ps + ","
        "\"payload_sha256\":\"" + digest + "\","
        "\"receipt_session_id\":\"" + session_id + "\","
        "\"register_id\":\"r\","
        "\"tenant_id\":\"t\"}";
    cmd = normalise(cmd);
    // Sign with key = 32 bytes of 'k'
    const std::string key(32, 'k');
    unsigned char mac[32]; unsigned n{};
    assert(HMAC(EVP_sha256(), key.data(), key.size(),
                (const unsigned char*)cmd.data(), cmd.size(), mac, &n) && n == 32);
    unsigned char encoded[48]{}; int len = EVP_EncodeBlock(encoded, mac, 32);
    std::string sig((char*)encoded, len);
    while (!sig.empty() && sig.back() == '=') sig.pop_back();
    for (char& c : sig) { if (c == '+') c = '-'; else if (c == '/') c = '_'; }
    // Add signature and re-normalise
    auto* j = cJSON_Parse(cmd.c_str()); assert(j);
    cJSON_AddStringToObject(j, "signature", sig.c_str());
    beefiscal::go_json::Value v; std::string e;
    assert(beefiscal::go_json::from_cjson(j, v, e));
    assert(beefiscal::go_json::encode(v, cmd, e));
    cJSON_Delete(j); return cmd;
}

// Build HTTP (local) SALE_FINALIZE — no HMAC, no timestamp.
// requires client_sale_surrogate_id in the outer command.
static std::string http_sale_finalize(
    const char* op_id, const char* session_id,
    const char* items_json, const char* payments_json)
{
    std::string ps = std::string("{\"items\":") + items_json
        + ",\"payments\":" + payments_json + "}";
    ps = normalise(ps);
    std::string digest = sha256_hex(ps);
    std::string cmd = std::string("{")
        + "\"action\":\"SALE_FINALIZE\","
        "\"binding_generation\":1,"
        "\"canonical_payload\":" + ps + ","
        "\"client_sale_surrogate_id\":\"" + op_id + "\","
        "\"edge_device_id\":\"e\","
        "\"intent_id\":\"" + op_id + "\","
        "\"payload_sha256\":\"" + digest + "\","
        "\"receipt_session_id\":\"" + session_id + "\","
        "\"register_id\":\"r\","
        "\"tenant_id\":\"t\"}";
    return normalise(cmd);
}

static CompositeBinding test_binding() {
    CompositeBinding b;
    b.tenant_id = "t"; b.register_id = "r"; b.edge_device_id = "e";
    b.generation = 1;
    b.authority.unp_prefix = "DT636497";
    b.fiscal.fiscal_device_number = "DT636497";
    b.authority.unp_range_start = 1; b.authority.unp_range_end = 100;
    return b;
}

struct TestClockState { CommandInstant instant{1704067201, 0}; bool failed{}; };
static bool test_clock(CommandInstant& out, void* ctx) {
    auto& s = *static_cast<TestClockState*>(ctx);
    out = s.instant; return !s.failed;
}

// Base64-encoded key = 32 bytes of 0x6B ('k').
// Matches the signing key used in mqtt_sale_finalize() above.
static const char* HMAC_KEY = "a2tra2tra2tra2tra2tra2tra2tra2tra2tra2tra2s=";

// Golden fixture data (invariant 9: numbers from fixture, not test code)
// items_1200: 1 item × 12.00 EUR = 1200¢
static const char* ITEMS_1200 =
    "[{\"line_id\":\"00000000-0000-4000-8000-000000000003\","
    "\"name\":\"Test\",\"quantity\":\"1\","
    "\"tax_group\":\"A\","
    "\"unit_price\":{\"amount\":\"12.00\",\"currency\":\"EUR\"}}]";
// payments summing to 1200¢ (CASH 12.00)
static const char* PAYMENTS_1200 =
    "[{\"amount\":{\"amount\":\"12.00\",\"currency\":\"EUR\"},"
    "\"payment_id\":\"00000000-0000-4000-8000-000000000004\","
    "\"type\":\"CASH\"}]";
// payments summing to 1199¢ (off by one)
static const char* PAYMENTS_1199 =
    "[{\"amount\":{\"amount\":\"11.99\",\"currency\":\"EUR\"},"
    "\"payment_id\":\"00000000-0000-4000-8000-000000000004\","
    "\"type\":\"CASH\"}]";
// payments with unsupported type NHIF
static const char* PAYMENTS_NHIF =
    "[{\"amount\":{\"amount\":\"12.00\",\"currency\":\"EUR\"},"
    "\"payment_id\":\"00000000-0000-4000-8000-000000000004\","
    "\"type\":\"NHIF\"}]";

static const char* OP_01 = "00000000-0000-4000-8001-000000000001";
static const char* OP_02 = "00000000-0000-4000-8002-000000000001";
static const char* OP_03 = "00000000-0000-4000-8003-000000000001";
static const char* OP_04 = "00000000-0000-4000-8004-000000000001";
static const char* OP_05 = "00000000-0000-4000-8005-000000000001";
static const char* SID   = "00000000-0000-4000-8000-000000000002";
static const char* UNP_1 = "DT636497-0001-0000001";
static const char* UNP_2 = "DT636497-0001-0000002";
// SERVER_ASSIGNED UNPs for TST-S3-05: sequences outside authority range [1,100]
// so the device does not reject them as in-range allocations.
static const char* UNP_A = "DT636497-0001-0000101";
static const char* UNP_B = "DT636497-0001-0000102";

static std::vector<std::string> g_results;
static esp_err_t result_sink(const char*, const char* json, Ingress, BleReplyOwner, void*) {
    g_results.push_back(json); return ESP_OK;
}

// ---------------------------------------------------------------------------
// TAP helpers
// ---------------------------------------------------------------------------
static int g_tap_count = 0;
static void tap_ok(const char* name) {
    std::cout << "ok " << ++g_tap_count << " - " << name << "\n";
}
static void tap_not_ok(const char* name, const char* msg) {
    std::cout << "not ok " << ++g_tap_count << " - " << name << "\n";
    std::cerr << "  # FAIL " << name << ": " << msg << "\n";
}

// ---------------------------------------------------------------------------
// TST-S3-01  executor.ready()==false → REJECTED_BEFORE_IO, no UNP consumed
// ---------------------------------------------------------------------------
static void test_rejected_before_io() {
    const char* name = "rejected-before-io";
    struct NotReady : ReceiptExecutor {
        int calls{};
        bool ready() override { return false; }
        ExecutionResult execute(const ReceiptPlan&) override { ++calls; return{}; }
        ExecutionResult recover(const RecoveryItem&) override { return{}; }
    } exec;

    std::string path = db_path("rejected-before-io");
    remove(path.c_str());
    DurableStorage storage;
    if (storage.open_database_for_test(path.c_str()) != ESP_OK ||
        storage.configure_authority("DT636497", 1, 100, 1) != ESP_OK) {
        tap_not_ok(name, "storage init failed"); return;
    }
    auto binding = test_binding();
    TestClockState cs;
    CommandClock clock{test_clock, &cs};
    IntentProcessor proc(storage, binding, exec, HMAC_KEY, result_sink, nullptr, nullptr, &clock);
    g_results.clear();

    auto cmd = mqtt_sale_finalize(OP_01, SID, ITEMS_1200, PAYMENTS_1200, UNP_1);
    proc.accept({(const uint8_t*)cmd.data(), cmd.size(), Ingress::Mqtt});

    if (exec.calls != 0) { tap_not_ok(name, "executor called with ready=false"); return; }
    if (scalar(path.c_str(), "SELECT count(*) FROM operations") != "0") {
        tap_not_ok(name, "operation row created"); return;
    }
    if (scalar(path.c_str(), "SELECT next_unp FROM authority_state") != "1") {
        tap_not_ok(name, "UNP counter advanced"); return;
    }
    bool got = false;
    for (const auto& r : g_results)
        if (r.find("REJECTED_BEFORE_IO") != std::string::npos) { got = true; break; }
    if (!got) { tap_not_ok(name, "REJECTED_BEFORE_IO not in result_sink output"); return; }
    tap_ok(name);
}

// ---------------------------------------------------------------------------
// TST-S3-02  CommandClock returns {0,0} (untrusted time) → MQTT rejected
// ---------------------------------------------------------------------------
static void test_trusted_time_required() {
    const char* name = "trusted-time-required";
    struct Ready : ReceiptExecutor {
        int calls{};
        bool ready() override { return true; }
        ExecutionResult execute(const ReceiptPlan&) override {
            ++calls;
            return{ExecutionCertainty::Committed, "fiscal-1"};
        }
        ExecutionResult recover(const RecoveryItem&) override { return{}; }
    } exec;

    std::string path = db_path("trusted-time-required");
    remove(path.c_str());
    DurableStorage storage;
    if (storage.open_database_for_test(path.c_str()) != ESP_OK ||
        storage.configure_authority("DT636497", 1, 100, 1) != ESP_OK) {
        tap_not_ok(name, "storage init failed"); return;
    }
    auto binding = test_binding();
    TestClockState cs; cs.instant = {0, 0}; // clock cannot provide trusted time
    CommandClock clock{test_clock, &cs};
    IntentProcessor proc(storage, binding, exec, HMAC_KEY, result_sink, nullptr, nullptr, &clock);
    g_results.clear();

    auto cmd = mqtt_sale_finalize(OP_02, SID, ITEMS_1200, PAYMENTS_1200, UNP_1);
    auto rc = proc.accept({(const uint8_t*)cmd.data(), cmd.size(), Ingress::Mqtt});

    if (rc == ESP_OK) { tap_not_ok(name, "accept succeeded with zero clock"); return; }
    if (exec.calls != 0) { tap_not_ok(name, "executor called"); return; }
    if (scalar(path.c_str(), "SELECT count(*) FROM operations") != "0") {
        tap_not_ok(name, "operation row created"); return;
    }
    if (scalar(path.c_str(), "SELECT next_unp FROM authority_state") != "1") {
        tap_not_ok(name, "UNP counter advanced"); return;
    }
    tap_ok(name);
}

// ---------------------------------------------------------------------------
// TST-S3-03  payment type "NHIF" (unsupported) → reject before reserve/I/O
// ---------------------------------------------------------------------------
static void test_unsupported_payment_nhif() {
    const char* name = "unsupported-payment-nhif";
    struct Ready : ReceiptExecutor {
        int calls{};
        bool ready() override { return true; }
        ExecutionResult execute(const ReceiptPlan&) override { ++calls; return{}; }
        ExecutionResult recover(const RecoveryItem&) override { return{}; }
    } exec;

    std::string path = db_path("nhif");
    remove(path.c_str());
    DurableStorage storage;
    if (storage.open_database_for_test(path.c_str()) != ESP_OK ||
        storage.configure_authority("DT636497", 1, 100, 1) != ESP_OK) {
        tap_not_ok(name, "storage init failed"); return;
    }
    auto binding = test_binding();
    IntentProcessor proc(storage, binding, exec, HMAC_KEY, result_sink, nullptr, nullptr, nullptr);
    g_results.clear();

    // HTTP ingress: no HMAC required, no timestamp check
    auto cmd = http_sale_finalize(OP_03, SID, ITEMS_1200, PAYMENTS_NHIF);
    auto rc = proc.accept({(const uint8_t*)cmd.data(), cmd.size(), Ingress::Http});

    if (rc == ESP_OK) { tap_not_ok(name, "NHIF payment accepted"); return; }
    if (exec.calls != 0) { tap_not_ok(name, "executor called"); return; }
    if (scalar(path.c_str(), "SELECT count(*) FROM operations") != "0") {
        tap_not_ok(name, "operation row created"); return;
    }
    tap_ok(name);
}

// ---------------------------------------------------------------------------
// TST-S3-04  items Σ=1200¢, payments Σ=1199¢ → PAYMENT_SUM_MISMATCH before I/O
// ---------------------------------------------------------------------------
static void test_payment_total_mismatch() {
    const char* name = "payment-total-mismatch";
    struct Ready : ReceiptExecutor {
        int calls{};
        bool ready() override { return true; }
        ExecutionResult execute(const ReceiptPlan&) override { ++calls; return{}; }
        ExecutionResult recover(const RecoveryItem&) override { return{}; }
    } exec;

    std::string path = db_path("total-mismatch");
    remove(path.c_str());
    DurableStorage storage;
    if (storage.open_database_for_test(path.c_str()) != ESP_OK ||
        storage.configure_authority("DT636497", 1, 100, 1) != ESP_OK) {
        tap_not_ok(name, "storage init failed"); return;
    }
    auto binding = test_binding();
    IntentProcessor proc(storage, binding, exec, HMAC_KEY, result_sink, nullptr, nullptr, nullptr);
    g_results.clear();

    // HTTP: items = 1200¢ but payments = 1199¢ → sum mismatch
    auto cmd = http_sale_finalize(OP_04, SID, ITEMS_1200, PAYMENTS_1199);
    auto rc = proc.accept({(const uint8_t*)cmd.data(), cmd.size(), Ingress::Http});

    if (rc == ESP_OK) { tap_not_ok(name, "mismatched totals accepted"); return; }
    if (exec.calls != 0) { tap_not_ok(name, "executor called"); return; }
    if (scalar(path.c_str(), "SELECT count(*) FROM operations") != "0") {
        tap_not_ok(name, "operation row created"); return;
    }
    tap_ok(name);
}

// ---------------------------------------------------------------------------
// TST-S3-05  same op_id, different payload → PAYLOAD_CONFLICT delivered
// ---------------------------------------------------------------------------
static void test_payload_conflict_result_delivered() {
    const char* name = "payload-conflict-result-delivered";
    struct Ready : ReceiptExecutor {
        int calls{};
        bool ready() override { return true; }
        ExecutionResult execute(const ReceiptPlan&) override {
            ++calls;
            return{ExecutionCertainty::Committed, "fiscal-1", "", "term-1", "rrn-1", "auth-1"};
        }
        ExecutionResult recover(const RecoveryItem&) override { return{}; }
    } exec;

    std::string path = db_path("payload-conflict");
    remove(path.c_str());
    DurableStorage storage;
    if (storage.open_database_for_test(path.c_str()) != ESP_OK ||
        storage.configure_authority("DT636497", 1, 100, 1) != ESP_OK) {
        tap_not_ok(name, "storage init failed"); return;
    }
    auto binding = test_binding();
    TestClockState cs;
    CommandClock clock{test_clock, &cs};
    IntentProcessor proc(storage, binding, exec, HMAC_KEY, result_sink, nullptr, nullptr, &clock);
    g_results.clear();

    // First command: op_id=OP_05, UNP_A → should succeed.
    // UNP_A has sequence 101, outside authority range [1,100], so the
    // SERVER_ASSIGNED in-range guard in reserve_command does not reject it.
    auto cmd1 = mqtt_sale_finalize(OP_05, SID, ITEMS_1200, PAYMENTS_1200, UNP_A);
    auto rc1 = proc.accept({(const uint8_t*)cmd1.data(), cmd1.size(), Ingress::Mqtt});
    if (rc1 != ESP_OK) {
        std::string msg = "first accept failed rc=";
        msg += std::to_string(rc1);
        msg += " cmd_size=";
        msg += std::to_string(cmd1.size());
        tap_not_ok(name, msg.c_str()); return;
    }
    if (exec.calls != 1) { tap_not_ok(name, "executor not called on first accept"); return; }

    // Second command: same op_id=OP_05 but different UNP → different digest → PayloadConflict
    g_results.clear();
    auto cmd2 = mqtt_sale_finalize(OP_05, SID, ITEMS_1200, PAYMENTS_1200, UNP_B);
    proc.accept({(const uint8_t*)cmd2.data(), cmd2.size(), Ingress::Mqtt});

    bool got_conflict = false;
    for (const auto& r : g_results)
        if (r.find("PAYLOAD_CONFLICT") != std::string::npos) { got_conflict = true; break; }
    if (!got_conflict) {
        tap_not_ok(name, "PAYLOAD_CONFLICT not delivered to result_sink"); return;
    }
    if (exec.calls != 1) {
        tap_not_ok(name, "executor called again on conflict"); return;
    }
    tap_ok(name);
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------
int main(int argc, char** argv) {
    if (argc < 2) {
        std::cerr << "usage: " << argv[0] << " <tmp-dir>\n";
        return 2;
    }
    g_dir = argv[1];

    const int total = 5;
    std::cout << "TAP version 13\n";
    std::cout << "1.." << total << "\n";

    test_rejected_before_io();
    test_trusted_time_required();
    test_unsupported_payment_nhif();
    test_payment_total_mismatch();
    test_payload_conflict_result_delivered();

    return g_tap_count == total ? 0 : 1;
}
