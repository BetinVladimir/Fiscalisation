#pragma once
#include <string>
#include "esp_err.h"
#include "provisioned_binding.h"

namespace beefiscal::idf{
struct RuntimeSecrets{std::string root_ca_pem,client_certificate_pem,client_key_pem,
  command_hmac_key,sync_ack_hmac_key;};
class RuntimeConfigLoader final{
 public:
  esp_err_t load_secrets(const CompositeBinding&,RuntimeSecrets&)const;
 private:
  static esp_err_t read_protected_nvs(const std::string&,std::string&);
};
}
