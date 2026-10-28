#pragma once
#include <openssl/sha.h>
inline int mbedtls_sha256(const unsigned char*p,size_t n,unsigned char*out,int){return SHA256(p,n,out)?0:-1;}
