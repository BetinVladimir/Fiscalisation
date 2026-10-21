#include "device_time_anchor.h"
#include "installed_authority_guard.h"
#include "go_json.h"
#include "mbedtls/base64.h"
#include "mbedtls/ecdsa.h"
#include "mbedtls/pk.h"
#include "mbedtls/sha256.h"
#include <algorithm>
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
namespace beefiscal::idf {namespace {
using Value=go_json::Value;
constexpr char kRequestDomain[]="BEEFISCAL_DEVICE_TIME_ANCHOR_REQUEST_V1\n";
constexpr char kTimeDomain[]="BEEFISCAL_DEVICE_TIME_ANCHOR_V1\n";
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
bool verify_root(const TimeAnchorEnvelope& envelope,const char* pem,const char* kid) {
  if(!pem||!*pem||!kid||!*kid||envelope.key_id.size()>128||envelope.key_id!=kid||envelope.signature.size()>96)return false;
  std::vector<uint8_t> signature;if(!decode64(envelope.signature,72,signature))return false;
  mbedtls_pk_context key;mbedtls_pk_init(&key);
  int rc=mbedtls_pk_parse_public_key(&key,reinterpret_cast<const uint8_t*>(pem),strlen(pem)+1);
  if(!rc&&(!mbedtls_pk_can_do(&key,MBEDTLS_PK_ECDSA)||mbedtls_ecp_keypair_get_group_id(mbedtls_pk_ec(key))!=MBEDTLS_ECP_DP_SECP256R1))rc=-1;
  const std::string preimage=std::string(kTimeDomain)+envelope.canonical_json;uint8_t digest[32];
  if(!rc)rc=mbedtls_sha256(reinterpret_cast<const uint8_t*>(preimage.data()),preimage.size(),digest,0);
  if(!rc)rc=mbedtls_pk_verify(&key,MBEDTLS_MD_SHA256,digest,sizeof(digest),signature.data(),signature.size());
  mbedtls_pk_free(&key);return rc==0;
}
bool strict_json_text(const std::string& raw){
 for(size_t i=0;i<raw.size();){const auto c=static_cast<unsigned char>(raw[i]);if(c==0)return false;if(c<128){++i;continue;}const size_t n=c>=0xc2&&c<=0xdf?2:c>=0xe0&&c<=0xef?3:c>=0xf0&&c<=0xf4?4:0;if(!n||i+n>raw.size())return false;for(size_t j=1;j<n;++j)if((static_cast<unsigned char>(raw[i+j])&0xc0)!=0x80)return false;const auto b=static_cast<unsigned char>(raw[i+1]);if((c==0xe0&&b<0xa0)||(c==0xed&&b>=0xa0)||(c==0xf0&&b<0x90)||(c==0xf4&&b>=0x90))return false;i+=n;}
 auto word=[&](size_t at,unsigned& out){if(at+4>raw.size())return false;out=0;for(size_t i=at;i<at+4;++i){char c=raw[i];int d=c>='0'&&c<='9'?c-'0':c>='a'&&c<='f'?c-'a'+10:c>='A'&&c<='F'?c-'A'+10:-1;if(d<0)return false;out=out*16+unsigned(d);}return true;};
 bool quoted=false;for(size_t i=0;i<raw.size();++i){if(raw[i]=='"'){quoted=!quoted;continue;}if(!quoted||raw[i]!='\\')continue;if(++i==raw.size())return false;if(raw[i]!='u')continue;unsigned u{};if(!word(i+1,u)||u==0)return false;i+=4;if(u>=0xdc00&&u<=0xdfff)return false;if(u>=0xd800&&u<=0xdbff){unsigned low{};if(i+6>=raw.size()||raw[i+1]!='\\'||raw[i+2]!='u'||!word(i+3,low)||low<0xdc00||low>0xdfff)return false;i+=6;}}
 return true;
}
bool strict_der(const std::string& encoded){std::vector<uint8_t> sig;if(!decode64(encoded,72,sig)||sig.size()<8||sig[0]!=0x30||sig[1]!=sig.size()-2)return false;size_t at=2;for(int i=0;i<2;++i){if(at+2>sig.size()||sig[at++]!=2)return false;const size_t n=sig[at++];if(n<1||n>33||at+n>sig.size()||(sig[at]&128)||(n>1&&sig[at]==0&&!(sig[at+1]&128)))return false;at+=n;}return at==sig.size();}
std::string trust_fingerprint(const std::string& pem,const std::string& kid){if(pem.empty()||kid.empty()||kid.size()>128||kid.find('\0')!=std::string::npos)return {};mbedtls_pk_context pk;mbedtls_pk_init(&pk);int rc=mbedtls_pk_parse_public_key(&pk,reinterpret_cast<const uint8_t*>(pem.c_str()),pem.size()+1);unsigned char der[160];if(!rc&&mbedtls_pk_can_do(&pk,MBEDTLS_PK_ECDSA)&&mbedtls_ecp_keypair_get_group_id(mbedtls_pk_ec(pk))==MBEDTLS_ECP_DP_SECP256R1)rc=mbedtls_pk_write_pubkey_der(&pk,der,sizeof(der));else rc=-1;std::string out;if(rc>0)out=hash(std::string(reinterpret_cast<char*>(der+sizeof(der)-rc),size_t(rc))+"\n"+kid);mbedtls_pk_free(&pk);return out;}
Value request_value(const TimeAnchorRequest& r,const std::string& kid){return Value::object({{"kind",Value::string("DEVICE_TIME_ANCHOR_REQUEST")},{"version",Value::integer_value(1)},{"purpose",Value::string("trusted-device-time")},{"scope",Value::string("device.time.anchor")},{"policy_version",Value::integer_value(1)},{"device_id",Value::string(r.scope.device_id)},{"device_kid",Value::string(r.scope.device_kid)},{"device_spki_sha256",Value::string(r.scope.device_spki_sha256)},{"tenant_id",Value::string(r.scope.tenant_id)},{"location_id",Value::string(r.scope.location_id)},{"register_id",Value::string(r.scope.register_id)},{"binding_sha256",Value::string(r.scope.binding_sha256)},{"binding_generation",Value::integer_value(r.scope.binding_generation)},{"boot_nonce",Value::string(encode64(r.boot_nonce.data(),32))},{"request_nonce",Value::string(encode64(r.request_nonce.data(),32))},{"request_sequence",Value::integer_value(r.sequence)},{"signer_key_id",Value::string(kid)}});}
bool equal(Value& v,const char* key,const std::string& expected){std::string actual;return text(v,key,actual)&&actual==expected;}
bool response(const std::string& raw,const TimeAnchorRequest& r,const std::string& digest,const std::string& kid,TimeAnchorClaims& c){
 if(raw.empty()||raw.size()>3072||!strict_json_text(raw))return false;
 Value v;std::string error,canonical;int64_t version{},policy{};
 if(!go_json::parse(raw.data(),raw.size(),v,error)||v.kind!=Value::Kind::Map||v.fields.size()!=22)return false;
 if(!integer(v,"version",version)||version!=1||!integer(v,"policy_version",policy)||policy!=1||!integer(v,"binding_generation",c.scope.binding_generation)||c.scope.binding_generation<1||!integer(v,"request_sequence",c.sequence)||c.sequence<1||!integer(v,"server_unix_ms",c.server_unix_ms)||c.server_unix_ms<1||!integer(v,"uncertainty_ms",c.uncertainty_ms)||c.uncertainty_ms<0||!integer(v,"max_rtt_ms",c.max_rtt_ms)||c.max_rtt_ms<1||!integer(v,"max_holdover_ms",c.max_holdover_ms)||c.max_holdover_ms<1)return false;
 const auto& s=r.scope;
 if(!equal(v,"kind","DEVICE_TIME_ANCHOR")||!equal(v,"purpose","trusted-device-time")||!equal(v,"scope","device.time.anchor")||!equal(v,"signer_key_id",kid)||!equal(v,"request_sha256",digest)||!equal(v,"device_id",s.device_id)||!equal(v,"device_kid",s.device_kid)||!equal(v,"device_spki_sha256",s.device_spki_sha256)||!equal(v,"tenant_id",s.tenant_id)||!equal(v,"location_id",s.location_id)||!equal(v,"register_id",s.register_id)||!equal(v,"binding_sha256",s.binding_sha256)||c.scope.binding_generation!=s.binding_generation||c.sequence!=r.sequence||!equal(v,"boot_nonce",encode64(r.boot_nonce.data(),32))||!equal(v,"request_nonce",encode64(r.request_nonce.data(),32)))return false;
 if(!go_json::encode(v,canonical,error)||canonical!=raw)return false;
 c.scope=s;c.boot_nonce=r.boot_nonce;c.request_nonce=r.request_nonce;c.clock_healthy=true;return true;
}
}
bool parse_time_anchor_envelope(const std::string& raw,TimeAnchorEnvelope& out){out.clear();if(raw.empty()||raw.size()>4096||!strict_json_text(raw))return false;Value v;std::string error;if(!go_json::parse(raw.data(),raw.size(),v,error)||v.kind!=Value::Kind::Map||v.fields.size()!=4||!text(v,"algorithm",out.algorithm,32)||!text(v,"canonical_json",out.canonical_json,3072)||!text(v,"key_id",out.key_id)||!text(v,"signature",out.signature,96)){out.clear();return false;}return true;}
bool encode_time_anchor_envelope(const TimeAnchorEnvelope& e,std::string& out){std::string error;out.clear();if(!go_json::encode(Value::object({{"algorithm",Value::string(e.algorithm)},{"canonical_json",Value::string(e.canonical_json)},{"key_id",Value::string(e.key_id)},{"signature",Value::string(e.signature)}}),out,error)||out.size()>4096){out.clear();return false;}return true;}
DeviceTimeAnchorCoordinator::DeviceTimeAnchorCoordinator(DeviceIdentity& i,EncryptedProvisioning& e,ProvisionedBindingStore& b,TimeAnchorCore& c):identity_(i),encrypted_(e),binding_(b),core_(c),root_pem_(EDGE_BINDING_SIGNING_PUBLIC_KEY_PEM),root_kid_(EDGE_BINDING_SIGNING_KEY_ID),trust_(trust_fingerprint(root_pem_,root_kid_)){}
#if defined(BEEFISCAL_TIME_ANCHOR_CRYPTO_HOST_TEST)
DeviceTimeAnchorCoordinator::DeviceTimeAnchorCoordinator(DeviceIdentity& i,EncryptedProvisioning& e,ProvisionedBindingStore& b,TimeAnchorCore& c,const char* pem,const char* kid):identity_(i),encrypted_(e),binding_(b),core_(c),root_pem_(pem?pem:""),root_kid_(kid?kid:""),trust_(trust_fingerprint(root_pem_,root_kid_)){}
#endif
bool DeviceTimeAnchorCoordinator::scope(TimeAnchorScope& out){CompositeBinding b;auto rc=encrypted_.load_binding(b);if(rc==ESP_ERR_NVS_NOT_FOUND)rc=binding_.load(b);if(rc!=ESP_OK||b.schema_version!=3||b.generation<1||!identity_.ready()||identity_.kid()!=b.authority.transaction_signing_kid||trust_.empty())return false;uint8_t binding_hash[32];if(!uuid(b.edge_device_id)||!hex(b.payload_sha256,binding_hash,32))return false;for(const auto* id:{&b.tenant_id,&b.location_id,&b.register_id,&b.edge_device_id})if(id->empty()||id->size()>128||id->find('\0')!=std::string::npos)return false;std::vector<uint8_t> der;if(!decode64(identity_.public_key_der_base64url(),160,der))return false;out={b.edge_device_id,identity_.kid(),hash(std::string(reinterpret_cast<char*>(der.data()),der.size())),b.tenant_id,b.location_id,b.register_id,b.payload_sha256,b.generation};return true;}
esp_err_t DeviceTimeAnchorCoordinator::trusted_start_boot(){InstalledAuthorityGuard installed(installed_authority_mutex());std::lock_guard<std::mutex> lock(mutex_);TimeAnchorScope s;if(!scope(s)){core_.invalidate();return ESP_ERR_INVALID_STATE;}return core_.start_boot(s)==TimeAnchorError::Ok?ESP_OK:ESP_ERR_INVALID_STATE;}
esp_err_t DeviceTimeAnchorCoordinator::trusted_binding_changed(){InstalledAuthorityGuard installed(installed_authority_mutex());std::lock_guard<std::mutex> lock(mutex_);TimeAnchorScope s;if(!scope(s)){core_.invalidate();return ESP_ERR_INVALID_STATE;}return core_.change_binding(s)==TimeAnchorError::Ok?ESP_OK:ESP_ERR_INVALID_STATE;}
void DeviceTimeAnchorCoordinator::trusted_invalidate(){InstalledAuthorityGuard installed(installed_authority_mutex());std::lock_guard<std::mutex> lock(mutex_);core_.invalidate();}
void DeviceTimeAnchorCoordinator::retire_prepared(const ExpectedTimeAnchorExchange& expected){InstalledAuthorityGuard installed(installed_authority_mutex());std::lock_guard<std::mutex> lock(mutex_);if(expected.owner_==this&&expected.trust_==trust_)core_.retire_request_if_current(expected.request_);}
esp_err_t DeviceTimeAnchorRequestSigner::begin(TimeAnchorEnvelope& out,ExpectedTimeAnchorExchange& expected){out.clear();expected={};auto& c=coordinator_;InstalledAuthorityGuard installed(installed_authority_mutex());std::lock_guard<std::mutex> lock(c.mutex_);TimeAnchorScope s;if(!c.scope(s))return ESP_ERR_INVALID_STATE;TimeAnchorRequest r;if(c.core_.begin_request(r)!=TimeAnchorError::Ok)return ESP_ERR_INVALID_STATE;if(!(r.scope==s)){c.core_.retire_request_if_current(r);return ESP_ERR_INVALID_STATE;}PreparedDeviceTimeRequestPermit permit;std::string error;if(!go_json::encode(request_value(r,c.root_kid_),permit.canonical_,error)){c.core_.retire_request_if_current(r);return ESP_FAIL;}uint8_t signature[64]{};if(c.identity_.sign_time_anchor_request(permit,signature)!=ESP_OK){c.core_.retire_request_if_current(r);return ESP_FAIL;}TimeAnchorScope current;if(!c.scope(current)||!(current==s)||!c.core_.is_current_request(r)){c.core_.retire_request_if_current(r);return ESP_ERR_INVALID_STATE;}out={"ES256_P1363",permit.canonical_,s.device_kid,encode64(signature,64)};if(out.signature.size()!=86){out.clear();c.core_.retire_request_if_current(r);return ESP_FAIL;}expected.request_=r;expected.canonical_=permit.canonical_;expected.hash_=hash(std::string(kRequestDomain)+permit.canonical_);expected.trust_=c.trust_;expected.owner_=&c;return ESP_OK;}
TimeAnchorError DeviceTimeAnchorVerifier::verify_and_accept(const ExpectedTimeAnchorExchange& expected,const TimeAnchorEnvelope& envelope){auto& c=coordinator_;if(expected.owner_!=&c||expected.trust_!=c.trust_||envelope.algorithm!="ES256_DER"||envelope.key_id!=c.root_kid_||expected.hash_!=hash(std::string(kRequestDomain)+expected.canonical_)||!strict_der(envelope.signature))return TimeAnchorError::InvalidSample;TimeAnchorClaims claims;if(!response(envelope.canonical_json,expected.request_,expected.hash_,c.root_kid_,claims)||!verify_root(envelope,c.root_pem_.c_str(),c.root_kid_.c_str()))return TimeAnchorError::InvalidSample;InstalledAuthorityGuard installed(installed_authority_mutex());std::lock_guard<std::mutex> lock(c.mutex_);TimeAnchorScope current;if(!c.scope(current)||!(current==expected.request_.scope)||!c.core_.is_current_request(expected.request_))return TimeAnchorError::StaleTicket;return c.core_.accept(VerifiedTimeAnchorPermit(expected.request_,claims));}
}
