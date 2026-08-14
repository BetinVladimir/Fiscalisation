#include "edge_runtime_port.h"
#include "esp_nimble_hci.h"
#include "host/ble_hs.h"
#include "host/util/util.h"
#include "nimble/nimble_port.h"
#include "nimble/nimble_port_freertos.h"
#include "services/gap/ble_svc_gap.h"
#include "services/gatt/ble_svc_gatt.h"
#include "bluepad_ble_central.h"
#include <cstdlib>
#include <algorithm>
#include <cstring>
#include <vector>
#include "mbedtls/sha256.h"

namespace beefiscal::idf { namespace {
CommandSink sink;void* sink_context;const char* name;uint8_t own_addr_type;
ProvisioningSink provisioning_sink;void* provisioning_context;bool provisioning_only;
uint16_t event_value_handle;uint16_t active_connection=BLE_HS_CONN_HANDLE_NONE;
constexpr size_t kMaximumIntent=8192;
constexpr size_t kFrameHeader=57;std::vector<uint8_t>frame_payload;uint8_t frame_id[16]{};
uint8_t frame_digest[32]{};size_t frame_total{};
#define EDGE_UUID(suffix) BLE_UUID128_INIT(0x53,0x49,0x46,0x45,0x45,0x42,0x4f,0x9e,0x7a,0x4c,0x6d,0x7c,(suffix),0x00,0x6f,0x7b)
static const ble_uuid128_t service_uuid=EDGE_UUID(0x00);
static const ble_uuid128_t control_uuid=EDGE_UUID(0x01);
static const ble_uuid128_t command_uuid=EDGE_UUID(0x02);
static const ble_uuid128_t event_uuid=EDGE_UUID(0x03);
static const ble_uuid128_t flow_uuid=EDGE_UUID(0x04);

int receive_provisioning(uint16_t,uint16_t,ble_gatt_access_ctxt* access,void*){
  if(!provisioning_only||!provisioning_sink||access->op!=BLE_GATT_ACCESS_OP_WRITE_CHR)
    return BLE_ATT_ERR_WRITE_NOT_PERMITTED;
  const size_t size=OS_MBUF_PKTLEN(access->om);
  if(size==0||size>kMaximumIntent)return BLE_ATT_ERR_INVALID_ATTR_VALUE_LEN;
  auto*payload=static_cast<uint8_t*>(malloc(size));if(!payload)return BLE_ATT_ERR_INSUFFICIENT_RES;
  uint16_t copied=0;const int flattened=ble_hs_mbuf_to_flat(access->om,payload,size,&copied);
  const esp_err_t result=flattened==0&&copied==size
    ?provisioning_sink(payload,size,provisioning_context):ESP_ERR_INVALID_SIZE;
  free(payload);return result==ESP_OK?0:BLE_ATT_ERR_UNLIKELY;
}
int receive_command(uint16_t,uint16_t,ble_gatt_access_ctxt* access,void*){
  if(access->op!=BLE_GATT_ACCESS_OP_WRITE_CHR||!sink)return BLE_ATT_ERR_WRITE_NOT_PERMITTED;
  const size_t size=OS_MBUF_PKTLEN(access->om);
  if(size<=kFrameHeader||size>512)return BLE_ATT_ERR_INVALID_ATTR_VALUE_LEN;
  auto* payload=static_cast<uint8_t*>(malloc(size));
  if(!payload)return BLE_ATT_ERR_INSUFFICIENT_RES;
  uint16_t copied=0;const int flattened=ble_hs_mbuf_to_flat(access->om,payload,size,&copied);
  esp_err_t accepted=ESP_ERR_INVALID_SIZE;
  if(flattened==0&&copied==size&&memcmp(payload,"BFF1",4)==0){
    const size_t total=(size_t(payload[20])<<8)|payload[21];
    const size_t offset=(size_t(payload[22])<<8)|payload[23];const bool final=payload[24]==1;
    const size_t body=size-kFrameHeader;
    if(total&&total<=kMaximumIntent&&offset+body<=total){
      if(offset==0){frame_payload.clear();frame_payload.reserve(total);frame_total=total;
        memcpy(frame_id,payload+4,16);memcpy(frame_digest,payload+25,32);}
      if(frame_total==total&&offset==frame_payload.size()&&memcmp(frame_id,payload+4,16)==0&&
         memcmp(frame_digest,payload+25,32)==0){frame_payload.insert(frame_payload.end(),payload+kFrameHeader,payload+size);
        accepted=ESP_OK;if(final&&frame_payload.size()==frame_total){uint8_t digest[32]{};
          if(mbedtls_sha256(frame_payload.data(),frame_payload.size(),digest,0)==0&&memcmp(digest,frame_digest,32)==0)
            accepted=sink({frame_payload.data(),frame_payload.size(),Ingress::Ble},sink_context);
          else accepted=ESP_ERR_INVALID_CRC;
          frame_payload.clear();frame_total=0;}}
    }
  }
  free(payload);
  return accepted==ESP_OK?0:BLE_ATT_ERR_UNLIKELY;
}
int receive_flow(uint16_t,uint16_t,ble_gatt_access_ctxt*,void*){return 0;}
const ble_gatt_chr_def characteristics[]={
  {.uuid=&control_uuid.u,.access_cb=receive_provisioning,.arg=nullptr,.descriptors=nullptr,.flags=BLE_GATT_CHR_F_WRITE,.min_key_size=0,.val_handle=nullptr,.cpfd=nullptr},
  {.uuid=&command_uuid.u,.access_cb=receive_command,.arg=nullptr,.descriptors=nullptr,.flags=BLE_GATT_CHR_F_WRITE|BLE_GATT_CHR_F_WRITE_NO_RSP,.min_key_size=0,.val_handle=nullptr,.cpfd=nullptr},
  {.uuid=&event_uuid.u,.access_cb=nullptr,.arg=nullptr,.descriptors=nullptr,.flags=BLE_GATT_CHR_F_NOTIFY,.min_key_size=0,.val_handle=&event_value_handle,.cpfd=nullptr},
  {.uuid=&flow_uuid.u,.access_cb=receive_flow,.arg=nullptr,.descriptors=nullptr,.flags=BLE_GATT_CHR_F_WRITE,.min_key_size=0,.val_handle=nullptr,.cpfd=nullptr},
  {}};
const ble_gatt_svc_def services[]={
  {.type=BLE_GATT_SVC_TYPE_PRIMARY,.uuid=&service_uuid.u,.includes=nullptr,.characteristics=characteristics},
  {}};
void advertise();
int gap(ble_gap_event* event,void*){
  switch(event->type){
    case BLE_GAP_EVENT_CONNECT:
      if(event->connect.status==0)active_connection=event->connect.conn_handle;
      else advertise();
      break;
    case BLE_GAP_EVENT_DISCONNECT:
      active_connection=BLE_HS_CONN_HANDLE_NONE;advertise();break;
    default:break;
  }
  return 0;
}
void advertise(){
  ble_hs_adv_fields fields{};fields.flags=BLE_HS_ADV_F_DISC_GEN|BLE_HS_ADV_F_BREDR_UNSUP;
  fields.name=reinterpret_cast<uint8_t*>(const_cast<char*>(name));fields.name_len=strlen(name);fields.name_is_complete=1;
  fields.uuids128=const_cast<ble_uuid128_t*>(&service_uuid);fields.num_uuids128=1;fields.uuids128_is_complete=1;
  if(ble_gap_adv_set_fields(&fields)!=0)return;
  ble_gap_adv_params parameters{};parameters.conn_mode=BLE_GAP_CONN_MODE_UND;parameters.disc_mode=BLE_GAP_DISC_MODE_GEN;
  ble_gap_adv_start(own_addr_type,nullptr,BLE_HS_FOREVER,&parameters,gap,nullptr);
}
void synchronized(){if(ble_hs_util_ensure_addr(0)==0&&ble_hs_id_infer_auto(0,&own_addr_type)==0){advertise();bluepad_ble_on_host_sync();}}
void host(void*){nimble_port_run();nimble_port_freertos_deinit();}
}}

