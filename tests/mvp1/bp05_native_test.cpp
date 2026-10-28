// BP-05 native acceptance test: UNKNOWN resolution via 0x1A+0x24 and purchase block.
// Compiled by test_bp_05_1.py / test_bp_05_2.py with FakeBluePad stubs.
#include "fake_bluepad.h"
#include <cassert>
#include <cstring>
#include <iostream>
#include <thread>
#include <chrono>
#include <functional>
#include <deque>
#include <atomic>
#include "../../IoT/firmware/edge-agent-s3/idf/main/bluepad_ble_central.cpp"
using namespace beefiscal::idf;
namespace fake {
std::mutex mutex;std::deque<std::function<void()>> events;std::string mode;std::vector<uint8_t> incoming;std::vector<std::vector<uint8_t>> commands;
ble_gap_event_fn*gcb;void*garg;std::atomic<int> txwrites{0};int setupwrites=0;bool finished=false;std::atomic<ble_gatt_attr_fn*> held{nullptr};std::atomic<void*> heldarg{nullptr};
const auto origin=std::chrono::steady_clock::now();std::atomic<int64_t> extra{0};
void enqueue(std::function<void()> f){std::lock_guard<std::mutex>l(mutex);events.push_back(f);}
bool pump(){std::function<void()> f;{std::lock_guard<std::mutex>l(mutex);if(events.empty())return false;f=events.front();events.pop_front();}f();return true;}
std::vector<uint8_t> hex(const char*t){std::vector<uint8_t>b;while(*t){unsigned n=0;for(int i=0;i<2;++i){char c=*t++;n=n*16+(c<='9'?c-'0':c-'a'+10);}b.push_back(n);}return b;}
std::vector<uint8_t> device(uint8_t command,std::vector<uint8_t>b){std::vector<uint8_t>o{0x3e,command,0,uint8_t(b.size()>>8),uint8_t(b.size())};o.insert(o.end(),b.begin(),b.end());uint8_t x=0;for(auto v:o)x^=v;o.push_back(x);return o;}
void notify(std::vector<uint8_t>b,uint16_t handle=5,void*arg=nullptr){os_mbuf m{b};ble_gap_event e{};e.type=BLE_GAP_EVENT_NOTIFY_RX;e.notify_rx.conn_handle=7;e.notify_rx.attr_handle=handle;e.notify_rx.om=&m;gcb(&e,arg?arg:garg);}
void disconnect(){ble_gap_event e{};e.type=BLE_GAP_EVENT_DISCONNECT;e.disconnect.conn.conn_handle=7;gcb(&e,garg);}
void respond(const std::vector<uint8_t>&cmd){
 commands.push_back(cmd);assert(cmd.size()>=7);uint8_t sub=cmd[5];
 if(sub==1){
  // Ack the start command always
  notify(hex("3e000000003e"));
  if(mode=="ack-only"||mode=="ack-only-reverse"){return;}  // No final event: purchase will timeout -> UNKNOWN
  // Normal approved response for other modes
  notify(device(0x0e,hex("01df050400000000df0604000000008104000000019f410400000367")));
 }else if(sub==2){
  auto receipt=hex("df0306303030303033363781040000000c000000df100200039f410400000367df050400000000df060400000000df070652524e313233df0806415554483031");
  notify(device(0,receipt));
 }else if(sub==3){
  notify(hex("3e000000003e"));
 }else if(sub==0x1a){
  // GET PINPAD STATUS
  if(mode=="ack-only-reverse"){
   // Return flag=1 (R: reverse needed)
   notify(device(0,{1}));
  }else{
   // Return flag=0 (no reverse needed)
   notify(device(0,{0}));
  }
 }else if(sub==0x24){
  // GET REPORT TAGS BY STAN: return COMMITTED (DF05=0) with rrn/auth
  auto data=hex("df050400000000df070652524e313233df0806415554483031");
  notify(device(0,data));
 }else{
  assert(false);
 }
}
}
int ble_uuid_from_str(ble_uuid_any_t*u,const char*t){std::strncpy(u->u.text,t,39);return 0;}int ble_uuid_cmp(const ble_uuid_t*a,const ble_uuid_t*b){return std::strcmp(a->text,b->text);}uint16_t ble_uuid_u16(const ble_uuid_t*u){return u->value;}
int ble_hs_id_infer_auto(int,uint8_t*v){*v=0;return 0;}int ble_hs_adv_parse_fields(ble_hs_adv_fields*f,const uint8_t*p,uint8_t n){f->name=(uint8_t*)p;f->name_len=n;return 0;}
int ble_gap_disc(uint8_t,int,const ble_gap_disc_params*,ble_gap_event_fn*cb,void*a){fake::gcb=cb;fake::garg=a;fake::enqueue([=]{ble_gap_event e{};e.type=BLE_GAP_EVENT_DISC;std::string name="BluePad";e.disc.data=(uint8_t*)name.data();e.disc.length_data=name.size();cb(&e,a);});return 0;}int ble_gap_disc_cancel(){return 0;}int ble_gap_connect(uint8_t,const ble_addr_t*,int,const void*,ble_gap_event_fn*cb,void*a){fake::gcb=cb;fake::garg=a;fake::enqueue([=]{ble_gap_event e{};e.type=BLE_GAP_EVENT_CONNECT;e.connect.conn_handle=7;cb(&e,a);});return 0;}int ble_gap_terminate(uint16_t,uint8_t){return 0;}
int ble_gattc_exchange_mtu(uint16_t,ble_gatt_mtu_fn*,void*){return 0;}
int ble_gattc_disc_svc_by_uuid(uint16_t c,const ble_uuid_t*u,ble_gatt_disc_svc_fn*cb,void*a){auto uuid=*u;fake::enqueue([=]{ble_gatt_error ok{},done{BLE_HS_EDONE};ble_gatt_svc v{};v.start_handle=1;v.end_handle=12;v.uuid.u=uuid;cb(c,&ok,&v,a);cb(c,&done,nullptr,a);});return 0;}
int ble_gattc_disc_all_chrs(uint16_t c,uint16_t first,uint16_t last,ble_gatt_chr_fn*cb,void*a){assert(first==1&&last==12);fake::enqueue([=]{ble_gatt_error ok{},done{BLE_HS_EDONE};const char*uuid[]={kWrite,kRead,kPower,kWake};uint16_t def[]={2,4,7,10};for(int i=0;i<4;++i){ble_gatt_chr v{};v.def_handle=def[i];v.val_handle=def[i]+1;v.properties=(i==0||i==3)?BLE_GATT_CHR_PROP_WRITE:BLE_GATT_CHR_PROP_NOTIFY;ble_uuid_from_str(&v.uuid,uuid[i]);cb(c,&ok,&v,a);}cb(c,&done,nullptr,a);});return 0;}
int ble_gattc_disc_all_dscs(uint16_t c,uint16_t first,uint16_t last,ble_gatt_dsc_fn*cb,void*a){assert((first==5&&last==6)||(first==8&&last==9));fake::enqueue([=]{ble_gatt_error ok{},done{BLE_HS_EDONE};ble_gatt_dsc v{};v.handle=first+1;v.uuid.u.value=0x2902;cb(c,&ok,first,&v,a);cb(c,&done,first,nullptr,a);});return 0;}
int ble_gattc_read(uint16_t c,uint16_t handle,ble_gatt_attr_fn*cb,void*a){assert(handle==8);fake::enqueue([=]{ble_gatt_error ok{};os_mbuf m{{0x31}};ble_gatt_attr v{handle,0,&m};cb(c,&ok,&v,a);});return 0;}
int ble_gattc_write_flat(uint16_t c,uint16_t handle,const void*p,uint16_t n,ble_gatt_attr_fn*cb,void*a){std::vector<uint8_t>b((const uint8_t*)p,(const uint8_t*)p+n);
 if(handle!=3){++fake::setupwrites;
  fake::enqueue([=]{ble_gatt_error e{};ble_gatt_attr v{handle,0,nullptr};cb(c,&e,&v,a);});return 0;}
 ++fake::txwrites;assert(n<=19);
 fake::enqueue([=]{
  ble_gatt_error e{};ble_gatt_attr v{handle,0,nullptr};cb(c,&e,&v,a);
  fake::incoming.insert(fake::incoming.end(),b.begin(),b.end());if(fake::incoming.size()>=5){size_t size=(size_t(fake::incoming[3])<<8)+fake::incoming[4]+6;if(fake::incoming.size()==size){auto cmd=fake::incoming;fake::incoming.clear();fake::respond(cmd);}}
 });return 0;
}
int os_mbuf_copydata(const os_mbuf*m,int offset,int n,void*p){if(offset<0||n<0||size_t(offset+n)>m->bytes.size())return 1;std::memcpy(p,m->bytes.data()+offset,n);return 0;}
SemaphoreHandle_t xSemaphoreCreateBinary(){return new FakeSemaphore;}
int xSemaphoreTake(SemaphoreHandle_t s,uint32_t ticks){std::unique_lock<std::mutex>l(s->mutex);if(!s->given&&ticks)s->cv.wait_for(l,std::chrono::microseconds(int64_t(ticks)*10),[&]{return s->given;});if(!s->given)return 0;s->given=false;return 1;}
int xSemaphoreGive(SemaphoreHandle_t s){{std::lock_guard<std::mutex>l(s->mutex);s->given=true;}s->cv.notify_one();return 1;}
int64_t esp_timer_get_time(){return 1000000+fake::extra+std::chrono::duration_cast<std::chrono::microseconds>(std::chrono::steady_clock::now()-fake::origin).count()*100;}
void setup(){PaymentEndpointBinding b;b.present=true;b.vendor="DATECS";b.model="BLUEPAD-50 PLUS";b.transport="BLE_GATT";b.ble_identity="BluePad";b.service_uuid="d839fc3c-84dd-4c36-9126-000000000001";b.tx_characteristic_uuid=kWrite;b.rx_characteristic_uuid=kRead;assert(bluepad_ble_start(b)==ESP_OK);bluepad_ble_on_host_sync();while(fake::pump()){}fake::extra+=3000000;}

