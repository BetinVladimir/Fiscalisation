#include "edge_runtime_port.h"
#include "durable_storage.h"
#include "provisioned_binding.h"
#include "runtime_config.h"
#include "intent_processor.h"
#include "command_queue.h"
#include "canonical_cbor.h"
#include "cJSON.h"
#include "device_identity.h"
#include "encrypted_provisioning.h"
#include "provisioned_network.h"
#include "mbedtls/base64.h"
#include <algorithm>
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

beefiscal::idf::EncryptedProvisioning encrypted_provisioning(device_identity,binding_store,EDGE_BINDING_SIGNING_PUBLIC_KEY_PEM,EDGE_BINDING_SIGNING_KEY_ID);

beefiscal::idf::ProvisionedRuntime provisioned;

uint8_t setup_nonce[32]{};
 int64_t nonce_created{};
 bool setup_mode{};

std::string base64url(const uint8_t*p,size_t n){std::vector<uint8_t>out((n+2)/3*4+1);
 size_t z{};

 if(mbedtls_base64_encode(out.data(),out.size(),&z,p,n))return{};
 std::string value(reinterpret_cast<char*>(out.data()),z);
 for(char&c:value){if(c=='+')c='-';
 else if(c=='/')c='_';
 }while(!value.empty()&&value.back()=='=')value.pop_back();

 return value;
 }
esp_err_t copy_json(cJSON*root,char*out,size_t*size){if(!root||!out||!size){cJSON_Delete(root);

 return ESP_ERR_INVALID_ARG;
 }char*raw=cJSON_PrintUnformatted(root);
 cJSON_Delete(root);

 if(!raw)return ESP_ERR_NO_MEM;
 const size_t n=strlen(raw);

 if(n>*size){cJSON_free(raw);

 return ESP_ERR_INVALID_SIZE;
 }memcpy(out,raw,n);
 *size=n;
 cJSON_free(raw);

 return ESP_OK;
 }
esp_err_t provisioning_info(char*out,size_t*size,void*){
 const auto now=esp_timer_get_time();

 if(!nonce_created||now-nonce_created>300000000){esp_fill_random(setup_nonce,sizeof(setup_nonce));
 nonce_created=now;
 }
 auto*root=cJSON_CreateObject();
 cJSON_AddNumberToObject(root,"protocol_version",3);
 cJSON_AddStringToObject(root,"device_kid",device_identity.kid().c_str());
 cJSON_AddStringToObject(root,"setup_nonce",base64url(setup_nonce,sizeof(setup_nonce)).c_str());
 cJSON_AddNumberToObject(root,"generation",provisioned.binding.generation);
 cJSON_AddBoolToObject(root,"accepts_install",setup_mode);

 return copy_json(root,out,size);

}
esp_err_t provisioning_status(char*out,size_t*size,void*){
 beefiscal::idf::ProvisionedRuntime persisted;
 const auto loaded=encrypted_provisioning.load(persisted);

 if(loaded==ESP_OK){provisioned.binding=persisted.binding;
 provisioned.package_sha256=persisted.package_sha256;
 persisted.clear();
 }
 else if(loaded!=ESP_ERR_NVS_NOT_FOUND)return loaded;

 auto*root=cJSON_CreateObject();

 if(provisioned.binding.generation<1){cJSON_AddStringToObject(root,"state","UNPROVISIONED");

 return copy_json(root,out,size);
 }
 std::string signature;

 if(device_identity.sign_provisioning_install(provisioned.binding.payload_sha256,provisioned.binding.generation,provisioned.package_sha256,signature)!=ESP_OK){cJSON_Delete(root);

 return ESP_FAIL;
 }
 cJSON_AddStringToObject(root,"state","INSTALLED");
 cJSON_AddStringToObject(root,"device_kid",device_identity.kid().c_str());
 cJSON_AddStringToObject(root,"binding_sha256",provisioned.binding.payload_sha256.c_str());
 cJSON_AddStringToObject(root,"package_sha256",provisioned.package_sha256.c_str());
 cJSON_AddNumberToObject(root,"generation",provisioned.binding.generation);
 cJSON_AddStringToObject(root,"signature",signature.c_str());

 return copy_json(root,out,size);

}
void restart_after_ack(void*){vTaskDelay(pdMS_TO_TICKS(500));
 esp_restart();
 }

beefiscal::idf::IntentProcessor*processor{};

beefiscal::idf::CommandQueue command_queue;

beefiscal::idf::DeviceIo*health_io{};

