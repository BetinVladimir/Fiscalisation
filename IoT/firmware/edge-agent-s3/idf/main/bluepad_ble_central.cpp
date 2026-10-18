#include "bluepad_ble_central.h"
#include "host/ble_gap.h"
#include "host/ble_gatt.h"
#include "host/ble_hs.h"
#include "host/ble_hs_adv.h"
#include "host/ble_uuid.h"
#include "freertos/FreeRTOS.h"
#include "freertos/semphr.h"
#include "esp_timer.h"
#include <algorithm>
#include <cstdint>
#include <cstring>
#include <deque>
#include <limits>
#include <map>
#include <mutex>
#include <vector>

namespace beefiscal::idf { namespace {
// SDK v9 BluetoothLeConnector; Pinpad Commands v3 (2025), pages 3,9,33-35,51-53.
constexpr const char* kServicePrefix="d839fc3c-84dd-4c36-9126-";
constexpr const char* kPower="22ffc547-1bef-48e2-aa87-b87e23ac0bbd";
constexpr const char* kWake="f953144b-e33a-4079-b202-e3d7c1f3dbb0";
constexpr const char* kRead="1f6b14c9-97fa-4f1e-aaa6-7e152fdd04f4";
constexpr const char* kWrite="b378db85-4ec3-4daa-828e-1b99607bd6a0";
constexpr size_t kChunk=19,kPayload=4096,kNotification=514,kQueuedFrames=8;
constexpr int64_t kCommandUs=5000000,kTransactionUs=120000000,kInterbyteUs=2000000;
enum class Phase {Stopped,Configured,Scanning,Connecting,Services,Characteristics,ReadDescriptors,PowerDescriptors,ReadCccd,PowerCccd,PowerRead,WaitPower,WakeWrite,Settling,Ready,Failed};
struct Frame {uint8_t command{},status{};std::vector<uint8_t> data;};
struct Characteristic {uint16_t definition{},value{};uint8_t properties{};};
struct State {
 Phase phase{Phase::Stopped};PaymentEndpointBinding config;uint32_t epoch{},serial{},write_id{},completed_id{};
 uint16_t connection{BLE_HS_CONN_HANDLE_NONE},service_start{},service_end{},read_cccd{},power_cccd{};
 Characteristic tx,rx,power,wake;std::vector<uint16_t> definitions;
 bool configured{},host_synced{},active{},fault{},quarantined{},wake_required{},write_pending{};
 int write_status{};int64_t settle_until{},setup_until{},last_byte{};
 std::vector<uint8_t> receive;std::deque<Frame> frames;
} s;
std::mutex state_mutex,operation_mutex;
// Serialize submission with disconnect/reset; never hold state_mutex across NimBLE calls.
std::recursive_mutex submission_mutex;
SemaphoreHandle_t frame_sem{},write_sem{}; // Static lifetime: late callbacks never access freed semaphores.
uint8_t own_addr_type{};ble_uuid_any_t service_uuid{},tx_uuid{},rx_uuid{},power_uuid{},wake_uuid{};
void* token(uint32_t n){return reinterpret_cast<void*>(uintptr_t(n));}
uint32_t value(void*p){return uint32_t(reinterpret_cast<uintptr_t>(p));}
bool current(uint16_t connection,void*arg){return value(arg)==s.epoch&&connection==s.connection&&s.connection!=BLE_HS_CONN_HANDLE_NONE;}
void fault_locked(){s.fault=true;s.quarantined=true;s.phase=Phase::Failed;s.receive.clear();s.frames.clear();if(frame_sem)xSemaphoreGive(frame_sem);if(write_sem)xSemaphoreGive(write_sem);}
bool ready_locked(){if(s.connection!=BLE_HS_CONN_HANDLE_NONE&&s.phase!=Phase::Ready&&s.phase!=Phase::Failed&&esp_timer_get_time()>s.setup_until)fault_locked();if(s.phase==Phase::Settling&&esp_timer_get_time()>=s.settle_until)s.phase=Phase::Ready;return s.phase==Phase::Ready&&!s.fault&&!s.quarantined&&s.connection!=BLE_HS_CONN_HANDLE_NONE;}
void reset_link_locked(){s.connection=BLE_HS_CONN_HANDLE_NONE;s.service_start=s.service_end=s.read_cccd=s.power_cccd=0;s.tx={};s.rx={};s.power={};s.wake={};s.definitions.clear();s.receive.clear();s.frames.clear();s.last_byte=0;s.write_pending=false;}
uint8_t checksum(const uint8_t*p,size_t n){uint8_t x=0;for(size_t i=0;i<n;++i)x^=p[i];return x;}
std::vector<uint8_t> packet(const std::vector<uint8_t>&data){std::vector<uint8_t> p{0x3e,0x3d,0,uint8_t(data.size()>>8),uint8_t(data.size())};p.insert(p.end(),data.begin(),data.end());p.push_back(checksum(p.data(),p.size()));return p;}
void tlv(std::vector<uint8_t>&out,uint16_t tag,const std::vector<uint8_t>&data){if(tag>255)out.push_back(uint8_t(tag>>8));out.push_back(uint8_t(tag));out.push_back(uint8_t(data.size()));out.insert(out.end(),data.begin(),data.end());}
void consume_locked(const uint8_t*bytes,size_t n){
 if(!s.active)return;
 const int64_t now=esp_timer_get_time();
 if(!s.receive.empty()&&now-s.last_byte>kInterbyteUs){fault_locked();return;}
 for(size_t i=0;i<n&&!s.fault;++i){
  if(s.receive.empty()&&bytes[i]!=0x3e){fault_locked();return;}
  s.receive.push_back(bytes[i]);s.last_byte=now;
  if(s.receive.size()<5)continue;
  const size_t payload=(size_t(s.receive[3])<<8)|s.receive[4];
  if(payload>kPayload){fault_locked();return;}
  if(s.receive.size()!=payload+6)continue;
  if(checksum(s.receive.data(),s.receive.size()-1)!=s.receive.back()||s.frames.size()>=kQueuedFrames){fault_locked();return;}
  s.frames.push_back({s.receive[1],s.receive[2],std::vector<uint8_t>(s.receive.begin()+5,s.receive.end()-1)});s.receive.clear();xSemaphoreGive(frame_sem);
 }
}
int gap(ble_gap_event*,void*);int service_cb(uint16_t,const ble_gatt_error*,const ble_gatt_svc*,void*);
int characteristic_cb(uint16_t,const ble_gatt_error*,const ble_gatt_chr*,void*);
int descriptor_cb(uint16_t,const ble_gatt_error*,uint16_t,const ble_gatt_dsc*,void*);
int setup_write_cb(uint16_t,const ble_gatt_error*,ble_gatt_attr*,void*);
int power_read_cb(uint16_t,const ble_gatt_error*,ble_gatt_attr*,void*);
enum class ActionType {None,Service,Characteristics,ReadDescriptors,PowerDescriptors,ReadCccd,PowerCccd,PowerRead,Wake};
struct Action {ActionType type{ActionType::None};uint16_t connection{},first{},last{};uint32_t epoch{};std::vector<uint8_t> bytes;};
Action action_locked(ActionType type,uint16_t first=0,uint16_t last=0){return {type,s.connection,first,last,s.epoch,{}};}
uint16_t descriptor_end_locked(uint16_t value_handle){uint16_t end=s.service_end;for(auto definition:s.definitions)if(definition>value_handle)end=std::min<uint16_t>(end,uint16_t(definition-1));return end;}
void run(Action a){
 std::lock_guard<std::recursive_mutex> submission(submission_mutex);
 {std::lock_guard<std::mutex> lock(state_mutex);if(a.epoch!=s.epoch||a.connection!=s.connection||s.quarantined)return;}
 int rc=0;
 switch(a.type){
 case ActionType::None:return;
 case ActionType::Service:rc=ble_gattc_disc_svc_by_uuid(a.connection,&service_uuid.u,service_cb,token(a.epoch));break;
 case ActionType::Characteristics:rc=ble_gattc_disc_all_chrs(a.connection,a.first,a.last,characteristic_cb,token(a.epoch));break;
 case ActionType::ReadDescriptors:case ActionType::PowerDescriptors:rc=ble_gattc_disc_all_dscs(a.connection,a.first,a.last,descriptor_cb,token(a.epoch));break;
 case ActionType::ReadCccd:case ActionType::PowerCccd:{uint8_t on[]={1,0};rc=ble_gattc_write_flat(a.connection,a.first,on,2,setup_write_cb,token(a.epoch));break;}
 case ActionType::PowerRead:rc=ble_gattc_read(a.connection,a.first,power_read_cb,token(a.epoch));break;
 case ActionType::Wake:rc=ble_gattc_write_flat(a.connection,a.first,a.bytes.data(),uint16_t(a.bytes.size()),setup_write_cb,token(a.epoch));break;
 }
 if(rc){std::lock_guard<std::mutex>lock(state_mutex);if(a.epoch==s.epoch&&a.connection==s.connection)fault_locked();}
}
void scan(){
 std::lock_guard<std::recursive_mutex> submission(submission_mutex);
 uint32_t epoch;
 {std::lock_guard<std::mutex>lock(state_mutex);if(!s.configured||!s.host_synced||s.quarantined)return;if(s.epoch==UINT32_MAX){fault_locked();return;}epoch=++s.epoch;reset_link_locked();s.phase=Phase::Scanning;}
 ble_gap_disc_params p{};p.filter_duplicates=1;
 if(ble_gap_disc(own_addr_type,BLE_HS_FOREVER,&p,gap,token(epoch))){std::lock_guard<std::mutex>lock(state_mutex);if(s.epoch==epoch)fault_locked();}
}
int service_cb(uint16_t c,const ble_gatt_error*e,const ble_gatt_svc*v,void*arg){Action next;
 {std::lock_guard<std::mutex>lock(state_mutex);if(!current(c,arg)||s.phase!=Phase::Services)return 0;
 if(e->status==0&&v){if(s.service_start){fault_locked();return 0;}s.service_start=v->start_handle;s.service_end=v->end_handle;}
 else if(e->status==BLE_HS_EDONE&&s.service_start&&s.service_end>s.service_start){s.phase=Phase::Characteristics;next=action_locked(ActionType::Characteristics,s.service_start,s.service_end);}
 else fault_locked();}run(next);return 0;
}
int characteristic_cb(uint16_t c,const ble_gatt_error*e,const ble_gatt_chr*v,void*arg){Action next;
 {std::lock_guard<std::mutex>lock(state_mutex);if(!current(c,arg)||s.phase!=Phase::Characteristics)return 0;
 if(e->status==0&&v){if(v->def_handle<s.service_start||v->val_handle<=v->def_handle||v->val_handle>s.service_end||std::find(s.definitions.begin(),s.definitions.end(),v->def_handle)!=s.definitions.end()){fault_locked();return 0;}s.definitions.push_back(v->def_handle);Characteristic*dst=nullptr;
 if(ble_uuid_cmp(&v->uuid.u,&tx_uuid.u)==0)dst=&s.tx;else if(ble_uuid_cmp(&v->uuid.u,&rx_uuid.u)==0)dst=&s.rx;else if(ble_uuid_cmp(&v->uuid.u,&power_uuid.u)==0)dst=&s.power;else if(ble_uuid_cmp(&v->uuid.u,&wake_uuid.u)==0)dst=&s.wake;
 if(dst){if(dst->value){fault_locked();return 0;}*dst={v->def_handle,v->val_handle,v->properties};}}
 else if(e->status==BLE_HS_EDONE&&s.tx.value&&s.rx.value&&s.power.value&&s.wake.value&&(s.tx.properties&BLE_GATT_CHR_PROP_WRITE)&&(s.wake.properties&BLE_GATT_CHR_PROP_WRITE)&&(s.rx.properties&BLE_GATT_CHR_PROP_NOTIFY)&&(s.power.properties&BLE_GATT_CHR_PROP_NOTIFY)){s.phase=Phase::ReadDescriptors;next=action_locked(ActionType::ReadDescriptors,s.rx.value,descriptor_end_locked(s.rx.value));}
 else fault_locked();}run(next);return 0;
}
int descriptor_cb(uint16_t c,const ble_gatt_error*e,uint16_t chr,const ble_gatt_dsc*v,void*arg){Action next;
 {std::lock_guard<std::mutex>lock(state_mutex);if(!current(c,arg)||(s.phase!=Phase::ReadDescriptors&&s.phase!=Phase::PowerDescriptors))return 0;
 const bool read=s.phase==Phase::ReadDescriptors;const uint16_t expected=read?s.rx.value:s.power.value;
 if(chr!=expected)return 0;
 auto&handle=read?s.read_cccd:s.power_cccd;
 if(e->status==0&&v){if(v->handle<=expected||v->handle>descriptor_end_locked(expected)){fault_locked();return 0;}if(ble_uuid_u16(&v->uuid.u)==BLE_GATT_DSC_CLT_CFG_UUID16){if(handle){fault_locked();return 0;}handle=v->handle;}}
 else if(e->status==BLE_HS_EDONE&&handle){if(read){s.phase=Phase::PowerDescriptors;next=action_locked(ActionType::PowerDescriptors,s.power.value,descriptor_end_locked(s.power.value));}else{s.phase=Phase::ReadCccd;next=action_locked(ActionType::ReadCccd,s.read_cccd);}}
 else fault_locked();}run(next);return 0;
}
int setup_write_cb(uint16_t c,const ble_gatt_error*e,ble_gatt_attr*v,void*arg){Action next;
 {std::lock_guard<std::mutex>lock(state_mutex);if(!current(c,arg))return 0;
 const uint16_t expected=s.phase==Phase::ReadCccd?s.read_cccd:s.phase==Phase::PowerCccd?s.power_cccd:s.phase==Phase::WakeWrite?s.wake.value:0;
 if(!expected)return 0;
 if(e->status){fault_locked();return 0;}
 if(!v||v->handle!=expected){fault_locked();return 0;}
 if(s.phase==Phase::ReadCccd){s.phase=Phase::PowerCccd;next=action_locked(ActionType::PowerCccd,s.power_cccd);}
 else if(s.phase==Phase::PowerCccd){s.phase=Phase::PowerRead;next=action_locked(ActionType::PowerRead,s.power.value);}
 else {s.phase=Phase::Settling;s.settle_until=esp_timer_get_time()+2200000;}}
 run(next);return 0;
}
int power_read_cb(uint16_t c,const ble_gatt_error*e,ble_gatt_attr*v,void*arg){
 std::lock_guard<std::mutex>lock(state_mutex);if(!current(c,arg)||s.phase!=Phase::PowerRead)return 0;
 uint8_t power=0;if(e->status||!v||v->handle!=s.power.value||!v->om||OS_MBUF_PKTLEN(v->om)!=1||os_mbuf_copydata(v->om,0,1,&power)){fault_locked();return 0;}
 if(power==0x31){s.phase=Phase::Settling;s.settle_until=esp_timer_get_time()+200000;}
 else if(power==0x30){s.phase=Phase::WaitPower;s.wake_required=true;}
 else {fault_locked();}
 return 0;
}
int gap(ble_gap_event*e,void*arg){std::lock_guard<std::recursive_mutex> submission(submission_mutex);Action next;bool connect=false,rescan=false;ble_addr_t peer{};
 {std::lock_guard<std::mutex>lock(state_mutex);if(value(arg)!=s.epoch)return 0;
 switch(e->type){
 case BLE_GAP_EVENT_DISC:{if(s.phase!=Phase::Scanning)break;ble_hs_adv_fields fields{};if(ble_hs_adv_parse_fields(&fields,e->disc.data,e->disc.length_data)==0&&fields.name&&std::string(reinterpret_cast<const char*>(fields.name),fields.name_len)==s.config.ble_identity){s.phase=Phase::Connecting;peer=e->disc.addr;connect=true;}break;}
 case BLE_GAP_EVENT_CONNECT:if(s.phase!=Phase::Connecting)break;if(e->connect.status){s.phase=Phase::Configured;rescan=true;}else{s.connection=e->connect.conn_handle;s.phase=Phase::Services;s.setup_until=esp_timer_get_time()+30000000;next=action_locked(ActionType::Service);}break;
 case BLE_GAP_EVENT_NOTIFY_RX:{if(e->notify_rx.conn_handle!=s.connection||s.phase==Phase::Failed||!e->notify_rx.om)break;const size_t n=OS_MBUF_PKTLEN(e->notify_rx.om);if(n>kNotification){fault_locked();break;}std::vector<uint8_t> bytes(n);if(os_mbuf_copydata(e->notify_rx.om,0,int(n),bytes.data())){fault_locked();break;}
 if(e->notify_rx.attr_handle==s.power.value){if(n!=1){fault_locked();break;}if(bytes[0]==0x30){if(s.active){fault_locked();break;}s.phase=Phase::WaitPower;s.wake_required=true;}else if(bytes[0]==0x31&&s.phase==Phase::WaitPower&&s.wake_required){s.phase=Phase::WakeWrite;s.wake_required=false;next=action_locked(ActionType::Wake,s.wake.value);next.bytes.assign(s.config.ble_identity.begin(),s.config.ble_identity.end());}else if(bytes[0]!=0x31)fault_locked();}
 else if(e->notify_rx.attr_handle==s.rx.value){consume_locked(bytes.data(),bytes.size());}
 break;}
 case BLE_GAP_EVENT_DISCONNECT:if(e->disconnect.conn.conn_handle!=s.connection)break;if(s.active)fault_locked();reset_link_locked();s.phase=Phase::Configured;rescan=!s.quarantined;xSemaphoreGive(frame_sem);xSemaphoreGive(write_sem);break;
 default:break;
 }}
 run(next);if(connect){ble_gap_disc_cancel();if(ble_gap_connect(own_addr_type,&peer,30000,nullptr,gap,arg)){std::lock_guard<std::mutex>lock(state_mutex);if(value(arg)==s.epoch)fault_locked();}}if(rescan)scan();return 0;
}
int write_cb(uint16_t c,const ble_gatt_error*e,ble_gatt_attr*v,void*arg){std::lock_guard<std::mutex>lock(state_mutex);if(!s.write_pending||s.write_id!=value(arg)||c!=s.connection||(!e->status&&(!v||v->handle!=s.tx.value)))return 0;s.completed_id=s.write_id;s.write_status=e->status;s.write_pending=false;xSemaphoreGive(write_sem);return 0;}
uint32_t ticks_left(int64_t deadline){const int64_t us=deadline-esp_timer_get_time();return us<=0?0:uint32_t((us+999)/1000);}
enum class Send {Sent,NotSent,Unknown};
Send send_frame(const std::vector<uint8_t>&bytes,uint32_t epoch){bool possible=false;const int64_t deadline=esp_timer_get_time()+kCommandUs;
 for(size_t offset=0;offset<bytes.size();offset+=kChunk){uint32_t id;uint16_t c,handle;int rc;
 {std::lock_guard<std::recursive_mutex> submission(submission_mutex);
 {std::lock_guard<std::mutex>lock(state_mutex);if(s.epoch!=epoch||!ready_locked()||s.serial==UINT32_MAX)return possible?Send::Unknown:Send::NotSent;while(xSemaphoreTake(write_sem,0)==pdTRUE){};id=++s.serial;s.write_id=id;s.write_pending=true;s.completed_id=0;c=s.connection;handle=s.tx.value;}
 const size_t n=std::min(kChunk,bytes.size()-offset);rc=ble_gattc_write_flat(c,handle,bytes.data()+offset,uint16_t(n),write_cb,token(id));
 }
 if(rc){std::lock_guard<std::mutex>lock(state_mutex);if(s.write_id==id)s.write_pending=false;return possible?Send::Unknown:Send::NotSent;}
 possible=true;const auto ticks=ticks_left(deadline);if(!ticks||xSemaphoreTake(write_sem,pdMS_TO_TICKS(ticks))!=pdTRUE)return Send::Unknown;
 {std::lock_guard<std::mutex>lock(state_mutex);if(s.epoch!=epoch||s.completed_id!=id||s.write_status||!ready_locked())return Send::Unknown;}
 }return Send::Sent;
}
bool next_frame(Frame&out,uint32_t epoch,int64_t deadline){for(;;){int64_t wake=deadline;
 {std::lock_guard<std::mutex>lock(state_mutex);if(s.epoch!=epoch||s.fault||!ready_locked())return false;if(!s.frames.empty()){out=std::move(s.frames.front());s.frames.pop_front();return true;}
 if(!s.receive.empty()){wake=std::min(wake,s.last_byte+kInterbyteUs);if(esp_timer_get_time()>=wake){fault_locked();return false;}}}
 auto ticks=ticks_left(wake);if(!ticks)return false;
 if(xSemaphoreTake(frame_sem,pdMS_TO_TICKS(ticks))!=pdTRUE){std::lock_guard<std::mutex>lock(state_mutex);if(!s.receive.empty()&&esp_timer_get_time()>=s.last_byte+kInterbyteUs)fault_locked();return false;}}}
bool response(Frame&out,uint32_t epoch,bool allow_events=false){const int64_t deadline=esp_timer_get_time()+kCommandUs;while(next_frame(out,epoch,deadline)){if(out.command==0)return out.status==0;if(allow_events&&out.command==0x0e)continue;return false;}return false;}
using Tags=std::map<uint32_t,std::vector<uint8_t>>;
bool parse_tags(const std::vector<uint8_t>&bytes,Tags&out){for(size_t p=0;p<bytes.size();){uint32_t tag=bytes[p++];if((tag&31)==31){int count=0;uint8_t b;do{if(p==bytes.size()||++count>2)return false;b=bytes[p++];tag=(tag<<8)|b;}while(b&128);}if(p==bytes.size())return false;size_t n=bytes[p++];if(n&128){size_t count=n&127;if(!count||count>2||count>bytes.size()-p)return false;n=0;while(count--)n=(n<<8)|bytes[p++];}if(n>bytes.size()-p||out.count(tag))return false;out[tag]=std::vector<uint8_t>(bytes.begin()+p,bytes.begin()+p+n);p+=n;}return true;}
bool integer(const Tags&t,uint32_t tag,uint32_t&out){auto i=t.find(tag);if(i==t.end()||i->second.size()!=4)return false;out=0;for(auto b:i->second)out=(out<<8)|b;return true;}
bool stan(const Tags&t,std::string&out){auto i=t.find(0x9f41);if(i==t.end()||i->second.size()!=4)return false;out.clear();for(auto b:i->second){if((b>>4)>9||(b&15)>9)return false;out+=char('0'+(b>>4));out+=char('0'+(b&15));}return out!="00000000";}
bool text(const Tags&t,uint32_t tag,std::string&out){auto i=t.find(tag);if(i==t.end()||i->second.empty()||i->second.size()>64)return false;for(auto b:i->second)if(b<32||b>126)return false;out.assign(i->second.begin(),i->second.end());return true;}
PaymentOutcome purchase(const std::string&id,int64_t amount){
 std::unique_lock<std::mutex>single(operation_mutex,std::try_to_lock);if(!single.owns_lock())return {PaymentCertainty::NotSent,"","","","BLUEPAD_BUSY"};
 if(id.empty()||id.size()>64||amount<=0||uint64_t(amount)>UINT32_MAX||std::any_of(id.begin(),id.end(),[](unsigned char c){return c<32||c>126;}))return {PaymentCertainty::NotSent,"","","","BLUEPAD_ARGUMENT"};
 uint32_t epoch;{std::lock_guard<std::mutex>lock(state_mutex);if(!ready_locked())return {PaymentCertainty::NotSent,"","","","BLUEPAD_UNAVAILABLE"};epoch=s.epoch;s.active=true;s.receive.clear();s.frames.clear();s.last_byte=0;while(xSemaphoreTake(frame_sem,0)==pdTRUE){}}
 auto finish=[&](PaymentOutcome result,bool quarantine){std::lock_guard<std::mutex>lock(state_mutex);s.active=false;s.receive.clear();s.frames.clear();if(quarantine)fault_locked();return result;};
 auto unknown=[&](const char*error){return finish({PaymentCertainty::Unknown,"","","",error},true);};
 std::vector<uint8_t> start{1,3};tlv(start,0x81,{uint8_t(amount>>24),uint8_t(amount>>16),uint8_t(amount>>8),uint8_t(amount)});tlv(start,0xdf03,std::vector<uint8_t>(id.begin(),id.end()));
 const auto sent=send_frame(packet(start),epoch);if(sent==Send::NotSent)return finish({PaymentCertainty::NotSent,"","","","BLUEPAD_WRITE_NOT_SENT"},false);if(sent!=Send::Sent)return unknown("BLUEPAD_PARTIAL_WRITE");
 Frame ack;if(!response(ack,epoch)||!ack.data.empty())return unknown("BLUEPAD_START_UNCONFIRMED");
 Frame final;const int64_t deadline=esp_timer_get_time()+kTransactionUs;bool complete=false;
 while(next_frame(final,epoch,deadline)){if(final.command!=0x0e||final.status||final.data.empty())return unknown("BLUEPAD_UNEXPECTED_EVENT");if(final.data[0]==2)continue;if(final.data[0]!=1)return unknown("BLUEPAD_UNSUPPORTED_EVENT");complete=true;break;}
 if(!complete)return unknown("BLUEPAD_FINAL_TIMEOUT");
 Tags event;std::vector<uint8_t> fields(final.data.begin()+1,final.data.end());uint32_t result,error,event_amount;std::string event_stan;
 if(!parse_tags(fields,event)||!integer(event,0xdf05,result)||!integer(event,0xdf06,error)||!integer(event,0x81,event_amount)||!stan(event,event_stan))return unknown("BLUEPAD_FINAL_INVALID");
 // The final event itself has no merchant reference. Read last receipt tags,
 // exactly as SDK DatecsPayWorker.resultProceed, and bind DF03 before approval.
 const std::vector<uint8_t> request{2,0xdf,3,0x81,0xdf,0x10,0x9f,0x41,0xdf,5,0xdf,6,0xdf,7,0xdf,8};
 if(send_frame(packet(request),epoch)!=Send::Sent||!response(ack,epoch,true))return unknown("BLUEPAD_RECEIPT_UNAVAILABLE");
 Tags receipt;uint32_t proof_result,proof_error,proof_amount;std::string reference,proof_stan,rrn,auth;
 if(!parse_tags(ack.data,receipt)||!text(receipt,0xdf03,reference)||reference!=id||!integer(receipt,0x81,proof_amount)||proof_amount!=uint64_t(amount)||!integer(receipt,0xdf05,proof_result)||!integer(receipt,0xdf06,proof_error)||proof_result!=result||proof_error!=error||!stan(receipt,proof_stan)||proof_stan!=event_stan||receipt[0xdf10]!=std::vector<uint8_t>({0,3})||event_amount!=proof_amount)return unknown("BLUEPAD_RECEIPT_MISMATCH");
 // Vendor p63: only 0 APPROVED / 1 DECLINED are terminal bank decisions.
 if(result>1||(result==0&&error!=0))return unknown("BLUEPAD_RESULT_UNRESOLVED");
 const bool approved=result==0;
 if(approved&&(!text(receipt,0xdf07,rrn)||!text(receipt,0xdf08,auth)))return unknown("BLUEPAD_APPROVAL_PROOF_MISSING");
 if(event.count(0xdf07)&&event[0xdf07]!=receipt[0xdf07])return unknown("BLUEPAD_RRN_MISMATCH");
 if(event.count(0xdf08)&&event[0xdf08]!=receipt[0xdf08])return unknown("BLUEPAD_AUTH_MISMATCH");
 // END is only sent after a final, correlated result; never abort an unknown payment.
 if(send_frame(packet({3,0,uint8_t(approved?1:0)}),epoch)!=Send::Sent||!response(ack,epoch,true)||!ack.data.empty())return unknown("BLUEPAD_END_UNCONFIRMED");
 return finish({approved?PaymentCertainty::Approved:PaymentCertainty::Declined,proof_stan,rrn,auth,approved?"":"BLUEPAD_DECLINED_"+std::to_string(error)},false);
}
} // namespace
esp_err_t bluepad_ble_start(const PaymentEndpointBinding&v){
 std::lock_guard<std::mutex>lock(state_mutex);
 if(s.configured)return ESP_ERR_INVALID_STATE;
 if(!v.present||v.vendor!="DATECS"||v.model!="BLUEPAD-50 PLUS"||v.transport!="BLE_GATT"||v.ble_identity.empty()||v.ble_identity.size()>19||v.service_uuid.compare(0,std::strlen(kServicePrefix),kServicePrefix)!=0||v.tx_characteristic_uuid!=kWrite||v.rx_characteristic_uuid!=kRead||ble_uuid_from_str(&service_uuid,v.service_uuid.c_str())||ble_uuid_from_str(&tx_uuid,kWrite)||ble_uuid_from_str(&rx_uuid,kRead)||ble_uuid_from_str(&power_uuid,kPower)||ble_uuid_from_str(&wake_uuid,kWake))return ESP_ERR_INVALID_ARG;
 if(!frame_sem)frame_sem=xSemaphoreCreateBinary();
 if(!write_sem)write_sem=xSemaphoreCreateBinary();
 if(!frame_sem||!write_sem)return ESP_ERR_NO_MEM;
 s.config=v;s.configured=true;s.phase=Phase::Configured;return ESP_OK;
}
void bluepad_ble_on_host_reset(){std::lock_guard<std::recursive_mutex> submission(submission_mutex);std::lock_guard<std::mutex>lock(state_mutex);s.host_synced=false;if(s.active)fault_locked();if(s.epoch!=UINT32_MAX)++s.epoch;else fault_locked();reset_link_locked();s.phase=s.quarantined?Phase::Failed:Phase::Configured;if(frame_sem)xSemaphoreGive(frame_sem);if(write_sem)xSemaphoreGive(write_sem);}
void bluepad_ble_on_host_sync(){std::lock_guard<std::recursive_mutex> submission(submission_mutex);uint8_t address;if(ble_hs_id_infer_auto(0,&address))return;{std::lock_guard<std::mutex>lock(state_mutex);if(!s.configured||s.host_synced)return;s.host_synced=true;own_addr_type=address;}scan();}
bool bluepad_ble_ready(){std::lock_guard<std::mutex>lock(state_mutex);if(s.connection!=BLE_HS_CONN_HANDLE_NONE&&s.phase!=Phase::Ready&&s.phase!=Phase::Failed&&esp_timer_get_time()>s.setup_until)fault_locked();return ready_locked();}
PaymentOutcome bluepad_ble_purchase(const std::string&id,int64_t amount){return purchase(id,amount);}
PaymentOutcome bluepad_ble_lookup(const std::string&){return {PaymentCertainty::NotSent,"","","","BLUEPAD_QUERY_PROFILE_UNVERIFIED"};}
PaymentOutcome bluepad_ble_reverse(const std::string&,int64_t){return {PaymentCertainty::NotSent,"","","","BLUEPAD_REVERSAL_PROFILE_UNVERIFIED"};}
} // namespace beefiscal::idf
