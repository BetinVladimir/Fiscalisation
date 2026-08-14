#include "edge_runtime_port.h"
#include "durable_storage.h"
#include "provisioned_binding.h"
#include "runtime_config.h"
#include "intent_processor.h"
#include "command_queue.h"
#include "canonical_cbor.h"
#include "cJSON.h"
#include "device_identity.h"
#include "sync_runtime.h"
#include "profile_executor.h"
#include "local_http_server.h"
#include "spa_deployment_manager.h"
#if __has_include("BindingTrustAnchor.h")
#include "BindingTrustAnchor.h"
#endif
#include "esp_log.h"
#include "esp_random.h"
#include "esp_system.h"
#include "esp_timer.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "nvs_flash.h"
#include <cstring>
#include <ctime>
#include <string>

namespace { constexpr char TAG[]="beefiscal-edge";
beefiscal::idf::DurableStorage storage;
#ifndef EDGE_BINDING_SIGNING_PUBLIC_KEY_PEM
#define EDGE_BINDING_SIGNING_PUBLIC_KEY_PEM ""
#endif
#ifndef EDGE_BINDING_SIGNING_KEY_ID
#define EDGE_BINDING_SIGNING_KEY_ID ""
#endif
beefiscal::idf::ProvisionedBindingStore binding_store(
  EDGE_BINDING_SIGNING_PUBLIC_KEY_PEM,EDGE_BINDING_SIGNING_KEY_ID);
beefiscal::idf::CompositeBinding binding;
beefiscal::idf::RuntimeSecrets secrets;
beefiscal::idf::DeviceIdentity device_identity;
beefiscal::idf::IntentProcessor*processor{};
beefiscal::idf::CommandQueue command_queue;
beefiscal::idf::DeviceIo*health_io{};
bool local_fiscal_ready(void*context){auto*io=static_cast<beefiscal::idf::DeviceIo*>(context);return io&&io->fiscal_ready();}
bool local_payment_ready(void*context){auto*io=static_cast<beefiscal::idf::DeviceIo*>(context);return io&&io->payment_ready();}
void health_task(void*) {
  uint64_t sequence=0;const uint32_t boot=esp_random();
  for(;;){
    if(health_io){
      const bool fiscal=health_io->fiscal_ready();
      const bool payment=!binding.payment.present||health_io->payment_ready();
      const char*state=fiscal&&payment?"READY":fiscal?"DEGRADED":"OFFLINE";
      char observed[32]{};const time_t now=time(nullptr);struct tm utc{};
      if(now>1700000000&&gmtime_r(&now,&utc))strftime(observed,sizeof(observed),"%Y-%m-%dT%H:%M:%SZ",&utc);
      else strcpy(observed,"1970-01-01T00:00:00Z");
      char body[2048];
      snprintf(body,sizeof(body),"{\"schema_version\":1,\"adapter_device_id\":\"%s\",\"register_id\":\"%s\",\"boot_id\":\"%08lx\",\"sequence\":%llu,\"binding_generation\":%lld,\"firmware_version\":\"mvp1\",\"adapter_state\":\"%s\",\"endpoints\":[{\"role\":\"ADAPTER\",\"configured\":true,\"reachable\":true,\"state\":\"READY\",\"driver_id\":\"edge-agent-s3\",\"protocol_version\":\"2026-08-14\"},{\"role\":\"FISCAL_DEVICE\",\"configured\":true,\"reachable\":%s,\"state\":\"%s\",\"vendor\":\"%s\",\"model\":\"%s\",\"driver_id\":\"%s\",\"protocol_version\":\"%s\"}%s],\"observed_at\":\"%s\"}",binding.edge_device_id.c_str(),binding.register_id.c_str(),(unsigned long)boot,(unsigned long long)++sequence,(long long)binding.generation,state,fiscal?"true":"false",fiscal?"READY":"OFFLINE",binding.fiscal.vendor.c_str(),binding.fiscal.model.c_str(),binding.profile==beefiscal::idf::EdgeProfile::DatecsDp150BluePad50?"datecs":"daisy",binding.profile==beefiscal::idf::EdgeProfile::DatecsDp150BluePad50?"2.11.4":"2.0-4",binding.payment.present?(payment?",{\"role\":\"PAYMENT_TERMINAL\",\"configured\":true,\"reachable\":true,\"state\":\"READY\",\"vendor\":\"DATECS\",\"model\":\"BLUEPAD-50 PLUS\",\"driver_id\":\"datecspay\",\"protocol_version\":\"1.9\"}":",{\"role\":\"PAYMENT_TERMINAL\",\"configured\":true,\"reachable\":false,\"state\":\"OFFLINE\",\"vendor\":\"DATECS\",\"model\":\"BLUEPAD-50 PLUS\",\"driver_id\":\"datecspay\",\"protocol_version\":\"1.9\"}"):"",observed);
      beefiscal::idf::mqtt_publish_status(binding.tenant_id.c_str(),binding.edge_device_id.c_str(),body,true);
    }
    vTaskDelay(pdMS_TO_TICKS(15000));
  }
}
void deployment_task(void*context){auto*manager=static_cast<beefiscal::idf::SpaDeploymentManager*>(context);for(;;){manager->check_and_activate();vTaskDelay(pdMS_TO_TICKS(21600000));}}
esp_err_t command(const beefiscal::idf::CommandView& view,void* context) {
  auto*value=static_cast<beefiscal::idf::IntentProcessor*>(context);
  return value?value->accept(view):ESP_ERR_INVALID_STATE;
}
esp_err_t result(const char*operation_id,const char*json,beefiscal::idf::Ingress ingress,void*){
  if(ingress!=beefiscal::idf::Ingress::Ble)return ESP_OK;
  cJSON*root=cJSON_Parse(json);if(!root)return ESP_ERR_INVALID_ARG;
  auto*state=cJSON_GetObjectItemCaseSensitive(root,"state");
  if(cJSON_IsString(state)){
    if(strcmp(state->valuestring,"COMMITTED")==0)cJSON_SetValuestring(state,"FISCALIZED");
    else if(strcmp(state->valuestring,"RECOVERY_REQUIRED")==0)cJSON_SetValuestring(state,"FISCAL_RESULT_UNKNOWN");
    else if(strcmp(state->valuestring,"COMPENSATED")==0)cJSON_SetValuestring(state,"REJECTED");
  }
  uint8_t*encoded=nullptr;size_t size=0;esp_err_t e=beefiscal::idf::canonical_json_to_cbor(root,&encoded,&size);cJSON_Delete(root);
  if(e==ESP_OK)e=beefiscal::idf::ble_publish_event(operation_id,encoded,size);
  free(encoded);
  return e;
}
esp_err_t provision(const uint8_t*data,size_t size,void*context){
  // BFPE v1: magic[4], version[1], key-id-len[1], signature-len BE[2],
  // canonical JSON length BE[4], followed by key-id, base64url signature, JSON.
  if(!data||size<12||memcmp(data,"BFPE",4)!=0||data[4]!=1)return ESP_ERR_INVALID_ARG;
  const size_t key_size=data[5],signature_size=(size_t(data[6])<<8)|data[7];
  const size_t json_size=(size_t(data[8])<<24)|(size_t(data[9])<<16)|
    (size_t(data[10])<<8)|data[11];
  if(!key_size||!signature_size||!json_size||12+key_size+signature_size+json_size!=size)
    return ESP_ERR_INVALID_SIZE;
  std::string key((const char*)data+12,key_size);
  std::string signature((const char*)data+12+key_size,signature_size);
  const char*json=(const char*)data+12+key_size+signature_size;
  auto*store=static_cast<beefiscal::idf::ProvisionedBindingStore*>(context);
  const beefiscal::idf::SignedBindingEnvelope envelope{json,json_size,
    signature.c_str(),key.c_str()};
  const esp_err_t installed=store?store->install(envelope):ESP_ERR_INVALID_STATE;
  if(installed==ESP_OK){ESP_LOGI(TAG,"signed binding installed; restarting");
    esp_restart();}
  return installed;
}}
extern "C" void app_main(void) {
  ESP_ERROR_CHECK(nvs_flash_init());
  ESP_ERROR_CHECK(beefiscal::idf::crypto_self_test());
#ifndef EDGE_SD_CLK
#define EDGE_SD_CLK -1
#define EDGE_SD_CMD -1
#define EDGE_SD_D0 -1
#endif
  constexpr beefiscal::idf::StorageConfig storage_config{
    "/sdcard","/beefiscal/edge-agent.db",EDGE_SD_CLK,EDGE_SD_CMD,EDGE_SD_D0,true};
  // No command transport is exposed if durable media cannot be opened.
  ESP_ERROR_CHECK(storage.open(storage_config));
  // Identity precedes provisioning: its public key/kid is part of the backend
  // activation request and the subsequently signed composite binding.
  if(device_identity.open()!=ESP_OK){
    ESP_LOGE(TAG,"persistent P-256 device identity unavailable");return;
  }
  // A signed, monotonically versioned binding is the authorization boundary.
  // Unprovisioned devices expose no fiscal command transport.
  if(binding_store.open()!=ESP_OK){
    ESP_LOGE(TAG,"binding trust configuration unavailable; all radio ingress disabled");return;
  }
  if(binding_store.load(binding)!=ESP_OK){
    ESP_LOGW(TAG,"unprovisioned: only signed binding installation is enabled");
    constexpr beefiscal::idf::BleConfig setup{"BeeFiscal-setup"};
    ESP_ERROR_CHECK(beefiscal::idf::ble_provisioning_start(setup,provision,&binding_store));return;
  }
  beefiscal::idf::RuntimeConfigLoader config;
  if(config.load_secrets(binding,secrets)!=ESP_OK){
    ESP_LOGE(TAG,"binding present but protected MQTT credentials unavailable");return;
  }
  if(device_identity.kid()!=binding.authority.transaction_signing_kid){
    ESP_LOGE(TAG,"device transaction identity does not match signed binding");return;
  }
  ESP_ERROR_CHECK(storage.configure_authority(binding.authority.unp_prefix.c_str(),
    binding.authority.unp_range_start,binding.authority.unp_range_end,binding.generation));
  static beefiscal::idf::EspIdfDeviceIo physical_io(binding);
  if(!physical_io.begin()){ESP_LOGE(TAG,"physical profile transport failed to start");return;}
  static beefiscal::idf::ProfileExecutor physical_executor(binding,physical_io,&storage);
  static beefiscal::idf::IntentProcessor runtime(storage,binding,physical_executor,
    secrets.command_hmac_key,result,nullptr);
  processor=&runtime;
  ESP_ERROR_CHECK(processor->recover_pending());
  ESP_ERROR_CHECK(command_queue.start(command,processor));
  if(binding.profile==beefiscal::idf::EdgeProfile::DaisyCompactS01){
    const beefiscal::idf::UsbCdcIdentity usb_binding{binding.fiscal.usb_vid,
      binding.fiscal.usb_pid,binding.fiscal.usb_interface};
    ESP_ERROR_CHECK(beefiscal::idf::usb_cdc_runtime_start(usb_binding));
  }
  const std::string binding_topic="tenants/"+binding.tenant_id+"/devices/"+binding.edge_device_id+"/bindings";
  const beefiscal::idf::MqttConfig mqtt_binding{binding.mqtt.uri.c_str(),
    binding.mqtt.command_topic.c_str(),binding_topic.c_str(),binding.mqtt.sync_topic.c_str(),binding.mqtt.ack_topic.c_str(),
    binding.mqtt.client_id.c_str(),secrets.root_ca_pem.c_str(),
    secrets.client_certificate_pem.c_str(),secrets.client_key_pem.c_str(),binding.tenant_id.c_str(),binding.edge_device_id.c_str(),binding.register_id.c_str(),binding.generation};
  const esp_err_t mqtt_status=beefiscal::idf::mqtt_runtime_start(mqtt_binding,
    beefiscal::idf::queued_command_sink,&command_queue,
    beefiscal::idf::sync_runtime_accept_ack,nullptr,provision,&binding_store);
  if(mqtt_status!=ESP_ERR_INVALID_STATE)ESP_ERROR_CHECK(mqtt_status);
  health_io=&physical_io;xTaskCreate(health_task,"device-health",6144,nullptr,4,nullptr);
  ESP_ERROR_CHECK(beefiscal::idf::sync_runtime_start(storage,binding,device_identity,
    secrets.sync_ack_hmac_key.c_str()));
  const beefiscal::idf::BleConfig ble_binding{binding.ble_advertising_identity.c_str()};
  ESP_ERROR_CHECK(beefiscal::idf::ble_runtime_start(ble_binding,
    beefiscal::idf::queued_command_sink,&command_queue));
  if(binding.local_http.enabled){static beefiscal::idf::SpaDeploymentManager deployments(binding);const beefiscal::idf::LocalHttpRuntime local{&binding,&storage,&command_queue,local_fiscal_ready,local_payment_ready,&physical_io,"/sdcard/beeloy/spa/slot-a",&deployments};ESP_ERROR_CHECK(beefiscal::idf::local_http_server_start(local));xTaskCreate(deployment_task,"spa-deployment",8192,&deployments,3,nullptr);}
  ESP_LOGI(TAG,"profile %s generation %lld initialized; execution remains journal-gated",
    beefiscal::idf::edge_profile_name(binding.profile),(long long)binding.generation);
}
