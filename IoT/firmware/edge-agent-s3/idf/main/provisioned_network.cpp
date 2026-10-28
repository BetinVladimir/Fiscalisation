#include "provisioned_network.h"
#include "esp_event.h"
#include "esp_netif.h"
#include "esp_netif_sntp.h"
#include "esp_wifi.h"
#include "freertos/FreeRTOS.h"
#include "freertos/event_groups.h"
#include "mbedtls/platform_util.h"
#include <cstring>
namespace beefiscal::idf {namespace {
EventGroupHandle_t events;
 constexpr EventBits_t ip_ready=1,failed=2;
 unsigned retries;

void event(void*,esp_event_base_t base,int32_t id,void*){
 if(base==WIFI_EVENT&&id==WIFI_EVENT_STA_START){if(esp_wifi_connect()!=ESP_OK)xEventGroupSetBits(events,failed);
 }
 if(base==WIFI_EVENT&&id==WIFI_EVENT_STA_DISCONNECTED){xEventGroupClearBits(events,ip_ready);

 if(++retries<=3){if(esp_wifi_connect()!=ESP_OK)xEventGroupSetBits(events,failed);
 }else xEventGroupSetBits(events,failed);
 }
 if(base==IP_EVENT&&id==IP_EVENT_STA_GOT_IP){retries=0;
 xEventGroupSetBits(events,ip_ready);
 }
}
}
esp_err_t provisioned_network_start(const std::string&ssid,const std::string&password){
 if(ssid.empty()||ssid.size()>32||ssid.find('\0')!=std::string::npos||password.size()<8||password.size()>63||password.find('\0')!=std::string::npos)return ESP_ERR_INVALID_ARG;

 if(events)return ESP_ERR_INVALID_STATE;

 auto e=esp_netif_init();

 if(e!=ESP_OK)return e;
 e=esp_event_loop_create_default();

 if(e!=ESP_OK&&e!=ESP_ERR_INVALID_STATE)return e;

 if(!esp_netif_create_default_wifi_sta())return ESP_ERR_NO_MEM;

 events=xEventGroupCreate();

 if(!events)return ESP_ERR_NO_MEM;

 if((e=esp_event_handler_register(WIFI_EVENT,ESP_EVENT_ANY_ID,event,nullptr))!=ESP_OK||(e=esp_event_handler_register(IP_EVENT,IP_EVENT_STA_GOT_IP,event,nullptr))!=ESP_OK)return e;

 esp_sntp_config_t time_config=ESP_NETIF_SNTP_DEFAULT_CONFIG("pool.ntp.org");
 if((e=esp_netif_sntp_init(&time_config))!=ESP_OK)return e;
 wifi_init_config_t init=WIFI_INIT_CONFIG_DEFAULT();

 if((e=esp_wifi_init(&init))!=ESP_OK)return e;

 // Credentials are persisted only as device-encrypted provisioning ciphertext.
 if((e=esp_wifi_set_storage(WIFI_STORAGE_RAM))!=ESP_OK||(e=esp_wifi_set_mode(WIFI_MODE_STA))!=ESP_OK)return e;

 wifi_config_t config{};
 memcpy(config.sta.ssid,ssid.data(),ssid.size());
 memcpy(config.sta.password,password.data(),password.size());
 config.sta.threshold.authmode=WIFI_AUTH_WPA2_PSK;
 config.sta.pmf_cfg.capable=true;

 e=esp_wifi_set_config(WIFI_IF_STA,&config);

 mbedtls_platform_zeroize(&config,sizeof(config));

 if(e!=ESP_OK)return e;

 if((e=esp_wifi_start())!=ESP_OK)return e;

 const auto state=xEventGroupWaitBits(events,ip_ready|failed,pdFALSE,pdFALSE,pdMS_TO_TICKS(5000));

 if(!(state&ip_ready))return ESP_ERR_TIMEOUT;
 return esp_netif_sntp_sync_wait(pdMS_TO_TICKS(5000));

}
}
