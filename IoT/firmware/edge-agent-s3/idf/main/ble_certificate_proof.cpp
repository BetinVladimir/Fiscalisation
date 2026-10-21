#include "ble_certificate_proof.h"
#include "installed_authority_guard.h"
#include "go_json.h"
#include "mbedtls/base64.h"
#include "mbedtls/ecdsa.h"
#include "mbedtls/pk.h"
#include "mbedtls/sha256.h"
#include "nvs.h"
#include <algorithm>
#include <array>
#include <cstring>
#include <limits>
#include <vector>
#if __has_include("BindingTrustAnchor.h")
#include "BindingTrustAnchor.h"
#endif
#ifndef EDGE_BINDING_SIGNING_PUBLIC_KEY_PEM
#define EDGE_BINDING_SIGNING_PUBLIC_KEY_PEM ""
#endif
#ifndef EDGE_BINDING_SIGNING_KEY_ID
#define EDGE_BINDING_SIGNING_KEY_ID ""
#endif
namespace beefiscal::idf { namespace {
using Value = go_json::Value;
constexpr size_t kSlots = 4, kSlotBytes = 181, kCacheBytes = 4 + kSlots * kSlotBytes;
constexpr char kChallengeDomain[] = "BEEFISCAL_BLE_SERVER_CERT_CHALLENGE_V1\n";
constexpr char kProofDomain[] = "BEEFISCAL_BLE_SERVER_CERT_PROOF_V1\n";
class UnavailableTime final : public TrustedBLECertificateTime {
 public: bool now(int64_t&) const override { return false; }
};
struct Challenge {
  std::string id, nonce, device, spki, tenant, location, reg, binding, name, hash;
  int64_t issued{}, expires{}, generation{}, certificate_expiry{};
};
struct Slot {
  bool used{};
  std::string id, hash, binding;
  int64_t generation{}, expires{};
  std::array<uint8_t, 64> signature{};
};
using Cache = std::array<Slot, kSlots>;
std::string encode64(const uint8_t* data, size_t n) {
  std::vector<uint8_t> encoded(4 * ((n + 2) / 3) + 1); size_t z = 0;
  if (mbedtls_base64_encode(encoded.data(), encoded.size(), &z, data, n)) return {};
  std::string out(reinterpret_cast<char*>(encoded.data()), z);
  for (char& c : out) { if (c == '+') c = '-'; else if (c == '/') c = '_'; }
  while (!out.empty() && out.back() == '=') out.pop_back();
  return out;
}
bool decode64(const std::string& input, size_t maximum, std::vector<uint8_t>& out) {
  out.clear();
  if (input.empty() || input.size() > 4 * ((maximum + 2) / 3) || input.size() % 4 == 1) return false;
  for (char c : input) if (!((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
      (c >= '0' && c <= '9') || c == '-' || c == '_')) return false;
  std::string padded = input;
  for (char& c : padded) { if (c == '-') c = '+'; else if (c == '_') c = '/'; }
  while (padded.size() % 4) padded += '=';
  out.resize(maximum); size_t n = 0;
  if (mbedtls_base64_decode(out.data(), out.size(), &n,
      reinterpret_cast<const uint8_t*>(padded.data()), padded.size())) { out.clear(); return false; }
  out.resize(n); return encode64(out.data(), out.size()) == input;
}
std::string hash(const std::string& data) {
  uint8_t digest[32];
  if (mbedtls_sha256(reinterpret_cast<const uint8_t*>(data.data()), data.size(), digest, 0)) return {};
  static constexpr char digits[] = "0123456789abcdef"; std::string out;
  for (uint8_t b : digest) { out += digits[b >> 4]; out += digits[b & 15]; }
  return out;
}
bool hex(const std::string& s, uint8_t* out, size_t size) {
  if (s.size() != size * 2) return false;
  auto digit=[](char c)->int { return c >= '0' && c <= '9' ? c-'0' : c >= 'a' && c <= 'f' ? c-'a'+10 : -1; };
  for (size_t i=0;i<size;i++) { int a=digit(s[2*i]),b=digit(s[2*i+1]); if(a<0||b<0)return false; out[i]=uint8_t((a<<4)|b); }
  return true;
}
bool uuid(const std::string& s) {
  if(s.size()!=36)return false;
  for(size_t i=0;i<s.size();i++) { if(i==8||i==13||i==18||i==23){if(s[i]!='-')return false;}
    else if(!((s[i]>='0'&&s[i]<='9')||(s[i]>='a'&&s[i]<='f')||(s[i]>='A'&&s[i]<='F')))return false; }
  return true;
}
Value* field(Value& value,const char* key) { for(auto& p:value.fields)if(p.first==key)return &p.second;return nullptr; }
bool text(Value& value,const char* key,std::string& out,size_t maximum=128) {
  auto* p=field(value,key);if(!p||p->kind!=Value::Kind::String||p->text.empty()||p->text.size()>maximum||p->text.find('\0')!=std::string::npos)return false;
  out=p->text;return true;
}
bool integer(Value& value,const char* key,int64_t& n) {
  auto* p=field(value,key);if(!p||!go_json::integer(*p,n))return false;
  *p=Value::integer_value(n);return true;
}
bool parse_challenge(const std::string& raw, Challenge& c) {
  if(raw.empty()||raw.size()>8192)return false;
  Value value;std::string error,kind,purpose,canonical;int64_t version{};
  if(!go_json::parse(raw.data(),raw.size(),value,error)||value.kind!=Value::Kind::Map||value.fields.size()!=16)return false;
  if(!text(value,"kind",kind)||kind!="BLE_SERVER_CERT_CHALLENGE"||!text(value,"purpose",purpose)||purpose!="ble-tls-server"||
      !integer(value,"version",version)||version!=1||!integer(value,"issued_at",c.issued)||!integer(value,"expires_at",c.expires)||
      !integer(value,"certificate_not_after",c.certificate_expiry)||!integer(value,"binding_generation",c.generation)||
      !text(value,"challenge_id",c.id)||!text(value,"nonce",c.nonce,43)||!text(value,"device_id",c.device)||
      !text(value,"device_spki_sha256",c.spki,64)||!text(value,"tenant_id",c.tenant)||!text(value,"location_id",c.location)||
      !text(value,"register_id",c.reg)||!text(value,"binding_sha256",c.binding,64)||!text(value,"server_name",c.name))return false;
  if(!go_json::encode(value,canonical,error)||canonical!=raw||!uuid(c.id)||!uuid(c.device)||c.generation<1||c.issued<1700000000||
      c.issued>std::numeric_limits<int64_t>::max()-86400||c.expires!=c.issued+120||c.certificate_expiry!=c.issued+86400||
      c.name!="device-"+c.device+".ble.beefiscal.invalid")return false;
  uint8_t d[32];std::vector<uint8_t> nonce;
  return hex(c.spki,d,32)&&hex(c.binding,d,32)&&decode64(c.nonce,32,nonce)&&nonce.size()==32;
}
bool verify_root(const BLECertificateChallengeEnvelope& envelope,const char* pem,const char* kid) {
  if(!pem||!*pem||!kid||!*kid||envelope.key_id.size()>128||envelope.key_id!=kid||envelope.signature.size()>96)return false;
  std::vector<uint8_t> signature;if(!decode64(envelope.signature,72,signature))return false;
  mbedtls_pk_context key;mbedtls_pk_init(&key);
  int rc=mbedtls_pk_parse_public_key(&key,reinterpret_cast<const uint8_t*>(pem),strlen(pem)+1);
  if(!rc&&(!mbedtls_pk_can_do(&key,MBEDTLS_PK_ECDSA)||mbedtls_ecp_keypair_get_group_id(mbedtls_pk_ec(key))!=MBEDTLS_ECP_DP_SECP256R1))rc=-1;
  const std::string preimage=std::string(kChallengeDomain)+envelope.canonical_json;uint8_t digest[32];
  if(!rc)rc=mbedtls_sha256(reinterpret_cast<const uint8_t*>(preimage.data()),preimage.size(),digest,0);
  if(!rc)rc=mbedtls_pk_verify(&key,MBEDTLS_MD_SHA256,digest,sizeof(digest),signature.data(),signature.size());
  mbedtls_pk_free(&key);return rc==0;
}
bool verify_cached(const DeviceIdentity& identity,const std::string& proof,const std::array<uint8_t,64>& signature) {
  std::vector<uint8_t> der;if(!decode64(identity.public_key_der_base64url(),160,der))return false;
  mbedtls_pk_context key;mbedtls_pk_init(&key);mbedtls_mpi r,s;mbedtls_mpi_init(&r);mbedtls_mpi_init(&s);
  int rc=mbedtls_pk_parse_public_key(&key,der.data(),der.size());
  if(!rc)rc=mbedtls_mpi_read_binary(&r,signature.data(),32);
  if(!rc)rc=mbedtls_mpi_read_binary(&s,signature.data()+32,32);
  const std::string preimage=std::string(kProofDomain)+proof;uint8_t digest[32];
  if(!rc)rc=mbedtls_sha256(reinterpret_cast<const uint8_t*>(preimage.data()),preimage.size(),digest,0);
  if(!rc)rc=mbedtls_ecdsa_verify(&mbedtls_pk_ec(key)->MBEDTLS_PRIVATE(grp),digest,sizeof(digest),&mbedtls_pk_ec(key)->MBEDTLS_PRIVATE(Q),&r,&s);
  mbedtls_mpi_free(&r);mbedtls_mpi_free(&s);mbedtls_pk_free(&key);return rc==0;
}
void number(uint8_t* p,int64_t value) { for(int i=7;i>=0;i--){p[i]=uint8_t(value&255);value>>=8;} }
bool number(const uint8_t* p,int64_t& value) {uint64_t n=0;for(int i=0;i<8;i++)n=(n<<8)|p[i];if(n>uint64_t(std::numeric_limits<int64_t>::max()))return false;value=int64_t(n);return true;}
std::array<uint8_t,kCacheBytes> serialize(const Cache& cache) {
  std::array<uint8_t,kCacheBytes> raw{};memcpy(raw.data(),"BPC1",4);
  for(size_t i=0;i<kSlots;i++){const Slot& s=cache[i];if(!s.used)continue;uint8_t* p=raw.data()+4+i*kSlotBytes;p[0]=1;
    memcpy(p+1,s.id.data(),36);hex(s.hash,p+37,32);hex(s.binding,p+69,32);number(p+101,s.generation);number(p+109,s.expires);memcpy(p+117,s.signature.data(),64);}
  return raw;
}
esp_err_t load_cache(Cache& cache,bool established,bool& present) {
  cache={};present=false;nvs_handle_t handle{};auto rc=nvs_open("ble-cert-pop",NVS_READONLY,&handle);
  if(rc==ESP_ERR_NVS_NOT_FOUND)return established?ESP_ERR_INVALID_STATE:ESP_OK;
  if(rc!=ESP_OK)return rc;
  size_t size=0;rc=nvs_get_blob(handle,"cache-v1",nullptr,&size);
  if(rc==ESP_ERR_NVS_NOT_FOUND){nvs_close(handle);return established?ESP_ERR_INVALID_STATE:ESP_OK;}
  if(rc!=ESP_OK||size!=kCacheBytes){nvs_close(handle);return rc==ESP_OK?ESP_ERR_INVALID_SIZE:rc;}
  std::array<uint8_t,kCacheBytes> raw{};rc=nvs_get_blob(handle,"cache-v1",raw.data(),&size);nvs_close(handle);
  if(rc!=ESP_OK)return rc;
  if(size!=kCacheBytes)return ESP_ERR_INVALID_SIZE;
  if(memcmp(raw.data(),"BPC1",4))return ESP_ERR_INVALID_CRC;
  static constexpr char digits[]="0123456789abcdef";
  for(size_t i=0;i<kSlots;i++){const uint8_t* p=raw.data()+4+i*kSlotBytes;if(p[0]>1)return ESP_ERR_INVALID_CRC;if(!p[0]){for(size_t j=1;j<kSlotBytes;j++)if(p[j])return ESP_ERR_INVALID_CRC;continue;}
    Slot& s=cache[i];s.used=true;s.id.assign(reinterpret_cast<const char*>(p+1),36);if(!uuid(s.id)||!number(p+101,s.generation)||s.generation<1||!number(p+109,s.expires)||s.expires<1700000000)return ESP_ERR_INVALID_CRC;
    for(size_t j=0;j<32;j++){s.hash+=digits[p[37+j]>>4];s.hash+=digits[p[37+j]&15];s.binding+=digits[p[69+j]>>4];s.binding+=digits[p[69+j]&15];}
    memcpy(s.signature.data(),p+117,64);for(size_t j=0;j<i;j++)if(cache[j].used&&cache[j].id==s.id)return ESP_ERR_INVALID_CRC;
  }
  present=true;return ESP_OK;
}
esp_err_t save_cache(const Cache& cache) {
  const auto raw=serialize(cache);nvs_handle_t handle{};auto rc=nvs_open("ble-cert-pop",NVS_READWRITE,&handle);
  if(rc!=ESP_OK)return rc;
  rc=nvs_set_blob(handle,"cache-v1",raw.data(),raw.size());if(rc==ESP_OK)rc=nvs_commit(handle);nvs_close(handle);return rc;
}
bool same_binding(const CompositeBinding& a,const CompositeBinding& b) {
  return a.payload_sha256==b.payload_sha256&&a.generation==b.generation&&a.edge_device_id==b.edge_device_id&&a.tenant_id==b.tenant_id&&
      a.location_id==b.location_id&&a.register_id==b.register_id&&a.authority.transaction_signing_kid==b.authority.transaction_signing_kid;
}
}
TrustedBLECertificateTime& unavailable_ble_certificate_time(){static UnavailableTime time;return time;}
BLEServerCertificateProofSigner::BLEServerCertificateProofSigner(DeviceIdentity& i,EncryptedProvisioning& e,ProvisionedBindingStore& b,TrustedBLECertificateTime& t)
    :identity_(i),encrypted_(e),legacy_(b),time_(t),root_pem_(EDGE_BINDING_SIGNING_PUBLIC_KEY_PEM),root_key_id_(EDGE_BINDING_SIGNING_KEY_ID){}
#if defined(BEEFISCAL_BLE_PROOF_HOST_TEST)
BLEServerCertificateProofSigner::BLEServerCertificateProofSigner(DeviceIdentity& i,EncryptedProvisioning& e,ProvisionedBindingStore& b,TrustedBLECertificateTime& t,const char* pem,const char* kid)
    :identity_(i),encrypted_(e),legacy_(b),time_(t),root_pem_(pem),root_key_id_(kid){}
#endif
esp_err_t BLEServerCertificateProofSigner::installed_binding(CompositeBinding& binding) {
  auto rc=encrypted_.load_binding(binding);
  return rc==ESP_ERR_NVS_NOT_FOUND?legacy_.load(binding):rc;
}
esp_err_t BLEServerCertificateProofSigner::prove(const BLECertificateChallengeEnvelope& envelope,BLECertificateProofEnvelope& out) {
  InstalledAuthorityGuard guard(installed_authority_mutex());out.clear();
  Challenge c;if(!parse_challenge(envelope.canonical_json,c)||!verify_root(envelope,root_pem_,root_key_id_)||!identity_.ready())return ESP_ERR_INVALID_ARG;
  CompositeBinding binding;auto rc=installed_binding(binding);if(rc!=ESP_OK)return rc;
  std::vector<uint8_t> der;if(!decode64(identity_.public_key_der_base64url(),160,der))return ESP_ERR_INVALID_STATE;
  const std::string spki=hash(std::string(reinterpret_cast<char*>(der.data()),der.size()));
  if(binding.schema_version!=3||binding.generation<1||identity_.kid()!=binding.authority.transaction_signing_kid||c.spki!=spki||c.device!=binding.edge_device_id||c.tenant!=binding.tenant_id||
      c.location!=binding.location_id||c.reg!=binding.register_id||c.binding!=binding.payload_sha256||c.generation!=binding.generation)return ESP_ERR_INVALID_STATE;
  int64_t now{};if(!time_.now(now)||now<1700000000||now<c.issued||now>=c.expires)return ESP_ERR_INVALID_STATE;
  c.hash=hash(std::string(kChallengeDomain)+envelope.canonical_json);
  Value proof=Value::object({{"kind",Value::string("BLE_SERVER_CERT_PROOF")},{"version",Value::integer_value(1)},
      {"purpose",Value::string("ble-tls-server")},{"challenge_id",Value::string(c.id)},
      {"challenge_sha256",Value::string(c.hash)},{"device_spki_sha256",Value::string(c.spki)}});
  VerifiedBLECertificateProofPermit permit;std::string error;if(!go_json::encode(proof,permit.canonical_proof,error))return ESP_FAIL;
  Cache cache;bool present=false;rc=load_cache(cache,cache_established_,present);if(rc!=ESP_OK){recovery_pending_=true;return rc;}
  cache_established_|=present;if(recovery_pending_){recovery_pending_=false;return ESP_ERR_INVALID_STATE;}
  Slot* slot=nullptr;bool cached=false;
  for(auto& current:cache){if(current.used&&current.id==c.id){if(current.hash!=c.hash||current.binding!=c.binding||current.generation!=c.generation||current.expires!=c.expires||!verify_cached(identity_,permit.canonical_proof,current.signature))return ESP_ERR_INVALID_CRC;slot=&current;cached=true;break;}}
  if(!cached){for(auto& current:cache)if(current.used&&current.expires<=now)current={};for(auto& current:cache)if(!current.used){slot=&current;break;}
    if(!slot)return ESP_ERR_NO_MEM;
    slot->used=true;slot->id=c.id;slot->hash=c.hash;slot->binding=c.binding;slot->generation=c.generation;slot->expires=c.expires;
    rc=identity_.sign_ble_certificate_proof(permit,slot->signature.data());if(rc!=ESP_OK)return rc;
    rc=save_cache(cache);if(rc!=ESP_OK){recovery_pending_=true;return rc;}
    Cache confirmed;bool exists=false;rc=load_cache(confirmed,true,exists);
    if(rc!=ESP_OK||!exists||serialize(confirmed)!=serialize(cache)){recovery_pending_=true;return rc==ESP_OK?ESP_ERR_INVALID_CRC:rc;}
    cache_established_=true;
  }
  CompositeBinding current;rc=installed_binding(current);int64_t publication_time{};
  if(rc!=ESP_OK||!same_binding(binding,current)||!time_.now(publication_time)||publication_time<c.issued||publication_time>=c.expires)return ESP_ERR_INVALID_STATE;
  out.canonical_json=permit.canonical_proof;out.signature=encode64(slot->signature.data(),slot->signature.size());
  if(out.signature.size()!=86){out.clear();return ESP_FAIL;}return ESP_OK;
}
}
