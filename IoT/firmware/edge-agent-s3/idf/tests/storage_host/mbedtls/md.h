#pragma once
#include <openssl/hmac.h>
#define MBEDTLS_MD_SHA256 1
inline const EVP_MD*mbedtls_md_info_from_type(int){return EVP_sha256();}
inline int mbedtls_md_hmac(const EVP_MD*m,const unsigned char*k,size_t kn,const unsigned char*p,size_t n,unsigned char*out){return HMAC(m,k,kn,p,n,out,nullptr)?0:-1;}
