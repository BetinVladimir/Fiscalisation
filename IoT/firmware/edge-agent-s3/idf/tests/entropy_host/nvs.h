#pragma once
#include "esp_err.h"
#include <array>
#include <cstring>
using nvs_handle_t=int;
constexpr int NVS_READWRITE=1;
inline bool test_identity_present=false,test_fail_commit=false;
inline std::array<unsigned char,32>test_identity_scalar{},test_pending_scalar{};
inline int test_nvs_writes=0;
inline esp_err_t nvs_open(const char*,int,nvs_handle_t*h){*h=1;return ESP_OK;}
inline void nvs_close(nvs_handle_t){}
inline esp_err_t nvs_get_blob(nvs_handle_t,const char*,void*out,size_t*n){if(!test_identity_present)return ESP_ERR_NVS_NOT_FOUND;if(*n<32)return ESP_ERR_INVALID_SIZE;*n=32;memcpy(out,test_identity_scalar.data(),32);return ESP_OK;}
inline esp_err_t nvs_set_blob(nvs_handle_t,const char*,const void*p,size_t n){if(n!=32)return ESP_ERR_INVALID_SIZE;++test_nvs_writes;memcpy(test_pending_scalar.data(),p,32);return ESP_OK;}
inline esp_err_t nvs_commit(nvs_handle_t){if(test_fail_commit)return ESP_FAIL;test_identity_scalar=test_pending_scalar;test_identity_present=true;return ESP_OK;}
