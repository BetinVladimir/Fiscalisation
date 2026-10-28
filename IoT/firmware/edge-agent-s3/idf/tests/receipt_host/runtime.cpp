#include <cassert>
#include <algorithm>
#include <fstream>
#include <iostream>
#include <cstring>
#include <deque>
#include <memory>
#include <type_traits>
#ifndef SYNC_SOURCE
#define SYNC_SOURCE "../../main/sync_runtime.cpp"
#endif
#include "profile_executor.h"
#include "device_identity.h"
#include "profile_executor.h"
#include "uart_runtime.h"
#include "bluepad_ble_central.h"
using namespace beefiscal::idf;
static EdgeProfile profile;static std::deque<uint8_t>response;static std::vector<unsigned>commands;static std::string published;static unsigned refund_calls{};static std::string scenario,unp="DT636497-0001-0000042";static bool reversal{};
static void word(std::vector<uint8_t>&v,uint16_t n){for(int s:{12,8,4,0})v.push_back(uint8_t(0x30+((n>>s)&15)));}
static int write_device(const uint8_t*p,size_t n){bool datecs=profile==EdgeProfile::DatecsDp150BluePad50;size_t at=datecs?10:4;assert(n>=at+6&&p[0]==1&&p[n-1]==3);unsigned cmd=datecs?0:p[3];if(datecs)for(size_t i=6;i<10;i++)cmd=(cmd<<4)|(p[i]-0x30);std::string data((const char*)p+at,n-at-6);unsigned index=commands.size();assert(p[datecs?5:2]==0x20+index);
 commands.push_back(cmd);std::string reply=datecs?(cmd==43||cmd==48||cmd==56?"0\t470\t":"0\t"):(cmd==48||cmd==56?"000003,000002":"");
 if(cmd==51)reply=datecs?"0\t470\t2.65\t2.65\t0.00\t0.00\t0.00\t0.00\t0.00\t0.00\t0.00\t":"2.65,2.65,0.00,0.00,0.00,0.00,0.00,0.00,0.00";
 if(cmd==56&&scenario=="bad-close")reply=datecs?"0\t0\t":"bad";
 if(cmd==76)reply=datecs?"0\t0\t470\t1\t2.65\t2.65\t":"0,1,2.65,2.65,0";
 if(cmd==113)reply="470";
 const std::string observed_unp=scenario=="wrong-unp"?"DT636497-0001-0000043":scenario=="substring"?unp+"0":unp;
 const std::string number=scenario=="wrong-number"?"471":"470";
 const std::string date=scenario=="bad-date"?"99-99-2026 14:00:00":"17-10-2026 14:00:00";
 if(cmd==125){if(data.rfind("0\t",0)==0)reply="0\t"+number+"\t470\t"+date+"\t"+std::to_string(scenario=="wrong-type"?2:reversal?6:1)+"\t1\t";else reply=scenario=="line-limit"?"0\tno identifier here\t":"0\tUNP: "+observed_unp+"\t";}
 if(cmd==119){reply="P"+number+"\t"+(scenario=="bad-date"?"99.99.2026 14.00.00":"17.10.2026 14.00.00")+"\t"+(scenario=="not-ejt"?"65":"193")+"\t"+std::to_string(scenario=="wrong-type"?15:reversal?2:0)+"\t1\t0\t"+observed_unp+"\t0";if(reversal)reply+=scenario=="wrong-link"?"\t12345678\t270\t0":scenario=="wrong-fm"?"\t87654321\t269\t0":"\t12345678\t269\t0";}
 const std::string serial=scenario=="wrong-serial"?"DT636498":"DT636497";
 if(cmd==123)reply="0\t"+serial+"\t12345678\tCompany\tAddress\t123456789\tShop\tAddress\t";
 if(cmd==90)reply="VER123 17-10-2026 14:00,ABCD,0000,6,"+serial+",12345678";
 std::vector<uint8_t>frame{1};if(datecs){word(frame,uint16_t(reply.size()+0x33));frame.push_back(p[5]);word(frame,cmd);}else{frame.push_back(uint8_t(reply.size()+0x2b));frame.push_back(p[2]);frame.push_back(uint8_t(cmd));}frame.insert(frame.end(),reply.begin(),reply.end());frame.push_back(4);frame.insert(frame.end(),datecs?8:6,0x80);frame.push_back(5);uint16_t sum=0;for(size_t i=1;i<frame.size();++i)sum+=frame[i];word(frame,sum);frame.push_back(3);response.insert(response.end(),frame.begin(),frame.end());return int(n);}
