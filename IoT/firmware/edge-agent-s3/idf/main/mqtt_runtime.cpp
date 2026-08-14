#include "edge_runtime_port.h"
#include "mqtt_client.h"
#include <cstring>
#include <vector>
#include <string>

namespace beefiscal::idf { namespace {
CommandSink sink; void* ctx;AckSink ack_sink;void*ack_ctx;ProvisioningSink binding_sink;void*binding_ctx;esp_mqtt_client_handle_t client;
const char* commands;const char*bindings; const char* sync_topic;const char*acks;
const char*applied_tenant;const char*applied_device;const char*applied_register;int64_t applied_generation;
std::vector<uint8_t> assembly;int expected_length;bool assembling_command;bool assembling_ack;bool assembling_binding;
void event(void*,esp_event_base_t,int32_t id,void* raw){
  auto*e=(esp_mqtt_event_handle_t)raw;
  if(id==MQTT_EVENT_CONNECTED){esp_mqtt_client_subscribe(client,commands,1);if(bindings)esp_mqtt_client_subscribe(client,bindings,1);if(acks)esp_mqtt_client_subscribe(client,acks,1);if(applied_generation>0)mqtt_publish_binding_applied(applied_tenant,applied_device,applied_register,applied_generation);return;}
  if(id!=MQTT_EVENT_DATA||!sink)return;
  if(e->current_data_offset==0){
    assembling_command=e->topic_len==(int)strlen(commands)&&memcmp(e->topic,commands,e->topic_len)==0;
    assembling_ack=acks&&e->topic_len==(int)strlen(acks)&&memcmp(e->topic,acks,e->topic_len)==0;
    assembling_binding=bindings&&e->topic_len==(int)strlen(bindings)&&memcmp(e->topic,bindings,e->topic_len)==0;
    expected_length=e->total_data_len;assembly.clear();
    if((!assembling_command&&!assembling_ack&&!assembling_binding)||expected_length<1||expected_length>8192){assembling_command=false;assembling_ack=false;assembling_binding=false;return;}
    assembly.reserve(expected_length);
  }
  if((!assembling_command&&!assembling_ack&&!assembling_binding)||e->current_data_offset!=(int)assembly.size()||
     assembly.size()+e->data_len>(size_t)expected_length){assembling_command=false;assembly.clear();return;}
  assembly.insert(assembly.end(),(const uint8_t*)e->data,(const uint8_t*)e->data+e->data_len);
  if((int)assembly.size()==expected_length){if(assembling_command)sink({assembly.data(),assembly.size(),Ingress::Mqtt},ctx);else if(assembling_binding&&binding_sink)binding_sink(assembly.data(),assembly.size(),binding_ctx);else if(ack_sink)ack_sink(assembly.data(),assembly.size(),ack_ctx);
    assembling_command=false;assembling_ack=false;assembling_binding=false;assembly.clear();}
}}
esp_err_t mqtt_runtime_start(const MqttConfig& value,CommandSink command_sink,void* context,AckSink acksink,void*ackcontext,ProvisioningSink bindingsink,void*bindingcontext){
  if(!command_sink)return ESP_ERR_INVALID_ARG;
  if(!value.uri||strncmp(value.uri,"mqtts://",8)!=0||!value.command_topic||!value.binding_topic||
     !value.sync_topic||!value.ack_topic||!value.client_id||!value.root_ca_pem||
     !value.client_certificate_pem||!value.client_key_pem)return ESP_ERR_INVALID_STATE;
  sink=command_sink;ctx=context;ack_sink=acksink;ack_ctx=ackcontext;binding_sink=bindingsink;binding_ctx=bindingcontext;commands=value.command_topic;bindings=value.binding_topic;sync_topic=value.sync_topic;acks=value.ack_topic;
  applied_tenant=value.applied_tenant;applied_device=value.applied_device;applied_register=value.applied_register;applied_generation=value.applied_generation;
  esp_mqtt_client_config_t cfg{};
  cfg.broker.address.uri=value.uri;
  cfg.broker.verification.certificate=value.root_ca_pem;
  cfg.credentials.client_id=value.client_id;
  cfg.credentials.authentication.certificate=value.client_certificate_pem;
  cfg.credentials.authentication.key=value.client_key_pem;
  cfg.session.disable_clean_session=true;cfg.session.keepalive=30;
  cfg.network.reconnect_timeout_ms=1000;
  client=esp_mqtt_client_init(&cfg);if(!client)return ESP_ERR_NO_MEM;
  const esp_err_t registered=esp_mqtt_client_register_event(client,MQTT_EVENT_ANY,event,nullptr);
  if(registered!=ESP_OK)return registered;
  return esp_mqtt_client_start(client);
}
int mqtt_publish_sync(const char*batch_id,const uint8_t*data,size_t size){
  if(!client||!sync_topic||!batch_id||!*batch_id||!data||!size)return-1;
  std::string topic=std::string(sync_topic)+"/"+batch_id;
  return esp_mqtt_client_publish(client,topic.c_str(),(const char*)data,(int)size,1,0);
}
int mqtt_publish_binding_applied(const char*tenant,const char*device,const char*register_id,int64_t generation){if(!client||!tenant||!device||!register_id||generation<1)return-1;std::string topic="tenants/"+std::string(tenant)+"/devices/"+device+"/bindings/acks";std::string body="{\"adapter_device_id\":\""+std::string(device)+"\",\"register_id\":\""+register_id+"\",\"generation\":"+std::to_string(generation)+"}";return esp_mqtt_client_publish(client,topic.c_str(),body.c_str(),body.size(),1,0);}
int mqtt_publish_status(const char*tenant,const char*device,const char*payload,bool retained){if(!client||!tenant||!device||!payload||!*payload)return-1;std::string topic="tenants/"+std::string(tenant)+"/devices/"+device+"/status";return esp_mqtt_client_publish(client,topic.c_str(),payload,0,1,retained?1:0);}
}
