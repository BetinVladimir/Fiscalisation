#pragma once
#include <cstddef>
#include <cstdint>
#include <string>
#include "esp_err.h"
namespace beefiscal::idf{
class DeviceIdentity final{
 public:DeviceIdentity();~DeviceIdentity();
  // First open/reload seeds the identity DRBG: invoke under with_startup_entropy
  // before RF/ADC initialization. Later signing/ECDH never toggles SAR entropy.
  esp_err_t open();bool ready()const;
  esp_err_t sign_hash_hex(const char*hash_hex,std::string&signature)const;
  esp_err_t provisioning_shared_secret(const uint8_t peer_point[65],uint8_t shared[32])const;
  // Local TLS only: never transmit this value over BLE/HTTP or log it.
  esp_err_t sign_provisioning_install(const std::string&binding_hash,int64_t generation,const std::string&package_hash,std::string&signature)const;
  esp_err_t local_tls_key_pem(std::string&)const;
  std::string public_key_der_base64url()const;std::string kid()const;
 private:struct Impl;Impl*impl_;
};
}
