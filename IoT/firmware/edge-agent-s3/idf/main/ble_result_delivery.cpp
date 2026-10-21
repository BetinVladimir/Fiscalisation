#include "ble_result_delivery.h"
#include "canonical_cbor.h"
#include <cstdlib>
#include <cstring>
namespace beefiscal::idf {
esp_err_t deliver_ble_result(const char*operation_id,const char*json,Ingress ingress,BleReplyOwner owner,void*){
  if(ingress!=Ingress::Ble)return ESP_OK;

  cJSON*root=cJSON_Parse(json);

 if(!root)return ESP_ERR_INVALID_ARG;

  auto*state=cJSON_GetObjectItemCaseSensitive(root,"state");

  if(cJSON_IsString(state)){
    if(strcmp(state->valuestring,"COMMITTED")==0)cJSON_SetValuestring(state,"FISCALIZED");

    else if(strcmp(state->valuestring,"RECOVERY_REQUIRED")==0)cJSON_SetValuestring(state,"FISCAL_RESULT_UNKNOWN");

    else if(strcmp(state->valuestring,"COMPENSATED")==0)cJSON_SetValuestring(state,"REJECTED");

  }
  uint8_t*encoded=nullptr;
 size_t size=0;
 esp_err_t e=canonical_json_to_cbor(root,&encoded,&size);
 cJSON_Delete(root);

  if(e==ESP_OK)e=ble_publish_event(owner,operation_id,encoded,size);

  free(encoded);

  return e;

}
}
