#include "runtime_config.h"
#include "nvs.h"
#include <vector>

namespace beefiscal::idf{
esp_err_t RuntimeConfigLoader::read_protected_nvs(const std::string&key,std::string&out){
  if(key.empty()||key.size()>15)return ESP_ERR_INVALID_ARG;
  nvs_handle_t h{};esp_err_t e=nvs_open("edge-secrets",NVS_READONLY,&h);if(e!=ESP_OK)return e;
  size_t size=0;e=nvs_get_str(h,key.c_str(),nullptr,&size);if(e!=ESP_OK){nvs_close(h);return e;}
  std::vector<char>value(size);e=nvs_get_str(h,key.c_str(),value.data(),&size);nvs_close(h);
  if(e!=ESP_OK||size<2)return e==ESP_OK?ESP_ERR_INVALID_SIZE:e;
  out.assign(value.data(),size-1);return ESP_OK;
}
esp_err_t RuntimeConfigLoader::load_secrets(const CompositeBinding&b,RuntimeSecrets&s)const{
  s={};esp_err_t e=read_protected_nvs(b.mqtt.root_ca_ref,s.root_ca_pem);if(e!=ESP_OK)return e;
  if((e=read_protected_nvs(b.mqtt.client_certificate_ref,s.client_certificate_pem))!=ESP_OK)return e;
  if((e=read_protected_nvs(b.mqtt.client_key_ref,s.client_key_pem))!=ESP_OK)return e;
  if((e=read_protected_nvs(b.authority.command_hmac_ref,s.command_hmac_key))!=ESP_OK)return e;
  if((e=read_protected_nvs(b.authority.sync_ack_hmac_ref,s.sync_ack_hmac_key))!=ESP_OK)return e;
  if(s.root_ca_pem.find("BEGIN CERTIFICATE")==std::string::npos||
     s.client_certificate_pem.find("BEGIN CERTIFICATE")==std::string::npos||
     s.client_key_pem.find("PRIVATE KEY")==std::string::npos||
     s.command_hmac_key.size()<32||s.sync_ack_hmac_key.size()<32){s={};return ESP_ERR_INVALID_CRC;}
  return ESP_OK;
}
}
