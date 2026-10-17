#include "intent_processor.h"
#include "cJSON.h"
#include "go_wire.h"
#include "canonical_cbor.h"
#include "mbedtls/base64.h"
#include "mbedtls/md.h"
#include "mbedtls/sha256.h"
#include <algorithm>
#include <cstdio>
#include <cstring>
#include <ctime>
#include <memory>
#include <utility>

namespace beefiscal::idf {
namespace {
struct Del {
  void operator()(cJSON *v) const { cJSON_Delete(v); }
};
using Json = std::unique_ptr<cJSON, Del>;
bool text(const cJSON *p, const char *n, std::string &o, size_t max = 4096) {
  auto *v = cJSON_GetObjectItemCaseSensitive(p, n);
  if (!cJSON_IsString(v) || !v->valuestring)
    return false;
  size_t z = strlen(v->valuestring);
  if (!z || z > max)
    return false;
  o.assign(v->valuestring, z);
  return true;
}
bool uuid(const std::string &s) {
  if (s.size() != 36)
    return false;
  for (size_t i = 0; i < s.size(); i++) {
    if (i == 8 || i == 13 || i == 18 || i == 23) {
      if (s[i] != '-')
        return false;
    } else if (!isxdigit((unsigned char)s[i]))
      return false;
  }
  return true;
}
bool money(const std::string &s, int64_t &minor) {
  if (s.size() < 4)
    return false;
  auto dot = s.find('.');
  if (dot == std::string::npos || s.size() - dot != 3)
    return false;
  int64_t whole = 0, fraction = 0;
  for (size_t i = 0; i < dot; i++) {
    if (!isdigit((unsigned char)s[i]))
      return false;
    whole = whole * 10 + s[i] - '0';
    if (whole > 1000000000)
      return false;
  }
  if (!isdigit(s[dot + 1]) || !isdigit(s[dot + 2]))
    return false;
  fraction = (s[dot + 1] - '0') * 10 + s[dot + 2] - '0';
  minor = whole * 100 + fraction;
  return minor > 0;
}
std::string hash(const char *d, size_t z) {
  uint8_t out[32]{};
  if (mbedtls_sha256((const uint8_t *)d, z, out, 0) != 0)
    return {};
  static constexpr char h[] = "0123456789abcdef";
  std::string s(64, '0');
  for (size_t i = 0; i < 32; i++) {
    s[i * 2] = h[out[i] >> 4];
    s[i * 2 + 1] = h[out[i] & 15];
  }
  return s;
}
int64_t now() {
  const auto value = std::time(nullptr);
  return value >= 1704067200 ? value : 0;
}
const char *state(ExecutionCertainty c) {
  switch (c) {
  case ExecutionCertainty::Committed:
    return "COMMITTED";
  case ExecutionCertainty::Compensated:
    return "COMPENSATED";
  case ExecutionCertainty::Unknown:
    return "RECOVERY_REQUIRED";
  case ExecutionCertainty::DeviceUnavailable:
    return "RECOVERY_REQUIRED";
  default:
    return "REJECTED";
  }
}
std::string result_json(const std::string &operation_id, const ExecutionResult &r,
                        const std::string &operation_type = {}) {
  Json root(cJSON_CreateObject());
  if (!root) return {};
  const std::pair<const char *, const std::string *> fields[] = {
      {"error_code", &r.error_code}, {"fiscal_reference", &r.fiscal_reference},
      {"operation_id", &operation_id}, {"state", nullptr},
      {"terminal_reference", &r.terminal_reference}, {"rrn", &r.rrn},
      {"auth_code", &r.auth_code}};
  for (const auto &field : fields)
    if (!cJSON_AddStringToObject(root.get(), field.first,
        field.second ? field.second->c_str() : state(r.certainty))) return {};
  if (!operation_type.empty() && !cJSON_AddStringToObject(root.get(), "operation_type", operation_type.c_str())) return {};
  char *raw = cJSON_PrintUnformatted(root.get());
  if (!raw) return {};
  std::string result(raw);
  cJSON_free(raw);
  return result;
}
std::string accepted_json(const ReceiptPlan &p, const CompositeBinding &b,
                          const AuthorityReservation &a) {
  cJSON *root = cJSON_CreateObject();
  cJSON_AddNumberToObject(root, "operation_sequence", a.operation_sequence);
  cJSON_AddNumberToObject(root, "unp_sequence", a.unp_sequence);
  cJSON *command = cJSON_AddObjectToObject(root, "command");
  cJSON_AddStringToObject(command, "operation_id", p.operation_id.c_str());
  cJSON_AddStringToObject(command, "tenant_id", b.tenant_id.c_str());
  cJSON_AddStringToObject(command, "register_id", b.register_id.c_str());
  cJSON_AddStringToObject(command, "device_id", b.edge_device_id.c_str());
  cJSON_AddStringToObject(command, "command_type",
      p.command_type == "SALE_REVERSE" ? "REVERSAL" : p.command_type.c_str());
  cJSON *payload = cJSON_Parse(p.canonical_payload.c_str());
  cJSON_AddItemToObject(command, "payload",
                        payload ? payload : cJSON_CreateObject());
  char *raw = cJSON_PrintUnformatted(root);
  std::string out = raw ? raw : "";
  cJSON_free(raw);
  cJSON_Delete(root);
  return out;
}
bool verify_hmac(go_json::Value root, const std::string &key) {
  auto *signature = go_json::field(root, "signature");
  if (!signature || signature->kind != go_json::Value::Kind::String)
    return false;
  std::string supplied = signature->text;
  root.fields.erase(std::remove_if(root.fields.begin(), root.fields.end(),
      [](const auto &f) { return f.first == "signature"; }), root.fields.end());
  std::string canonical, error;
  if (!go_json::encode(root, canonical, error)) return false;
  for (char &c : supplied) {
    if (c == '-')
      c = '+';
    else if (c == '_')
      c = '/';
  }
  while (supplied.size() % 4)
    supplied += '=';
  uint8_t got[64], expected[32];
  size_t n = 0;
  if (mbedtls_base64_decode(got, sizeof(got), &n, (uint8_t *)supplied.data(),
                            supplied.size()) ||
      n != 32)
    return false;
  std::string decoded_key = key;
  for (char &c : decoded_key) {
    if (c == '-')
      c = '+';
    else if (c == '_')
      c = '/';
  }
  while (decoded_key.size() % 4)
    decoded_key += '=';
  uint8_t key_bytes[64];
  size_t key_size = 0;
  if (mbedtls_base64_decode(key_bytes, sizeof(key_bytes), &key_size,
                            (uint8_t *)decoded_key.data(),
                            decoded_key.size()) ||
      key_size < 32)
    return false;
  auto *info = mbedtls_md_info_from_type(MBEDTLS_MD_SHA256);
  if (mbedtls_md_hmac(info, key_bytes, key_size, (uint8_t *)canonical.data(),
                      canonical.size(), expected))
    return false;
  uint8_t difference = 0;
  for (size_t i = 0; i < 32; i++)
    difference |= got[i] ^ expected[i];
  return difference == 0;
}
} // namespace
IntentProcessor::IntentProcessor(DurableStorage &s, const CompositeBinding &b,
                                 ReceiptExecutor &e, const std::string &key,
                                 ResultSink sink, void *context)
    : storage_(s), binding_(b), executor_(e), command_hmac_key_(key),
      result_sink_(sink), result_context_(context) {}
esp_err_t IntentProcessor::decode(const CommandView &view, ReceiptPlan &p) {
  if (!view.data || !view.size || view.size > 8192)
    return ESP_ERR_INVALID_ARG;
  cJSON *decoded = nullptr;
  go_json::Value canonical_root;
  std::string json_error;
  if (view.ingress == Ingress::Ble) {
    if (canonical_cbor_to_json(view.data, view.size, &decoded) != ESP_OK)
      return ESP_ERR_INVALID_ARG;
    if (!go_json::from_cjson(decoded, canonical_root, json_error)) {
      cJSON_Delete(decoded); return ESP_ERR_INVALID_ARG;
    }
  } else {
    if (!go_json::parse((const char*)view.data, view.size, canonical_root, json_error))
      return ESP_ERR_INVALID_ARG;
    std::string business_json;
    if (!go_json::encode(canonical_root, business_json, json_error))
      return ESP_ERR_INVALID_ARG;
    decoded = cJSON_ParseWithLengthOpts(business_json.data(), business_json.size(),
                                        nullptr, false);
  }
  Json root(decoded);
  if (!root || !cJSON_IsObject(root.get()) ||
      canonical_root.kind != go_json::Value::Kind::Map ||
      go_json::contains_nul(canonical_root) ||
      !go_json::safe_cjson_numbers(canonical_root))
    return ESP_ERR_INVALID_ARG;
  if (view.ingress == Ingress::Mqtt &&
      !verify_hmac(canonical_root, command_hmac_key_))
    return ESP_ERR_INVALID_CRC;
  std::string tenant, reg, device, provided;
  const bool local =
      view.ingress == Ingress::Ble || view.ingress == Ingress::Http;
  auto *payload = local ? cJSON_GetObjectItemCaseSensitive(root.get(), "canonical_payload")
                        : cJSON_GetObjectItemCaseSensitive(root.get(), "payload");
  auto *fence = cJSON_GetObjectItemCaseSensitive(
      root.get(), local ? "binding_generation" : "fencing_token");
  if (!cJSON_IsObject(payload) || !cJSON_IsNumber(fence) ||
      fence->valuedouble != fence->valueint ||
      fence->valueint != binding_.generation ||
      !text(root.get(), "tenant_id", tenant) ||
      !text(root.get(), "register_id", reg) ||
      !(local ? text(root.get(), "edge_device_id", device)
              : text(root.get(), "device_id", device)) ||
      tenant != binding_.tenant_id || reg != binding_.register_id ||
      device != binding_.edge_device_id)
    return ESP_ERR_INVALID_STATE;
  if (local) {
    if (!text(root.get(), "intent_id", p.operation_id) ||
        !text(root.get(), "action", p.command_type))
      return ESP_ERR_INVALID_ARG;
    if (!text(root.get(), "receipt_session_id", p.receipt_session_id))
      return ESP_ERR_INVALID_ARG;
    if (!text(root.get(), "payload_sha256", provided, 64))
      return ESP_ERR_INVALID_ARG;
  } else {
    if (!text(root.get(), "client_operation_id", p.operation_id) &&
        !text(root.get(), "operation_id", p.operation_id))
      return ESP_ERR_INVALID_ARG;
    if (!text(root.get(), "receipt_session_id", p.receipt_session_id) ||
        !text(root.get(), "command_type", p.command_type) ||
        !text(root.get(), "payload_sha256", provided, 64))
      return ESP_ERR_INVALID_ARG;
  }
  if (!uuid(p.operation_id) || !uuid(p.receipt_session_id))
    return ESP_ERR_INVALID_ARG;
  // Preserve authenticated JSON/payload bytes; normalize only the internal plan.
  if (p.command_type == "REVERSAL") p.command_type = "SALE_REVERSE";
  const std::vector<std::string> allowed = {
      "SALE_FINALIZE",   "SALE_REVERSE", "REPORT_X",
      "REPORT_Z",        "CASH_IN",      "CASH_OUT",
      "DEVICE_IDENTITY", "DEVICE_TIME",  "PRINTER_TEST"};
  if (std::find(allowed.begin(), allowed.end(), p.command_type) ==
      allowed.end())
    return ESP_ERR_NOT_SUPPORTED;
  const auto *canonical_payload = go_json::field(canonical_root,
      local ? "canonical_payload" : "payload");
  if (!canonical_payload || !go_json::encode(*canonical_payload,
                                            p.canonical_payload, json_error))
    return ESP_ERR_INVALID_ARG;
  p.payload_digest =
      hash(p.canonical_payload.data(), p.canonical_payload.size());
  if (p.payload_digest != provided)
    return ESP_ERR_INVALID_CRC;
  // Non-sale commands are first-class operations. UNP is allocated durably by
  // reserve_command() and is never accepted as client authority.
  if (p.command_type == "SALE_FINALIZE" &&
      !text(payload, "server_sale_id", p.sale_id)) {
    if (local)
      p.sale_id = p.operation_id;
    else
      return ESP_ERR_INVALID_ARG;
  }
  p.route_snapshot =
      "{\"command_type\":\"" + std::string(p.command_type == "SALE_REVERSE" ? "REVERSAL" : p.command_type) + "\",\"binding_generation\":" + std::to_string(binding_.generation) +
      ",\"edge_device_id\":\"" + binding_.edge_device_id +
      "\",\"fiscal_device_id\":\"" + binding_.fiscal.device_id + "\"" +
      (binding_.payment.present
           ? ",\"payment_device_id\":\"" + binding_.payment.device_id + "\""
           : "") +
      "}";
  if (p.command_type == "REPORT_X" || p.command_type == "REPORT_Z" ||
      p.command_type == "DEVICE_IDENTITY" || p.command_type == "DEVICE_TIME" ||
      p.command_type == "PRINTER_TEST")
    return ESP_OK;
  if (p.command_type == "CASH_IN" || p.command_type == "CASH_OUT") {
    auto *a = cJSON_GetObjectItemCaseSensitive(payload, "amount");
    std::string currency;
    int64_t minor = 0;
    if (!cJSON_IsObject(a) || !text(a, "amount", p.amount, 32) ||
        !text(a, "currency", currency, 3) || currency != "EUR" ||
        !money(p.amount, minor))
      return ESP_ERR_INVALID_ARG;
    return ESP_OK;
  }
  if (p.command_type == "SALE_REVERSE") {
    auto *original =
        cJSON_GetObjectItemCaseSensitive(payload, "original_document");
    auto *number =
        cJSON_GetObjectItemCaseSensitive(original, "document_number");
    auto *reason = cJSON_GetObjectItemCaseSensitive(original, "reason");
    auto *legacy_reason = cJSON_GetObjectItemCaseSensitive(payload, "reason");
    if (!reason) reason = legacy_reason;
    else if (legacy_reason && (!cJSON_IsNumber(legacy_reason) ||
             legacy_reason->valuedouble != reason->valuedouble))
      return ESP_ERR_INVALID_ARG;
    if (!cJSON_IsObject(original) || !cJSON_IsNumber(number) ||
        number->valuedouble != number->valueint || number->valueint < 1 || number->valueint > 9999999 ||
        !text(original, "document_datetime", p.original_datetime, 32) ||
        !text(original, "fiscal_memory_number", p.original_fmin, 16) ||
        !cJSON_IsNumber(reason) || reason->valuedouble != reason->valueint ||
        reason->valueint < 0 || reason->valueint > 2)
      return ESP_ERR_INVALID_ARG;
    auto *reason_code = cJSON_GetObjectItemCaseSensitive(payload, "reason_code");
    if (reason_code) {
      std::string code;
      if (!text(payload, "reason_code", code, 32)) return ESP_ERR_INVALID_ARG;
      const int expected = code == "OPERATOR_ERROR" ? 0 :
          (code == "CUSTOMER_RETURN" || code == "CUSTOMER_COMPLAINT") ? 1 :
          code == "TAX_BASE_REDUCTION" ? 2 : -1;
      if (expected != reason->valueint) return ESP_ERR_INVALID_ARG;
    }
    p.original_document = number->valueint;
    // Canonical backend semantics are Datecs:0 operator,1 return,2 tax base.
    // Daisy v2.0-4 p17 swaps0/1; Datecs PM v2.11.4 section4.6 does not.
    p.reversal_reason = binding_.profile == EdgeProfile::DaisyCompactS01 && reason->valueint < 2
                            ? 1 - reason->valueint : reason->valueint;
    std::string original_unp;
    if (cJSON_GetObjectItemCaseSensitive(original, "original_unp")) {
      if (!text(original, "original_unp", original_unp, 21)) return ESP_ERR_INVALID_ARG;
    } else if (!text(payload, "unp", original_unp, 21)) return ESP_ERR_INVALID_ARG;
    std::string legacy_unp;
    if (text(payload, "unp", legacy_unp, 21) && legacy_unp != original_unp)
      return ESP_ERR_INVALID_ARG;
    if (original_unp.size() != 21 || original_unp[8] != '-' || original_unp[13] != '-')
      return ESP_ERR_INVALID_ARG;
    for (size_t i=0;i<original_unp.size();++i) {
      const unsigned char c=original_unp[i];
      if (i<2 ? (c<'A'||c>'Z') : i<8 || i>13 ? (c<'0'||c>'9') : i==8 || i==13 ? false : !((c>='A'&&c<='Z')||(c>='a'&&c<='z')||(c>='0'&&c<='9')))
        return ESP_ERR_INVALID_ARG;
    }
    p.unp = original_unp;
  }
  auto *items = cJSON_GetObjectItemCaseSensitive(payload, "items");
  auto *payments = cJSON_GetObjectItemCaseSensitive(payload, "payments");
  if (!cJSON_IsArray(items) || !cJSON_IsArray(payments) ||
      cJSON_GetArraySize(items) < 1 || cJSON_GetArraySize(payments) < 1)
    return ESP_ERR_INVALID_ARG;
  cJSON *item = nullptr;
  cJSON_ArrayForEach(item, items) {
    IntentLine l;
    auto *price = cJSON_GetObjectItemCaseSensitive(item, "unit_price");
    std::string price_currency;
    if ((!text(item, "line_id", l.id) && !text(item, "item_id", l.id)) ||
        !uuid(l.id) || !text(item, "name", l.name, 240) ||
        !text(item, "quantity", l.quantity, 32) || !cJSON_IsObject(price) ||
        !text(price, "amount", l.unit_price, 32) ||
        !text(price, "currency", price_currency, 3) || price_currency != "EUR" ||
        !text(item, "tax_group", l.tax_group, 8))
      return ESP_ERR_INVALID_ARG;
    auto *d = cJSON_GetObjectItemCaseSensitive(item, "discount");
    if (d && !cJSON_IsNull(d)) {
      std::string discount_currency;
      if (!cJSON_IsObject(d) || !text(d, "amount", l.discount, 32) ||
          !text(d, "currency", discount_currency, 3) || discount_currency != "EUR")
        return ESP_ERR_INVALID_ARG;
    }
    p.items.push_back(std::move(l));
  }
  int64_t total = 0;
  cJSON *payment = nullptr;
  cJSON_ArrayForEach(payment, payments) {
    IntentPayment v;
    auto *amount = cJSON_GetObjectItemCaseSensitive(payment, "amount");
    std::string currency;
    if (!text(payment, "payment_id", v.id) || !uuid(v.id) ||
        !text(payment, "type", v.type, 8) ||
        (v.type != "CASH" && v.type != "CARD") || !cJSON_IsObject(amount) ||
        !text(amount, "amount", v.amount, 32) ||
        !text(amount, "currency", currency, 3) || currency != "EUR" ||
        !money(v.amount, v.amount_minor))
      return ESP_ERR_INVALID_ARG;
    total += v.amount_minor;
    p.payments.push_back(std::move(v));
  }
  return total > 0 ? ESP_OK : ESP_ERR_INVALID_ARG;
}
esp_err_t IntentProcessor::execute(ReceiptPlan &p, Ingress ingress) {
  const int64_t at = now();
  std::string event = "received-" + p.operation_id;
  // Never consume authority or touch a final device when the durable journal
  // cannot retain the command, result and sync evidence.
  if (!storage_.can_accept_operation())
    return ESP_ERR_NO_MEM;
  std::vector<InitialPayment> initial;
  for (const auto &v : p.payments)
    initial.push_back({v.id, v.amount_minor});
  auto reserved = storage_.reserve_command(
      p.operation_id.c_str(), p.payload_digest.c_str(),
      ingress == Ingress::Mqtt   ? "MQTT"
      : ingress == Ingress::Http ? "HTTP"
                                 : "BLE",
      p.route_snapshot.c_str(), event.c_str(), p.receipt_session_id.c_str(),
      p.canonical_payload.c_str(), initial, at,
      [&](const AuthorityReservation &a) {
        if (p.command_type != "SALE_REVERSE") p.unp = a.unp;
        return accepted_json(p, binding_, a);
      });
  if (reserved.status == ReservationStatus::Duplicate) {
    if (result_sink_ && !reserved.prior_result.empty())
      result_sink_(p.operation_id.c_str(), reserved.prior_result.c_str(),
                   ingress, result_context_);
    return ESP_OK;
  }
  if (reserved.status == ReservationStatus::PayloadConflict)
    return ESP_ERR_INVALID_CRC;
  if (reserved.status != ReservationStatus::New)
    return ESP_FAIL;
  if (p.command_type != "SALE_REVERSE") p.unp = reserved.authority.unp;
  if (!executor_.ready()) {
    std::string unavailable =
        "{\"error_code\":\"PHYSICAL_DEVICE_UNAVAILABLE\",\"operation_id\":\"" +
        p.operation_id + "\",\"state\":\"RECOVERY_REQUIRED\"}";
    storage_.set_operation_state(p.operation_id.c_str(), "RECOVERY_REQUIRED",
                                 unavailable.c_str(), at);
    if (result_sink_)
      result_sink_(p.operation_id.c_str(), unavailable.c_str(), ingress,
                   result_context_);
    return ESP_ERR_INVALID_STATE;
  }
  // A durable checkpoint is mandatory before any final-device side effect.
  const auto checkpoint = storage_.set_operation_state(
      p.operation_id.c_str(), "EXECUTING", nullptr, at);
  if (checkpoint != ESP_OK)
    return checkpoint;
  auto r = executor_.execute(p);
  auto result = result_json(p.operation_id, r, p.command_type == "SALE_REVERSE" ? "REVERSAL" : p.command_type);
  if (result.empty()) return ESP_ERR_NO_MEM;
  // CARD states and references belong to individual executor checkpoints.
  // An operation-level result cannot safely overwrite per-tender evidence.
  std::vector<InitialPayment> cash_payments;
  for (const auto &v : p.payments)
    if (v.type == "CASH")
      cash_payments.push_back({v.id, v.amount_minor});
  const std::string eid = "result-" + p.operation_id;
  const auto stored = storage_.complete_operation(
      p.operation_id.c_str(), p.receipt_session_id.c_str(), state(r.certainty),
      p.canonical_payload.c_str(), result.c_str(), cash_payments, eid.c_str(),
      "RECEIPT_RESULT", hash(result.data(), result.size()).c_str(), now());
  if (stored == ESP_OK && result_sink_)
    result_sink_(p.operation_id.c_str(), result.c_str(), ingress,
                 result_context_);
  return stored;
}
esp_err_t IntentProcessor::accept(const CommandView &v) {
  ReceiptPlan p;
  esp_err_t e = decode(v, p);
  return e == ESP_OK ? execute(p, v.ingress) : e;
}
esp_err_t IntentProcessor::recover_pending() {
  esp_err_t result = ESP_OK;
  const auto scanned = storage_.recovery(128, [&](const RecoveryItem &i) {
    // Legacy incomplete snapshots cannot be projected atomically. Preserve the
    // pending operation and do not contact devices without its receipt context.
    if (i.receipt_id.empty() || i.canonical_payload.empty()) {
      result = ESP_FAIL;
      return true;
    }
    auto r = executor_.recover(i);
    if (r.terminal_reference.empty()) r.terminal_reference = i.terminal_reference;
    if (r.rrn.empty()) r.rrn = i.rrn;
    if (r.auth_code.empty()) r.auth_code = i.auth_code;
    Json route(cJSON_Parse(i.route_snapshot.c_str()));
    std::string operation_type;
    text(route.get(), "command_type", operation_type, 32);
    const auto payload = result_json(i.operation_id, r, operation_type);
    if (payload.empty()) { result = ESP_ERR_NO_MEM; return true; }
    const int64_t at = now();
    const auto digest = hash(payload.data(), payload.size());
    // Recovery may progress UNKNOWN -> COMMITTED. Content-address the evidence
    // so an earlier uncertain result cannot mask the later terminal projection.
    const std::string event = "recovery-" + i.operation_id + "-" + digest;
    // RecoveryItem currently exposes only one selected payment. Its result is
    // not sufficient authority to rewrite all (or an arbitrary selected) tender
    // records. Keep executor-owned durable payment checkpoints unchanged.
    if (storage_.complete_operation(
            i.operation_id.c_str(), i.receipt_id.c_str(), state(r.certainty),
            i.canonical_payload.c_str(), payload.c_str(), {}, event.c_str(),
            "RECOVERY_RESULT", digest.c_str(), at) != ESP_OK)
      result = ESP_FAIL;
    return true;
  });
  return scanned == ESP_OK ? result : scanned;
}
} // namespace beefiscal::idf
