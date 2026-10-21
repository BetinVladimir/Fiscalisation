#include "fake_ble.h"
#include "ble_result_delivery.h"
#include "canonical_cbor.h"
#include "command_queue.h"
#include "intent_processor.h"
#include "mbedtls/sha256.h"
#include <cassert>
#include <chrono>
#include <condition_variable>
#include <cstring>
#include <deque>
#include <fstream>
#include <iostream>
#include <memory>
#include <mutex>
#include <sstream>
#include <thread>
#include <vector>
#include <sqlite3.h>
#include "../../main/ble_runtime.cpp"
#include "../../main/command_queue.cpp"
using namespace beefiscal::idf;
// Only RTOS/NimBLE are fake. The real production queue worker runs to its next
// empty receive after its current processor call has completed, then terminates.
struct FakeDrainComplete {};
struct FakeQueue { size_t depth,size;std::mutex mutex;std::deque<std::vector<uint8_t>> items; };
static std::vector<std::unique_ptr<FakeQueue>> queues;
static std::vector<std::unique_ptr<bool>> semaphores;
static void(*task_fn)(void*);static void*task_arg;
static bool force_timeout;static int wait_calls;
static void drain(){assert(task_fn);try{task_fn(task_arg);assert(false);}catch(const FakeDrainComplete&) {}}
QueueHandle_t xQueueCreate(size_t depth,size_t size){auto q=std::make_unique<FakeQueue>();q->depth=depth;q->size=size;auto*p=q.get();queues.push_back(std::move(q));return p;}
int xQueueSend(QueueHandle_t q,const void*value,int timeout){assert(timeout==0);std::lock_guard<std::mutex>lock(q->mutex);if(q->items.size()==q->depth)return pdFALSE;auto*p=static_cast<const uint8_t*>(value);q->items.emplace_back(p,p+q->size);return pdTRUE;}
int xQueueReceive(QueueHandle_t q,void*out,int timeout){assert(timeout==portMAX_DELAY);std::lock_guard<std::mutex>lock(q->mutex);if(q->items.empty())throw FakeDrainComplete{};memcpy(out,q->items.front().data(),q->size);q->items.pop_front();return pdTRUE;}
int xTaskCreate(void(*fn)(void*),const char*,unsigned,void*arg,unsigned,void*){assert(!task_fn);task_fn=fn;task_arg=arg;return pdPASS;}
SemaphoreHandle_t xSemaphoreCreateBinary(){auto value=std::make_unique<bool>(false);auto*p=value.get();semaphores.push_back(std::move(value));return p;}
int xSemaphoreGive(SemaphoreHandle_t s){*s=true;return pdTRUE;}
int xSemaphoreTake(SemaphoreHandle_t s,int){++wait_calls;if(force_timeout){force_timeout=false;return pdFALSE;}if(!*s)drain();const bool ready=*s;*s=false;return ready?pdTRUE:pdFALSE;}
void vSemaphoreDelete(SemaphoreHandle_t s){for(auto&i:semaphores)if(i.get()==s){i.reset();return;}assert(false);}
ble_hs_cfg_t ble_hs_cfg{};
static void*advertisement_arg;static void*connection_arg;
static std::vector<std::vector<uint8_t>> notifications;
static bool fail_notify;static int allocation_count,delivery_count;static esp_err_t delivery_result,processor_result;
static BleReplyOwner admitted,delivered;
static IntentProcessor*processor;
static CommandQueue command_queue;
int ble_hs_mbuf_to_flat(os_mbuf*m,void*p,size_t n,uint16_t*copied){memcpy(p,m->bytes.data(),n);*copied=uint16_t(n);return 0;}
os_mbuf*ble_hs_mbuf_from_flat(const void*p,size_t n){++allocation_count;auto*m=new os_mbuf;auto*b=static_cast<const uint8_t*>(p);m->bytes.assign(b,b+n);return m;}
void os_mbuf_free_chain(os_mbuf*m){delete m;}
int ble_gatts_notify_custom(uint16_t connection,uint16_t,os_mbuf*m){assert(connection==1);notifications.push_back(m->bytes);delete m;return fail_notify?1:0;}
uint16_t ble_att_mtu(uint16_t){return 185;}
int ble_gap_adv_set_fields(const ble_hs_adv_fields*){return 0;}
int ble_gap_adv_rsp_set_fields(const ble_hs_adv_fields*){return 0;}
int ble_gap_adv_start(uint8_t,void*,int,const ble_gap_adv_params*,int(*)(ble_gap_event*,void*),void*arg){advertisement_arg=arg;return 0;}
int ble_gap_adv_stop(){return 0;}
int ble_hs_util_ensure_addr(int){return 0;}int ble_hs_id_infer_auto(int,uint8_t*t){*t=1;return 0;}
esp_err_t esp_nimble_hci_init(){return ESP_OK;}esp_err_t nimble_port_init(){return ESP_OK;}
void nimble_port_run(){}void nimble_port_freertos_deinit(){}void nimble_port_freertos_init(void(*)(void*)){ble_hs_cfg.sync_cb();}
void ble_svc_gap_init(){}void ble_svc_gatt_init(){}int ble_svc_gap_device_name_set(const char*){return 0;}int ble_gatts_count_cfg(const ble_gatt_svc_def*){return 0;}int ble_gatts_add_svcs(const ble_gatt_svc_def*){return 0;}
namespace beefiscal::idf {void bluepad_ble_on_host_sync(){}void bluepad_ble_on_host_reset(){}}
static void connect(){connection_arg=advertisement_arg;ble_gap_event e{};e.type=BLE_GAP_EVENT_CONNECT;e.connect.status=0;e.connect.conn_handle=1;gap(&e,connection_arg);assert(active_connection==1);}
static void replace_owner(){ble_hs_cfg.reset_cb(77);ble_hs_cfg.sync_cb();connect();assert(connection_generation!=admitted.generation);}
static bool strip_owner;
static esp_err_t enqueue(const CommandView&v,void*){admitted=v.ble_reply;CommandView copied=v;if(strip_owner)copied.ble_reply={};const auto result=queued_command_sink(copied,&command_queue);copied.ble_reply={};return result;}
static esp_err_t process(const CommandView&v,void*){processor_result=processor->accept(v);return processor_result;}
static esp_err_t deliver(const char*id,const char*json,Ingress ingress,BleReplyOwner owner,void*){++delivery_count;delivered=owner;delivery_result=deliver_ble_result(id,json,ingress,owner,nullptr);return delivery_result;}
static std::string read(const char*p){std::ifstream f(p);assert(f);std::stringstream s;s<<f.rdbuf();return s.str();}
static std::string text(cJSON*p,const char*k){auto*v=cJSON_GetObjectItemCaseSensitive(p,k);assert(cJSON_IsString(v));return v->valuestring;}
static std::string scalar(const char*path,const char*query){sqlite3*d{};sqlite3_stmt*s{};assert(sqlite3_open(path,&d)==SQLITE_OK);assert(sqlite3_prepare_v2(d,query,-1,&s,nullptr)==SQLITE_OK);assert(sqlite3_step(s)==SQLITE_ROW);auto*t=sqlite3_column_text(s,0);std::string out=t?(const char*)t:"";sqlite3_finalize(s);sqlite3_close(d);return out;}
struct Executor:ReceiptExecutor {
 int calls{},recover_calls{};bool available=true,hold=false,unknown=false,entered=false,released=false;
 std::mutex mutex;std::condition_variable cv;
 bool ready()override{return available;}
 ExecutionResult execute(const ReceiptPlan&)override{++calls;if(hold){std::unique_lock<std::mutex>lock(mutex);entered=true;cv.notify_all();assert(cv.wait_for(lock,std::chrono::seconds(5),[&]{return released;}));}return{unknown?ExecutionCertainty::Unknown:ExecutionCertainty::Committed,"470","","","",""};}
 ExecutionResult recover(const RecoveryItem&)override{++recover_calls;return{ExecutionCertainty::Committed,"471","","","",""};}
};
static void send(const std::vector<uint8_t>&payload){uint8_t digest[32];assert(mbedtls_sha256(payload.data(),payload.size(),digest,0)==0);for(size_t offset=0;offset<payload.size();offset+=125){size_t n=std::min<size_t>(125,payload.size()-offset);os_mbuf packet;packet.bytes.assign(57+n,0);auto*f=packet.bytes.data();memcpy(f,"BFF1",4);f[20]=payload.size()>>8;f[21]=payload.size();f[22]=offset>>8;f[23]=offset;f[24]=offset+n==payload.size();memcpy(f+25,digest,32);memcpy(f+57,payload.data()+offset,n);ble_gatt_access_ctxt a{BLE_GATT_ACCESS_OP_WRITE_CHR,&packet};assert(receive_command(1,0,&a,nullptr)==0);}}
static void overwrite_borrowed_assembly(){
 os_mbuf packet;packet.bytes.assign(58,0);memcpy(packet.bytes.data(),"BFF1",4);packet.bytes[21]=2;packet.bytes[57]=0xee;
 ble_gatt_access_ctxt a{BLE_GATT_ACCESS_OP_WRITE_CHR,&packet};assert(receive_command(1,0,&a,nullptr)==0);
}
static void live_result(const std::string&id){assert(delivery_result==ESP_OK&&!notifications.empty());assert(delivered.connection==admitted.connection&&delivered.generation==admitted.generation);std::vector<uint8_t>payload;for(const auto&f:notifications)payload.insert(payload.end(),f.begin()+57,f.end());cJSON*root{};assert(canonical_cbor_to_json(payload.data(),payload.size(),&root)==ESP_OK);assert(text(root,"operation_id")==id);assert(text(root,"state")=="FISCALIZED");cJSON_Delete(root);}
int main(int argc,char**argv){assert(argc==5);const std::string mode=argv[4];const bool reconciliation=mode=="reconcile-live"||mode=="reconcile-reset";auto*fixture=cJSON_Parse(read(argv[1]).c_str());assert(fixture);auto*command=cJSON_GetObjectItemCaseSensitive(fixture,"command");auto*binding_json=cJSON_GetObjectItemCaseSensitive(fixture,"binding");auto*acceptance=cJSON_GetObjectItemCaseSensitive(cJSON_GetObjectItemCaseSensitive(fixture,"acceptance_event"),"payload");acceptance=cJSON_GetObjectItemCaseSensitive(acceptance,"acceptance_evidence");
 CompositeBinding binding;binding.tenant_id=text(binding_json,"tenant_id");binding.location_id=text(binding_json,"location_id");binding.register_id=text(binding_json,"register_id");binding.edge_device_id=text(binding_json,"edge_device_id");binding.generation=cJSON_GetObjectItemCaseSensitive(binding_json,"generation")->valuedouble;binding.payload_sha256=text(reconciliation?cJSON_GetObjectItemCaseSensitive(fixture,"grant"):acceptance,"binding_sha256");auto*fiscal=cJSON_GetObjectItemCaseSensitive(binding_json,"fiscal_endpoint");binding.fiscal.device_id=text(fiscal,"device_id");binding.fiscal.fiscal_device_number=text(fiscal,"fiscal_device_number");binding.fiscal.fiscal_memory_number=text(fiscal,"fiscal_memory_number");auto*a=cJSON_GetObjectItemCaseSensitive(binding_json,"operational_authority");binding.authority.unp_prefix=text(a,"unp_prefix");binding.authority.unp_range_start=cJSON_GetObjectItemCaseSensitive(a,"unp_range_start")->valuedouble;binding.authority.unp_range_end=cJSON_GetObjectItemCaseSensitive(a,"unp_range_end")->valuedouble;
 const std::string pem=read(argv[2]),kid=text(reconciliation?cJSON_GetObjectItemCaseSensitive(command,"reconciliation_authorization"):cJSON_GetObjectItemCaseSensitive(fixture,"authorization"),"key_id"),operation=text(command,"client_operation_id");ExecutionAuthorizationTrust trust{pem.c_str(),kid.c_str(),int64_t(cJSON_GetObjectItemCaseSensitive(fixture,"now")->valuedouble)};
 remove(argv[3]);DurableStorage storage;assert(storage.open_database_for_test(argv[3])==ESP_OK);assert(storage.configure_authority(binding.authority.unp_prefix.c_str(),binding.authority.unp_range_start,binding.authority.unp_range_end,binding.generation)==ESP_OK);
 if(reconciliation){
  auto*grant=cJSON_GetObjectItemCaseSensitive(fixture,"grant");auto*expected=cJSON_GetObjectItemCaseSensitive(grant,"expected");
  const std::string original=text(expected,"operation_id"),receipt=text(expected,"receipt_id"),payment=text(expected,"payment_id");
  binding.payment.present=true;binding.payment.device_id=text(grant,"terminal_id");
  assert(storage.reserve_command(original.c_str(),std::string(64,'a').c_str(),"HTTP","{\"payment_device_id\":\"terminal-1\"}","received",receipt.c_str(),"{}",{{payment,250}},trust.now,[](const AuthorityReservation&){return "{}";},nullptr,"A001").status==ReservationStatus::New);
  assert(storage.set_operation_state(original.c_str(),"EXECUTING",nullptr,trust.now)==ESP_OK);
  assert(storage.begin_payment_saga(original.c_str(),receipt.c_str(),{{payment,250}},trust.now,"terminal-1")==ESP_OK);
  PaymentRecord row;assert(storage.payment(payment.c_str(),row)==ESP_OK);
  assert(storage.checkpoint_payment(row,"RESERVED","CARD_SENDING","PAYMENTS",trust.now)==ESP_OK);
  assert(storage.checkpoint_payment(row,"CARD_SENDING","CARD_UNKNOWN","PAYMENTS",trust.now)==ESP_OK);
  assert(storage.set_operation_state(original.c_str(),"RECOVERY_REQUIRED",nullptr,trust.now)==ESP_OK);
 }
 Executor executor;executor.available=mode!="unavailable";executor.hold=mode=="executing-reset";executor.unknown=mode=="recovery";
 IntentProcessor actual_processor(storage,binding,executor,"",deliver,nullptr,&trust);processor=&actual_processor;
 assert(command_queue.start(process,nullptr)==ESP_OK);assert(ble_runtime_start({"origin-test"},enqueue,nullptr)==ESP_OK);connect();
 uint8_t*raw{};size_t size{};assert(canonical_json_to_cbor(command,&raw,&size)==ESP_OK);std::vector<uint8_t>wire(raw,raw+size);free(raw);
 strip_owner=mode=="missing-owner";fail_notify=mode=="notify-error";
 if(mode=="http"){
   char*json=cJSON_PrintUnformatted(command);CommandView v{reinterpret_cast<uint8_t*>(json),strlen(json),Ingress::Http};assert(command_queue.enqueue(v)==ESP_OK);cJSON_free(json);drain();
 }else if(mode=="queue-wait"||mode=="queue-wait-timeout"){
   admitted={active_connection.load(),connection_generation.load()};CommandView v{wire.data(),wire.size(),Ingress::Ble,admitted};force_timeout=mode=="queue-wait-timeout";
   const auto result=command_queue.enqueue_and_wait(v,100);assert(result==(mode=="queue-wait"?ESP_OK:ESP_ERR_TIMEOUT));
 }else {
   send(wire);assert(admitted.valid());overwrite_borrowed_assembly();std::fill(wire.begin(),wire.end(),0); // queue owns immutable bytes
   if(mode=="queued-reset"||mode=="reconcile-reset")replace_owner();
   if(mode=="executing-reset"){
     std::thread worker(drain);{std::unique_lock<std::mutex>lock(executor.mutex);assert(executor.cv.wait_for(lock,std::chrono::seconds(5),[&]{return executor.entered;}));}
     assert(scalar(argv[3],"SELECT state FROM operations")=="EXECUTING");replace_owner();
     {std::lock_guard<std::mutex>lock(executor.mutex);executor.released=true;executor.cv.notify_all();}worker.join();
   }else drain();
 }
 if(delivery_count!=1)std::cerr<<"mode="<<mode<<" processor_rc="<<processor_result<<" executor_calls="<<executor.calls<<" operations="<<scalar(argv[3],"SELECT count(*) FROM operations")<<"\n";
 assert(delivery_count==1);
 if(mode!="missing-owner"&&mode!="http")assert(delivered.connection==admitted.connection&&delivered.generation==admitted.generation);
 if(reconciliation){
  assert(executor.calls==0&&executor.recover_calls==0&&processor_result==ESP_OK);
  assert(scalar(argv[3],"SELECT state FROM operations WHERE first_transport='ADMIN'")=="COMMITTED");
  assert(scalar(argv[3],"SELECT state FROM payments")=="CARD_APPROVED");
  assert(scalar(argv[3],"SELECT count(*) FROM event_outbox WHERE kind='PAYMENT_RECONCILED'")=="1");
  if(mode=="reconcile-reset")assert(delivery_result==ESP_ERR_INVALID_STATE&&notifications.empty()&&allocation_count==0);
  else live_result(operation);
  storage.close();assert(storage.open_database_for_test(argv[3])==ESP_OK);
  assert(scalar(argv[3],"SELECT count(*) FROM operations")=="2");
  cJSON_Delete(fixture);std::cout<<mode<<" actual admin queue/proof/SQLite/delivery PASS\n";return 0;
 }
 if(mode=="unavailable"){
   assert(executor.calls==0&&processor_result==ESP_ERR_INVALID_STATE);assert(scalar(argv[3],"SELECT state FROM operations")=="RECOVERY_REQUIRED");
 }else{
   assert(executor.calls==1&&processor_result==ESP_OK);assert(scalar(argv[3],"SELECT state FROM operations")==(mode=="recovery"?"RECOVERY_REQUIRED":"COMMITTED"));
   assert(scalar(argv[3],"SELECT count(*) FROM event_outbox WHERE kind='RECEIPT_RESULT'")=="1");
 }
 if(mode=="queued-reset"||mode=="executing-reset"||mode=="missing-owner")assert(delivery_result==ESP_ERR_INVALID_STATE&&notifications.empty()&&allocation_count==0);
 else if(mode=="notify-error")assert(delivery_result==ESP_FAIL&&notifications.size()==1);
 else if(mode=="http")assert(delivery_result==ESP_OK&&notifications.empty()&&allocation_count==0);
 else if(mode!="unavailable"&&mode!="recovery")live_result(operation);
 if(mode=="duplicate"){
   notifications.clear();assert(canonical_json_to_cbor(command,&raw,&size)==ESP_OK);wire.assign(raw,raw+size);free(raw);send(wire);drain();assert(executor.calls==1&&delivery_count==2&&processor_result==ESP_OK);live_result(operation);
 }
 if(mode=="recovery"){
   const int before=delivery_count;notifications.clear();replace_owner();assert(actual_processor.recover_pending()==ESP_OK&&executor.recover_calls==1);assert(delivery_count==before&&notifications.empty());assert(scalar(argv[3],"SELECT state FROM operations")=="COMMITTED");
 }
 // Current owner must never be captured for an invalid or foreign origin.
 notifications.clear();allocation_count=0;const uint8_t byte=0;
 assert(ble_publish_event({},operation.c_str(),&byte,1)==ESP_ERR_INVALID_STATE&&allocation_count==0);
 BleReplyOwner foreign{1,connection_generation.load()-1};assert(ble_publish_event(foreign,operation.c_str(),&byte,1)==ESP_ERR_INVALID_STATE&&allocation_count==0);
 assert(deliver_ble_result("bad","not-json",Ingress::Mqtt,{},nullptr)==ESP_OK);assert(deliver_ble_result("bad","not-json",Ingress::Http,{},nullptr)==ESP_OK);assert(notifications.empty());
 storage.close();assert(storage.open_database_for_test(argv[3])==ESP_OK);assert(!scalar(argv[3],"SELECT result_payload FROM operations").empty());
 assert(scalar(argv[3],"SELECT count(*) FROM operations")=="1");cJSON_Delete(fixture);std::cout<<mode<<" actual queue/processor/SQLite/CBOR/delivery/BLE PASS\n";
}
