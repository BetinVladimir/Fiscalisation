#pragma once
#include <openssl/evp.h>
inline int mbedtls_base64_decode(unsigned char*out,size_t cap,size_t*n,const unsigned char*in,size_t len){if(cap<len/4*3)return -1;int z=EVP_DecodeBlock(out,in,len);if(z<0)return -1;while(len&&in[len-1]=='='){--z;--len;}*n=z;return 0;}
