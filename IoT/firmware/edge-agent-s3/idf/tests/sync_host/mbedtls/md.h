#pragma once
#include <openssl/hmac.h>
#include <vector>
#define MBEDTLS_MD_SHA256 1
struct mbedtls_md_context_t{std::vector<unsigned char>key,body;};
inline const EVP_MD*mbedtls_md_info_from_type(int){return EVP_sha256();}
inline void mbedtls_md_init(mbedtls_md_context_t*){}
inline int mbedtls_md_setup(mbedtls_md_context_t*,const EVP_MD*,int){return 0;}
inline int mbedtls_md_hmac_starts(mbedtls_md_context_t*c,const unsigned char*p,size_t n){c->key.assign(p,p+n);return 0;}
inline int mbedtls_md_hmac_update(mbedtls_md_context_t*c,const unsigned char*p,size_t n){c->body.insert(c->body.end(),p,p+n);return 0;}
inline int mbedtls_md_hmac_finish(mbedtls_md_context_t*c,unsigned char*out){return HMAC(EVP_sha256(),c->key.data(),c->key.size(),c->body.data(),c->body.size(),out,nullptr)?0:-1;}
inline void mbedtls_md_free(mbedtls_md_context_t*){}

inline int mbedtls_md_hmac(const EVP_MD*m,const unsigned char*k,size_t kn,const unsigned char*p,size_t n,unsigned char*out){return HMAC(m,k,kn,p,n,out,nullptr)?0:-1;}
