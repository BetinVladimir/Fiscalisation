#include "payment_reconciliation.h"
#include "go_wire.h"

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
const V* lookup(const V&v,const char*k){if(v.kind!=V::Kind::Map)return nullptr;for(const auto&f:v.fields)if(f.first==k)return &f.second;return nullptr;}
bool text(const V&v,const char*k,std::string&out,size_t limit=1024){const auto*p=lookup(v,k);if(!p||p->kind!=V::Kind::String||p->text.empty()||p->text.size()>limit||p->text.find('\0')!=std::string::npos)return false;out=p->text;return true;}
bool eq(const V&v,const char*k,const std::string&s){std::string x;return text(v,k,x)&&x==s;}
bool integer(const V&v,const char*k,int64_t&n){const auto*p=lookup(v,k);return p&&go_json::integer(*p,n);}
bool encode(const V&v,std::string&s){std::string error;return go_json::encode(v,s,error);}
bool hash(const std::string&s,uint8_t d[32],std::string&hex){if(mbedtls_sha256(reinterpret_cast<const uint8_t*>(s.data()),s.size(),d,0))return false;const char*h="0123456789abcdef";hex.clear();for(int i=0;i<32;i++){hex+=h[d[i]>>4];hex+=h[d[i]&15];}return true;}
bool b64(const std::string&s,std::vector<uint8_t>&out,size_t max){if(s.empty()||s.size()%4==1||s.size()>max*2)return false;for(char c:s)if(!((c>='A'&&c<='Z')||(c>='a'&&c<='z')||(c>='0'&&c<='9')||c=='-'||c=='_'))return false;std::string p=s;for(char&c:p){if(c=='-')c='+';if(c=='_')c='/';}while(p.size()%4)p+='=';out.resize(max);size_t n=0;if(mbedtls_base64_decode(out.data(),out.size(),&n,reinterpret_cast<const uint8_t*>(p.data()),p.size()))return false;out.resize(n);std::vector<uint8_t>encoded(4*((n+2)/3)+1);size_t z=0;if(mbedtls_base64_encode(encoded.data(),encoded.size(),&z,out.data(),out.size()))return false;std::string canonical(reinterpret_cast<char*>(encoded.data()),z);for(char&c:canonical){if(c=='+')c='-';if(c=='/')c='_';}while(!canonical.empty()&&canonical.back()=='=')canonical.pop_back();return canonical==s;}
bool signature(const uint8_t*key,size_t keysize,const std::string&data,const std::string&sig){std::vector<uint8_t>decoded;if(!key||!keysize||!b64(sig,decoded,80))return false;mbedtls_pk_context pk;mbedtls_pk_init(&pk);int rc=mbedtls_pk_parse_public_key(&pk,key,keysize);uint8_t digest[32];std::string unused;if(rc==0&&(!mbedtls_pk_can_do(&pk,MBEDTLS_PK_ECDSA)||mbedtls_ecp_keypair_get_group_id(mbedtls_pk_ec(pk))!=MBEDTLS_ECP_DP_SECP256R1))rc=-1;if(rc==0&&!hash(data,digest,unused))rc=-1;if(rc==0)rc=mbedtls_pk_verify(&pk,MBEDTLS_MD_SHA256,digest,32,decoded.data(),decoded.size());mbedtls_pk_free(&pk);return rc==0;}
}
namespace {
bool row(const V&v,PaymentRecord&out){
 if(v.kind!=V::Kind::Map||v.fields.size()!=11)return false;
 if(!text(v,"payment_id",out.payment_id,128)||!text(v,"operation_id",out.operation_id,128)||!text(v,"receipt_id",out.receipt_id,128)||!text(v,"state",out.state,32))return false;
 std::string amount;if(!text(v,"amount_minor",amount,19)||amount[0]=='0')return false;int64_t n=0;for(char c:amount){if(c<'0'||c>'9'||n>(INT64_MAX-(c-'0'))/10)return false;n=n*10+c-'0';}if(n<=0)return false;out.amount_minor=n;
 const char*names[]={"terminal_reference","rrn","auth_code","reversal_reference","reversal_rrn","reversal_auth_code"};std::string*values[]={&out.terminal_reference,&out.rrn,&out.auth_code,&out.reversal_reference,&out.reversal_rrn,&out.reversal_auth_code};for(int i=0;i<6;++i){auto*p=lookup(v,names[i]);if(!p||p->kind!=V::Kind::String||p->text.size()>128||p->text.find('\0')!=std::string::npos)return false;*values[i]=p->text;}return true;
}
}
esp_err_t verify_payment_reconciliation(const V&command,const CompositeBinding&binding,const ExecutionAuthorizationTrust&trust,VerifiedPaymentReconciliation&out){
 out={};auto reject=[](){return ESP_ERR_INVALID_STATE;};
 auto*envelope=lookup(command,"reconciliation_authorization");if(!envelope||!trust.public_key_pem||!*trust.public_key_pem||!trust.key_id||!*trust.key_id||trust.now<1700000000)return reject();
 std::string encoded,claimed,sig;if(!eq(command,"command","PAYMENT_RECONCILE")||!eq(*envelope,"algorithm","ES256_DER")||!eq(*envelope,"key_id",trust.key_id)||!text(*envelope,"payload",encoded,12000)||!text(*envelope,"sha256",claimed,64)||!text(*envelope,"signature",sig,120))return reject();
 std::vector<uint8_t>raw;if(!b64(encoded,raw,8192))return reject();V grant;std::string error,canonical;if(!go_json::parse((const char*)raw.data(),raw.size(),grant,error)||go_json::contains_nul(grant)||!encode(grant,canonical)||canonical!=std::string((const char*)raw.data(),raw.size())||grant.kind!=V::Kind::Map||grant.fields.size()!=18)return reject();
 uint8_t digest[32];std::string actual;if(!hash(canonical,digest,actual)||actual!=claimed||!signature((const uint8_t*)trust.public_key_pem,strlen(trust.public_key_pem)+1,"BEELOY-PAYMENT-RECONCILIATION-V1\n"+canonical,sig))return reject();
 int64_t version,generation,nbf,exp;if(!integer(grant,"version",version)||version!=1||!eq(grant,"kind","PAYMENT_RECONCILIATION")||!eq(grant,"scope","payment.reconcile")||!integer(grant,"nbf",nbf)||!integer(grant,"exp",exp)||nbf<1700000000||exp<=nbf||exp-nbf>300||trust.now<nbf||trust.now>=exp)return reject();
 if(!binding.payment.present||binding.payment.device_id.empty()||!eq(grant,"tenant_id",binding.tenant_id)||!eq(grant,"location_id",binding.location_id)||!eq(grant,"register_id",binding.register_id)||!eq(grant,"edge_device_id",binding.edge_device_id)||!eq(grant,"terminal_id",binding.payment.device_id)||!eq(grant,"binding_sha256",binding.payload_sha256)||!integer(grant,"binding_generation",generation)||generation!=binding.generation)return reject();
 VerifiedPaymentReconciliation candidate;candidate.evidence.authorization_sha256=actual;
 if(!text(grant,"reconciliation_id",candidate.evidence.reconciliation_id,36)||candidate.evidence.reconciliation_id.size()!=36||!text(grant,"actor_id",candidate.evidence.actor_id,128)||!text(grant,"evidence_reference",candidate.evidence.evidence_reference,1024)||!text(grant,"evidence_sha256",candidate.evidence.evidence_sha256,64)||candidate.evidence.evidence_sha256.size()!=64||!eq(command,"client_operation_id",candidate.evidence.reconciliation_id))return reject();
 for(char c:candidate.evidence.evidence_sha256)if(!((c>='0'&&c<='9')||(c>='a'&&c<='f')))return reject();
 for(const char*alias:{"intent_id","operation_id"}){auto*p=lookup(command,alias);if(p&&!eq(command,alias,candidate.evidence.reconciliation_id))return reject();}for(const char*alias:{"action","command_type"}){auto*p=lookup(command,alias);if(p&&!eq(command,alias,"PAYMENT_RECONCILE"))return reject();}
 const auto*a=lookup(grant,"expected");const auto*b=lookup(grant,"observed");if(!a||!b||!row(*a,candidate.expected)||!row(*b,candidate.observed))return reject();candidate.evidence.terminal_id=binding.payment.device_id;out=std::move(candidate);return ESP_OK;
}
esp_err_t apply_payment_reconciliation(const V&command,const CompositeBinding&binding,DurableStorage&storage,const ExecutionAuthorizationTrust*explicit_trust){
 const ExecutionAuthorizationTrust production{EDGE_BINDING_SIGNING_PUBLIC_KEY_PEM,EDGE_BINDING_SIGNING_KEY_ID,static_cast<int64_t>(std::time(nullptr))};const auto&trust=explicit_trust?*explicit_trust:production;VerifiedPaymentReconciliation verified;const auto rc=verify_payment_reconciliation(command,binding,trust,verified);if(rc!=ESP_OK)return rc;
 return storage.reconcile_payment(verified.expected,verified.observed,verified.observed.state.c_str(),trust.now,&verified.evidence);
}
}
