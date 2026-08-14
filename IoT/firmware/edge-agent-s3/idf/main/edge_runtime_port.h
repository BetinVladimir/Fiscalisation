#pragma once
#include <cstddef>
#include <cstdint>
#include "esp_err.h"

namespace beefiscal::idf {
enum class Ingress : uint8_t { Mqtt, Ble };
struct CommandView { const uint8_t* data; size_t size; Ingress ingress; };
using CommandSink = esp_err_t (*)(const CommandView&, void*);
using ProvisioningSink = esp_err_t (*)(const uint8_t*, size_t, void*);

esp_err_t crypto_self_test();
struct MqttConfig {
  const char* uri; const char* command_topic; const char* binding_topic;const char* sync_topic;const char* ack_topic;
  const char* client_id; const char* root_ca_pem;
  const char* client_certificate_pem; const char* client_key_pem;
  const char* applied_tenant;const char* applied_device;const char* applied_register;int64_t applied_generation;
};
using AckSink=esp_err_t(*)(const uint8_t*,size_t,void*);
esp_err_t mqtt_runtime_start(const MqttConfig&,CommandSink,void*,AckSink=nullptr,void* = nullptr,ProvisioningSink=nullptr,void* = nullptr);
int mqtt_publish_sync(const char*batch_id,const uint8_t*,size_t);
int mqtt_publish_binding_applied(const char*tenant,const char*device,const char*register_id,int64_t generation);
struct BleConfig { const char* advertising_name; };
esp_err_t ble_runtime_start(const BleConfig&, CommandSink, void*);
esp_err_t ble_publish_event(const char* message_uuid,const uint8_t*,size_t);
esp_err_t ble_provisioning_start(const BleConfig&, ProvisioningSink, void*);
struct UsbCdcIdentity { uint16_t vid; uint16_t pid; uint8_t interface_index; };
esp_err_t usb_cdc_runtime_start(const UsbCdcIdentity&);
bool usb_cdc_ready();
int usb_cdc_read(uint8_t*, size_t, uint32_t);
int usb_cdc_write(const uint8_t*, size_t, uint32_t);
}