static int read_device(uint8_t*p,size_t n){size_t got=std::min(n,response.size());for(size_t i=0;i<got;i++){p[i]=response.front();response.pop_front();}return int(got);}
namespace beefiscal::idf {
esp_err_t uart_runtime_start(const FiscalEndpointBinding&){return ESP_OK;}bool uart_runtime_ready(){return true;}int uart_runtime_write(const uint8_t*p,size_t n,uint32_t){assert(profile==EdgeProfile::DatecsDp150BluePad50);return write_device(p,n);}int uart_runtime_read(uint8_t*p,size_t n,uint32_t){return read_device(p,n);}bool usb_cdc_ready(){return true;}int usb_cdc_write(const uint8_t*p,size_t n,uint32_t){assert(profile==EdgeProfile::DaisyCompactS01);return write_device(p,n);}int usb_cdc_read(uint8_t*p,size_t n,uint32_t){return read_device(p,n);}
esp_err_t bluepad_ble_start(const PaymentEndpointBinding&){return ESP_OK;}bool bluepad_ble_ready(){return false;}PaymentOutcome bluepad_ble_purchase(const std::string&,int64_t){assert(false);return{};}PaymentOutcome bluepad_ble_lookup(const std::string&){return{};}PaymentOutcome bluepad_ble_reverse(const std::string&,int64_t){++refund_calls;assert(false);return{};}
DeviceIdentity::DeviceIdentity(){}DeviceIdentity::~DeviceIdentity(){}bool DeviceIdentity::ready()const{return true;}esp_err_t DeviceIdentity::sign_hash_hex(const char*h,std::string&s)const{s=std::string("test-key:")+h;return ESP_OK;}int mqtt_publish_sync(const char*,const uint8_t*p,size_t n){published.assign((const char*)p,n);return 1;}
}
int main(int argc,char**argv){assert(argc==4);profile=std::string(argv[1])=="datecs"?EdgeProfile::DatecsDp150BluePad50:EdgeProfile::DaisyCompactS01;reversal=std::string(argv[2])=="reversal";scenario=argv[3];CompositeBinding b;b.profile=profile;b.authority.unp_prefix="DT636497";auto&c=b.fiscal.execution;c.present=true;c.operator_number=1;c.operator_count=30;c.till_number=profile==EdgeProfile::DatecsDp150BluePad50?1:0;c.operator_password_ref="fixture";c.device_configuration_id="fixture";c.datecs_card_mode=1;c.daisy_card_code='N';EspIdfDeviceIo io(b,profile==EdgeProfile::DatecsDp150BluePad50?"00000000":"0000");ReceiptPlan p;p.operation_id="op";p.receipt_session_id="receipt";p.command_type=reversal?"SALE_REVERSE":"SALE_FINALIZE";p.unp=unp;p.original_document=269;p.original_datetime="11-08-26 12:30:45";p.original_fmin="12345678";p.reversal_reason=1;p.items.push_back({"line","Item","1.000","2.65","A",""});p.payments.push_back({"payment","CASH","2.65",265});ExecutionResult result;
 const std::string action=argv[2];
 if(action.rfind("recover-",0)==0){RecoveryItem i;i.canonical_unp=unp;i.route_snapshot=action=="recover-sale"?"{\"command_type\":\"SALE_FINALIZE\"}":"{\"command_type\":\"REVERSAL\"}";result=io.fiscal_receipt_proof(i);if(action=="recover-reversal"){assert(result.certainty==ExecutionCertainty::Unknown&&commands.empty()&&result.fiscal_reference.empty());return 0;}}
 else { result=reversal?io.fiscal_command(p):io.fiscalize(p); }
 if(scenario=="valid"){assert(result.certainty==ExecutionCertainty::Committed);assert(result.fiscal_reference=="470");}else{assert(result.certainty==ExecutionCertainty::Unknown);assert(result.fiscal_reference.empty());}assert(refund_calls==0);for(unsigned money_cmd:{43u,48u,49u,53u,56u}){assert(std::count(commands.begin(),commands.end(),money_cmd)<=1);if(action.rfind("recover-",0)==0)assert(std::count(commands.begin(),commands.end(),money_cmd)==0);}assert(commands.size()<=102);std::cout<<argv[1]<<" "<<argv[2]<<" "<<scenario<<" receipt proof PASS\n";}
