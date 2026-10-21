#include "authority_limits.h"
#include "execution_authorization.h"
#include "authority_unp.h"
#include "mbedtls/base64.h"
#include "mbedtls/pk.h"
#include "mbedtls/ecp.h"
#include "mbedtls/sha256.h"
#include <cstring>
#include <ctime>
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
using V=go_json::Value;
const V* field(const V&v,const char*k){if(v.kind!=V::Kind::Map)return nullptr;for(const auto&f:v.fields)if(f.first==k)return &f.second;return nullptr;}
bool text(const V&v,const char*k,std::string&out,size_t limit=1024){const auto*p=field(v,k);if(!p||p->kind!=V::Kind::String||p->text.empty()||p->text.size()>limit||p->text.find('\0')!=std::string::npos)return false;out=p->text;return true;}
bool eq(const V&v,const char*k,const std::string&s){std::string x;return text(v,k,x)&&x==s;}
bool integer(const V&v,const char*k,int64_t&n){const auto*p=field(v,k);return p&&go_json::integer(*p,n);}
bool encode(const V&v,std::string&s){std::string error;return go_json::encode(v,s,error);}
bool hash(const std::string&s,uint8_t d[32],std::string&hex){if(mbedtls_sha256(reinterpret_cast<const uint8_t*>(s.data()),s.size(),d,0))return false;const char*h="0123456789abcdef";hex.clear();for(int i=0;i<32;i++){hex+=h[d[i]>>4];hex+=h[d[i]&15];}return true;}
bool b64(const std::string&s,std::vector<uint8_t>&out,size_t max){if(s.empty()||s.size()%4==1||s.size()>max*2)return false;for(char c:s)if(!((c>='A'&&c<='Z')||(c>='a'&&c<='z')||(c>='0'&&c<='9')||c=='-'||c=='_'))return false;std::string p=s;for(char&c:p){if(c=='-')c='+';if(c=='_')c='/';}while(p.size()%4)p+='=';out.resize(max);size_t n=0;if(mbedtls_base64_decode(out.data(),out.size(),&n,reinterpret_cast<const uint8_t*>(p.data()),p.size()))return false;out.resize(n);std::vector<uint8_t>encoded(4*((n+2)/3)+1);size_t z=0;if(mbedtls_base64_encode(encoded.data(),encoded.size(),&z,out.data(),out.size()))return false;std::string canonical(reinterpret_cast<char*>(encoded.data()),z);for(char&c:canonical){if(c=='+')c='-';if(c=='/')c='_';}while(!canonical.empty()&&canonical.back()=='=')canonical.pop_back();return canonical==s;}
bool signature(const uint8_t*key,size_t keysize,const std::string&data,const std::string&sig){std::vector<uint8_t>decoded;if(!key||!keysize||!b64(sig,decoded,80))return false;mbedtls_pk_context pk;mbedtls_pk_init(&pk);int rc=mbedtls_pk_parse_public_key(&pk,key,keysize);uint8_t digest[32];std::string unused;if(rc==0&&(!mbedtls_pk_can_do(&pk,MBEDTLS_PK_ECDSA)||mbedtls_ecp_keypair_get_group_id(mbedtls_pk_ec(pk))!=MBEDTLS_ECP_DP_SECP256R1))rc=-1;if(rc==0&&!hash(data,digest,unused))rc=-1;if(rc==0)rc=mbedtls_pk_verify(&pk,MBEDTLS_MD_SHA256,digest,32,decoded.data(),decoded.size());mbedtls_pk_free(&pk);return rc==0;}
V nullable(const V&v,const char*k,bool&valid){const V*p=field(v,k);if(!p||p->kind==V::Kind::Null)return V::null();if(p->kind!=V::Kind::String||p->text.size()>256||p->text.find('\0')!=std::string::npos){valid=false;return V::null();}return p->text.empty()?V::null():*p;}
bool alias(const V&v,const char*k,const std::string&expected){const auto*p=field(v,k);return !p||(p->kind==V::Kind::String&&p->text==expected);}
}
esp_err_t verify_execution_authorization(const V&command,const CompositeBinding&b,const ExecutionAuthorizationTrust&t,VerifiedExecutionAuthorization&out){
 out={};auto reject=[](){return ESP_ERR_INVALID_STATE;};
 const auto*envelope=field(command,"execution_authorization");const auto*proof=field(command,"client_proof");
 if(!t.public_key_pem||!*t.public_key_pem||!t.key_id||!*t.key_id||t.now<1700000000||!envelope||!proof)return reject();
 std::string encoded,claimed,issuer_signature;if(!eq(*envelope,"algorithm","ES256_DER")||!eq(*envelope,"key_id",t.key_id)||!text(*envelope,"payload",encoded,12000)||!text(*envelope,"sha256",claimed,64)||!text(*envelope,"signature",issuer_signature,120))return reject();
 std::vector<uint8_t>raw;if(!b64(encoded,raw,8192))return reject();V authorization;std::string error,canonical;if(!go_json::parse(reinterpret_cast<char*>(raw.data()),raw.size(),authorization,error)||!encode(authorization,canonical)||canonical!=std::string(reinterpret_cast<char*>(raw.data()),raw.size()))return reject();
 uint8_t digest[32];std::string actual;if(!hash(canonical,digest,actual)||claimed!=actual||!signature(reinterpret_cast<const uint8_t*>(t.public_key_pem),strlen(t.public_key_pem)+1,"BEELOY-EXECUTION-AUTHORIZATION-V1\n"+canonical,issuer_signature))return reject();
 int64_t version{},generation{},nbf{},exp{};if(!integer(authorization,"version",version)||version!=1||!eq(authorization,"kind","EXECUTION_AUTHORIZATION")||!eq(authorization,"scope","fiscal.execute")||!integer(authorization,"nbf",nbf)||!integer(authorization,"exp",exp)||nbf<1700000000||exp<=nbf||exp-nbf>900||t.now<nbf||t.now>=exp)return reject();
 if(!eq(authorization,"tenant_id",b.tenant_id)||!eq(authorization,"location_id",b.location_id)||!eq(authorization,"register_id",b.register_id)||!eq(authorization,"edge_device_id",b.edge_device_id)||!eq(authorization,"fiscal_device_id",b.fiscal.device_id)||!eq(authorization,"fiscal_device_number",b.fiscal.fiscal_device_number)||!eq(authorization,"fiscal_memory_number",b.fiscal.fiscal_memory_number)||!eq(authorization,"binding_sha256",b.payload_sha256)||!integer(authorization,"binding_generation",generation)||generation!=b.generation)return reject();
 const auto*authority=field(authorization,"authority");int64_t start{},end{};if(!authority||!eq(*authority,"binding_sha256",b.payload_sha256)||!eq(*authority,"unp_prefix",b.authority.unp_prefix)||!integer(*authority,"range_start",start)||!integer(*authority,"range_end",end)||start!=b.authority.unp_range_start||end!=b.authority.unp_range_end)return reject();
 VerifiedExecutionAuthorization candidate;candidate.authorization_sha256=actual;
 if(!text(authorization,"authorization_id",candidate.authorization_id,36)||!text(authorization,"workstation_session_id",candidate.workstation_session_id,256)||!text(authorization,"actor_subject",candidate.actor_id,kAuthorityActorMaxBytes)||!text(authorization,"app_instance_id",candidate.app_instance_id,256)||!text(authorization,"operator_id",candidate.operator_id,256)||!text(authorization,"operator_code",candidate.operator_code,4)||candidate.operator_code.size()!=4)return reject();
 // The prefix identifies the fiscal device; the signed authorization identifies its operator.
 if(b.authority.unp_prefix.size()!=8)return reject();
 std::string operation,receipt,payload_hash,action;if(!text(command,"intent_id",operation,36)||!text(command,"receipt_session_id",receipt,36)||!text(command,"payload_sha256",payload_hash,64)||!text(command,"action",action,32)||!alias(command,"client_operation_id",operation)||!alias(command,"operation_id",operation)||!alias(command,"command_type",action))return reject();
 if(!eq(command,"tenant_id",b.tenant_id)||!eq(command,"register_id",b.register_id)||!eq(command,"edge_device_id",b.edge_device_id)||!integer(command,"binding_generation",generation)||generation!=b.generation)return reject();
 const auto*commands=field(authorization,"allowed_commands");bool allowed=false;if(!commands||commands->kind!=V::Kind::Array||commands->elements.empty()||commands->elements.size()>16)return reject();for(const auto&v:commands->elements){if(v.kind!=V::Kind::String)return reject();allowed|=v.text==action;}if(!allowed)return reject();
 const auto*payload=field(command,"canonical_payload");std::string payload_bytes,payload_actual;if(!payload||payload->kind!=V::Kind::Map||!encode(*payload,payload_bytes)||!hash(payload_bytes,digest,payload_actual)||payload_hash!=payload_actual||!eq(*payload,"operator_code",candidate.operator_code)||!eq(*payload,"operator_id",candidate.operator_id))return reject();
 bool valid=true;const V server=nullable(command,"server_sale_id",valid),surrogate=nullable(command,"client_sale_surrogate_id",valid);if(!valid)return reject();
 const auto*unp_authority=field(authorization,"unp_authority");candidate.unp_source="LEASED";
 if(action!="SALE_FINALIZE"){if(unp_authority)return reject();}
 else if(unp_authority){
  if(!eq(*unp_authority,"payload_sha256",payload_hash)||!eq(*unp_authority,"operation_id",operation)||!eq(*unp_authority,"receipt_session_id",receipt)||!eq(*unp_authority,"source","SERVER_ASSIGNED")||!text(*unp_authority,"server_sale_id",candidate.server_sale_id,128)||!text(*unp_authority,"unp",candidate.assigned_unp,21)||!authority_unp_valid(candidate.assigned_unp)||candidate.assigned_unp.substr(0,8)!=b.fiscal.fiscal_device_number||candidate.assigned_unp.substr(9,4)!=candidate.operator_code||server.kind!=V::Kind::String||server.text!=candidate.server_sale_id||!eq(*payload,"server_sale_id",candidate.server_sale_id)||!eq(*payload,"unp_source","SERVER_ASSIGNED")||!eq(*payload,"unp",candidate.assigned_unp))return reject();
  candidate.unp_source="SERVER_ASSIGNED";
 }else{
  if(server.kind!=V::Kind::Null||field(*payload,"unp"))return reject();
  const auto*source=field(*payload,"unp_source");if(source&&(source->kind!=V::Kind::String||source->text!="LEASED"))return reject();
 }

 V transcript=V::object({{"kind",V::string("EXECUTION_INTENT_PROOF")},{"version",V::integer_value(1)},{"authorization_sha256",V::string(actual)},{"operation_id",V::string(operation)},{"receipt_session_id",V::string(receipt)},{"payload_sha256",V::string(payload_hash)},{"server_sale_id",server},{"client_sale_surrogate_id",surrogate},{"binding_sha256",V::string(b.payload_sha256)},{"binding_generation",V::integer_value(b.generation)},{"command_type",V::string(action)}});
 std::string transcript_bytes,client_key,client_signature;if(!encode(transcript,transcript_bytes)||!hash(transcript_bytes,digest,candidate.client_proof_sha256)||!eq(*proof,"algorithm","ES256_DER")||!text(*proof,"signature",client_signature,120)||!text(authorization,"client_signing_public_key_der_base64",client_key,256))return reject();
 std::vector<uint8_t>key;if(!b64(client_key,key,256)||!signature(key.data(),key.size(),"BEELOY-EXECUTION-INTENT-PROOF-V1\n"+transcript_bytes,client_signature))return reject();
 out=std::move(candidate);return ESP_OK;
}
esp_err_t verify_execution_authorization(const V&command,const CompositeBinding&b,VerifiedExecutionAuthorization&out){return verify_execution_authorization(command,b,{EDGE_BINDING_SIGNING_PUBLIC_KEY_PEM,EDGE_BINDING_SIGNING_KEY_ID,static_cast<int64_t>(std::time(nullptr))},out);}
}
