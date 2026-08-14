#include "local_http_server.h"
#include "cJSON.h"
#include "esp_http_server.h"
#include "local_token_validator.h"
#include <cstdio>
#include <cstring>
#include <ctime>
#include <memory>
#include <string>
#include <sys/stat.h>
#include <vector>

namespace beefiscal::idf {
namespace {
constexpr size_t kMaxBody = 8192;
struct Runtime {
  LocalHttpRuntime value;
  httpd_handle_t server{};
} runtime;
struct DeleteJson {
  void operator()(cJSON *value) const { cJSON_Delete(value); }
};
using Json = std::unique_ptr<cJSON, DeleteJson>;

esp_err_t send(httpd_req_t *request, int status, const char *type,
               const std::string &body) {
  const char *text = status == 200   ? "200 OK"
                     : status == 202 ? "202 Accepted"
                     : status == 401 ? "401 Unauthorized"
                     : status == 404 ? "404 Not Found"
                     : status == 409 ? "409 Conflict"
                     : status == 422 ? "422 Unprocessable Entity"
                     : status == 503 ? "503 Service Unavailable"
                                     : "500 Internal Server Error";
  httpd_resp_set_status(request, text);
  httpd_resp_set_type(request, type);
  httpd_resp_set_hdr(request, "Cache-Control", "no-store");
  httpd_resp_set_hdr(request, "X-Content-Type-Options", "nosniff");
  return httpd_resp_send(request, body.data(), body.size());
}
esp_err_t problem(httpd_req_t *request, int status, const char *code) {
  return send(request, status, "application/problem+json",
              std::string("{\"status\":") + std::to_string(status) +
                  ",\"title\":\"" + code + "\",\"type\":\"about:blank\"}");
}
bool header(httpd_req_t *request, const char *name, std::string &out,
            size_t max = 2048) {
  const size_t size = httpd_req_get_hdr_value_len(request, name);
  if (!size || size > max)
    return false;
  std::vector<char> buffer(size + 1);
  if (httpd_req_get_hdr_value_str(request, name, buffer.data(),
                                  buffer.size()) != ESP_OK)
    return false;
  out.assign(buffer.data(), size);
  return true;
}
bool authorize(httpd_req_t *request, const char *scope) {
  std::string value;
  if (!header(request, "Authorization", value) ||
      value.rfind("Bearer ", 0) != 0)
    return false;
  LocalTokenClaims claims;
  return validate_local_token(value.c_str() + 7, scope, *runtime.value.binding,
                              claims) == ESP_OK;
}
bool body(httpd_req_t *request, std::string &out) {
  if (request->content_len <= 0 || request->content_len > (int)kMaxBody)
    return false;
  out.resize(request->content_len);
  size_t offset = 0;
  while (offset < out.size()) {
    const int read =
        httpd_req_recv(request, out.data() + offset, out.size() - offset);
    if (read <= 0)
      return false;
    offset += read;
  }
  return true;
}
std::string iso8601(int64_t epoch) {
  char out[32]{};
  time_t value = epoch;
  tm utc{};
  gmtime_r(&value, &utc);
  strftime(out, sizeof(out), "%Y-%m-%dT%H:%M:%SZ", &utc);
  return out;
}
std::string operation_json(const OperationRecord &operation) {
  Json root(operation.result_payload.empty()
                ? cJSON_CreateObject()
                : cJSON_Parse(operation.result_payload.c_str()));
  if (!root)
    root.reset(cJSON_CreateObject());
  if (!cJSON_GetObjectItemCaseSensitive(root.get(), "operation_id"))
    cJSON_AddStringToObject(root.get(), "operation_id",
                            operation.operation_id.c_str());
  if (!cJSON_GetObjectItemCaseSensitive(root.get(), "state"))
    cJSON_AddStringToObject(root.get(), "state", operation.state.c_str());
  const char *certainty =
      operation.state == "COMMITTED" ? "PROVEN_SUCCESS"
      : operation.state == "REJECTED" || operation.state == "COMPENSATED"
          ? "PROVEN_FAILURE"
      : operation.state == "RESERVED" ? "NOT_SENT"
                                      : "UNKNOWN";
  cJSON_AddStringToObject(root.get(), "certainty", certainty);
  cJSON_AddStringToObject(root.get(), "updated_at",
                          iso8601(operation.updated_at).c_str());
  char *raw = cJSON_PrintUnformatted(root.get());
  std::string result = raw ? raw : "{}";
  cJSON_free(raw);
  return result;
}
esp_err_t liveness(httpd_req_t *request) {
  return send(request, 200, "application/json",
              "{\"api_version\":\"2026-08-14\",\"status\":\"alive\"}");
}
esp_err_t deployment(httpd_req_t *request) {
  const auto state = runtime.value.deployments
                         ? runtime.value.deployments->state()
                         : DeploymentState{};
  return send(request, 200, "application/json",
              "{\"application_id\":\"" + state.application_id +
                  "\",\"build_id\":\"" + state.build_id +
                  "\",\"error_code\":\"" + state.error_code +
                  "\",\"state\":\"" + state.state + "\",\"version\":\"" +
                  state.version + "\"}");
}
esp_err_t device(httpd_req_t *request) {
  if (!authorize(request, "fiscal.read"))
    return problem(request, 401, "TOKEN_INVALID");
  const bool fiscal =
      runtime.value.fiscal_ready &&
      runtime.value.fiscal_ready(runtime.value.readiness_context);
  const bool payment =
      !runtime.value.binding->payment.present ||
      (runtime.value.payment_ready &&
       runtime.value.payment_ready(runtime.value.readiness_context));
  char json[1400];
  snprintf(json, sizeof(json),
           "{\"adapter_device_id\":\"%s\",\"binding_generation\":%lld,"
           "\"endpoints\":[{\"configured\":true,\"reachable\":%s,\"role\":"
           "\"FISCAL_DEVICE\",\"state\":\"%s\"},{\"configured\":%s,"
           "\"reachable\":%s,\"role\":\"PAYMENT_TERMINAL\",\"state\":\"%s\"}],"
           "\"observed_at\":\"%s\",\"register_id\":\"%s\",\"state\":\"%s\"}",
           runtime.value.binding->edge_device_id.c_str(),
           (long long)runtime.value.binding->generation,
           fiscal ? "true" : "false", fiscal ? "READY" : "OFFLINE",
           runtime.value.binding->payment.present ? "true" : "false",
           payment ? "true" : "false", payment ? "READY" : "OFFLINE",
           iso8601(std::time(nullptr)).c_str(),
           runtime.value.binding->register_id.c_str(),
           fiscal && payment ? "READY"
           : fiscal          ? "DEGRADED"
                             : "OFFLINE");
  return send(request, fiscal ? 200 : 503, "application/json", json);
}
esp_err_t intents(httpd_req_t *request) {
  if (!authorize(request, "fiscal.execute"))
    return problem(request, 401, "TOKEN_INVALID");
  std::string version, idempotency;
  if (!header(request, "X-Beeloy-API-Version", version, 32) ||
      version != "2026-08-14" ||
      !header(request, "Idempotency-Key", idempotency, 64))
    return problem(request, 422, "REQUIRED_HEADERS");
  std::string raw;
  if (!body(request, raw))
    return problem(request, 422, "BODY_INVALID");
  Json root(cJSON_ParseWithLengthOpts(raw.data(), raw.size(), nullptr, false));
  if (!root || !cJSON_IsObject(root.get()))
    return problem(request, 422, "INTENT_INVALID");
  auto *operation =
      cJSON_GetObjectItemCaseSensitive(root.get(), "client_operation_id");
  auto *command = cJSON_GetObjectItemCaseSensitive(root.get(), "command");
  if (!cJSON_IsString(operation) || !cJSON_IsString(command) ||
      idempotency != operation->valuestring)
    return problem(request, 422, "IDEMPOTENCY_KEY_MISMATCH");
  OperationRecord existing;
  const bool duplicate = runtime.value.storage->operation(
                             operation->valuestring, existing) == ESP_OK;
  if (!cJSON_GetObjectItemCaseSensitive(root.get(), "intent_id"))
    cJSON_AddStringToObject(root.get(), "intent_id", operation->valuestring);
  if (!cJSON_GetObjectItemCaseSensitive(root.get(), "action"))
    cJSON_AddStringToObject(root.get(), "action", command->valuestring);
  if (!cJSON_GetObjectItemCaseSensitive(root.get(), "edge_device_id"))
    cJSON_AddStringToObject(root.get(), "edge_device_id",
                            runtime.value.binding->edge_device_id.c_str());
  char *normalized = cJSON_PrintUnformatted(root.get());
  if (!normalized)
    return problem(request, 500, "OUT_OF_MEMORY");
  const esp_err_t executed = runtime.value.queue->enqueue_and_wait(
      {(const uint8_t *)normalized, strlen(normalized), Ingress::Http});
  cJSON_free(normalized);
  OperationRecord stored;
  if (runtime.value.storage->operation(operation->valuestring, stored) !=
      ESP_OK)
    return problem(request, executed == ESP_ERR_INVALID_CRC ? 409 : 422,
                   "INTENT_REJECTED");
  if (executed == ESP_ERR_INVALID_CRC)
    return problem(request, 409, "IDEMPOTENCY_PAYLOAD_CONFLICT");
  return send(request, duplicate ? 200 : 202, "application/json",
              operation_json(stored));
}
std::string operation_id(const char *uri) {
  std::string path = uri ? uri : "";
  const std::string prefix = "/beeloy/local/v1/operations/";
  if (path.rfind(prefix, 0) != 0)
    return {};
  path.erase(0, prefix.size());
  const auto suffix = path.find(":reconcile");
  if (suffix != std::string::npos)
    path.resize(suffix);
  return path;
}
esp_err_t operations(httpd_req_t *request) {
  if (!authorize(request, "fiscal.read"))
    return problem(request, 401, "TOKEN_INVALID");
  const auto id = operation_id(request->uri);
  OperationRecord operation;
  if (id.empty() ||
      runtime.value.storage->operation(id.c_str(), operation) != ESP_OK)
    return problem(request, 404, "OPERATION_NOT_FOUND");
  return send(request, 200, "application/json", operation_json(operation));
}
bool suffix(const std::string &value, const char *ending) {
  const size_t size = strlen(ending);
  return value.size() >= size &&
         value.compare(value.size() - size, size, ending) == 0;
}
const char *mime(const std::string &path) {
  if (suffix(path, ".html"))
    return "text/html";
  if (suffix(path, ".js"))
    return "text/javascript";
  if (suffix(path, ".css"))
    return "text/css";
  if (suffix(path, ".json"))
    return "application/json";
  return "application/octet-stream";
}
esp_err_t static_file(httpd_req_t *request) {
  std::string relative =
      strcmp(request->uri, "/") ? request->uri + 1 : "index.html";
  if (relative.empty() || relative.find("..") != std::string::npos ||
      relative.find('\\') != std::string::npos)
    return problem(request, 404, "NOT_FOUND");
  const std::string active = runtime.value.deployments
                                 ? runtime.value.deployments->active_root()
                                 : runtime.value.spa_root;
  const std::string path = active + "/" + relative;
  struct stat info{};
  if (stat(path.c_str(), &info) || !S_ISREG(info.st_mode) ||
      info.st_size > 4 * 1024 * 1024)
    return problem(request, 404, "NOT_FOUND");
  FILE *file = fopen(path.c_str(), "rb");
  if (!file)
    return problem(request, 404, "NOT_FOUND");
  httpd_resp_set_type(request, mime(path));
  httpd_resp_set_hdr(request, "X-Content-Type-Options", "nosniff");
  httpd_resp_set_hdr(request, "Content-Security-Policy",
                     "default-src 'self'; connect-src 'self' http:; object-src "
                     "'none'; base-uri 'none'; frame-ancestors 'none'");
  httpd_resp_set_hdr(request, "Cache-Control",
                     relative == "index.html"
                         ? "no-cache"
                         : "public,max-age=31536000,immutable");
  char chunk[2048];
  size_t count;
  esp_err_t result = ESP_OK;
  while ((count = fread(chunk, 1, sizeof(chunk), file)) > 0)
    if (httpd_resp_send_chunk(request, chunk, count) != ESP_OK) {
      result = ESP_FAIL;
      break;
    }
  fclose(file);
  httpd_resp_send_chunk(request, nullptr, 0);
  return result;
}
} // namespace
esp_err_t local_http_server_start(const LocalHttpRuntime &config) {
  if (runtime.server || !config.binding || !config.storage || !config.queue ||
      !config.binding->local_http.enabled)
    return ESP_ERR_INVALID_ARG;
  runtime.value = config;
  httpd_config_t server = HTTPD_DEFAULT_CONFIG();
  server.server_port = config.binding->local_http.port;
  server.uri_match_fn = httpd_uri_match_wildcard;
  server.max_uri_handlers = 12;
  server.stack_size = 8192;
  server.recv_wait_timeout = 15;
  server.send_wait_timeout = 120;
  if (httpd_start(&runtime.server, &server) != ESP_OK)
    return ESP_FAIL;
  const httpd_uri_t routes[] = {
      {"/beeloy/local/v1/healthz", HTTP_GET, liveness, nullptr},
      {"/beeloy/local/v1/deployment", HTTP_GET, deployment, nullptr},
      {"/beeloy/local/v1/readyz", HTTP_GET, device, nullptr},
      {"/beeloy/local/v1/device", HTTP_GET, device, nullptr},
      {"/beeloy/local/v1/intents", HTTP_POST, intents, nullptr},
      {"/beeloy/local/v1/operations/*", HTTP_GET, operations, nullptr},
      {"/beeloy/local/v1/operations/*", HTTP_POST, operations, nullptr},
      {"/*", HTTP_GET, static_file, nullptr}};
  for (const auto &route : routes)
    if (httpd_register_uri_handler(runtime.server, &route) != ESP_OK)
      return ESP_FAIL;
  return ESP_OK;
}
} // namespace beefiscal::idf
