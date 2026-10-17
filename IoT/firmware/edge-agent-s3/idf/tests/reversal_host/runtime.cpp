#include <cassert>
#include <fstream>
#include <iostream>
#include <cstring>
#include <deque>
#include <memory>
#include <type_traits>
#ifndef SYNC_SOURCE
#define SYNC_SOURCE "../../main/sync_runtime.cpp"
#endif
#include SYNC_SOURCE
#include "profile_executor.h"
#include "uart_runtime.h"
#include "bluepad_ble_central.h"
using namespace beefiscal::idf;
static EdgeProfile profile;static std::deque<uint8_t>response;static std::vector<unsigned>commands;static std::string expected_open,published;static unsigned refund_calls{};static bool modern_profile{};static unsigned reversal_vendor{};
static void word(std::vector<uint8_t>&v,uint16_t n){for(int s:{12,8,4,0})v.push_back(uint8_t(0x30+((n>>s)&15)));}
static int write_device(const uint8_t*p,size_t n){bool datecs=profile==EdgeProfile::DatecsDp150BluePad50;size_t at=datecs?10:4;assert(n>=at+6&&p[0]==1&&p[n-1]==3);unsigned cmd=datecs?0:p[3];if(datecs)for(size_t i=6;i<10;i++)cmd=(cmd<<4)|(p[i]-0x30);std::string data((const char*)p+at,n-at-6);unsigned index=commands.size();const std::vector<unsigned> order=datecs?std::vector<unsigned>{43,49,51,53,56,125,125,123}:std::vector<unsigned>{48,49,51,53,56,113,119,90};assert(index<order.size()&&cmd==order[index]);assert(p[datecs?5:2]==0x20+index);if(index==0){assert(data==expected_open);}
 commands.push_back(cmd);std::string reply=datecs?(cmd==43||cmd==48||cmd==56?"0\t470\t":"0\t"):(cmd==43||cmd==48||cmd==56?"000003,000002":"");if(cmd==51)reply=datecs?"0\t470\t2.65\t2.65\t0.00\t0.00\t0.00\t0.00\t0.00\t0.00\t0.00\t":"2.65,2.65,0.00,0.00,0.00,0.00,0.00,0.00,0.00";if(cmd==113)reply="470";
 if(cmd==119)reply="P470\t17.10.2026 14.00.00\t193\t"+std::to_string(reversal_vendor+1)+"\t1\t0\tDT636497-0001-0000042\t0\t12345678\t269\t0";
 if(cmd==125)reply=data.rfind("0\t",0)==0?"0\t470\t470\t17-10-2026 14:00:00\t6\t1\t":"0\tUNP: DT636497-0001-0000042\t";
 if(cmd==123)reply="0\tDT636497\t12345678\tCompany\tAddress\t123456789\tShop\tAddress\t";
 if(cmd==90)reply="VER123 17-10-2026 14:00,ABCD,0000,6,DT636497,12345678";
 std::vector<uint8_t>frame{1};if(datecs){word(frame,uint16_t(reply.size()+0x33));frame.push_back(p[5]);word(frame,cmd);}else{frame.push_back(uint8_t(reply.size()+0x2b));frame.push_back(p[2]);frame.push_back(uint8_t(cmd));}frame.insert(frame.end(),reply.begin(),reply.end());frame.push_back(4);frame.insert(frame.end(),datecs?8:6,0x80);frame.push_back(5);uint16_t sum=0;for(size_t i=1;i<frame.size();++i)sum+=frame[i];word(frame,sum);frame.push_back(3);response.insert(response.end(),frame.begin(),frame.end());return int(n);}
