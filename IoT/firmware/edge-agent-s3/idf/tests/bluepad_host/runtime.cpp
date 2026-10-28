#include "fake_bluepad.h"
#include <cassert>
#include <cstring>
#include <iostream>
#include <thread>
#include <chrono>
#include <functional>
#include <deque>
#include <atomic>
#include "../../main/bluepad_ble_central.cpp"
using namespace beefiscal::idf;
namespace fake {
std::mutex mutex;std::deque<std::function<void()>> events;std::string mode;std::vector<uint8_t> incoming;std::vector<std::vector<uint8_t>> commands;
ble_gap_event_fn*gcb;void*garg;std::atomic<int> txwrites{0};int setupwrites=0;bool finished=false;std::atomic<ble_gatt_attr_fn*> held{nullptr};std::atomic<void*> heldarg{nullptr};
const auto origin=std::chrono::steady_clock::now();std::atomic<int64_t> extra{0};
void enqueue(std::function<void()> f){std::lock_guard<std::mutex>l(mutex);events.push_back(f);}
bool pump(){std::function<void()> f;{std::lock_guard<std::mutex>l(mutex);if(events.empty())return false;f=events.front();events.pop_front();}f();return true;}
std::vector<uint8_t> hex(const char*t){std::vector<uint8_t>b;while(*t){unsigned n=0;for(int i=0;i<2;++i){char c=*t++;n=n*16+(c<='9'?c-'0':c-'a'+10);}b.push_back(n);}return b;}
// Independent device-side packet builder, never calls production encoder or TLV helpers.
std::vector<uint8_t> device(uint8_t command,std::vector<uint8_t>b){std::vector<uint8_t>o{0x3e,command,0,uint8_t(b.size()>>8),uint8_t(b.size())};o.insert(o.end(),b.begin(),b.end());uint8_t x=0;for(auto v:o)x^=v;o.push_back(x);return o;}
void notify(std::vector<uint8_t>b,uint16_t handle=5,void*arg=nullptr){os_mbuf m{b};ble_gap_event e{};e.type=BLE_GAP_EVENT_NOTIFY_RX;e.notify_rx.conn_handle=7;e.notify_rx.attr_handle=handle;e.notify_rx.om=&m;gcb(&e,arg?arg:garg);}
void disconnect(){ble_gap_event e{};e.type=BLE_GAP_EVENT_DISCONNECT;e.disconnect.conn.conn_handle=7;gcb(&e,garg);}
void respond(const std::vector<uint8_t>&cmd){
 commands.push_back(cmd);assert(cmd.size()>=7);uint8_t sub=cmd[5];
 if(sub==1){assert(cmd==hex("3e3d0000110103810400000001df030631323334353649"));
  if(mode=="early-final"){notify(device(0x0e,hex("01df050400000000df0604000000008104000000019f410400000367")));return;}
  if(mode!="coalesced")notify(hex("3e000000003e"));
  if(mode=="intermediate"||mode=="intermediate-only")notify(device(0x0e,{2}));
  if(mode=="intermediate-only"||mode=="ack-only")return;
  auto final=hex("01df050400000000df0604000000008104000000019f410400000367");
  if(mode=="final-missing")final=hex("01df050400000000");
  if(mode=="declined")final[7]=1;
  if(mode=="result-error")final[7]=2;
  if(mode=="result-retry")final[7]=4;
  if(mode=="bad-bcc"){auto f=device(0x0e,final);f.back()^=1;notify(f);return;}
  if(mode=="oversize"){notify(hex("3e0e00ffff"));return;}
  if(mode=="fragment-timeout"){notify(hex("3e0e00001c01"));return;}
  if(mode=="queue-overflow"){for(int i=0;i<10;++i)notify(device(0x0e,{2}));return;}
  if(mode=="notification-overflow"){notify(std::vector<uint8_t>(515,0));return;}
  auto frame=device(0x0e,final);if(mode=="coalesced"){auto ack=hex("3e000000003e");ack.insert(ack.end(),frame.begin(),frame.end());notify(ack);}else if(mode=="fragmented"){for(auto b:frame)notify({b});}else notify(frame);
 }else if(sub==2){assert(cmd==device(0x3d,hex("02df0381df109f41df05df06df07df08")));
  auto receipt=hex("df0306313233343536810400000001df100200039f410400000367df050400000000df060400000000df070652524e313233df0806415554483031");
  if(mode=="declined")receipt[33]=1;
  if(mode=="result-error")receipt[33]=2;
  if(mode=="result-retry")receipt[33]=4;
  if(mode=="invalid-stan")receipt[26]=0xfa;
  if(mode=="foreign-df03")receipt[3]='X';
  if(mode=="missing-tags")receipt=hex("df0306313233343536");
  if(mode=="wrong-type")receipt[19]=1;
  if(mode=="wrong-stan")receipt[26]=0x68;
  if(mode=="duplicate-tag"){auto d=hex("df0306313233343536");receipt.insert(receipt.end(),d.begin(),d.end());}
  if(mode=="invalid-ber")receipt=hex("df0382ffff00");
  notify(device(0,receipt));
 }else if(sub==3){assert(cmd==hex(mode=="declined"?"3e3d00000303000003":"3e3d00000303000102"));if(mode=="end-failure")notify(device(0,{1}));else notify(hex("3e000000003e"));}
 else assert(false);
}
}
int ble_uuid_from_str(ble_uuid_any_t*u,const char*t){std::strncpy(u->u.text,t,39);return 0;}int ble_uuid_cmp(const ble_uuid_t*a,const ble_uuid_t*b){return std::strcmp(a->text,b->text);}uint16_t ble_uuid_u16(const ble_uuid_t*u){return u->value;}
int ble_hs_id_infer_auto(int,uint8_t*v){*v=0;return 0;}int ble_hs_adv_parse_fields(ble_hs_adv_fields*f,const uint8_t*p,uint8_t n){f->name=(uint8_t*)p;f->name_len=n;return 0;}
int ble_gap_disc(uint8_t,int,const ble_gap_disc_params*,ble_gap_event_fn*cb,void*a){fake::gcb=cb;fake::garg=a;return 0;}int ble_gap_disc_cancel(){return 0;}int ble_gap_connect(uint8_t,const ble_addr_t*,int,const void*,ble_gap_event_fn*cb,void*a){fake::gcb=cb;fake::garg=a;fake::enqueue([=]{ble_gap_event e{};e.type=BLE_GAP_EVENT_CONNECT;e.connect.conn_handle=7;cb(&e,a);});return 0;}int ble_gap_terminate(uint16_t,uint8_t){return 0;}
int ble_gattc_exchange_mtu(uint16_t,ble_gatt_mtu_fn*,void*){return 0;}
int ble_gattc_disc_svc_by_uuid(uint16_t c,const ble_uuid_t*u,ble_gatt_disc_svc_fn*cb,void*a){auto uuid=*u;fake::enqueue([=]{ble_gatt_error ok{},done{BLE_HS_EDONE};ble_gatt_svc v{};v.start_handle=1;v.end_handle=12;v.uuid.u=uuid;cb(c,&ok,&v,a);cb(c,&done,nullptr,a);});return 0;}
int ble_gattc_disc_all_chrs(uint16_t c,uint16_t first,uint16_t last,ble_gatt_chr_fn*cb,void*a){assert(first==1&&last==12);fake::enqueue([=]{ble_gatt_error ok{},done{BLE_HS_EDONE};const char*uuid[]={kWrite,kRead,kPower,kWake};uint16_t def[]={2,4,7,10};for(int i=0;i<4;++i){if(fake::mode=="missing-power"&&i==2)continue;ble_gatt_chr v{};v.def_handle=def[i];v.val_handle=def[i]+1;v.properties=(i==0||i==3)?BLE_GATT_CHR_PROP_WRITE:BLE_GATT_CHR_PROP_NOTIFY;ble_uuid_from_str(&v.uuid,uuid[i]);cb(c,&ok,&v,a);}cb(c,&done,nullptr,a);});return 0;}
int ble_gattc_disc_all_dscs(uint16_t c,uint16_t first,uint16_t last,ble_gatt_dsc_fn*cb,void*a){assert((first==5&&last==6)||(first==8&&last==9));fake::enqueue([=]{ble_gatt_error ok{},done{BLE_HS_EDONE};ble_gatt_dsc v{};v.handle=first+1;v.uuid.u.value=0x2902;cb(c,&ok,first,&v,a);cb(c,&done,first,nullptr,a);});return 0;}
int ble_gattc_read(uint16_t c,uint16_t handle,ble_gatt_attr_fn*cb,void*a){assert(handle==8);fake::enqueue([=]{ble_gatt_error ok{uint16_t(fake::mode=="power-failure"?1:0)};os_mbuf m{{uint8_t(fake::mode=="wake"||fake::mode=="wake-failure"?0x30:0x31)}};ble_gatt_attr v{handle,0,&m};cb(c,&ok,&v,a);if(fake::mode=="wake"||fake::mode=="wake-failure")fake::enqueue([]{fake::notify({0x31},8);});});return 0;}
int ble_gattc_write_flat(uint16_t c,uint16_t handle,const void*p,uint16_t n,ble_gatt_attr_fn*cb,void*a){std::vector<uint8_t>b((const uint8_t*)p,(const uint8_t*)p+n);
 if(handle!=3){++fake::setupwrites;assert(!bluepad_ble_ready());if(handle==11)assert(b==std::vector<uint8_t>({'B','l','u','e','P','a','d'}));else assert(b==std::vector<uint8_t>({1,0}));
  if(fake::mode=="cccd-timeout")return 0;
  fake::enqueue([=]{ble_gatt_error e{uint16_t((fake::mode=="cccd-failure"||(fake::mode=="wake-failure"&&handle==11))?1:0)};ble_gatt_attr v{handle,0,nullptr};cb(c,&e,e.status?nullptr:&v,a);});return 0;}
 const int count=++fake::txwrites;assert(n<=19);
 if(fake::mode=="immediate-failure"&&count==1)return 1;
 if(fake::mode=="partial-immediate"&&count==2)return 1;
 if(fake::mode=="write-timeout"&&count==1)return 0;
 if(fake::mode=="completion-held"&&count==1){fake::heldarg.store(a);fake::held.store(cb);return 0;}
 fake::enqueue([=]{
  if(fake::mode=="disconnect"&&count==1){fake::disconnect();return;}
  if(fake::mode=="host-reset"&&count==1){bluepad_ble_on_host_reset();return;}
  ble_gatt_error e{uint16_t(fake::mode=="completion-failure"&&count==1?1:0)};ble_gatt_attr v{handle,0,nullptr};cb(c,&e,e.status?nullptr:&v,a);if(e.status)return;
  fake::incoming.insert(fake::incoming.end(),b.begin(),b.end());if(fake::incoming.size()>=5){size_t size=(size_t(fake::incoming[3])<<8)+fake::incoming[4]+6;if(fake::incoming.size()==size){auto cmd=fake::incoming;fake::incoming.clear();fake::respond(cmd);}}
 });return 0;
}
int os_mbuf_copydata(const os_mbuf*m,int offset,int n,void*p){if(offset<0||n<0||size_t(offset+n)>m->bytes.size())return 1;std::memcpy(p,m->bytes.data()+offset,n);return 0;}
SemaphoreHandle_t xSemaphoreCreateBinary(){return new FakeSemaphore;}
int xSemaphoreTake(SemaphoreHandle_t s,uint32_t ticks){std::unique_lock<std::mutex>l(s->mutex);if(!s->given&&ticks)s->cv.wait_for(l,std::chrono::microseconds(int64_t(ticks)*10),[&]{return s->given;});if(!s->given)return 0;s->given=false;return 1;}
int xSemaphoreGive(SemaphoreHandle_t s){{std::lock_guard<std::mutex>l(s->mutex);s->given=true;}s->cv.notify_one();return 1;}
int64_t esp_timer_get_time(){return 1000000+fake::extra+std::chrono::duration_cast<std::chrono::microseconds>(std::chrono::steady_clock::now()-fake::origin).count()*100;}
void setup(){PaymentEndpointBinding b;b.present=true;b.vendor="DATECS";b.model="BLUEPAD-50 PLUS";b.transport="BLE_GATT";b.ble_identity="BluePad";b.service_uuid="d839fc3c-84dd-4c36-9126-000000000001";b.tx_characteristic_uuid=kWrite;b.rx_characteristic_uuid=kRead;assert(bluepad_ble_start(b)==ESP_OK);assert(bluepad_ble_start(b)==ESP_ERR_INVALID_STATE);bluepad_ble_on_host_sync();ble_gap_event e{};e.type=BLE_GAP_EVENT_DISC;std::string name="BluePad";e.disc.data=(uint8_t*)name.data();e.disc.length_data=name.size();fake::gcb(&e,fake::garg);while(fake::pump()){}fake::extra+=3000000;}
int main(int argc,char**argv){assert(argc==2);fake::mode=argv[1];setup();
 if(fake::mode=="cccd-timeout"||fake::mode=="cccd-failure"||fake::mode=="missing-power"||fake::mode=="power-failure"||fake::mode=="wake-failure"){assert(!bluepad_ble_ready());fake::extra+=31000000;assert(!bluepad_ble_ready());std::cout<<argv[1]<<" PASS\n";return 0;}
 assert(bluepad_ble_ready());
 if(fake::mode=="stale-generation"){void*old=fake::garg;fake::disconnect();while(fake::pump()){}ble_gap_event d{};d.type=BLE_GAP_EVENT_CONNECT;d.connect.conn_handle=7;fake::gcb(&d,old);assert(!bluepad_ble_ready());std::cout<<argv[1]<<" PASS\n";return 0;}
 assert(bluepad_ble_lookup("123456").certainty==PaymentCertainty::NotSent);assert(bluepad_ble_reverse("123456",1).certainty==PaymentCertainty::NotSent);assert(fake::txwrites==0);
 PaymentOutcome result;std::atomic<bool> done{false};std::thread worker([&]{result=bluepad_ble_purchase("123456",1);done=true;});
 auto limit=std::chrono::steady_clock::now()+std::chrono::seconds(5);bool held_checked=false;
 while(!done&&std::chrono::steady_clock::now()<limit){if(!fake::pump())std::this_thread::sleep_for(std::chrono::microseconds(50));
 if(fake::mode=="completion-held"&&fake::held&&!held_checked){held_checked=true;assert(fake::txwrites==1);assert(bluepad_ble_purchase("123456",1).certainty==PaymentCertainty::NotSent);auto old=fake::held.load();auto arg=fake::heldarg.load();ble_gatt_error e{1};old(7,&e,nullptr,reinterpret_cast<void*>(reinterpret_cast<uintptr_t>(arg)+1));assert(fake::txwrites==1);old(7,&e,nullptr,arg);}}
 assert(done);worker.join();
 const bool success=fake::mode=="approved"||fake::mode=="intermediate"||fake::mode=="wake"||fake::mode=="coalesced"||fake::mode=="fragmented";
 if(success){assert(result.certainty==PaymentCertainty::Approved);assert(result.reference=="00000367"&&result.rrn=="RRN123"&&result.auth_code=="AUTH01");assert(fake::commands.size()==3);assert(bluepad_ble_ready());}
 else if(fake::mode=="declined"){assert(result.certainty==PaymentCertainty::Declined);assert(fake::commands.size()==3);assert(bluepad_ble_ready());}
 else if(fake::mode=="immediate-failure"){assert(result.certainty==PaymentCertainty::NotSent);assert(bluepad_ble_ready());}
 else {if(result.certainty!=PaymentCertainty::Unknown)std::cerr<<"unexpected "<<int(result.certainty)<<" "<<result.error_code<<"\n";assert(result.certainty==PaymentCertainty::Unknown);assert(!bluepad_ble_ready());int before=fake::txwrites;fake::notify(fake::hex("3e000000003e"));assert(bluepad_ble_purchase("123456",1).certainty==PaymentCertainty::NotSent);assert(fake::txwrites==before);}
 std::cout<<argv[1]<<" PASS "<<result.error_code<<"\n";
}
