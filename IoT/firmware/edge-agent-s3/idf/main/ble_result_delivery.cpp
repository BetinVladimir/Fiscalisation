#include "ble_result_delivery.h"
#include "canonical_cbor.h"
#include <cstdlib>
#include <cstring>
namespace beefiscal::idf {namespace {
using V=go_json::Value;
bool normalize(V&v,unsigned depth){
 if(depth>16)return false;
 if(v.kind==V::Kind::Number){int64_t n;if(v.text=="-0"||!go_json::integer(v,n))return false;v=V::integer_value(n);}
 if(v.kind==V::Kind::String&&v.text.find('\0')!=std::string::npos)return false;
 for(auto&f:v.fields){if(f.first.find('\0')!=std::string::npos||!normalize(f.second,depth+1))return false;}
 for(auto&item:v.elements)if(!normalize(item,depth+1))return false;
 return true;
}
bool parse_result_value(const char*json,V&value){
 if(!json)return false;
 const size_t size=strnlen(json,8193);if(!size||size>8192)return false;
 std::string error;return go_json::parse_strict_unicode(json,size,value,error)&&normalize(value,1)&&value.kind==V::Kind::Map;
}
}
esp_err_t deliver_ble_result(const char*operation_id,const char*json,Ingress ingress,BleReplyOwner owner,void*){
 if(ingress!=Ingress::Ble)return ESP_OK;
 V root;if(!parse_result_value(json,root))return ESP_ERR_INVALID_ARG;
 V*state=nullptr;for(auto&field:root.fields)if(field.first=="state")state=&field.second;
 if(!state||state->kind!=V::Kind::String)return ESP_ERR_INVALID_ARG;
 if(state->text=="COMMITTED")state->text="FISCALIZED";
 else if(state->text=="RECOVERY_REQUIRED")state->text="FISCAL_RESULT_UNKNOWN";
 else if(state->text=="COMPENSATED")state->text="REJECTED";
 uint8_t*encoded=nullptr;size_t size=0;esp_err_t e=canonical_value_to_cbor(root,&encoded,&size);
 if(e==ESP_OK)e=ble_publish_event(owner,operation_id,encoded,size);
 free(encoded);return e;
}
}
