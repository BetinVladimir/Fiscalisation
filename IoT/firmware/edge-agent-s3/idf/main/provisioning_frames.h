#pragma once
#include "provisioning_crypto.h"
#include "mbedtls/sha256.h"
#include <cstring>
namespace beefiscal::idf {
// BFPF: magic4, transfer-id16, totalBE4, offsetBE4, SHA25632, final1, body.
// One instance per BLE connection. Call reset on every disconnect/reconnect.
class ProvisioningFrames final {
 public:
 void reset(){bytes_.clear();total_=0;last_ms_=0;memset(id_,0,16);memset(digest_,0,32);}
 esp_err_t accept(const uint8_t*p,size_t n,uint64_t now_ms,std::vector<uint8_t>&completed){
  completed.clear();if(!p||n<=61||n>512||memcmp(p,"BFPF",4)||p[60]>1){reset();return ESP_ERR_INVALID_ARG;}
  const size_t total=number(p+20),offset=number(p+24),body=n-61;
  if(!total||total>kProvisioningMaxPacket||offset>total||body>total-offset||!now_ms||(last_ms_&&(now_ms<last_ms_||now_ms-last_ms_>10000))){reset();return ESP_ERR_INVALID_SIZE;}
  if(!total_){if(offset){reset();return ESP_ERR_INVALID_ARG;}total_=total;memcpy(id_,p+4,16);memcpy(digest_,p+28,32);bytes_.reserve(total);}
  if(total!=total_||offset!=bytes_.size()||memcmp(id_,p+4,16)||memcmp(digest_,p+28,32)||(bool(p[60])!=(offset+body==total))){reset();return ESP_ERR_INVALID_ARG;}
  bytes_.insert(bytes_.end(),p+61,p+n);last_ms_=now_ms;
  if(p[60]){uint8_t actual[32]{};if(mbedtls_sha256(bytes_.data(),bytes_.size(),actual,0)||memcmp(actual,digest_,32)){reset();return ESP_ERR_INVALID_CRC;}completed.swap(bytes_);reset();}
  return ESP_OK;
 }
 private:
 static uint32_t number(const uint8_t*p){return(uint32_t(p[0])<<24)|(uint32_t(p[1])<<16)|(uint32_t(p[2])<<8)|p[3];}
 std::vector<uint8_t>bytes_;size_t total_{};uint64_t last_ms_{};uint8_t id_[16]{},digest_[32]{};
};
}