static int read_device(uint8_t*p,size_t n){size_t got=std::min(n,response.size());for(size_t i=0;i<got;i++){p[i]=response.front();response.pop_front();}return int(got);}
namespace beefiscal::idf {
esp_err_t uart_runtime_start(const FiscalEndpointBinding&){return ESP_OK;}bool uart_runtime_ready(){return true;}int uart_runtime_write(const uint8_t*p,size_t n,uint32_t){assert(profile==EdgeProfile::DatecsDp150BluePad50);return write_device(p,n);}int uart_runtime_read(uint8_t*p,size_t n,uint32_t){return read_device(p,n);}bool usb_cdc_ready(){return true;}int usb_cdc_write(const uint8_t*p,size_t n,uint32_t){assert(profile==EdgeProfile::DaisyCompactS01);return write_device(p,n);}int usb_cdc_read(uint8_t*p,size_t n,uint32_t){return read_device(p,n);}
esp_err_t bluepad_ble_start(const PaymentEndpointBinding&){return ESP_OK;}bool bluepad_ble_ready(){return false;}PaymentOutcome bluepad_ble_purchase(const std::string&,int64_t){assert(false);return{};}PaymentOutcome bluepad_ble_lookup(const std::string&){return{};}PaymentOutcome bluepad_ble_reverse(const std::string&,int64_t){++refund_calls;assert(false);return{};}
DeviceIdentity::DeviceIdentity(){}DeviceIdentity::~DeviceIdentity(){}bool DeviceIdentity::ready()const{return true;}esp_err_t DeviceIdentity::sign_hash_hex(const char*h,std::string&s)const{s=std::string("test-key:")+h;return ESP_OK;}int mqtt_publish_sync(const char*,const uint8_t*p,size_t n){published.assign((const char*)p,n);return 1;}
}
template<class T,class=void>struct FiscalFixture {static void configure(T&,bool){}};
template<class T>struct FiscalFixture<T,std::void_t<decltype(std::declval<T>().execution)>> {
 static void configure(T& fiscal,bool datecs){modern_profile=true;auto& c=fiscal.execution;c.present=true;c.operator_number=1;c.operator_count=30;c.till_number=datecs?1:0;c.operator_password_ref="fixture-pass";c.device_configuration_id="fixture-config";c.datecs_card_mode=1;c.daisy_card_code='N';}
};
template<class T>std::unique_ptr<T> make_io(const CompositeBinding& b,const std::string& password){if constexpr(std::is_constructible_v<T,const CompositeBinding&,std::string>)return std::make_unique<T>(b,password);else return std::make_unique<T>(b);}
int main(int argc,char**argv){assert(argc==6);profile=std::string(argv[1])=="datecs"?EdgeProfile::DatecsDp150BluePad50:EdgeProfile::DaisyCompactS01;std::string reason=argv[5];unsigned code=reason=="OPERATOR_ERROR"?0:reason=="TAX_BASE_REDUCTION"?2:1;unsigned vendor=profile==EdgeProfile::DaisyCompactS01&&code<2?1-code:code;
 reversal_vendor=vendor;expected_open=profile==EdgeProfile::DatecsDp150BluePad50?"1\t00000000\t1\t"+std::to_string(vendor)+"\t269\t11-08-26 12:30:45\t12345678\t\t\t\tDT636497-0001-0000042\t":"1,0000,DT636497-0001-0000042\tR"+std::to_string(vendor)+",269,11-08-26 12:30:45\t12345678";
 std::ifstream f(argv[3]);std::string body(std::istreambuf_iterator<char>(f),{});DurableStorage storage;assert(storage.open_database_for_test(argv[2])==ESP_OK);assert(storage.configure_authority("DT636497-0001-",1,100,1)==ESP_OK);CompositeBinding binding;binding.tenant_id="tenant";binding.register_id="register";binding.edge_device_id="edge";binding.generation=1;binding.authority.unp_prefix="DT636497-0001-";binding.profile=profile;FiscalFixture<FiscalEndpointBinding>::configure(binding.fiscal,profile==EdgeProfile::DatecsDp150BluePad50);auto io=make_io<EspIdfDeviceIo>(binding,profile==EdgeProfile::DatecsDp150BluePad50?"00000000":"0000");ProfileExecutor executor(binding,*io,&storage);IntentProcessor processor(storage,binding,executor,"a2tra2tra2tra2tra2tra2tra2tra2tra2tra2tra2s=");auto rc=processor.accept({(const uint8_t*)body.data(),body.size(),Ingress::Mqtt});if(rc!=ESP_OK)std::cerr<<"actual reversal decode rc="<<rc<<"\n";if(reason=="negative"){assert(rc==ESP_ERR_INVALID_ARG&&commands.empty());return 0;}assert(rc==ESP_OK);const size_t expected_count=8;assert(commands.size()==expected_count&&refund_calls==0);assert(processor.accept({(const uint8_t*)body.data(),body.size(),Ingress::Mqtt})==ESP_OK&&commands.size()==expected_count);
 DeviceIdentity identity;assert(sync_runtime_start(storage,binding,identity,"a2tra2tra2tra2tra2tra2tra2tra2tra2tra2tra2s=")==ESP_OK);assert(publish_once()==ESP_OK);std::ofstream(argv[4])<<published;std::cout<<argv[1]<<" actual reversal vendor sequence PASS "<<reason<<"\n";}
