#pragma once
#include <cstddef>
#include <cstdint>
#include <string>
#include <vector>
#include "esp_err.h"
namespace beefiscal::idf {
constexpr size_t kProvisioningMaxPlaintext=32768,kProvisioningMaxPacket=34000;
using ProvisioningSharedSecret=esp_err_t(*)(void*,const uint8_t peer_point[65],uint8_t shared[32]);
struct ProvisioningSubject{std::string device_kid,issuer_kid;uint64_t generation_floor{};uint8_t setup_nonce[32]{};};
struct OpenedProvisioning{uint64_t generation{};uint8_t binding_sha256[32]{};std::vector<uint8_t>plaintext;void clear();};
// Trust key and expected subject come from firmware/factory state, never packet.
// A successful decrypt is not install/READY: caller must validate schema3 binding,
// exact binding digest/identity, secret inventory and atomic durable installation.
esp_err_t open_provisioning_package(const uint8_t*,size_t,const char*trusted_issuer_pem,
 const ProvisioningSubject&,ProvisioningSharedSecret,void*,OpenedProvisioning&);
}
