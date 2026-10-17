#pragma once
#include <cstddef>
#include "mbedtls/md.h"
struct mbedtls_pk_context{};
inline void mbedtls_pk_init(mbedtls_pk_context*){}
inline void mbedtls_pk_free(mbedtls_pk_context*){}
inline int mbedtls_pk_parse_public_key(mbedtls_pk_context*,const unsigned char*,size_t){return -1;}
inline int mbedtls_pk_verify(mbedtls_pk_context*,int,const unsigned char*,size_t,const unsigned char*,size_t){return -1;}
