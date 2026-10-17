#pragma once
#include "esp_err.h"
#include <array>
#include <cstring>
using nvs_handle_t=int;
constexpr int NVS_READWRITE=1;
inline std::array<unsigned char,32> factory_scalar{};
inline esp_err_t nvs_open(const char*,int,nvs_handle_t*h){*h=1;return ESP_OK;}
inline void nvs_close(nvs_handle_t){}
inline esp_err_t nvs_get_blob(nvs_handle_t,const char*k,void*out,size_t*n){if(strcmp(k,"p256")||*n!=32)return ESP_ERR_NVS_NOT_FOUND;memcpy(out,factory_scalar.data(),32);*n=32;return ESP_OK;}
inline esp_err_t nvs_set_blob(nvs_handle_t,const char*,const void*,size_t){return ESP_FAIL;}
inline esp_err_t nvs_commit(nvs_handle_t){return ESP_FAIL;}
