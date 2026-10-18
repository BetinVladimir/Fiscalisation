#pragma once
#include "device_identity.h"
#include "provisioning_crypto.h"
#include "runtime_config.h"
namespace beefiscal::idf {
struct ProvisionedRuntime {
 CompositeBinding binding;RuntimeSecrets secrets;std::string wifi_ssid,wifi_password,package_sha256;
 void clear();
};
// A single committed encrypted blob is the configuration transaction. The
// legacy binding namespace is never partially overwritten by this installer.
class EncryptedProvisioning final {
 public:EncryptedProvisioning(DeviceIdentity&,ProvisionedBindingStore&,const char*,const char*);
 esp_err_t load(ProvisionedRuntime&);
 esp_err_t install(const uint8_t*,size_t,const uint8_t fresh_nonce[32],ProvisionedRuntime&);
 private:DeviceIdentity&identity_;ProvisionedBindingStore&validator_;const char*issuer_pem_;const char*issuer_kid_;
 esp_err_t decode(const uint8_t*,size_t,const ProvisioningSubject&,ProvisionedRuntime&);
};
}
