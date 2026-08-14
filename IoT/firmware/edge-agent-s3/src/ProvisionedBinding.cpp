#include "ProvisionedBinding.h"
#include <mbedtls/base64.h>
#include <mbedtls/pk.h>
#include <mbedtls/sha256.h>
#include <mbedtls/x509_crt.h>

namespace beefiscal::edge {
namespace {
String field(const String& raw,const char* name){String m=String("\"")+name+"\":\"";int s=raw.indexOf(m);if(s<0)return{};s+=m.length();int e=raw.indexOf('"',s);return e<0?String():raw.substring(s,e);}
int64_t integerField(const String& raw,const char* name){String m=String("\"")+name+"\":";int s=raw.indexOf(m);return s<0?0:strtoll(raw.c_str()+s+m.length(),nullptr,10);}
const char* profileName(EdgeProfile p){return p==EdgeProfile::DatecsDp150BluePad50?"DATECS_DP150_BLUEPAD50":p==EdgeProfile::DaisyCompactS01?"DAISY_COMPACT_S01":"UNCONFIGURED";}
}
bool ProvisionedBindingStore::begin(){if(open_)return true;if(!pinnedCaPem_||String(pinnedCaPem_).indexOf("BEGIN CERTIFICATE")<0||!preferences_.begin("edge-binding",false))return false;open_=true;generation_=preferences_.getLong64("generation",0);return true;}
bool ProvisionedBindingStore::verify(const SignedBindingEnvelope& e)const{if(e.canonicalPayload.isEmpty()||e.signatureBase64Url.isEmpty())return false;String b=e.signatureBase64Url;b.replace('-','+');b.replace('_','/');while(b.length()%4)b+='=';uint8_t sig[512]{};size_t n=0;if(mbedtls_base64_decode(sig,sizeof(sig),&n,(const uint8_t*)b.c_str(),b.length())!=0)return false;uint8_t digest[32]{};if(mbedtls_sha256_ret((const uint8_t*)e.canonicalPayload.c_str(),e.canonicalPayload.length(),digest,0)!=0)return false;mbedtls_x509_crt ca;mbedtls_x509_crt_init(&ca);int r=mbedtls_x509_crt_parse(&ca,(const uint8_t*)pinnedCaPem_,strlen(pinnedCaPem_)+1);if(r==0)r=mbedtls_pk_verify(&ca.pk,MBEDTLS_MD_SHA256,digest,sizeof(digest),sig,n);mbedtls_x509_crt_free(&ca);return r==0;}
bool ProvisionedBindingStore::encode(const CompositeBinding& b,String& raw)const{if(b.generation<1||b.profile==EdgeProfile::Unconfigured)return false;raw=String("{\"edge_device_id\":\"")+b.edgeDeviceId+"\",\"generation\":"+b.generation+",\"location_id\":\""+b.locationId+"\",\"profile\":\""+profileName(b.profile)+"\",\"register_id\":\""+b.registerId+"\",\"tenant_id\":\""+b.tenantId+"\"}";return true;}
bool ProvisionedBindingStore::decode(const String& raw,CompositeBinding& b)const{b={};b.tenantId=field(raw,"tenant_id");b.locationId=field(raw,"location_id");b.registerId=field(raw,"register_id");b.edgeDeviceId=field(raw,"edge_device_id");b.generation=integerField(raw,"generation");String p=field(raw,"profile");b.profile=p=="DATECS_DP150_BLUEPAD50"?EdgeProfile::DatecsDp150BluePad50:p=="DAISY_COMPACT_S01"?EdgeProfile::DaisyCompactS01:EdgeProfile::Unconfigured;return!b.tenantId.isEmpty()&&!b.locationId.isEmpty()&&!b.registerId.isEmpty()&&!b.edgeDeviceId.isEmpty()&&b.generation>=1&&b.profile!=EdgeProfile::Unconfigured;}
bool ProvisionedBindingStore::install(const SignedBindingEnvelope& e){if(!open_||e.binding.generation<=generation_||!verify(e))return false;String expected;if(!encode(e.binding,expected)||expected!=e.canonicalPayload)return false;if(preferences_.putString("payload",expected)!=expected.length())return false;if(preferences_.putLong64("generation",e.binding.generation)!=sizeof(int64_t))return false;generation_=e.binding.generation;return true;}
bool ProvisionedBindingStore::load(CompositeBinding& b){if(!open_)return false;return decode(preferences_.getString("payload",""),b)&&b.generation==generation_;}
} // namespace beefiscal::edge
