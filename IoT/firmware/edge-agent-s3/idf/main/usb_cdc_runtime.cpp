#include "edge_runtime_port.h"
#include "usb/usb_host.h"
#include "usb/cdc_acm_host.h"
#include "freertos/FreeRTOS.h"
#include "freertos/queue.h"
#include "freertos/task.h"
#include <atomic>

namespace beefiscal::idf {
namespace {
constexpr size_t kRxCapacity = 4096;
std::atomic_bool ready{false};
cdc_acm_dev_hdl_t device = nullptr;
QueueHandle_t rx_queue = nullptr;
UsbCdcIdentity identity{};

bool on_data(const uint8_t* data, size_t size, void*) {
  if (!rx_queue) return false;
  for (size_t i = 0; i < size; ++i)
    if (xQueueSend(rx_queue, &data[i], 0) != pdTRUE) return false;
  return true;
}
void on_event(const cdc_acm_host_dev_event_data_t* event, void*) {
  if (event && event->type == CDC_ACM_HOST_DEVICE_DISCONNECTED) {
    ready.store(false); device = nullptr;
  }
}
void usb_events(void*) {
  const usb_host_config_t cfg{.skip_phy_setup=false,
                              .root_port_unpowered=false,
                              .intr_flags=ESP_INTR_FLAG_LEVEL1,
                              .enum_filter_cb=nullptr};
  if (usb_host_install(&cfg) != ESP_OK) vTaskDelete(nullptr);
  if (cdc_acm_host_install(nullptr) != ESP_OK) vTaskDelete(nullptr);
  for (;;) { uint32_t flags=0; usb_host_lib_handle_events(portMAX_DELAY,&flags); }
}
void device_discovery(void*) {
  const cdc_acm_host_device_config_t cfg{.connection_timeout_ms=1000,
    .out_buffer_size=512,.in_buffer_size=512,.event_cb=on_event,
    .data_cb=on_data,.user_arg=nullptr};
  for (;;) {
    if (!device) {
      cdc_acm_dev_hdl_t found=nullptr;
      if (cdc_acm_host_open(identity.vid,identity.pid,identity.interface_index,
                            &cfg,&found)==ESP_OK) { device=found; ready.store(true); }
    }
    vTaskDelay(pdMS_TO_TICKS(250));
  }
}
}
esp_err_t usb_cdc_runtime_start(const UsbCdcIdentity& configured) {
  if (configured.vid==0 || configured.pid==0) return ESP_ERR_INVALID_STATE;
  identity=configured;
  rx_queue=xQueueCreate(kRxCapacity,sizeof(uint8_t));
  if (!rx_queue) return ESP_ERR_NO_MEM;
  if (xTaskCreatePinnedToCore(usb_events,"usb-host",4096,nullptr,12,nullptr,0)!=pdPASS)
    return ESP_ERR_NO_MEM;
  return xTaskCreate(device_discovery,"usb-cdc",4096,nullptr,9,nullptr)==pdPASS
    ? ESP_OK : ESP_ERR_NO_MEM;
}
bool usb_cdc_ready(){return ready.load();}
int usb_cdc_read(uint8_t*out,size_t capacity,uint32_t timeout_ms){
  if(!out||!capacity||!rx_queue)return-1;
  size_t count=0;
  if(xQueueReceive(rx_queue,&out[count],pdMS_TO_TICKS(timeout_ms))!=pdTRUE)return 0;
  ++count;while(count<capacity&&xQueueReceive(rx_queue,&out[count],0)==pdTRUE)++count;
  return static_cast<int>(count);
}
int usb_cdc_write(const uint8_t*data,size_t size,uint32_t timeout_ms){
  const auto active=device;if(!active||!data||!size)return-1;
  return cdc_acm_host_data_tx_blocking(active,data,size,timeout_ms)==ESP_OK
    ? static_cast<int>(size):-1;
}
}
