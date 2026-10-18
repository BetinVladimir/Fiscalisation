#include "fake_ble.h"
#include <cassert>
#include <cstring>
#include <iostream>
#include <string>
#include <thread>
#include <condition_variable>
#include "../main/ble_runtime.cpp"
using namespace beefiscal::idf;
ble_hs_cfg_t ble_hs_cfg{};
static std::mutex test_mutex;static std::condition_variable test_cv;static bool block_init,init_entered,release_init,fence_test,notify_started,reconnect_done;static int mbuf_calls;
static int hci_calls;static bool hci_initialized;
static int failure_stage,adv_size,scan_size,notify_count,sink_count;static bool sync_now=true,advertising;static uint16_t mtu=185;static std::vector<std::vector<uint8_t>> notifications;
int ble_hs_mbuf_to_flat(os_mbuf*m,void*p,size_t n,uint16_t*copied){std::memcpy(p,m->bytes.data(),n);*copied=uint16_t(n);return 0;}
os_mbuf*ble_hs_mbuf_from_flat(const void*p,size_t n){if(fence_test&&++mbuf_calls==2){std::unique_lock<std::mutex> lock(test_mutex);test_cv.wait(lock,[]{return reconnect_done;});}auto*m=new os_mbuf;auto*b=static_cast<const uint8_t*>(p);m->bytes.assign(b,b+n);return m;}
void os_mbuf_free_chain(os_mbuf*m){delete m;}
int ble_gatts_notify_custom(uint16_t,uint16_t,os_mbuf*m){notifications.push_back(m->bytes);delete m;notify_count++;if(fence_test){std::lock_guard<std::mutex> lock(test_mutex);notify_started=true;test_cv.notify_all();}return failure_stage==8?1:0;}
uint16_t ble_att_mtu(uint16_t){return mtu;}
int ble_gap_adv_set_fields(const ble_hs_adv_fields*f){adv_size=(f->flags?3:0)+(f->num_uuids128?18:0)+(f->name_len?f->name_len+2:0);return failure_stage==4||adv_size>31?1:0;}
int ble_gap_adv_rsp_set_fields(const ble_hs_adv_fields*f){scan_size=f->name_len?f->name_len+2:0;return failure_stage==5||scan_size>31?1:0;}
int ble_gap_adv_start(uint8_t,void*,int,const ble_gap_adv_params*,int(*)(ble_gap_event*,void*),void*){advertising=failure_stage!=6;return advertising?0:1;}
int ble_gap_adv_stop(){advertising=false;return 0;}
int ble_hs_util_ensure_addr(int){return failure_stage==7?1:0;}int ble_hs_id_infer_auto(int,uint8_t*t){*t=1;return 0;}
esp_err_t esp_nimble_hci_init(){hci_calls++;if(hci_initialized)return ESP_ERR_INVALID_STATE;if(failure_stage==1)return ESP_FAIL;hci_initialized=true;return ESP_OK;}esp_err_t nimble_port_init(){if(block_init){std::unique_lock<std::mutex> lock(test_mutex);init_entered=true;test_cv.notify_all();test_cv.wait(lock,[]{return release_init;});}const auto result=esp_nimble_hci_init();return result!=ESP_OK?result:failure_stage==2?ESP_FAIL:ESP_OK;}
void nimble_port_run(){}void nimble_port_freertos_deinit(){}void nimble_port_freertos_init(void(*)(void*)){if(sync_now)ble_hs_cfg.sync_cb();}
void ble_svc_gap_init(){}void ble_svc_gatt_init(){}int ble_svc_gap_device_name_set(const char*){return failure_stage==3?1:0;}int ble_gatts_count_cfg(const ble_gatt_svc_def*){return failure_stage==9?1:0;}int ble_gatts_add_svcs(const ble_gatt_svc_def*){return failure_stage==10?1:0;}
int mbedtls_sha256(const unsigned char*,size_t,unsigned char*out,int){std::memset(out,0,32);return 0;}
SemaphoreHandle_t xSemaphoreCreateBinary(){static bool semaphore;semaphore=false;return &semaphore;}int xSemaphoreGive(SemaphoreHandle_t s){*s=true;return pdTRUE;}int xSemaphoreTake(SemaphoreHandle_t s,int){return *s?pdTRUE:0;}
static int central_reset_count;
namespace beefiscal::idf{void bluepad_ble_on_host_sync(){}void bluepad_ble_on_host_reset(){++central_reset_count;}}
static esp_err_t sink_fn(const CommandView&,void*){sink_count++;return ESP_OK;}
static esp_err_t provision_fn(const uint8_t*,size_t,void*){sink_count++;return ESP_OK;}
static esp_err_t read_source(char*out,size_t*n,void*){assert(*n>=2);memcpy(out,"{}",2);*n=2;return ESP_OK;}
static void reset(){block_init=false;init_entered=false;release_init=false;fence_test=false;notify_started=false;reconnect_done=false;mbuf_calls=0;connection_generation=0;hci_calls=0;hci_initialized=false;failure_stage=0;sync_now=true;advertising=false;initialization_started=false;startup_timed_out=false;startup_result=ESP_ERR_INVALID_STATE;active_connection=BLE_HS_CONN_HANDLE_NONE;notify_count=0;sink_count=0;notifications.clear();frame_payload.clear();frame_total=0;provisioning_frames.reset();provisioning_info_source=nullptr;provisioning_status_source=nullptr;provisioning_read_context=nullptr;}
static void connect(){ble_gap_event e{};e.type=BLE_GAP_EVENT_CONNECT;e.connect.status=0;e.connect.conn_handle=1;gap(&e,nullptr);}
int main(){
 reset();assert(ble_runtime_start({"BeeFiscal-setup"},sink_fn,nullptr)==ESP_OK);assert(adv_size==21&&scan_size==17&&advertising&&hci_calls==1);
 assert(ble_hs_cfg.reset_cb);const auto prior_resets=central_reset_count;ble_hs_cfg.reset_cb(7);assert(central_reset_count==prior_resets+1);
 const auto original_sink=sink;assert(ble_runtime_start({"replacement"},sink_fn,reinterpret_cast<void*>(1))==ESP_ERR_INVALID_STATE);assert(sink==original_sink&&sink_context==nullptr);
 for(int stage:{1,2,3,4,5,6,7,9,10}){reset();failure_stage=stage;assert(ble_runtime_start({"BeeFiscal-setup"},sink_fn,nullptr)!=ESP_OK);assert(!advertising);}
 reset();sync_now=false;assert(ble_runtime_start({"BeeFiscal-setup"},sink_fn,nullptr)==ESP_ERR_TIMEOUT);ble_hs_cfg.sync_cb();assert(!advertising);assert(ble_publish_event("11111111-1111-4111-8111-111111111111",reinterpret_cast<const uint8_t*>("x"),1)==ESP_ERR_INVALID_STATE);
 reset();std::string ascii(29,'a');assert(ble_runtime_start({ascii.c_str()},sink_fn,nullptr)==ESP_OK&&scan_size==31);
 reset();ascii+='a';assert(ble_runtime_start({ascii.c_str()},sink_fn,nullptr)==ESP_ERR_INVALID_ARG);
 reset();std::string utf8;for(int i=0;i<15;i++)utf8+="Б";assert(utf8.size()==30&&ble_runtime_start({utf8.c_str()},sink_fn,nullptr)==ESP_ERR_INVALID_ARG);
 reset();ble_provisioning_sources(read_source,read_source,nullptr);assert(ble_runtime_start({"BeeFiscal"},sink_fn,nullptr)==ESP_OK);os_mbuf info;ble_gatt_access_ctxt reading{BLE_GATT_ACCESS_OP_READ_CHR,&info};assert(read_provisioning(1,0,&reading,nullptr)==0&&info.bytes.size()==2);reading.op=BLE_GATT_ACCESS_OP_WRITE_CHR;assert(read_provisioning(1,0,&reading,nullptr)==BLE_ATT_ERR_READ_NOT_PERMITTED);
 const uint8_t data[400]{};
 for(auto value:{uint16_t(185),uint16_t(247),uint16_t(517)}){reset();mtu=value;assert(ble_runtime_start({"BeeFiscal-setup"},sink_fn,nullptr)==ESP_OK);connect();assert(ble_publish_event("11111111-1111-4111-8111-111111111111",data,sizeof(data))==ESP_OK);size_t offset=0;for(auto&frame:notifications){assert(frame.size()<=std::min<size_t>(value-3,512));assert(std::memcmp(frame.data(),"BFF1",4)==0);assert(((size_t(frame[22])<<8)|frame[23])==offset);offset+=frame.size()-57;}assert(offset==400);assert(notifications.front().size()==(value==185?182:185));}
 reset();mtu=60;assert(ble_runtime_start({"BeeFiscal-setup"},sink_fn,nullptr)==ESP_OK);connect();assert(ble_publish_event("11111111-1111-4111-8111-111111111111",data,1)==ESP_ERR_INVALID_SIZE&&notify_count==0);
 os_mbuf packet;packet.bytes.assign(58,0);ble_gatt_access_ctxt access{BLE_GATT_ACCESS_OP_WRITE_CHR,&packet};assert(receive_command(1,0,&access,nullptr)==BLE_ATT_ERR_INVALID_ATTR_VALUE_LEN&&sink_count==0);
 reset();mtu=23;assert(ble_provisioning_start({"BeeFiscal-setup"},provision_fn,nullptr)==ESP_OK);connect();packet.bytes.assign(512,0);assert(receive_provisioning(1,0,&access,nullptr)!=0&&sink_count==0);packet.bytes.assign(513,0);assert(receive_provisioning(1,0,&access,nullptr)==BLE_ATT_ERR_INVALID_ATTR_VALUE_LEN&&sink_count==0);
 reset();mtu=185;assert(ble_runtime_start({"BeeFiscal-setup"},sink_fn,nullptr)==ESP_OK);
 packet.bytes.assign(513,0);assert(receive_command(1,0,&access,nullptr)==BLE_ATT_ERR_INVALID_ATTR_VALUE_LEN&&sink_count==0);
 packet.bytes.assign(512,0);std::memcpy(packet.bytes.data(),"BFF1",4);packet.bytes[20]=1;packet.bytes[21]=199;packet.bytes[24]=1;
 assert(receive_command(1,0,&access,nullptr)==0&&sink_count==1);
 reset();mtu=185;assert(ble_provisioning_start({"BeeFiscal-setup"},provision_fn,nullptr)==ESP_OK);connect();packet.bytes.assign(62,0);memcpy(packet.bytes.data(),"BFPF",4);packet.bytes[23]=1;packet.bytes[60]=1;assert(receive_provisioning(1,0,&access,nullptr)==0&&sink_count==1);
 reset();mtu=61;assert(ble_runtime_start({"BeeFiscal-setup"},sink_fn,nullptr)==ESP_OK);connect();assert(ble_publish_event("11111111-1111-4111-8111-111111111111",data,2)==ESP_OK&&notifications.size()==2&&notifications[0].size()==58);
 reset();mtu=185;assert(ble_runtime_start({"BeeFiscal-setup"},sink_fn,nullptr)==ESP_OK);connect();failure_stage=8;assert(ble_publish_event("11111111-1111-4111-8111-111111111111",data,400)==ESP_FAIL&&notify_count==1);
 frame_payload.assign(3,1);frame_total=10;ble_gap_event disconnected{};disconnected.type=BLE_GAP_EVENT_DISCONNECT;gap(&disconnected,nullptr);assert(frame_payload.empty()&&frame_total==0);
 // Deterministic concurrent start: first caller owns globals before init blocks.
 reset();block_init=true;esp_err_t first_result=ESP_FAIL;
 std::thread starter([&]{first_result=ble_runtime_start({"first-owner"},sink_fn,reinterpret_cast<void*>(1));});
 {std::unique_lock<std::mutex> lock(test_mutex);test_cv.wait(lock,[]{return init_entered;});}
 assert(ble_runtime_start({"second-owner"},sink_fn,reinterpret_cast<void*>(2))==ESP_ERR_INVALID_STATE);
 assert(std::string(name)=="first-owner"&&sink_context==reinterpret_cast<void*>(1));
 {std::lock_guard<std::mutex> lock(test_mutex);release_init=true;test_cv.notify_all();}starter.join();assert(first_result==ESP_OK&&hci_calls==1);
 // Same numerical handle after reconnect is a different generation/session.
 reset();mtu=185;assert(ble_runtime_start({"BeeFiscal-setup"},sink_fn,nullptr)==ESP_OK);connect();fence_test=true;
 std::thread reconnect([&]{
  {std::unique_lock<std::mutex> lock(test_mutex);test_cv.wait(lock,[]{return notify_started;});}
  ble_gap_event e{};e.type=BLE_GAP_EVENT_DISCONNECT;gap(&e,nullptr);connect();
  {std::lock_guard<std::mutex> lock(test_mutex);reconnect_done=true;test_cv.notify_all();}
 });
 assert(ble_publish_event("11111111-1111-4111-8111-111111111111",data,400)==ESP_ERR_INVALID_STATE);reconnect.join();assert(notify_count==1);
 std::cout<<"actual BLE runtime host tests: OK\n";
}
