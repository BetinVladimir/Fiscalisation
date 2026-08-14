#include "device_identity.h"
#include "esp_random.h"
#include "mbedtls/base64.h"
#include "mbedtls/ecdsa.h"
#include "mbedtls/pk.h"
#include "mbedtls/sha256.h"
#include "nvs.h"
#include <cstring>
#include <vector>
namespace beefiscal::idf{namespace{
int random(void*,unsigned char*out,size_t n){esp_fill_random(out,n);return 0;}
std::string b64(const uint8_t*p,size_t n){std::vector<uint8_t>o(((n+2)/3)*4+1);size_t z=0;if(mbedtls_base64_encode(o.data(),o.size(),&z,p,n)!=0)return{};std::string s((char*)o.data(),z);for(char&c:s){if(c=='+')c='-';else if(c=='/')c='_';}while(!s.empty()&&s.back()=='=')s.pop_back();return s;}
bool hex(const char*s,uint8_t*out,size_t n){if(!s||strlen(s)!=n*2)return false;for(size_t i=0;i<n;i++){char p[3]={s[i*2],s[i*2+1],0};char*e=nullptr;unsigned long v=strtoul(p,&e,16);if(!e||*e)return false;out[i]=v;}return true;}
}
struct DeviceIdentity::Impl{mbedtls_ecp_keypair key;bool ok{};std::string public_der,kid;Impl(){mbedtls_ecp_keypair_init(&key);}~Impl(){mbedtls_ecp_keypair_free(&key);}};
DeviceIdentity::DeviceIdentity():impl_(new Impl){}DeviceIdentity::~DeviceIdentity(){delete impl_;}
esp_err_t DeviceIdentity::open(){if(impl_->ok)return ESP_OK;nvs_handle_t h{};esp_err_t e=nvs_open("edge-identity",NVS_READWRITE,&h);if(e!=ESP_OK)return e;uint8_t scalar[32]{};size_t z=sizeof(scalar);e=nvs_get_blob(h,"p256",scalar,&z);auto&group=impl_->key.MBEDTLS_PRIVATE(grp);auto&private_key=impl_->key.MBEDTLS_PRIVATE(d);auto&public_key=impl_->key.MBEDTLS_PRIVATE(Q);if(e==ESP_ERR_NVS_NOT_FOUND){if(mbedtls_ecp_gen_key(MBEDTLS_ECP_DP_SECP256R1,&impl_->key,random,nullptr)!=0){nvs_close(h);return ESP_FAIL;}if(mbedtls_mpi_write_binary(&private_key,scalar,sizeof(scalar))!=0||(e=nvs_set_blob(h,"p256",scalar,sizeof(scalar)))!=ESP_OK||(e=nvs_commit(h))!=ESP_OK){memset(scalar,0,sizeof(scalar));nvs_close(h);return e==ESP_OK?ESP_FAIL:e;}}else{if(e!=ESP_OK||z!=sizeof(scalar)||mbedtls_ecp_group_load(&group,MBEDTLS_ECP_DP_SECP256R1)!=0||mbedtls_mpi_read_binary(&private_key,scalar,z)!=0||mbedtls_ecp_mul(&group,&public_key,&private_key,&group.G,random,nullptr)!=0){memset(scalar,0,sizeof(scalar));nvs_close(h);return ESP_ERR_INVALID_CRC;}}memset(scalar,0,sizeof(scalar));nvs_close(h);
  mbedtls_pk_context pk;mbedtls_pk_init(&pk);if(mbedtls_pk_setup(&pk,mbedtls_pk_info_from_type(MBEDTLS_PK_ECKEY))!=0){mbedtls_pk_free(&pk);return ESP_FAIL;}if(mbedtls_ecp_group_copy(&mbedtls_pk_ec(pk)->MBEDTLS_PRIVATE(grp),&group)!=0||mbedtls_ecp_copy(&mbedtls_pk_ec(pk)->MBEDTLS_PRIVATE(Q),&public_key)!=0){mbedtls_pk_free(&pk);return ESP_FAIL;}uint8_t der[160];int n=mbedtls_pk_write_pubkey_der(&pk,der,sizeof(der));mbedtls_pk_free(&pk);if(n<=0)return ESP_FAIL;impl_->public_der=b64(der+sizeof(der)-n,n);uint8_t d[32];mbedtls_sha256((uint8_t*)impl_->public_der.data(),impl_->public_der.size(),d,0);impl_->kid="esp32-p256-"+b64(d,12);impl_->ok=true;return ESP_OK;}
bool DeviceIdentity::ready()const{return impl_->ok;}
esp_err_t DeviceIdentity::sign_hash_hex(const char*h,std::string&out)const{out.clear();if(!impl_->ok)return ESP_ERR_INVALID_STATE;uint8_t digest[32],double_digest[32],sig[64];if(!hex(h,digest,32)||mbedtls_sha256(digest,sizeof(digest),double_digest,0)!=0)return ESP_ERR_INVALID_ARG;mbedtls_mpi r,s;mbedtls_mpi_init(&r);mbedtls_mpi_init(&s);int e=mbedtls_ecdsa_sign(&impl_->key.MBEDTLS_PRIVATE(grp),&r,&s,&impl_->key.MBEDTLS_PRIVATE(d),double_digest,sizeof(double_digest),random,nullptr);if(e==0)e=mbedtls_mpi_write_binary(&r,sig,32);if(e==0)e=mbedtls_mpi_write_binary(&s,sig+32,32);mbedtls_mpi_free(&r);mbedtls_mpi_free(&s);if(e)return ESP_FAIL;out=impl_->kid+":"+b64(sig,sizeof(sig));return ESP_OK;}
std::string DeviceIdentity::public_key_der_base64url()const{return impl_->public_der;}std::string DeviceIdentity::kid()const{return impl_->kid;}
}
