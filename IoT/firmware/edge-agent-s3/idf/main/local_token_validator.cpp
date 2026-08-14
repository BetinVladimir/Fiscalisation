#include "local_token_validator.h"
#include "cJSON.h"
#include "mbedtls/base64.h"
#include "mbedtls/pk.h"
#include "mbedtls/sha256.h"
#include <algorithm>
#include <ctime>
#include <memory>
#include <vector>

namespace beefiscal::idf {
namespace {
struct Del {
  void operator()(cJSON *value) const { cJSON_Delete(value); }
};
using Json = std::unique_ptr<cJSON, Del>;
bool b64url(const std::string &raw, std::vector<uint8_t> &out) {
  std::string value = raw;
  for (char &c : value) {
    if (c == '-')
      c = '+';
    else if (c == '_')
      c = '/';
  }
  while (value.size() % 4)
    value += '=';
  out.resize(value.size());
  size_t size = 0;
  if (mbedtls_base64_decode(out.data(), out.size(), &size,
                            (const uint8_t *)value.data(), value.size()))
    return false;
  out.resize(size);
  return true;
}
bool text(const cJSON *root, const char *key, std::string &out) {
  auto *v = cJSON_GetObjectItemCaseSensitive(root, key);
  if (!cJSON_IsString(v) || !v->valuestring || !*v->valuestring)
    return false;
  out = v->valuestring;
  return true;
}
bool number(const cJSON *root, const char *key, int64_t &out) {
  auto *v = cJSON_GetObjectItemCaseSensitive(root, key);
  if (!cJSON_IsNumber(v) || v->valuedouble != v->valueint)
    return false;
  out = v->valueint;
  return true;
}
bool audience(const cJSON *root, const std::string &needle) {
  auto *v = cJSON_GetObjectItemCaseSensitive(root, "aud");
  if (cJSON_IsString(v))
    return needle == v->valuestring;
  if (!cJSON_IsArray(v))
    return false;
  cJSON *x = nullptr;
  cJSON_ArrayForEach(x, v) if (cJSON_IsString(x) &&
                               needle == x->valuestring) return true;
  return false;
}
bool scope(const std::string &scopes, const std::string &needle) {
  size_t at = 0;
  while (at < scopes.size()) {
    auto end = scopes.find(' ', at);
    if (scopes.substr(at, end == std::string::npos ? end : end - at) == needle)
      return true;
    if (end == std::string::npos)
      break;
    at = end + 1;
  }
  return false;
}
bool p1363_der(const std::vector<uint8_t> &raw, std::vector<uint8_t> &der) {
  if (raw.size() != 64)
    return false;
  auto integer = [](const uint8_t *p) {
    size_t at = 0;
    while (at < 31 && p[at] == 0)
      at++;
    std::vector<uint8_t> v(p + at, p + 32);
    if (v.empty())
      v.push_back(0);
    if (v[0] & 0x80)
      v.insert(v.begin(), 0);
    return v;
  };
  auto r = integer(raw.data()), s = integer(raw.data() + 32);
  const size_t n = 2 + r.size() + 2 + s.size();
  if (n > 127)
    return false;
  der = {0x30, (uint8_t)n, 0x02, (uint8_t)r.size()};
  der.insert(der.end(), r.begin(), r.end());
  der.push_back(0x02);
  der.push_back((uint8_t)s.size());
  der.insert(der.end(), s.begin(), s.end());
  return true;
}
} // namespace
esp_err_t validate_local_token(const char *jwt, const char *required,
                               const CompositeBinding &binding,
                               LocalTokenClaims &claims) {
  claims = {};
  if (!jwt || !required || !binding.local_http.enabled)
    return ESP_ERR_INVALID_STATE;
  std::string raw = jwt;
  const auto first = raw.find('.'), second = first == std::string::npos
                                                 ? first
                                                 : raw.find('.', first + 1);
  if (first == std::string::npos || second == std::string::npos ||
      raw.find('.', second + 1) != std::string::npos)
    return ESP_ERR_INVALID_ARG;
  std::vector<uint8_t> headerBytes, bodyBytes, signature, keyDer, signatureDer;
  if (!b64url(raw.substr(0, first), headerBytes) ||
      !b64url(raw.substr(first + 1, second - first - 1), bodyBytes) ||
      !b64url(raw.substr(second + 1), signature) ||
      !p1363_der(signature, signatureDer))
    return ESP_ERR_INVALID_CRC;
  headerBytes.push_back(0);
  bodyBytes.push_back(0);
  Json header(cJSON_Parse((const char *)headerBytes.data())),
      body(cJSON_Parse((const char *)bodyBytes.data()));
  std::string alg, kid;
  if (!header || !body || !text(header.get(), "alg", alg) || alg != "ES256" ||
      !text(header.get(), "kid", kid) ||
      kid != binding.local_http.token_signing_kid)
    return ESP_ERR_INVALID_CRC;
  if (!b64url(binding.local_http.token_public_key_der_base64, keyDer))
    return ESP_ERR_INVALID_STATE;
  mbedtls_pk_context key;
  mbedtls_pk_init(&key);
  int rc = mbedtls_pk_parse_public_key(&key, keyDer.data(), keyDer.size());
  const std::string signing = raw.substr(0, second);
  uint8_t digest[32]{};
  if (rc == 0)
    rc = mbedtls_sha256((const uint8_t *)signing.data(), signing.size(), digest,
                        0);
  if (rc == 0)
    rc = mbedtls_pk_verify(&key, MBEDTLS_MD_SHA256, digest, sizeof(digest),
                           signatureDer.data(), signatureDer.size());
  mbedtls_pk_free(&key);
  if (rc)
    return ESP_ERR_INVALID_CRC;
  std::string issuer, tenant, location, reg, adapter, scopes;
  if (!text(body.get(), "iss", issuer) ||
      issuer != binding.local_http.token_issuer ||
      !audience(body.get(), "beeloy-local-fiscal-adapter") ||
      !audience(body.get(), binding.edge_device_id) ||
      !text(body.get(), "tenant_id", tenant) || tenant != binding.tenant_id ||
      !text(body.get(), "location_id", location) ||
      location != binding.location_id ||
      !text(body.get(), "register_id", reg) || reg != binding.register_id ||
      !text(body.get(), "adapter_device_id", adapter) ||
      adapter != binding.edge_device_id || !text(body.get(), "scope", scopes) ||
      !scope(scopes, required))
    return ESP_ERR_INVALID_STATE;
  int64_t generation = 0, iat = 0, nbf = 0, exp = 0;
  if (!number(body.get(), "binding_generation", generation) ||
      generation != binding.generation || !number(body.get(), "iat", iat) ||
      !number(body.get(), "nbf", nbf) ||
      !number(body.get(), "exp", exp))
    return ESP_ERR_INVALID_STATE;
  const auto now = (int64_t)std::time(nullptr);
  if (now < 1704067200 || iat > now + 30 || nbf > now + 30 || exp <= now - 30 ||
      nbf < iat - 30 || exp <= iat || exp - iat > 900)
    return ESP_ERR_INVALID_STATE;
  if (!text(body.get(), "sub", claims.subject) ||
      !text(body.get(), "operator_id", claims.operator_id) ||
      !text(body.get(), "shift_id", claims.shift_id) ||
      !text(body.get(), "jti", claims.jti))
    return ESP_ERR_INVALID_STATE;
  claims.expires_at = exp;
  return ESP_OK;
}
} // namespace beefiscal::idf