static void pump_until(std::atomic<bool>&done){
 auto limit=std::chrono::steady_clock::now()+std::chrono::seconds(10);
 while(!done&&std::chrono::steady_clock::now()<limit){if(!fake::pump())std::this_thread::sleep_for(std::chrono::microseconds(50));}
}

int main(int argc,char**argv){assert(argc==2);std::string test=argv[1];

 if(test=="bp-05-1-drop-resolve"){
  // Purchase with ack-only (no final 0x0e event) -> UNKNOWN
  // Then lookup -> COMMITTED; verify no extra purchase I/O
  fake::mode="ack-only";setup();assert(bluepad_ble_ready());
  PaymentOutcome pur;std::atomic<bool>pur_done{false};
  std::thread t1([&]{pur=bluepad_ble_purchase("00000367",1);pur_done=true;});
  pump_until(pur_done);assert(pur_done);t1.join();
  assert(pur.certainty==PaymentCertainty::Unknown);
  assert(!bluepad_ble_ready());
  int writes_after_purchase=fake::txwrites.load();
  // Lookup resolves: 0x1A flag=0, 0x24 COMMITTED
  PaymentOutcome lup;std::atomic<bool>lup_done{false};
  std::thread t2([&]{lup=bluepad_ble_lookup("00000367");lup_done=true;});
  pump_until(lup_done);assert(lup_done);t2.join();
  if(lup.certainty!=PaymentCertainty::Approved){std::cerr<<"lookup certainty="<<int(lup.certainty)<<" error="<<lup.error_code<<"\n";assert(false);}
  assert(lup.rrn=="RRN123");assert(lup.auth_code=="AUTH01");
  // Only 2 extra writes for lookup (0x1A and 0x24), none for purchase
  assert(fake::txwrites.load()==writes_after_purchase+2);
  std::cout<<"bp-05-1-drop-resolve PASS\n";return 0;
 }

 if(test=="bp-05-2-reverse-block"){
  // Purchase -> UNKNOWN, lookup -> 0x1A returns R -> reverse_pending set
  // New purchase must be blocked without I/O
  fake::mode="ack-only-reverse";setup();assert(bluepad_ble_ready());
  PaymentOutcome pur;std::atomic<bool>pur_done{false};
  std::thread t1([&]{pur=bluepad_ble_purchase("00000367",1);pur_done=true;});
  pump_until(pur_done);assert(pur_done);t1.join();
  assert(pur.certainty==PaymentCertainty::Unknown);
  assert(!bluepad_ble_ready());
  // Lookup: 0x1A returns flag=1 (R), sets reverse_pending
  PaymentOutcome lup;std::atomic<bool>lup_done{false};
  std::thread t2([&]{lup=bluepad_ble_lookup("00000367");lup_done=true;});
  pump_until(lup_done);assert(lup_done);t2.join();
  assert(lup.certainty==PaymentCertainty::Unknown);
  assert(lup.error_code=="BLUEPAD_REVERSE_REQUIRED");
  // New purchase must be blocked, no additional I/O
  int writes_before=fake::txwrites.load();
  PaymentOutcome blocked=bluepad_ble_purchase("00000367",1);
  assert(blocked.certainty==PaymentCertainty::NotSent);
  assert(blocked.error_code=="NEW_PURCHASE_BLOCKED");
  assert(fake::txwrites.load()==writes_before);
  std::cout<<"bp-05-2-reverse-block PASS\n";return 0;
 }

 std::cerr<<"unknown test: "<<test<<"\n";return 1;
}
