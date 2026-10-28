#pragma once
#include <map>
#include <string>
#include <cstring>
#include "esp_err.h"
using nvs_handle_t=int;
constexpr int NVS_READONLY=0,NVS_READWRITE=1;
inline std::map<std::string,std::string> secret_values;
inline int secret_commits=0;
inline esp_err_t nvs_open(const char*,int,nvs_handle_t*h){*h=1;return ESP_OK;}
inline void nvs_close(nvs_handle_t){}
inline esp_err_t nvs_get_str(nvs_handle_t,const char*k,char*out,size_t*n){auto i=secret_values.find(k);if(i==secret_values.end())return ESP_ERR_NVS_NOT_FOUND;if(!out){*n=i->second.size()+1;return ESP_OK;}if(*n<i->second.size()+1)return ESP_ERR_INVALID_SIZE;memcpy(out,i->second.c_str(),i->second.size()+1);*n=i->second.size()+1;return ESP_OK;}
inline esp_err_t nvs_set_str(nvs_handle_t,const char*k,const char*v){secret_values[k]=v;return ESP_OK;}
inline esp_err_t nvs_commit(nvs_handle_t){++secret_commits;return ESP_OK;}
inline esp_err_t nvs_get_i64(nvs_handle_t,const char*,int64_t*){return ESP_ERR_NVS_NOT_FOUND;}
inline esp_err_t nvs_set_i64(nvs_handle_t,const char*,int64_t){return ESP_OK;}
inline esp_err_t nvs_get_blob(nvs_handle_t,const char*,void*,size_t*){return ESP_ERR_NVS_NOT_FOUND;}
inline esp_err_t nvs_set_blob(nvs_handle_t,const char*,const void*,size_t){return ESP_OK;}
