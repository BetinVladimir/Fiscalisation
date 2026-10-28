#pragma once
#include "esp_err.h"
#include <array>
#include <cstring>
#include <map>
#include <string>
#include <vector>
using nvs_handle_t=int;
constexpr int NVS_READWRITE=1,NVS_READONLY=0;
inline std::array<unsigned char,32>factory_scalar{};
inline std::map<int,std::string>nvs_handles;
inline std::map<std::string,std::vector<uint8_t>>nvs_durable,nvs_pending;
inline bool nvs_fail_commit=false;
inline esp_err_t nvs_open(const char*n,int mode,nvs_handle_t*h){if(mode==NVS_READONLY){bool found=false;for(auto&v:nvs_durable)found|=v.first.rfind(std::string(n)+"/",0)==0;if(!found)return ESP_ERR_NVS_NOT_FOUND;}*h=nvs_handles.size()+1;nvs_handles[*h]=n;return ESP_OK;}
inline void nvs_close(nvs_handle_t h){auto prefix=nvs_handles[h]+"/";for(auto i=nvs_pending.begin();i!=nvs_pending.end();){if(i->first.rfind(prefix,0)==0)i=nvs_pending.erase(i);else ++i;}}
inline esp_err_t nvs_get_blob(nvs_handle_t h,const char*k,void*out,size_t*n){if(nvs_handles[h]=="edge-identity"&&!strcmp(k,"p256")){if(out){if(*n<32)return ESP_ERR_INVALID_SIZE;memcpy(out,factory_scalar.data(),32);}*n=32;return ESP_OK;}auto i=nvs_durable.find(nvs_handles[h]+"/"+k);if(i==nvs_durable.end())return ESP_ERR_NVS_NOT_FOUND;if(out){if(*n<i->second.size())return ESP_ERR_INVALID_SIZE;memcpy(out,i->second.data(),i->second.size());}*n=i->second.size();return ESP_OK;}
inline esp_err_t nvs_set_blob(nvs_handle_t h,const char*k,const void*p,size_t n){auto*b=static_cast<const uint8_t*>(p);nvs_pending[nvs_handles[h]+"/"+k]={b,b+n};return ESP_OK;}
inline esp_err_t nvs_get_i64(nvs_handle_t h,const char*k,int64_t*v){size_t n=sizeof(*v);return nvs_get_blob(h,k,v,&n);}
inline esp_err_t nvs_set_i64(nvs_handle_t h,const char*k,int64_t v){return nvs_set_blob(h,k,&v,sizeof(v));}
inline esp_err_t nvs_commit(nvs_handle_t h){if(nvs_fail_commit)return ESP_FAIL;auto prefix=nvs_handles[h]+"/";for(auto&v:nvs_pending)if(v.first.rfind(prefix,0)==0)nvs_durable[v.first]=v.second;return ESP_OK;}
