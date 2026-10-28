#pragma once
#include <string>
#include "esp_err.h"
#include "provisioned_binding.h"

namespace beefiscal::idf{
struct RuntimeSecrets{std::string root_ca_pem,client_certificate_pem,client_key_pem,
  command_hmac_key,sync_ack_hmac_key,fiscal_operator_password;};
class RuntimeConfigLoader final{
 public:
  // Trusted local provisioning only; never expose this through unsigned BLE/HTTP.
  static esp_err_t provision_fiscal_password(const CompositeBinding&,const std::string&);
  esp_err_t load_secrets(const CompositeBinding&,RuntimeSecrets&)const;
 private:
  static esp_err_t read_protected_nvs(const std::string&,std::string&);
};
}
