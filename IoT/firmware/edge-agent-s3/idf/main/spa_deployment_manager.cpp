#include "spa_deployment_manager.h"
#include "cJSON.h"
#include "esp_crt_bundle.h"
#include "esp_http_client.h"
#include "mbedtls/base64.h"
#include "mbedtls/pk.h"
#include "mbedtls/sha256.h"
#include <algorithm>
#include <cctype>
#include <cerrno>
#include <cstdio>
#include <cstring>
#include <cstdlib>
#include <dirent.h>
#include <memory>
#include <sys/stat.h>
#include <unistd.h>
#include <vector>

namespace beefiscal::idf {
namespace {
struct Del {
  void operator()(cJSON *v) const { cJSON_Delete(v); }
};
using Json = std::unique_ptr<cJSON, Del>;
bool path_ok(const std::string &path) {
  if (path.empty() || path.size() > 240 || path[0] == '/' ||
      path.find("\\") != std::string::npos)
    return false;
  size_t at = 0;
  while (at <= path.size()) {
    auto end = path.find('/', at);
    auto part = path.substr(at, end == std::string::npos ? end : end - at);
    if (part.empty() || part == "..")
      return false;
    if (end == std::string::npos)
      break;
    at = end + 1;
  }
  for (char c : path)
    if (!(isalnum((unsigned char)c) || c == '.' || c == '_' || c == '-' ||
          c == '/'))
      return false;
  return true;
}
bool version_parts(const std::string &value, int out[3]) {
  size_t at = 0;
  for (int i = 0; i < 3; ++i) {
    const auto end = value.find(i == 2 ? '-' : '.', at);
    const auto part = value.substr(at, end == std::string::npos ? end : end - at);
    if (part.empty() || part.size() > 9 ||
        !std::all_of(part.begin(), part.end(), [](char c) { return isdigit((unsigned char)c); }))
      return false;
    out[i] = atoi(part.c_str());
    if (i < 2 && end == std::string::npos)
      return false;
    at = end == std::string::npos ? value.size() : end + 1;
  }
  return true;
}
int compare_versions(const std::string &left, const std::string &right) {
  int a[3]{}, b[3]{};
  if (!version_parts(left, a) || !version_parts(right, b))
    return -2;
  for (int i = 0; i < 3; ++i)
    if (a[i] != b[i])
      return a[i] < b[i] ? -1 : 1;
  return 0;
}
bool mkdirs(const std::string &path) {
  std::string current;
  for (size_t i = 1; i <= path.size(); i++)
    if (i == path.size() || path[i] == '/') {
      current = path.substr(0, i);
      if (!current.empty() && mkdir(current.c_str(), 0755) && errno != EEXIST)
        return false;
    }
  return true;
}
bool remove_tree(const std::string &path) {
  DIR *dir = opendir(path.c_str());
  if (!dir)
    return errno == ENOENT;
  dirent *entry;
  bool ok = true;
  while ((entry = readdir(dir))) {
    if (!strcmp(entry->d_name, ".") || !strcmp(entry->d_name, ".."))
      continue;
    std::string child = path + "/" + entry->d_name;
    struct stat st{};
    if (stat(child.c_str(), &st))
      ok = false;
    else if (S_ISDIR(st.st_mode))
      ok = remove_tree(child) && ok;
    else
      ok = unlink(child.c_str()) == 0 && ok;
  }
  closedir(dir);
  return rmdir(path.c_str()) == 0 && ok;
}
bool b64(const std::string &raw, std::vector<uint8_t> &out) {
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
esp_err_t get_memory(const std::string &url, size_t limit,
                     std::vector<uint8_t> &out) {
  out.clear();
  esp_http_client_config_t config{};
  config.url = url.c_str();
  config.timeout_ms = 30000;
  config.crt_bundle_attach = esp_crt_bundle_attach;
  config.disable_auto_redirect = true;
  auto client = esp_http_client_init(&config);
  if (!client)
    return ESP_ERR_NO_MEM;
  esp_err_t result = esp_http_client_open(client, 0);
  if (result == ESP_OK) {
    const int64_t declared = esp_http_client_fetch_headers(client);
    if (declared < 0 || (uint64_t)declared > limit)
      result = ESP_ERR_INVALID_SIZE;
    uint8_t buffer[2048];
    while (result == ESP_OK) {
      const int read =
          esp_http_client_read(client, (char *)buffer, sizeof(buffer));
      if (read < 0) {
        result = ESP_FAIL;
        break;
      }
      if (!read)
        break;
      if (out.size() + read > limit) {
        result = ESP_ERR_INVALID_SIZE;
        break;
      }
      out.insert(out.end(), buffer, buffer + read);
    }
    if (result == ESP_OK && esp_http_client_get_status_code(client) != 200)
      result = ESP_ERR_HTTP_BASE;
  }
  esp_http_client_close(client);
  esp_http_client_cleanup(client);
  return result;
}
std::string origin(const std::string &url) {
  auto scheme = url.find("://");
  if (scheme == std::string::npos)
    return {};
  auto slash = url.find('/', scheme + 3);
  return slash == std::string::npos ? url : url.substr(0, slash);
}
std::string hex_sha(const std::vector<uint8_t> &value) {
  uint8_t digest[32]{};
  if (mbedtls_sha256(value.data(), value.size(), digest, 0))
    return {};
  static const char chars[] = "0123456789abcdef";
  std::string out(64, '0');
  for (size_t i = 0; i < 32; i++) {
    out[i * 2] = chars[digest[i] >> 4];
    out[i * 2 + 1] = chars[digest[i] & 15];
  }
  return out;
}
bool verify_descriptor(cJSON *root, const LocalHttpBinding &trust) {
  auto *signature = cJSON_DetachItemFromObjectCaseSensitive(root, "signature");
  if (!cJSON_IsObject(signature)) {
    cJSON_Delete(signature);
    return false;
  }
  auto *kid = cJSON_GetObjectItemCaseSensitive(signature, "kid"),
       *alg = cJSON_GetObjectItemCaseSensitive(signature, "alg"),
       *value = cJSON_GetObjectItemCaseSensitive(signature, "value");
  if (!cJSON_IsString(kid) ||
      trust.deployment_signing_kid != kid->valuestring ||
      !cJSON_IsString(alg) || strcmp(alg->valuestring, "Ed25519") ||
      !cJSON_IsString(value)) {
    cJSON_Delete(signature);
    return false;
  }
  std::vector<uint8_t> sig, key;
  if (!b64(value->valuestring, sig) ||
      !b64(trust.deployment_public_key_base64, key)) {
    cJSON_Delete(signature);
    return false;
  }
  char *unsigned_json = cJSON_PrintUnformatted(root);
  cJSON_Delete(signature);
  if (!unsigned_json)
    return false;
  mbedtls_pk_context pk;
  mbedtls_pk_init(&pk);
  int rc = mbedtls_pk_parse_public_key(&pk, key.data(), key.size());
  if (rc == 0)
    rc = mbedtls_pk_verify(&pk, MBEDTLS_MD_NONE, (const uint8_t *)unsigned_json,
                           strlen(unsigned_json), sig.data(), sig.size());
  mbedtls_pk_free(&pk);
  cJSON_free(unsigned_json);
  return rc == 0;
}
} // namespace
SpaDeploymentManager::SpaDeploymentManager(const CompositeBinding &binding,
                                           const char *root)
    : binding_(binding), root_(root ? root : "") {
  load_state();
}
DeploymentState SpaDeploymentManager::state() const { return state_; }
std::string SpaDeploymentManager::active_root() const {
  FILE *f = fopen((root_ + "/active").c_str(), "rb");
  char slot[16]{};
  if (!f || !fgets(slot, sizeof(slot), f)) {
    if (f)
      fclose(f);
    return root_ + "/slot-a";
  }
  fclose(f);
  slot[strcspn(slot, "\r\n")] = 0;
  return root_ + "/" + (strcmp(slot, "slot-b") == 0 ? "slot-b" : "slot-a");
}
esp_err_t SpaDeploymentManager::load_state() {
  mkdirs(root_);
  FILE *f = fopen((root_ + "/state.json").c_str(), "rb");
  if (!f)
    return ESP_ERR_NOT_FOUND;
  char body[1024]{};
  const size_t size = fread(body, 1, sizeof(body) - 1, f);
  fclose(f);
  Json root(cJSON_ParseWithLengthOpts(body, size, nullptr, false));
  if (!root)
    return ESP_ERR_INVALID_CRC;
  auto *version = cJSON_GetObjectItemCaseSensitive(root.get(), "version"),
       *build = cJSON_GetObjectItemCaseSensitive(root.get(), "build_id");
  if (!cJSON_IsString(version) || !cJSON_IsString(build))
    return ESP_ERR_INVALID_ARG;
  state_.version = version->valuestring;
  state_.build_id = build->valuestring;
  state_.state = "ACTIVE";
  return ESP_OK;
}
esp_err_t SpaDeploymentManager::persist_state(const char *slot) {
  const std::string temp = root_ + "/state.tmp",
                    activeTemp = root_ + "/active.tmp";
  FILE *f = fopen(temp.c_str(), "wb");
  if (!f)
    return ESP_FAIL;
  const std::string json = "{\"build_id\":\"" + state_.build_id +
                           "\",\"version\":\"" + state_.version + "\"}";
  bool ok = fwrite(json.data(), 1, json.size(), f) == json.size() &&
            fflush(f) == 0 && fsync(fileno(f)) == 0;
  fclose(f);
  if (!ok || rename(temp.c_str(), (root_ + "/state.json").c_str()))
    return ESP_FAIL;
  f = fopen(activeTemp.c_str(), "wb");
  if (!f)
    return ESP_FAIL;
  ok = fwrite(slot, 1, strlen(slot), f) == strlen(slot) && fflush(f) == 0 &&
       fsync(fileno(f)) == 0;
  fclose(f);
  return ok && !rename(activeTemp.c_str(), (root_ + "/active").c_str())
             ? ESP_OK
             : ESP_FAIL;
}
esp_err_t SpaDeploymentManager::check_and_activate() {
  const auto &trust = binding_.local_http;
  if (!trust.enabled ||
      trust.deployment_descriptor_url.rfind("https://", 0) != 0 ||
      trust.deployment_signing_kid.empty() ||
      trust.deployment_public_key_base64.empty())
    return ESP_ERR_INVALID_STATE;
  std::vector<uint8_t> descriptor;
  if (get_memory(trust.deployment_descriptor_url, 1024 * 1024, descriptor) !=
      ESP_OK) {
    state_.state = "FAILED";
    state_.error_code = "DESCRIPTOR_DOWNLOAD";
    return ESP_FAIL;
  }
  descriptor.push_back(0);
  Json root(cJSON_Parse((const char *)descriptor.data()));
  if (!root || !verify_descriptor(root.get(), trust)) {
    state_.state = "FAILED";
    state_.error_code = "DESCRIPTOR_SIGNATURE";
    return ESP_ERR_INVALID_CRC;
  }
  auto *schema = cJSON_GetObjectItemCaseSensitive(root.get(), "schema_version"),
       *app = cJSON_GetObjectItemCaseSensitive(root.get(), "application_id"),
       *version = cJSON_GetObjectItemCaseSensitive(root.get(), "version"),
       *build = cJSON_GetObjectItemCaseSensitive(root.get(), "build_id"),
       *entry = cJSON_GetObjectItemCaseSensitive(root.get(), "entrypoint"),
       *files = cJSON_GetObjectItemCaseSensitive(root.get(), "files");
  if (!cJSON_IsNumber(schema) || schema->valueint != 1 ||
      !cJSON_IsString(app) ||
      strcmp(app->valuestring, "com.beeloy.miniposweb") ||
      !cJSON_IsString(version) || !cJSON_IsString(build) ||
      !cJSON_IsString(entry) || strcmp(entry->valuestring, "index.html") ||
      !cJSON_IsArray(files) || cJSON_GetArraySize(files) < 1 ||
      cJSON_GetArraySize(files) > 512)
    return ESP_ERR_INVALID_ARG;
  if (state_.state == "ACTIVE" && state_.build_id == build->valuestring)
    return ESP_OK;
  if (state_.version != "none" && compare_versions(version->valuestring, state_.version) < 0) {
    state_.state = "FAILED";
    state_.error_code = "DEPLOYMENT_ROLLBACK_FORBIDDEN";
    return ESP_ERR_INVALID_VERSION;
  }
  const std::string current = active_root(),
                    slot = current == root_ + "/slot-a" ? "slot-b" : "slot-a",
                    staging = root_ + "/" + slot + ".tmp",
                    target = root_ + "/" + slot;
  remove_tree(staging);
  if (!mkdirs(staging))
    return ESP_FAIL;
  int64_t total = 0;
  const std::string base = origin(trust.deployment_descriptor_url);
  cJSON *file = nullptr;
  cJSON_ArrayForEach(file, files) {
    auto *path = cJSON_GetObjectItemCaseSensitive(file, "path"),
         *size = cJSON_GetObjectItemCaseSensitive(file, "size"),
         *sha = cJSON_GetObjectItemCaseSensitive(file, "sha256");
    if (!cJSON_IsString(path) || !path_ok(path->valuestring) ||
        !cJSON_IsNumber(size) || size->valueint < 0 ||
        size->valueint > 8388608 || !cJSON_IsString(sha) ||
        (total += size->valueint) > 32 * 1024 * 1024) {
      remove_tree(staging);
      return ESP_ERR_INVALID_SIZE;
    }
    std::vector<uint8_t> bytes;
    if (get_memory(base + "/" + path->valuestring, size->valueint + 1, bytes) !=
            ESP_OK ||
        bytes.size() != (size_t)size->valueint ||
        hex_sha(bytes) != sha->valuestring) {
      remove_tree(staging);
      return ESP_ERR_INVALID_CRC;
    }
    const std::string destination = staging + "/" + path->valuestring;
    auto slash = destination.rfind('/');
    if (!mkdirs(destination.substr(0, slash))) {
      remove_tree(staging);
      return ESP_FAIL;
    }
    FILE *out = fopen(destination.c_str(), "wb");
    const bool written =
        out && fwrite(bytes.data(), 1, bytes.size(), out) == bytes.size() &&
        fflush(out) == 0 && fsync(fileno(out)) == 0;
    if (out)
      fclose(out);
    if (!written) {
      remove_tree(staging);
      return ESP_FAIL;
    }
  }
  struct stat info{};
  if (stat((staging + "/index.html").c_str(), &info) ||
      !S_ISREG(info.st_mode)) {
    remove_tree(staging);
    return ESP_ERR_NOT_FOUND;
  }
  remove_tree(target);
  if (rename(staging.c_str(), target.c_str()))
    return ESP_FAIL;
  state_.version = version->valuestring;
  state_.build_id = build->valuestring;
  state_.state = "ACTIVE";
  state_.error_code.clear();
  return persist_state(slot.c_str());
}
} // namespace beefiscal::idf
