#include "edge_runtime_port.h"
#include "mqtt_client.h"
#include <algorithm>
#include <iostream>
#include <string>
#include <vector>
using namespace beefiscal::idf;
static mqtt_callback_t callback;static std::vector<std::string> subscriptions,acks_received,commands_received,bindings_received;static int failures;
esp_mqtt_client_handle_t esp_mqtt_client_init(const esp_mqtt_client_config_t*) {return reinterpret_cast<void*>(1);}
esp_err_t esp_mqtt_client_register_event(esp_mqtt_client_handle_t,int,mqtt_callback_t f,void*){callback=f;return ESP_OK;}
esp_err_t esp_mqtt_client_start(esp_mqtt_client_handle_t){return ESP_OK;}
int esp_mqtt_client_subscribe(esp_mqtt_client_handle_t,const char* s,int qos){if(qos!=1)++failures;subscriptions.emplace_back(s);return 1;}
int esp_mqtt_client_publish(esp_mqtt_client_handle_t,const char*,const char*,int,int,int){return 1;}
static void check(bool ok,const char* name){if(!ok){std::cerr<<"FAIL "<<name<<'\n';++failures;}}
static esp_err_t ack(const uint8_t* p,size_t n,void*){acks_received.emplace_back(reinterpret_cast<const char*>(p),n);return ESP_OK;}
static esp_err_t binding(const uint8_t* p,size_t n,void*){bindings_received.emplace_back(reinterpret_cast<const char*>(p),n);return ESP_OK;}
static esp_err_t command(const CommandView& v,void*){commands_received.emplace_back(reinterpret_cast<const char*>(v.data),v.size);return ESP_OK;}
static void data(const char* topic,const char* bytes,int size,int total,int offset){esp_mqtt_event_t e{};e.topic=topic;e.topic_len=topic?int(std::string(topic).size()):0;e.data=bytes;e.data_len=size;e.total_data_len=total;e.current_data_offset=offset;callback(nullptr,nullptr,MQTT_EVENT_DATA,&e);}
int main(){
 MqttConfig cfg{};cfg.uri="mqtts://broker";cfg.command_topic="t/device/commands";cfg.binding_topic="t/device/bindings";cfg.sync_topic="t/device/sync";cfg.ack_topic="t/device/sync/acks";cfg.client_id="id";cfg.root_ca_pem="ca";cfg.client_certificate_pem="cert";cfg.client_key_pem="key";
 check(mqtt_runtime_start(cfg,command,nullptr,ack,nullptr,binding,nullptr)==ESP_OK,"start");callback(nullptr,nullptr,MQTT_EVENT_CONNECTED,nullptr);
 check(std::find(subscriptions.begin(),subscriptions.end(),"t/device/sync/acks/+")!=subscriptions.end(),"subscribe ACK batch wildcard");
 check(std::find(subscriptions.begin(),subscriptions.end(),cfg.command_topic)!=subscriptions.end(),"unchanged command subscription");
 data("t/device/sync/acks/batch-1","abc",3,3,0);check(acks_received==std::vector<std::string>{"abc"},"backend suffixed ACK dispatch");acks_received.clear();
 for(auto topic:{"t/device/sync/acks","t/device/sync/acks/","t/device/sync/acks/a/b","t/other/sync/acks/a","t/device/sync/acksx/a","t/device/sync/acks/+","t/device/sync/acks/#"})data(topic,"x",1,1,0);
 check(acks_received.empty(),"foreign bare nested wildcard ACK rejected");
 data("t/device/sync/acks/batch-2","ab",2,4,0);data(nullptr,"cd",2,4,2);check(acks_received==std::vector<std::string>{"abcd"},"fragmented ACK dispatch");acks_received.clear();
 // An invalid continuation must erase every assembly flag, including ACK/binding.
 for(int kind=0;kind<2;++kind){const char* topic=kind?cfg.binding_topic:"t/device/sync/acks/b";
 data(topic,"ab",2,4,0);data(nullptr,"x",1,4,3);data(nullptr,"safe",4,4,0);
 check(acks_received.empty()&&bindings_received.empty(),"invalid offset reset");
 data(topic,"ab",2,4,0);data(nullptr,"cd",2,5,2);check(acks_received.empty()&&bindings_received.empty(),"changed total rejected");
 data(topic,"ab",2,4,0);data("foreign/topic","cd",2,4,2);check(acks_received.empty()&&bindings_received.empty(),"foreign continuation rejected");
 data(topic,"ab",2,4,0);callback(nullptr,nullptr,MQTT_EVENT_DISCONNECTED,nullptr);data(nullptr,"cd",2,4,2);check(acks_received.empty()&&bindings_received.empty(),"disconnect resets assembly");
 }
 data("t/device/sync/acks/b","x",1,8193,0);check(acks_received.empty(),"oversize total rejected");
 data("t/device/sync/acks/b","x",1,0,0);check(acks_received.empty(),"empty total rejected");
 data("t/device/sync/acks/b","x",2,1,0);check(acks_received.empty(),"fragment exceeds total");
 data("t/device/sync/acks/b",nullptr,1,1,0);check(acks_received.empty(),"null data rejected");
 data("t/device/sync/acks/b","x",-1,1,0);check(acks_received.empty(),"negative data size rejected");
 data("t/device/sync/acks/b","x",0,1,0);check(acks_received.empty(),"zero progress rejected");
 std::string large(8192,'z');data("t/device/sync/acks/max",large.data(),8192,8192,0);check(acks_received==std::vector<std::string>{large},"max message accepted");acks_received.clear();
 // Exact routes continue to work after invalid messages.
 data(cfg.command_topic,"cmd",3,3,0);data(cfg.binding_topic,"bind",4,4,0);data("t/device/sync/acks/final","ok",2,2,0);
 check(commands_received==std::vector<std::string>{"cmd"}&&bindings_received==std::vector<std::string>{"bind"}&&acks_received==std::vector<std::string>{"ok"},"valid dispatch after faults");
 if(failures){std::cerr<<failures<<" failures\n";return 1;}std::cout<<"actual MQTT runtime host tests: OK\n";
}