namespace beefiscal::idf {
esp_err_t ble_runtime_start(const BleConfig& config,CommandSink value,void* context){
  if(!value||!config.advertising_name||!*config.advertising_name)return ESP_ERR_INVALID_ARG;
  provisioning_only=false;sink=value;sink_context=context;name=config.advertising_name;
  ESP_ERROR_CHECK(esp_nimble_hci_init());nimble_port_init();
  ble_svc_gap_init();ble_svc_gatt_init();
  if(ble_svc_gap_device_name_set(name)!=0||ble_gatts_count_cfg(services)!=0||ble_gatts_add_svcs(services)!=0)return ESP_FAIL;
  ble_hs_cfg.sync_cb=synchronized;nimble_port_freertos_init(host);return ESP_OK;
}
esp_err_t ble_provisioning_start(const BleConfig&config,ProvisioningSink value,void*context){
  if(!value||!config.advertising_name||!*config.advertising_name)return ESP_ERR_INVALID_ARG;
  provisioning_only=true;provisioning_sink=value;provisioning_context=context;
  sink=nullptr;sink_context=nullptr;name=config.advertising_name;
  ESP_ERROR_CHECK(esp_nimble_hci_init());nimble_port_init();ble_svc_gap_init();ble_svc_gatt_init();
  if(ble_svc_gap_device_name_set(name)!=0||ble_gatts_count_cfg(services)!=0||
     ble_gatts_add_svcs(services)!=0)return ESP_FAIL;
  ble_hs_cfg.sync_cb=synchronized;nimble_port_freertos_init(host);return ESP_OK;
}
esp_err_t ble_publish_event(const char*uuid,const uint8_t* data,size_t size){
  if(!uuid||strlen(uuid)!=36||!data||size==0||size>kMaximumIntent)return ESP_ERR_INVALID_ARG;
  if(active_connection==BLE_HS_CONN_HANDLE_NONE)return ESP_ERR_INVALID_STATE;
  uint8_t id[16]{},digest[32]{};size_t p=0;for(size_t i=0;i<36;i++){if(uuid[i]=='-')continue;char pair[3]={uuid[i],uuid[++i],0};char*end=nullptr;unsigned long v=strtoul(pair,&end,16);if(!end||*end||p>=16)return ESP_ERR_INVALID_ARG;id[p++]=v;}if(p!=16||mbedtls_sha256(data,size,digest,0)!=0)return ESP_FAIL;
  constexpr size_t body_capacity=128;
  for(size_t offset=0;offset<size;offset+=body_capacity){size_t body=std::min(body_capacity,size-offset);std::vector<uint8_t>frame(kFrameHeader+body);memcpy(frame.data(),"BFF1",4);memcpy(frame.data()+4,id,16);frame[20]=size>>8;frame[21]=size;frame[22]=offset>>8;frame[23]=offset;frame[24]=offset+body==size;memcpy(frame.data()+25,digest,32);memcpy(frame.data()+kFrameHeader,data+offset,body);os_mbuf*packet=ble_hs_mbuf_from_flat(frame.data(),frame.size());if(!packet)return ESP_ERR_NO_MEM;if(ble_gatts_notify_custom(active_connection,event_value_handle,packet)!=0)return ESP_FAIL;}
  return ESP_OK;
}
}