bool local_fiscal_ready(void*context){auto*io=static_cast<beefiscal::idf::DeviceIo*>(context);

 return io&&io->fiscal_ready();
 }
bool local_payment_ready(void*context){auto*io=static_cast<beefiscal::idf::DeviceIo*>(context);

 return io&&io->payment_ready();
 }
void health_task(void*) {
  uint64_t sequence=0;
 const uint32_t boot=esp_random();

  for(;
 ;
 ){
    if(health_io){
      const bool fiscal=health_io->fiscal_ready();

      const bool payment=!binding.payment.present||health_io->payment_ready();

      const char*state=fiscal&&payment?"READY":fiscal?"DEGRADED":"OFFLINE";

      char observed[32]{};
 const time_t now=time(nullptr);
 struct tm utc{};

      if(now>1700000000&&gmtime_r(&now,&utc))strftime(observed,sizeof(observed),"%Y-%m-%dT%H:%M:%SZ",&utc);

      else strcpy(observed,"1970-01-01T00:00:00Z");

      char body[2048];

      snprintf(body,sizeof(body),"{\"schema_version\":1,\"adapter_device_id\":\"%s\",\"register_id\":\"%s\",\"boot_id\":\"%08lx\",\"sequence\":%llu,\"binding_generation\":%lld,\"firmware_version\":\"mvp1\",\"adapter_state\":\"%s\",\"endpoints\":[{\"role\":\"ADAPTER\",\"configured\":true,\"reachable\":true,\"state\":\"READY\",\"driver_id\":\"edge-agent-s3\",\"protocol_version\":\"2026-08-14\"},{\"role\":\"FISCAL_DEVICE\",\"configured\":true,\"reachable\":%s,\"state\":\"%s\",\"vendor\":\"%s\",\"model\":\"%s\",\"driver_id\":\"%s\",\"protocol_version\":\"%s\"}%s],\"observed_at\":\"%s\"}",binding.edge_device_id.c_str(),binding.register_id.c_str(),(unsigned long)boot,(unsigned long long)++sequence,(long long)binding.generation,state,fiscal?"true":"false",fiscal?"READY":"OFFLINE",binding.fiscal.vendor.c_str(),binding.fiscal.model.c_str(),binding.profile==beefiscal::idf::EdgeProfile::DatecsDp150BluePad50?"datecs":"daisy",binding.profile==beefiscal::idf::EdgeProfile::DatecsDp150BluePad50?"2.11.4":"2.0-4",binding.payment.present?(payment?",{\"role\":\"PAYMENT_TERMINAL\",\"configured\":true,\"reachable\":true,\"state\":\"READY\",\"vendor\":\"DATECS\",\"model\":\"BLUEPAD-50 PLUS\",\"driver_id\":\"datecspay\",\"protocol_version\":\"1.9\"}":",{\"role\":\"PAYMENT_TERMINAL\",\"configured\":true,\"reachable\":false,\"state\":\"OFFLINE\",\"vendor\":\"DATECS\",\"model\":\"BLUEPAD-50 PLUS\",\"driver_id\":\"datecspay\",\"protocol_version\":\"1.9\"}"):"",observed);

      beefiscal::idf::mqtt_publish_status(binding.tenant_id.c_str(),binding.edge_device_id.c_str(),body,true);

    }
    vTaskDelay(pdMS_TO_TICKS(15000));

  }
}
void deployment_task(void*context){auto*manager=static_cast<beefiscal::idf::SpaDeploymentManager*>(context);
 for(;
 ;
 ){manager->check_and_activate();
 vTaskDelay(pdMS_TO_TICKS(21600000));
 }}
