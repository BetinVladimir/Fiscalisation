#pragma once
#include <cstddef>
#include <cstdint>
#include <string>
#include "esp_err.h"
namespace beefiscal::idf{
class DeviceIdentity final{
 public:DeviceIdentity();~DeviceIdentity();esp_err_t open();bool ready()const;
  esp_err_t sign_hash_hex(const char*hash_hex,std::string&signature)const;
  std::string public_key_der_base64url()const;std::string kid()const;
 private:struct Impl;Impl*impl_;
};
}
