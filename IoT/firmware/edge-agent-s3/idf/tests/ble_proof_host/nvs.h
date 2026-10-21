#pragma once
#include "esp_err.h"
#include <array>
#include <cstring>
#include <map>
#include <string>
#include <vector>
#include <functional>
using nvs_handle_t=int;
constexpr int NVS_READWRITE=1,NVS_READONLY=0;
inline std::array<unsigned char,32>factory_scalar{};
inline std::map<int,std::string>nvs_handles;
inline std::map<std::string,std::vector<uint8_t>>nvs_durable,nvs_pending;
inline bool nvs_fail_commit=false;
inline bool lifecycle_generate_identity=false;
inline std::string lifecycle_read_error,lifecycle_write_error;
inline bool proof_read_error=false,proof_shrink_read=false;
inline int proof_commit_mode=0,proof_commits=0,identity_writes=0;
inline std::function<void(const std::string&)> commit_hook;
inline esp_err_t nvs_open(const char*n,int mode,nvs_handle_t*h){if(mode==NVS_READONLY&&!strcmp(n,"ble-cert-pop")&&proof_read_error)return ESP_FAIL;if(mode==NVS_READONLY){bool found=false;for(auto&v:nvs_durable)found|=v.first.rfind(std::string(n)+"/",0)==0;if(!found)return ESP_ERR_NVS_NOT_FOUND;}*h=nvs_handles.size()+1;nvs_handles[*h]=n;return ESP_OK;}
inline void nvs_close(nvs_handle_t h){auto prefix=nvs_handles[h]+"/";for(auto i=nvs_pending.begin();i!=nvs_pending.end();){if(i->first.rfind(prefix,0)==0)i=nvs_pending.erase(i);else ++i;}}
inline esp_err_t nvs_get_blob(nvs_handle_t h,const char*k,void*out,size_t*n){if(!lifecycle_read_error.empty()&&nvs_handles[h]==lifecycle_read_error)return ESP_FAIL;if(nvs_handles[h]=="ble-cert-pop"&&proof_read_error)return ESP_FAIL;if(nvs_handles[h]=="edge-identity"&&!strcmp(k,"p256")&&!lifecycle_generate_identity){if(out){if(*n<32)return ESP_ERR_INVALID_SIZE;memcpy(out,factory_scalar.data(),32);}*n=32;return ESP_OK;}auto i=nvs_durable.find(nvs_handles[h]+"/"+k);if(i==nvs_durable.end())return ESP_ERR_NVS_NOT_FOUND;if(out){if(*n<i->second.size())return ESP_ERR_INVALID_SIZE;const auto z=i->second.size()-(proof_shrink_read&&nvs_handles[h]=="ble-cert-pop"?1:0);memcpy(out,i->second.data(),z);*n=z;return ESP_OK;}*n=i->second.size();return ESP_OK;}
inline esp_err_t nvs_set_blob(nvs_handle_t h,const char*k,const void*p,size_t n){if(!lifecycle_write_error.empty()&&nvs_handles[h]==lifecycle_write_error)return ESP_FAIL;if(nvs_handles[h]=="edge-identity")identity_writes++;auto*b=static_cast<const uint8_t*>(p);nvs_pending[nvs_handles[h]+"/"+k]={b,b+n};return ESP_OK;}
inline esp_err_t nvs_get_i64(nvs_handle_t h,const char*k,int64_t*v){size_t n=sizeof(*v);return nvs_get_blob(h,k,v,&n);}
inline esp_err_t nvs_set_i64(nvs_handle_t h,const char*k,int64_t v){return nvs_set_blob(h,k,&v,sizeof(v));}
inline esp_err_t nvs_commit(nvs_handle_t h){
 const auto name=nvs_handles[h];if(commit_hook)commit_hook(name);
 if(nvs_fail_commit)return ESP_FAIL;
 if(name=="ble-cert-pop"){proof_commits++;if(proof_commit_mode==1)return ESP_FAIL;}
 const auto prefix=name+"/";for(const auto&v:nvs_pending)if(v.first.rfind(prefix,0)==0)nvs_durable[v.first]=v.second;
 if(name=="ble-cert-pop"&&proof_commit_mode==2)return ESP_FAIL;
 if(name=="ble-cert-pop"&&proof_commit_mode==3)proof_read_error=true;
 return ESP_OK;
}
