#pragma once
#include "device_identity.h"
#include "encrypted_provisioning.h"
#include "time_anchor_core.h"
#include <mutex>
namespace beefiscal::idf {
struct TimeAnchorEnvelope {std::string algorithm,canonical_json,key_id,signature;void clear(){algorithm.clear();canonical_json.clear();key_id.clear();signature.clear();}};
bool parse_time_anchor_envelope(const std::string&,TimeAnchorEnvelope&);
bool encode_time_anchor_envelope(const TimeAnchorEnvelope&,std::string&);
class DeviceTimeAnchorCoordinator;
class DeviceTimeAnchorRequestSigner;
class ExpectedTimeAnchorExchange final {
 public:ExpectedTimeAnchorExchange()=default;
 private:friend class DeviceTimeAnchorCoordinator;friend class DeviceTimeAnchorRequestSigner;friend class DeviceTimeAnchorVerifier;
 TimeAnchorRequest request_;std::string canonical_,hash_,trust_;const DeviceTimeAnchorCoordinator* owner_{};
};
class PreparedDeviceTimeRequestPermit final {
 private:explicit PreparedDeviceTimeRequestPermit()=default;friend class DeviceTimeAnchorRequestSigner;friend class DeviceIdentity;std::string canonical_;
};
class DeviceTimeAnchorCoordinator final {
 public:
 DeviceTimeAnchorCoordinator(DeviceIdentity&,EncryptedProvisioning&,ProvisionedBindingStore&,TimeAnchorCore&);
#if defined(BEEFISCAL_TIME_ANCHOR_CRYPTO_HOST_TEST)
 DeviceTimeAnchorCoordinator(DeviceIdentity&,EncryptedProvisioning&,ProvisionedBindingStore&,TimeAnchorCore&,const char*,const char*);
#endif
 esp_err_t trusted_start_boot();esp_err_t trusted_binding_changed();void trusted_invalidate();
 void retire_prepared(const ExpectedTimeAnchorExchange&);
 private:friend class DeviceTimeAnchorRequestSigner;friend class DeviceTimeAnchorVerifier;
 DeviceIdentity& identity_;EncryptedProvisioning& encrypted_;ProvisionedBindingStore& binding_;TimeAnchorCore& core_;std::mutex mutex_;
 const std::string root_pem_,root_kid_,trust_;
 bool scope(TimeAnchorScope&);
};
class DeviceTimeAnchorRequestSigner final {
 public:explicit DeviceTimeAnchorRequestSigner(DeviceTimeAnchorCoordinator& c):coordinator_(c){}
 esp_err_t begin(TimeAnchorEnvelope&,ExpectedTimeAnchorExchange&);
 private:DeviceTimeAnchorCoordinator& coordinator_;
};
class DeviceTimeAnchorVerifier final {
 public:explicit DeviceTimeAnchorVerifier(DeviceTimeAnchorCoordinator& c):coordinator_(c){}
 TimeAnchorError verify_and_accept(const ExpectedTimeAnchorExchange&,const TimeAnchorEnvelope&);
 private:DeviceTimeAnchorCoordinator& coordinator_;
};
}
