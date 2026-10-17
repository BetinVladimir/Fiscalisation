#include "provisioning_crypto.h"
#include "mbedtls/pk.h"
#include "mbedtls/ecdsa.h"
#include "mbedtls/hkdf.h"
#include "mbedtls/gcm.h"
#include "mbedtls/sha256.h"
#include "mbedtls/platform_util.h"
#include <cstring>
namespace beefiscal::idf {namespace {
constexpr size_t header_size=159;
uint32_t u32(const uint8_t*p){return(uint32_t(p[0])<<24)|(uint32_t(p[1])<<16)|(uint32_t(p[2])<<8)|p[3];}
uint64_t u64(const uint8_t*p){uint64_t n=0;for(size_t i=0;i<8;i++)n=(n<<8)|p[i];return n;}
bool valid_id(const std::string&s){if(s.empty()||s.size()>96)return false;for(unsigned char c:s)if(!((c>='a'&&c<='z')||(c>='A'&&c<='Z')||(c>='0'&&c<='9')||c=='-'||c=='_'||c=='.'))return false;return true;}
bool verify(const uint8_t*packet,size_t signed_size,const uint8_t*signature,const char*pem){
 if(!pem)return false;mbedtls_pk_context pk;mbedtls_pk_init(&pk);
 int e=mbedtls_pk_parse_public_key(&pk,reinterpret_cast<const uint8_t*>(pem),strlen(pem)+1);
 if(!e&&(!mbedtls_pk_can_do(&pk,MBEDTLS_PK_ECDSA)||mbedtls_pk_get_bitlen(&pk)!=256))e=-1;
 uint8_t digest[32]{};if(!e)e=mbedtls_sha256(packet,signed_size,digest,0);
 mbedtls_mpi r,s,half;mbedtls_mpi_init(&r);mbedtls_mpi_init(&s);mbedtls_mpi_init(&half);
 if(!e){auto*ec=mbedtls_pk_ec(pk);if(ec->MBEDTLS_PRIVATE(grp).id!=MBEDTLS_ECP_DP_SECP256R1)e=-1;
  if(!e)e=mbedtls_mpi_read_binary(&r,signature,32);if(!e)e=mbedtls_mpi_read_binary(&s,signature+32,32);
  if(!e)e=mbedtls_mpi_copy(&half,&ec->MBEDTLS_PRIVATE(grp).N);if(!e)e=mbedtls_mpi_shift_r(&half,1);
  if(!e&&mbedtls_mpi_cmp_mpi(&s,&half)>0)e=-1;
  if(!e)e=mbedtls_ecdsa_verify(&ec->MBEDTLS_PRIVATE(grp),digest,sizeof(digest),&ec->MBEDTLS_PRIVATE(Q),&r,&s);
 }
 mbedtls_mpi_free(&r);mbedtls_mpi_free(&s);mbedtls_mpi_free(&half);mbedtls_pk_free(&pk);return e==0;
}
}
void OpenedProvisioning::clear(){if(!plaintext.empty())mbedtls_platform_zeroize(plaintext.data(),plaintext.size());plaintext.clear();generation=0;memset(binding_sha256,0,sizeof(binding_sha256));}
esp_err_t open_provisioning_package(const uint8_t*p,size_t n,const char*pem,const ProvisioningSubject&expected,ProvisioningSharedSecret derive,void*context,OpenedProvisioning&out){
 out.clear();if(!p||n<header_size+16+64||n>kProvisioningMaxPacket||!derive||memcmp(p,"BFP2",4)||!valid_id(expected.device_kid)||!valid_id(expected.issuer_kid))return ESP_ERR_INVALID_ARG;
 const uint64_t generation=u64(p+4);const size_t device_size=p[153],issuer_size=p[154],cipher_size=u32(p+155),aad_size=header_size+device_size+issuer_size;
 uint8_t nonzero=0;for(size_t i=0;i<32;i++)nonzero|=expected.setup_nonce[i];
 if(!nonzero||generation<=expected.generation_floor||!device_size||device_size>96||!issuer_size||issuer_size>96||cipher_size<17||cipher_size>kProvisioningMaxPlaintext+16||aad_size+cipher_size+64!=n||memcmp(p+12,expected.setup_nonce,32))return ESP_ERR_INVALID_ARG;
 if(std::string(reinterpret_cast<const char*>(p+header_size),device_size)!=expected.device_kid||std::string(reinterpret_cast<const char*>(p+header_size+device_size),issuer_size)!=expected.issuer_kid)return ESP_ERR_INVALID_ARG;
 if(!verify(p,n-64,p+n-64,pem))return ESP_ERR_INVALID_CRC;
 uint8_t shared[32]{},key[32]{};if(derive(context,p+76,shared)!=ESP_OK){mbedtls_platform_zeroize(shared,sizeof(shared));return ESP_ERR_INVALID_CRC;}
 static const char domain[]="BeeFiscalProvisioningV2";std::vector<uint8_t>info(domain,domain+sizeof(domain));info.insert(info.end(),p,p+aad_size);
 int e=mbedtls_hkdf(mbedtls_md_info_from_type(MBEDTLS_MD_SHA256),p+12,32,shared,sizeof(shared),info.data(),info.size(),key,sizeof(key));mbedtls_platform_zeroize(shared,sizeof(shared));
 mbedtls_gcm_context g;mbedtls_gcm_init(&g);if(!e)e=mbedtls_gcm_setkey(&g,MBEDTLS_CIPHER_ID_AES,key,256);out.plaintext.resize(cipher_size-16);
 if(!e)e=mbedtls_gcm_auth_decrypt(&g,out.plaintext.size(),p+141,12,p,aad_size,p+aad_size+cipher_size-16,16,p+aad_size,out.plaintext.data());
 mbedtls_gcm_free(&g);mbedtls_platform_zeroize(key,sizeof(key));if(e){out.clear();return ESP_ERR_INVALID_CRC;}
 out.generation=generation;memcpy(out.binding_sha256,p+44,32);return ESP_OK;
}
}
