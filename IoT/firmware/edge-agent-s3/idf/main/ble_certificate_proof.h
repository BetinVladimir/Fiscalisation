#pragma once
#include "device_identity.h"
#include "encrypted_provisioning.h"
#include <cstdint>
#include <string>
namespace beefiscal::idf {
struct BLECertificateChallengeEnvelope {
  std::string canonical_json, signature, key_id;
};
struct BLECertificateProofEnvelope {
  std::string canonical_json, signature;
  void clear() { canonical_json.clear(); signature.clear(); }
};
// Internal authority, never supplied by a BLE request. No production trusted
// implementation is provided until a separate fresh device-nonce anchor exists.
class TrustedBLECertificateTime {
 public:
  virtual ~TrustedBLECertificateTime() = default;
  // Inclusive integer-second bounds. Security consumers must check the entire
  // interval; a midpoint cannot establish both not-before and expiry. Each
  // published endpoint must be nondecreasing within an authority epoch.
  virtual bool bounds(int64_t& lower_unix_seconds,
                      int64_t& upper_unix_seconds) const = 0;
};
TrustedBLECertificateTime& unavailable_ble_certificate_time();
struct VerifiedBLECertificateProofPermit {
 private:
  friend class BLEServerCertificateProofSigner;
  friend class DeviceIdentity;
  std::string canonical_proof;
};
class BLEServerCertificateProofSigner final {
 public:
  BLEServerCertificateProofSigner(DeviceIdentity&, EncryptedProvisioning&,
      ProvisionedBindingStore&, TrustedBLECertificateTime& = unavailable_ble_certificate_time());
#if defined(BEEFISCAL_BLE_PROOF_HOST_TEST)
  // Explicit host-only authority injection; absent from production firmware API.
  BLEServerCertificateProofSigner(DeviceIdentity&, EncryptedProvisioning&,
      ProvisionedBindingStore&, TrustedBLECertificateTime&, const char* root_pem,
      const char* root_key_id);
#endif
  esp_err_t prove(const BLECertificateChallengeEnvelope&, BLECertificateProofEnvelope&);
 private:
  DeviceIdentity& identity_;
  EncryptedProvisioning& encrypted_;
  ProvisionedBindingStore& legacy_;
  TrustedBLECertificateTime& time_;
  const char* root_pem_;
  const char* root_key_id_;
  bool recovery_pending_ = false, cache_established_ = false;
  esp_err_t installed_binding(CompositeBinding&);
};
}
