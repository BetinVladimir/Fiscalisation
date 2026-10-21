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
static std::mutex test_mutex;static std::condition_variable test_cv;static bool block_init,init_entered,release_init,fence_test,notify_started,reconnect_done;static int mbuf_calls;static bool second_mbuf_entered;static bool block_flatten,flatten_entered,release_flatten;
static void*advertisement_arg;static void*connection_arg;static int(*advertisement_callback)(ble_gap_event*,void*);
static int allocated_mbufs,freed_mbufs;
static int hci_calls;static bool hci_initialized;
static int failure_stage,adv_size,scan_size,notify_count,sink_count;static bool sync_now=true,advertising;static uint16_t mtu=185;static std::vector<std::vector<uint8_t>> notifications;
int ble_hs_mbuf_to_flat(os_mbuf*m,void*p,size_t n,uint16_t*copied){if(block_flatten){std::unique_lock<std::mutex> lock(test_mutex);flatten_entered=true;test_cv.notify_all();test_cv.wait(lock,[]{return release_flatten;});}std::memcpy(p,m->bytes.data(),n);*copied=uint16_t(n);return 0;}
os_mbuf*ble_hs_mbuf_from_flat(const void*p,size_t n){if(fence_test&&++mbuf_calls==2){std::unique_lock<std::mutex> lock(test_mutex);second_mbuf_entered=true;test_cv.notify_all();test_cv.wait(lock,[]{return reconnect_done;});}++allocated_mbufs;auto*m=new os_mbuf;auto*b=static_cast<const uint8_t*>(p);m->bytes.assign(b,b+n);return m;}
void os_mbuf_free_chain(os_mbuf*m){++freed_mbufs;delete m;}
int ble_gatts_notify_custom(uint16_t,uint16_t,os_mbuf*m){notifications.push_back(m->bytes);++freed_mbufs;delete m;notify_count++;if(fence_test){std::lock_guard<std::mutex> lock(test_mutex);notify_started=true;test_cv.notify_all();}return failure_stage==8?1:0;}
uint16_t ble_att_mtu(uint16_t){return mtu;}
int ble_gap_adv_set_fields(const ble_hs_adv_fields*f){adv_size=(f->flags?3:0)+(f->num_uuids128?18:0)+(f->name_len?f->name_len+2:0);return failure_stage==4||adv_size>31?1:0;}
int ble_gap_adv_rsp_set_fields(const ble_hs_adv_fields*f){scan_size=f->name_len?f->name_len+2:0;return failure_stage==5||scan_size>31?1:0;}
int ble_gap_adv_start(uint8_t,void*,int,const ble_gap_adv_params*,int(*callback)(ble_gap_event*,void*),void*arg){advertisement_callback=callback;advertisement_arg=arg;advertising=failure_stage!=6;return advertising?0:1;}
int ble_gap_adv_stop(){advertising=false;return 0;}
int ble_hs_util_ensure_addr(int){return failure_stage==7?1:0;}int ble_hs_id_infer_auto(int,uint8_t*t){*t=1;return 0;}
esp_err_t esp_nimble_hci_init(){hci_calls++;if(hci_initialized)return ESP_ERR_INVALID_STATE;if(failure_stage==1)return ESP_FAIL;hci_initialized=true;return ESP_OK;}esp_err_t nimble_port_init(){if(block_init){std::unique_lock<std::mutex> lock(test_mutex);init_entered=true;test_cv.notify_all();test_cv.wait(lock,[]{return release_init;});}const auto result=esp_nimble_hci_init();return result!=ESP_OK?result:failure_stage==2?ESP_FAIL:ESP_OK;}
void nimble_port_run(){}void nimble_port_freertos_deinit(){}void nimble_port_freertos_init(void(*)(void*)){if(sync_now)ble_hs_cfg.sync_cb();}
void ble_svc_gap_init(){}void ble_svc_gatt_init(){}int ble_svc_gap_device_name_set(const char*){return failure_stage==3?1:0;}int ble_gatts_count_cfg(const ble_gatt_svc_def*){return failure_stage==9?1:0;}int ble_gatts_add_svcs(const ble_gatt_svc_def*){return failure_stage==10?1:0;}
int mbedtls_sha256(const unsigned char*,size_t,unsigned char*out,int){std::memset(out,0,32);return 0;}
SemaphoreHandle_t xSemaphoreCreateBinary(){static bool semaphore;semaphore=false;return &semaphore;}int xSemaphoreGive(SemaphoreHandle_t s){*s=true;return pdTRUE;}int xSemaphoreTake(SemaphoreHandle_t s,int){return *s?pdTRUE:0;}
static int central_reset_count;
namespace beefiscal::idf{void bluepad_ble_on_host_sync(){}void bluepad_ble_on_host_reset(){
 bool unlocked=false,retired=false;
 std::thread observer([&]{unlocked=connection_mutex.try_lock();if(unlocked){
   retired=active_connection==BLE_HS_CONN_HANDLE_NONE&&startup_result!=ESP_OK&&frame_payload.empty()&&frame_total==0;
   connection_mutex.unlock();}});observer.join();
 assert(unlocked&&retired);++central_reset_count;
}}
static esp_err_t sink_fn(const CommandView&,void*){sink_count++;return ESP_OK;}
static esp_err_t provision_fn(const uint8_t*,size_t,void*){sink_count++;return ESP_OK;}
static esp_err_t read_source(char*out,size_t*n,void*){assert(*n>=2);memcpy(out,"{}",2);*n=2;return ESP_OK;}
static void reset(){second_mbuf_entered=false;block_flatten=false;flatten_entered=false;release_flatten=false;allocated_mbufs=freed_mbufs=0;advertisement_arg=connection_arg=nullptr;advertisement_callback=nullptr;block_init=false;init_entered=false;release_init=false;fence_test=false;notify_started=false;reconnect_done=false;mbuf_calls=0;connection_generation=0;hci_calls=0;hci_initialized=false;failure_stage=0;sync_now=true;advertising=false;initialization_started=false;startup_timed_out=false;startup_result=ESP_ERR_INVALID_STATE;active_connection=BLE_HS_CONN_HANDLE_NONE;notify_count=0;sink_count=0;notifications.clear();frame_payload.clear();frame_total=0;provisioning_frames.reset();provisioning_info_source=nullptr;provisioning_status_source=nullptr;provisioning_read_context=nullptr;}
static void connect(){assert(advertisement_callback);connection_arg=advertisement_arg;ble_gap_event e{};e.type=BLE_GAP_EVENT_CONNECT;e.connect.status=0;e.connect.conn_handle=1;advertisement_callback(&e,connection_arg);}
static void disconnect(uint16_t handle=1){ble_gap_event e{};e.type=BLE_GAP_EVENT_DISCONNECT;e.disconnect.conn.conn_handle=handle;gap(&e,connection_arg);}
int main(int argc,char**argv){
 reset();assert(ble_runtime_start({"BeeFiscal-setup"},sink_fn,nullptr)==ESP_OK);assert(adv_size==21&&scan_size==17&&advertising&&hci_calls==1);
 assert(ble_hs_cfg.reset_cb);const auto prior_resets=central_reset_count;ble_hs_cfg.reset_cb(7);assert(central_reset_count==prior_resets+1);
 const auto original_sink=sink;assert(ble_runtime_start({"replacement"},sink_fn,reinterpret_cast<void*>(1))==ESP_ERR_INVALID_STATE);assert(sink==original_sink&&sink_context==nullptr);
 for(int stage:{1,2,3,4,5,6,7,9,10}){reset();failure_stage=stage;assert(ble_runtime_start({"BeeFiscal-setup"},sink_fn,nullptr)!=ESP_OK);assert(!advertising);}
 reset();sync_now=false;assert(ble_runtime_start({"BeeFiscal-setup"},sink_fn,nullptr)==ESP_ERR_TIMEOUT);ble_hs_cfg.sync_cb();assert(!advertising);assert(ble_publish_event({active_connection.load(),connection_generation.load()},"11111111-1111-4111-8111-111111111111",reinterpret_cast<const uint8_t*>("x"),1)==ESP_ERR_INVALID_STATE);
 reset();std::string ascii(29,'a');assert(ble_runtime_start({ascii.c_str()},sink_fn,nullptr)==ESP_OK&&scan_size==31);
 reset();ascii+='a';assert(ble_runtime_start({ascii.c_str()},sink_fn,nullptr)==ESP_ERR_INVALID_ARG);
 reset();std::string utf8;for(int i=0;i<15;i++)utf8+="Б";assert(utf8.size()==30&&ble_runtime_start({utf8.c_str()},sink_fn,nullptr)==ESP_ERR_INVALID_ARG);
 reset();ble_provisioning_sources(read_source,read_source,nullptr);assert(ble_runtime_start({"BeeFiscal"},sink_fn,nullptr)==ESP_OK);os_mbuf info;ble_gatt_access_ctxt reading{BLE_GATT_ACCESS_OP_READ_CHR,&info};connect();assert(read_provisioning(1,0,&reading,nullptr)==0&&info.bytes.size()==2);reading.op=BLE_GATT_ACCESS_OP_WRITE_CHR;assert(read_provisioning(1,0,&reading,nullptr)==BLE_ATT_ERR_READ_NOT_PERMITTED);
 const uint8_t data[400]{};
 for(auto value:{uint16_t(185),uint16_t(247),uint16_t(517)}){reset();mtu=value;assert(ble_runtime_start({"BeeFiscal-setup"},sink_fn,nullptr)==ESP_OK);connect();assert(ble_publish_event({active_connection.load(),connection_generation.load()},"11111111-1111-4111-8111-111111111111",data,sizeof(data))==ESP_OK);size_t offset=0;for(auto&frame:notifications){assert(frame.size()<=std::min<size_t>(value-3,512));assert(std::memcmp(frame.data(),"BFF1",4)==0);assert(((size_t(frame[22])<<8)|frame[23])==offset);offset+=frame.size()-57;}assert(offset==400);assert(notifications.front().size()==(value==185?182:185));}
 reset();mtu=60;assert(ble_runtime_start({"BeeFiscal-setup"},sink_fn,nullptr)==ESP_OK);connect();assert(ble_publish_event({active_connection.load(),connection_generation.load()},"11111111-1111-4111-8111-111111111111",data,1)==ESP_ERR_INVALID_SIZE&&notify_count==0);
 os_mbuf packet;packet.bytes.assign(58,0);ble_gatt_access_ctxt access{BLE_GATT_ACCESS_OP_WRITE_CHR,&packet};assert(receive_command(1,0,&access,nullptr)==BLE_ATT_ERR_INVALID_ATTR_VALUE_LEN&&sink_count==0);
 reset();mtu=23;assert(ble_provisioning_start({"BeeFiscal-setup"},provision_fn,nullptr)==ESP_OK);connect();packet.bytes.assign(512,0);assert(receive_provisioning(1,0,&access,nullptr)!=0&&sink_count==0);packet.bytes.assign(513,0);assert(receive_provisioning(1,0,&access,nullptr)==BLE_ATT_ERR_INVALID_ATTR_VALUE_LEN&&sink_count==0);
 reset();mtu=185;assert(ble_runtime_start({"BeeFiscal-setup"},sink_fn,nullptr)==ESP_OK);connect();
 packet.bytes.assign(513,0);assert(receive_command(1,0,&access,nullptr)==BLE_ATT_ERR_INVALID_ATTR_VALUE_LEN&&sink_count==0);
 packet.bytes.assign(512,0);std::memcpy(packet.bytes.data(),"BFF1",4);packet.bytes[20]=1;packet.bytes[21]=199;packet.bytes[24]=1;
 disconnect();
 assert(receive_command(1,0,&access,nullptr)!=0&&sink_count==0); // no active peripheral connection
 connect();
 assert(receive_command(2,0,&access,nullptr)!=0&&sink_count==0); // central/foreign handle
 assert(receive_command(1,0,&access,nullptr)==0&&sink_count==1);
 disconnect();
 assert(receive_command(1,0,&access,nullptr)!=0&&sink_count==1); // disconnected handle
 connect();assert(receive_command(1,0,&access,nullptr)==0&&sink_count==2);
 reset();mtu=185;assert(ble_provisioning_start({"BeeFiscal-setup"},provision_fn,nullptr)==ESP_OK);connect();packet.bytes.assign(62,0);memcpy(packet.bytes.data(),"BFPF",4);packet.bytes[23]=1;packet.bytes[60]=1;assert(receive_provisioning(1,0,&access,nullptr)==0&&sink_count==1);
 reset();mtu=61;assert(ble_runtime_start({"BeeFiscal-setup"},sink_fn,nullptr)==ESP_OK);connect();assert(ble_publish_event({active_connection.load(),connection_generation.load()},"11111111-1111-4111-8111-111111111111",data,2)==ESP_OK&&notifications.size()==2&&notifications[0].size()==58);
 reset();mtu=185;assert(ble_runtime_start({"BeeFiscal-setup"},sink_fn,nullptr)==ESP_OK);connect();failure_stage=8;assert(ble_publish_event({active_connection.load(),connection_generation.load()},"11111111-1111-4111-8111-111111111111",data,400)==ESP_FAIL&&notify_count==1);
 frame_payload.assign(3,1);frame_total=10;disconnect();assert(frame_payload.empty()&&frame_total==0);
 // Final marker must exactly match the advertised payload boundary.
 reset();mtu=185;assert(ble_runtime_start({"BeeFiscal-setup"},sink_fn,nullptr)==ESP_OK);connect();
 packet.bytes.assign(58,0);memcpy(packet.bytes.data(),"BFF1",4);packet.bytes[21]=2;packet.bytes[24]=1;
 assert(receive_command(1,0,&access,nullptr)!=0&&sink_count==0); // premature final
 packet.bytes[21]=1;packet.bytes[24]=0;
 assert(receive_command(1,0,&access,nullptr)!=0&&sink_count==0); // missing final
 packet.bytes[24]=2;
 assert(receive_command(1,0,&access,nullptr)!=0&&sink_count==0); // invalid marker
 packet.bytes[21]=2;packet.bytes[24]=0;
 assert(receive_command(1,0,&access,nullptr)==0&&frame_payload.size()==1);
 packet.bytes[23]=1;packet.bytes[4]=1;packet.bytes[24]=1;
 assert(receive_command(1,0,&access,nullptr)!=0&&frame_payload.empty()&&frame_total==0); // corrupt continuation discards assembly
 packet.bytes[4]=0;
 assert(receive_command(1,0,&access,nullptr)!=0&&sink_count==0); // cannot resume rejected assembly
 packet.bytes[23]=0;packet.bytes[21]=1;
 assert(receive_command(1,0,&access,nullptr)==0&&sink_count==1); // fresh command recovers
 // Invalid ATT frame length must also invalidate an existing assembly.
 packet.bytes.assign(58,0);memcpy(packet.bytes.data(),"BFF1",4);packet.bytes[21]=2;
 assert(receive_command(1,0,&access,nullptr)==0&&frame_payload.size()==1);
 auto continuation=packet;continuation.bytes[23]=1;continuation.bytes[24]=1;
 packet.bytes.resize(57);
 assert(receive_command(1,0,&access,nullptr)==BLE_ATT_ERR_INVALID_ATTR_VALUE_LEN&&frame_payload.empty());
 packet=continuation;
 assert(receive_command(1,0,&access,nullptr)!=0&&sink_count==1);
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
  {std::unique_lock<std::mutex> lock(test_mutex);test_cv.wait(lock,[]{return second_mbuf_entered;});}
  disconnect();connect();
  {std::lock_guard<std::mutex> lock(test_mutex);reconnect_done=true;test_cv.notify_all();}
 });
 assert(ble_publish_event({active_connection.load(),connection_generation.load()},"11111111-1111-4111-8111-111111111111",data,400)==ESP_ERR_INVALID_STATE);reconnect.join();assert(notify_count==1);
 // Required reply token and canonical wire operation UUID fail before allocation.
 reset();assert(ble_runtime_start({"reply-owner"},sink_fn,nullptr)==ESP_OK);connect();
 const BleReplyOwner reply_owner{active_connection.load(),connection_generation.load()};
 assert(ble_publish_event({},"11111111-1111-4111-8111-111111111111",data,1)==ESP_ERR_INVALID_STATE&&allocated_mbufs==0);
 for(size_t index:{size_t(0),size_t(8),size_t(13),size_t(18),size_t(23)}){
   std::string malformed="11111111-1111-4111-8111-111111111111";malformed[index]=index==0?'+':'A';
   assert(ble_publish_event(reply_owner,malformed.c_str(),data,1)==ESP_ERR_INVALID_ARG&&allocated_mbufs==0);
 }
 std::string spaced=" 1111111-1111-4111-8111-111111111111";
 assert(ble_publish_event(reply_owner,spaced.c_str(),data,1)==ESP_ERR_INVALID_ARG&&allocated_mbufs==0);
 // Reset retires peripheral state before exact BluePad fanout. No disconnect
 // event is required: NimBLE may reset its host without delivering one.
 reset();mtu=185;assert(ble_runtime_start({"reset-owner"},sink_fn,nullptr)==ESP_OK);connect();
 packet.bytes.assign(58,0);memcpy(packet.bytes.data(),"BFF1",4);packet.bytes[21]=2;
 assert(receive_command(1,0,&access,nullptr)==0&&frame_payload.size()==1);
 const auto reset_generation=connection_generation.load();const auto reset_count=central_reset_count;
 const auto old_arg=connection_arg;
 ble_hs_cfg.reset_cb(91);
 assert(active_connection==BLE_HS_CONN_HANDLE_NONE); // RED against original runtime
 assert(startup_result!=ESP_OK&&connection_generation>reset_generation&&frame_payload.empty()&&frame_total==0);
 assert(central_reset_count==reset_count+1);
 packet.bytes[23]=1;packet.bytes[24]=1;
 assert(receive_command(1,0,&access,nullptr)!=0&&sink_count==0);
 assert(receive_flow(1,0,nullptr,nullptr)!=0);
 assert(ble_publish_event({active_connection.load(),connection_generation.load()},"11111111-1111-4111-8111-111111111111",data,sizeof(data))==ESP_ERR_INVALID_STATE&&notify_count==0);
 ble_hs_cfg.sync_cb();assert(startup_result==ESP_OK);connect();
 assert(receive_command(1,0,&access,nullptr)!=0&&sink_count==0); // cannot finish old assembly
 ble_gap_event old_disconnect{};old_disconnect.type=BLE_GAP_EVENT_DISCONNECT;old_disconnect.disconnect.conn.conn_handle=1;
 gap(&old_disconnect,old_arg);assert(active_connection==1); // same numeric handle, stale token
 disconnect(2);assert(active_connection==1); // foreign role/handle is inert
 packet.bytes[23]=0;packet.bytes[21]=1;
 assert(receive_command(1,0,&access,nullptr)==0&&sink_count==1);
 ble_hs_cfg.reset_cb(92);assert(central_reset_count==reset_count+2);
 ble_gap_event old_connect{};old_connect.type=BLE_GAP_EVENT_CONNECT;old_connect.connect.status=0;old_connect.connect.conn_handle=1;
 gap(&old_connect,old_arg);assert(active_connection==BLE_HS_CONN_HANDLE_NONE);
 ble_hs_cfg.sync_cb();gap(&old_connect,old_arg);assert(active_connection==BLE_HS_CONN_HANDLE_NONE);
 connect();assert(active_connection==1);disconnect();assert(active_connection==BLE_HS_CONN_HANDLE_NONE);
 std::cout<<"reset/handle-token/foreign-disconnect admission: PASS\n";
 // Provisioning BFPF assembly is connection-owned too; READ/FLOW never revive
 // an old handle between reset and fresh connection admission.
 reset();assert(ble_provisioning_start({"setup-reset"},provision_fn,nullptr,read_source,read_source)==ESP_OK);connect();
 packet.bytes.assign(62,0);memcpy(packet.bytes.data(),"BFPF",4);packet.bytes[23]=2;
 assert(receive_provisioning(1,0,&access,nullptr)==0&&sink_count==0);
 ble_hs_cfg.reset_cb(93);info.bytes.clear();reading.op=BLE_GATT_ACCESS_OP_READ_CHR;
 assert(read_provisioning(1,0,&reading,nullptr)!=0&&info.bytes.empty());
 ble_hs_cfg.sync_cb();assert(read_provisioning(1,0,&reading,nullptr)!=0);connect();
 packet.bytes[27]=1;packet.bytes[60]=1;
 assert(receive_provisioning(1,0,&access,nullptr)!=0&&sink_count==0);
 packet.bytes[27]=0;packet.bytes[23]=1;
 assert(receive_provisioning(1,0,&access,nullptr)==0&&sink_count==1);
 assert(read_provisioning(2,0,&reading,nullptr)!=0&&receive_flow(2,0,nullptr,nullptr)!=0);
 assert(read_provisioning(1,0,&reading,nullptr)==0&&receive_flow(1,0,nullptr,nullptr)==0);
 std::cout<<"provisioning/reset/read-flow admission: PASS\n";
 // Deterministic stale ingress: pause actual mbuf flatten outside the owner
 // mutex, reset+sync+reuse handle, then let the old callback resume.
 for(bool provisioning:{false,true}){
  reset();assert((provisioning?ble_provisioning_start({"stale-setup"},provision_fn,nullptr):ble_runtime_start({"stale-command"},sink_fn,nullptr))==ESP_OK);connect();
  packet.bytes.assign(provisioning?62:58,0);memcpy(packet.bytes.data(),provisioning?"BFPF":"BFF1",4);packet.bytes[provisioning?23:21]=1;packet.bytes[provisioning?60:24]=1;
  block_flatten=true;int received=0;
  std::thread ingress([&]{received=provisioning?receive_provisioning(1,0,&access,nullptr):receive_command(1,0,&access,nullptr);});
  {std::unique_lock<std::mutex> lock(test_mutex);test_cv.wait(lock,[]{return flatten_entered;});}
  ble_hs_cfg.reset_cb(94);ble_hs_cfg.sync_cb();connect();
  {std::lock_guard<std::mutex> lock(test_mutex);release_flatten=true;test_cv.notify_all();}
  ingress.join();assert(received!=0&&sink_count==0&&frame_payload.empty());block_flatten=false;
  assert((provisioning?receive_provisioning(1,0,&access,nullptr):receive_command(1,0,&access,nullptr))==0&&sink_count==1);
 }
 std::cout<<"paused command/provisioning ingress after reset+reused handle: PASS\n";
 // Pending locally-owned mbuf is rejected and freed exactly once after reset,
 // even if the controller immediately reuses handle1 for the new connection.
 reset();assert(ble_runtime_start({"notify-reset"},sink_fn,nullptr)==ESP_OK);connect();fence_test=true;
 std::thread resetter([&]{
  {std::unique_lock<std::mutex> lock(test_mutex);test_cv.wait(lock,[]{return second_mbuf_entered;});}
  ble_hs_cfg.reset_cb(95);ble_hs_cfg.sync_cb();connect();
  {std::lock_guard<std::mutex> lock(test_mutex);reconnect_done=true;test_cv.notify_all();}
 });
 assert(ble_publish_event({active_connection.load(),connection_generation.load()},"11111111-1111-4111-8111-111111111111",data,400)==ESP_ERR_INVALID_STATE);resetter.join();
 assert(notify_count==1&&allocated_mbufs==2&&freed_mbufs==2);
 std::cout<<"notify reset fence/mbuf ownership: PASS\n";
 // Reset/sync can never revive a startup that already timed out.
 reset();sync_now=false;assert(ble_runtime_start({"timed-out"},sink_fn,nullptr)==ESP_ERR_TIMEOUT);
 ble_hs_cfg.reset_cb(96);ble_hs_cfg.sync_cb();assert(startup_timed_out&&startup_result!=ESP_OK&&!advertising);
 // Last: generation exhaustion permanently quarantines until reboot; no wrap,
 // reinitialization, fresh sync or old callbacks may reset the generation.
 reset();assert(ble_runtime_start({"exhausted"},sink_fn,nullptr)==ESP_OK);const bool exhaust_connect=argc>1&&std::string(argv[1])=="overflow-connect";
 const bool exhaust_disconnect=argc>1&&std::string(argv[1])=="overflow-disconnect";
 connection_generation=exhaust_connect?UINT32_MAX:UINT32_MAX-1;
 assert(advertise()==0);connect();assert(connection_generation==UINT32_MAX);
 assert(active_connection==(exhaust_connect?BLE_HS_CONN_HANDLE_NONE:1));
 if(exhaust_disconnect){disconnect();assert(active_connection==BLE_HS_CONN_HANDLE_NONE&&connection_generation==UINT32_MAX);}
 const auto exhausted_arg=connection_arg;const auto before_exhausted_reset=central_reset_count;
 ble_hs_cfg.reset_cb(97);assert(connection_generation==UINT32_MAX&&active_connection==BLE_HS_CONN_HANDLE_NONE);
 ble_hs_cfg.sync_cb();gap(&old_connect,exhausted_arg);assert(active_connection==BLE_HS_CONN_HANDLE_NONE&&connection_generation==UINT32_MAX);
 assert(advertise()!=0&&ble_publish_event({active_connection.load(),connection_generation.load()},"11111111-1111-4111-8111-111111111111",data,1)==ESP_ERR_INVALID_STATE);
 assert(receive_command(1,0,&access,nullptr)!=0&&receive_flow(1,0,nullptr,nullptr)!=0);
 ble_hs_cfg.reset_cb(98);assert(central_reset_count==before_exhausted_reset+2&&connection_generation==UINT32_MAX);
 assert(ble_runtime_start({"bypass"},sink_fn,nullptr)==ESP_ERR_INVALID_STATE);
 std::cout<<"startup timeout/generation exhaustion permanent fence: PASS\n";
 std::cout<<"actual BLE runtime host tests: OK\n";
}