esp_err_t command(const beefiscal::idf::CommandView& view,void* context) {
  auto*value=static_cast<beefiscal::idf::IntentProcessor*>(context);

  return value?value->accept(view):ESP_ERR_INVALID_STATE;

}
esp_err_t result(const char*operation_id,const char*json,beefiscal::idf::Ingress ingress,void*){
  if(ingress!=beefiscal::idf::Ingress::Ble)return ESP_OK;

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
 esp_err_t e=beefiscal::idf::canonical_json_to_cbor(root,&encoded,&size);
 cJSON_Delete(root);

  if(e==ESP_OK)e=beefiscal::idf::ble_publish_event(operation_id,encoded,size);

  free(encoded);

  return e;

}
esp_err_t provision(const uint8_t*data,size_t size,void*){
 if(!setup_mode||!data)return ESP_ERR_INVALID_STATE;

 if(size==36&&memcmp(data,"BFRB",4)==0){
   if(provisioned.binding.generation<1)return ESP_ERR_INVALID_STATE;

 static const char hex[]="0123456789abcdef";
 std::string hash;
 for(size_t i=4;
 i<36;
 i++){hash.push_back(hex[data[i]>>4]);
 hash.push_back(hex[data[i]&15]);
 }
   if(hash!=provisioned.binding.payload_sha256)return ESP_ERR_INVALID_ARG;

   return xTaskCreate(restart_after_ack,"provision-reboot",2048,nullptr,3,nullptr)==pdPASS?ESP_OK:ESP_FAIL;

 }
 if(!nonce_created||esp_timer_get_time()-nonce_created>300000000)return ESP_ERR_TIMEOUT;

 auto installed=encrypted_provisioning.install(data,size,setup_nonce,provisioned);

 if(installed==ESP_OK){nonce_created=0;
 memset(setup_nonce,0,sizeof(setup_nonce));
 ESP_LOGI(TAG,"encrypted runtime configuration durably installed");
 }
 return installed;

}
}
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
    ESP_LOGE(TAG,"persistent P-256 device identity unavailable");
 return;

  }
  // A signed, monotonically versioned binding is the authorization boundary.
  // Unprovisioned devices expose no fiscal command transport.
  if(binding_store.open()!=ESP_OK){
    ESP_LOGE(TAG,"binding trust configuration unavailable; all radio ingress disabled");
 return;

  }
  beefiscal::idf::ble_provisioning_sources(provisioning_info,provisioning_status,nullptr);

  const auto secure_loaded=encrypted_provisioning.load(provisioned);

  beefiscal::idf::RuntimeConfigLoader config;

  if(secure_loaded==ESP_OK){binding=provisioned.binding;
 secrets=std::move(provisioned.secrets);
 }
  else if(secure_loaded!=ESP_ERR_NVS_NOT_FOUND||binding_store.load(binding)!=ESP_OK||config.load_secrets(binding,secrets)!=ESP_OK){
    setup_mode=true;
 ESP_LOGW(TAG,"runtime configuration unavailable: authenticated encrypted setup only");

    constexpr beefiscal::idf::BleConfig setup{"BeeFiscal-setup"};

    ESP_ERROR_CHECK(beefiscal::idf::ble_provisioning_start(setup,provision,nullptr,provisioning_info,provisioning_status));
 return;

  }
  if(secure_loaded==ESP_OK){const auto network=beefiscal::idf::provisioned_network_start(provisioned.wifi_ssid,provisioned.wifi_password);

 if(network!=ESP_OK)ESP_LOGW(TAG,"WiFi station unavailable; BLE operation remains available");
 }
  if(device_identity.kid()!=binding.authority.transaction_signing_kid){
    ESP_LOGE(TAG,"device transaction identity does not match signed binding");
 return;

  }
  ESP_ERROR_CHECK(storage.configure_authority(binding.authority.unp_prefix.c_str(),
    binding.authority.unp_range_start,binding.authority.unp_range_end,binding.generation));

  static beefiscal::idf::EspIdfDeviceIo physical_io(binding,secrets.fiscal_operator_password);

  if(!physical_io.begin()){ESP_LOGE(TAG,"physical profile transport failed to start");
 return;
 }
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

  health_io=&physical_io;
 xTaskCreate(health_task,"device-health",6144,nullptr,4,nullptr);

  ESP_ERROR_CHECK(beefiscal::idf::sync_runtime_start(storage,binding,device_identity,
    secrets.sync_ack_hmac_key.c_str()));

  const beefiscal::idf::BleConfig ble_binding{binding.ble_advertising_identity.c_str()};

  ESP_ERROR_CHECK(beefiscal::idf::ble_runtime_start(ble_binding,
    beefiscal::idf::queued_command_sink,&command_queue));

  if(binding.local_http.enabled){static beefiscal::idf::SpaDeploymentManager deployments(binding);
 const beefiscal::idf::LocalHttpRuntime local{&binding,&storage,&command_queue,local_fiscal_ready,local_payment_ready,&physical_io,"/sdcard/beeloy/spa/slot-a",&deployments};
 ESP_ERROR_CHECK(beefiscal::idf::local_http_server_start(local));
 xTaskCreate(deployment_task,"spa-deployment",8192,&deployments,3,nullptr);
 }
  ESP_LOGI(TAG,"profile %s generation %lld initialized; execution remains journal-gated",
    beefiscal::idf::edge_profile_name(binding.profile),(long long)binding.generation);

}
