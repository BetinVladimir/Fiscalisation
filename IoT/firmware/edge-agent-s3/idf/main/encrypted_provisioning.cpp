#include "encrypted_provisioning.h"
#include "cJSON.h"
#include "nvs.h"
#include "mbedtls/platform_util.h"
#include "mbedtls/sha256.h"
#include "mbedtls/x509_crt.h"
#include "mbedtls/base64.h"
#include <cstring>
#include <memory>
#include <algorithm>
namespace beefiscal::idf {namespace {
struct Delete{void operator()(cJSON*p)const{cJSON_Delete(p);
 }};

void wipe(std::string&s){if(!s.empty())mbedtls_platform_zeroize(s.data(),s.size());
 s.clear();
 }
bool keys(const cJSON*p,std::initializer_list<const char*>allowed){if(!cJSON_IsObject(p))return false;
 for(auto*c=p->child;
 c;
 c=c->next){bool found=false;
 for(auto*k:allowed)found|=c->string&&!strcmp(c->string,k);

 if(!found)return false;
 for(auto*d=c->next;
 d;
 d=d->next)if(d->string&&!strcmp(c->string,d->string))return false;
 }return true;
 }
bool text(const cJSON*p,const char*k,std::string&v,size_t max){auto*c=cJSON_GetObjectItemCaseSensitive(p,k);

 if(!cJSON_IsString(c)||!c->valuestring||!*c->valuestring||strlen(c->valuestring)>max)return false;
 v=c->valuestring;

 return true;
 }
bool certificate_matches(const RuntimeSecrets&s,const DeviceIdentity&identity){
 mbedtls_x509_crt root,client;

 mbedtls_x509_crt_init(&root);

 mbedtls_x509_crt_init(&client);

 int e=mbedtls_x509_crt_parse(&root,reinterpret_cast<const uint8_t*>(s.root_ca_pem.c_str()),s.root_ca_pem.size()+1);

 if(!e)e=mbedtls_x509_crt_parse(&client,reinterpret_cast<const uint8_t*>(s.client_certificate_pem.c_str()),s.client_certificate_pem.size()+1);

 uint8_t der[256]{},encoded[345]{};
 size_t written{};
 int n=e?-1:mbedtls_pk_write_pubkey_der(&client.pk,der,sizeof(der));

 if(n<=0)e=-1;

 if(!e)e=mbedtls_base64_encode(encoded,sizeof(encoded),&written,der+sizeof(der)-n,n);

 std::string public_key(reinterpret_cast<char*>(encoded),written);
 for(char&c:public_key){if(c=='+')c='-';
 else if(c=='/')c='_';
 }while(!public_key.empty()&&public_key.back()=='=')public_key.pop_back();

 mbedtls_x509_crt_free(&root);

 mbedtls_x509_crt_free(&client);

 return !e&&public_key==identity.public_key_der_base64url();

}
esp_err_t derive(void*ctx,const uint8_t peer[65],uint8_t shared[32]){return static_cast<DeviceIdentity*>(ctx)->provisioning_shared_secret(peer,shared);
 }
}
void ProvisionedRuntime::clear(){wipe(secrets.root_ca_pem);
 wipe(secrets.client_certificate_pem);
 wipe(secrets.client_key_pem);
 wipe(secrets.command_hmac_key);
 wipe(secrets.sync_ack_hmac_key);
 wipe(secrets.fiscal_operator_password);
 wipe(wifi_password);
 wifi_ssid.clear();
 package_sha256.clear();
 binding={};
 }
EncryptedProvisioning::EncryptedProvisioning(DeviceIdentity&i,ProvisionedBindingStore&v,const char*pem,const char*kid):identity_(i),validator_(v),issuer_pem_(pem),issuer_kid_(kid){}
esp_err_t EncryptedProvisioning::decode(const uint8_t*p,size_t n,const ProvisioningSubject&subject,ProvisionedRuntime&out){
 out.clear();
 OpenedProvisioning opened;
 auto e=open_provisioning_package(p,n,issuer_pem_,subject,derive,&identity_,opened);

 if(e!=ESP_OK)return e;

 // Reject encoded NULs, which cJSON's C-string interface cannot represent safely.
 std::string raw(opened.plaintext.begin(),opened.plaintext.end());opened.clear();
 if(raw.find("\\u0000")!=std::string::npos){wipe(raw);
 return ESP_ERR_INVALID_ARG;}
 const char*end{};std::unique_ptr<cJSON,Delete>root(cJSON_ParseWithLengthOpts(raw.data(),raw.size(),&end,false));
 if(!root||end!=raw.data()+raw.size()||!keys(root.get(),{"schema_version","binding","wifi","fiscal_operator_password","runtime_secrets"})){wipe(raw);
 return ESP_ERR_INVALID_ARG;}
 auto*schema=cJSON_GetObjectItemCaseSensitive(root.get(),"schema_version");auto*b=cJSON_GetObjectItemCaseSensitive(root.get(),"binding");auto*w=cJSON_GetObjectItemCaseSensitive(root.get(),"wifi");auto*s=cJSON_GetObjectItemCaseSensitive(root.get(),"runtime_secrets");
 std::string canonical,signature,kid;
 bool valid=cJSON_IsNumber(schema)&&schema->valuedouble==1&&keys(b,{"canonical_json","signature","key_id"})&&keys(w,{"ssid","password"})&&keys(s,{"root_ca_pem","client_certificate_pem","command_hmac_key","sync_ack_hmac_key"})&&text(b,"canonical_json",canonical,8192)&&text(b,"signature",signature,512)&&text(b,"key_id",kid,96)&&text(w,"ssid",out.wifi_ssid,32)&&text(w,"password",out.wifi_password,63)&&out.wifi_password.size()>=8&&text(root.get(),"fiscal_operator_password",out.secrets.fiscal_operator_password,8)&&text(s,"root_ca_pem",out.secrets.root_ca_pem,8192)&&text(s,"client_certificate_pem",out.secrets.client_certificate_pem,8192)&&text(s,"command_hmac_key",out.secrets.command_hmac_key,256)&&out.secrets.command_hmac_key.size()>=32&&text(s,"sync_ack_hmac_key",out.secrets.sync_ack_hmac_key,256)&&out.secrets.sync_ack_hmac_key.size()>=32;
 wipe(raw);
 if(!valid){out.clear();
 return ESP_ERR_INVALID_ARG;}
 SignedBindingEnvelope envelope{canonical.data(),canonical.size(),signature.c_str(),kid.c_str()};
 e=validator_.validate(envelope,out.binding);
 // open_provisioning_package authenticated these exact bytes in its header.
 static constexpr char hex[]="0123456789abcdef";std::string expected_hash;expected_hash.reserve(64);for(size_t i=44;i<76;i++){expected_hash.push_back(hex[p[i]>>4]);expected_hash.push_back(hex[p[i]&15]);}
 uint64_t generation=0;for(size_t i=4;i<12;i++)generation=(generation<<8)|p[i];
 const auto profile=out.binding.profile;bool password=out.secrets.fiscal_operator_password.size()==(profile==EdgeProfile::DatecsDp150BluePad50?8:4);for(char c:out.secrets.fiscal_operator_password)password&=c>='0'&&c<='9';
 if(e!=ESP_OK||out.binding.schema_version!=3||uint64_t(out.binding.generation)!=generation||out.binding.payload_sha256!=expected_hash||out.binding.authority.transaction_signing_kid!=identity_.kid()||kid!=issuer_kid_||!password||out.secrets.root_ca_pem.find("BEGIN CERTIFICATE")==std::string::npos||out.secrets.client_certificate_pem.find("BEGIN CERTIFICATE")==std::string::npos||!certificate_matches(out.secrets,identity_)||identity_.local_tls_key_pem(out.secrets.client_key_pem)!=ESP_OK){out.clear();
 return ESP_ERR_INVALID_CRC;}
 uint8_t packet_hash[32];
 if(mbedtls_sha256(p,n,packet_hash,0)){out.clear();
 return ESP_FAIL;}for(uint8_t c:packet_hash){out.package_sha256.push_back(hex[c>>4]);out.package_sha256.push_back(hex[c&15]);}
 return ESP_OK;
}
esp_err_t EncryptedProvisioning::load(ProvisionedRuntime&out){
 out.clear();nvs_handle_t h{};auto e=nvs_open("edge-provision",NVS_READONLY,&h);
 if(e!=ESP_OK)return e;
 size_t size{};e=nvs_get_blob(h,"active",nullptr,&size);
 if(e!=ESP_OK||size<239||size>kProvisioningMaxPacket){nvs_close(h);
 return e==ESP_OK?ESP_ERR_INVALID_SIZE:e;}std::vector<uint8_t>packet(size);e=nvs_get_blob(h,"active",packet.data(),&size);nvs_close(h);
 if(e!=ESP_OK)return e;
 ProvisioningSubject subject{identity_.kid(),issuer_kid_,0,{}};memcpy(subject.setup_nonce,packet.data()+12,32);
 return decode(packet.data(),packet.size(),subject,out);
}
esp_err_t EncryptedProvisioning::install(const uint8_t*p,size_t n,const uint8_t nonce[32],ProvisionedRuntime&out){
 out.clear();
 if(!nonce)return ESP_ERR_INVALID_ARG;
 ProvisionedRuntime previous;const auto loaded=load(previous);
 if(loaded!=ESP_OK&&loaded!=ESP_ERR_NVS_NOT_FOUND)return loaded;
 uint64_t floor=std::max<int64_t>(validator_.generation(),loaded==ESP_OK?previous.binding.generation:0);previous.clear();ProvisioningSubject subject{identity_.kid(),issuer_kid_,floor,{}};memcpy(subject.setup_nonce,nonce,32);
 auto e=decode(p,n,subject,out);
 if(e!=ESP_OK)return e;
 nvs_handle_t h{};e=nvs_open("edge-provision",NVS_READWRITE,&h);
 if(e==ESP_OK){e=nvs_set_blob(h,"active",p,n);
 if(e==ESP_OK)e=nvs_commit(h);
 nvs_close(h);}if(e!=ESP_OK){out.clear();
 return e;}
 const auto digest=out.binding.payload_sha256;out.clear();e=load(out);
 if(e!=ESP_OK||out.binding.payload_sha256!=digest){out.clear();
 return e==ESP_OK?ESP_ERR_INVALID_CRC:e;}return ESP_OK;
}
}
