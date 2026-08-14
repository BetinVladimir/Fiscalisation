#include "profile_executor.h"
#include "CommandPayload.hpp"
#include "FrameCodec.hpp"
#include <cassert>
#include <cstdio>
#include <string>
#include <vector>
using namespace beefiscal::idf;

// The production orchestrator references these only when a real journal is supplied.
esp_err_t DurableStorage::upsert_payment(const char*,const char*,const char*,const char*,int64_t,const char*,const char*,const char*,int64_t){return ESP_OK;}
esp_err_t DurableStorage::set_operation_state(const char*,const char*,const char*,int64_t){return ESP_OK;}

struct ProtocolFaithfulIo final:DeviceIo{
 EdgeProfile profile;bool fiscal_up{true},payment_up{true};
 ExecutionResult next_fiscal{ExecutionCertainty::Committed,"FISCAL-1","","","",""};
 PaymentOutcome next_purchase{PaymentCertainty::Approved,"STAN-1","RRN-1","AUTH-1",""};
 PaymentOutcome next_lookup{PaymentCertainty::Approved,"STAN-1","RRN-1","AUTH-1",""};
 PaymentOutcome next_reverse{PaymentCertainty::Approved,"STAN-1","RRN-1","AUTH-1",""};
 int fiscal_calls{},purchase_calls{},lookup_calls{},reverse_calls{};std::vector<std::string> commands;
 explicit ProtocolFaithfulIo(EdgeProfile value):profile(value){}
 bool fiscal_ready()override{return fiscal_up;}bool payment_ready()override{return payment_up;}
 static uint8_t hex(uint8_t v){return v<10?'0'+v:'A'+v-10;}static void hex_word(std::vector<uint8_t>&v,uint16_t n){v.push_back(hex(n>>12&15));v.push_back(hex(n>>8&15));v.push_back(hex(n>>4&15));v.push_back(hex(n&15));}static void nibble_word(std::vector<uint8_t>&v,uint16_t n){for(int s:{12,8,4,0})v.push_back(uint8_t(0x30+((n>>s)&15)));}
 bool roundtrip(uint16_t command,const std::vector<uint8_t>&payload){const uint8_t seq=0x20;std::vector<uint8_t>response;if(profile==EdgeProfile::DaisyCompactS01){response={1,uint8_t(payload.size()+0x2a),seq,(uint8_t)command};response.insert(response.end(),payload.begin(),payload.end());response.push_back(4);response.insert(response.end(),6,0);response.push_back(5);uint16_t sum=0;for(size_t i=1;i<response.size();i++)sum+=response[i];hex_word(response,sum);}else{response={1};nibble_word(response,uint16_t(payload.size()+0x2f));response.push_back(seq);nibble_word(response,command);response.insert(response.end(),payload.begin(),payload.end());response.push_back(4);response.insert(response.end(),8,0);response.push_back(5);uint16_t sum=0;for(size_t i=1;i<response.size();i++)sum+=response[i];nibble_word(response,sum);}response.push_back(3);bee::ParsedFrame decoded;std::string error;return(profile==EdgeProfile::DaisyCompactS01?bee::DaisyCodec::decode(response,decoded,error):bee::DatecsCodec::decode(response,decoded,error))&&decoded.command==command&&decoded.sequence==seq&&decoded.data==payload;}
 ExecutionResult fiscalize(const ReceiptPlan&p)override{fiscal_calls++;commands.push_back(profile==EdgeProfile::DaisyCompactS01?"DAISY_USB_RECEIPT":"DATECS_UART_RECEIPT");std::vector<uint8_t>payload;std::string error;bee::OpenReceiptPayload open{1,"0000",p.unp.empty()?"DY000600-OP01-0000001":p.unp,1,false};const bool built=profile==EdgeProfile::DaisyCompactS01?bee::buildDaisyOpenReceipt(open,payload,error):bee::buildDatecsOpenReceipt(open,payload,error);if(!built||!roundtrip(48,payload))return{ExecutionCertainty::Rejected,"","PROTOCOL_FAKE_REJECTED","","",""};return next_fiscal;}
 ExecutionResult fiscal_command(const ReceiptPlan&p)override{fiscal_calls++;commands.push_back((profile==EdgeProfile::DaisyCompactS01?"DAISY_USB_":"DATECS_UART_")+p.command_type);std::vector<uint8_t>payload;std::string error;uint16_t command=0;bool built=false;if(p.command_type=="REPORT_X"||p.command_type=="REPORT_Z"){command=69;built=profile==EdgeProfile::DaisyCompactS01?bee::buildDaisyDailyReport(p.command_type=="REPORT_Z",payload):bee::buildDatecsDailyReport(p.command_type=="REPORT_Z",payload);}else if(p.command_type=="CASH_IN"||p.command_type=="CASH_OUT"){command=70;bee::CashMovementPayload value{"10.00",p.command_type=="CASH_OUT"};built=profile==EdgeProfile::DaisyCompactS01?bee::buildDaisyCashMovement(value,payload,error):bee::buildDatecsCashMovement(value,payload,error);}else if(p.command_type=="SALE_REVERSE"){command=profile==EdgeProfile::DaisyCompactS01?48:43;bee::ReversalOpenPayload value{1,profile==EdgeProfile::DaisyCompactS01?"0000":"00000000",1,1,42,"11-08-26 12:30:45","12345678",false,"","","DY000600-OP01-0000001"};built=profile==EdgeProfile::DaisyCompactS01?bee::buildDaisyOpenReversal(value,payload,error):bee::buildDatecsOpenReversal(value,payload,error);}if(!built||!roundtrip(command,payload))return{ExecutionCertainty::Rejected,"","PROTOCOL_FAKE_REJECTED","","",""};return next_fiscal;}
 PaymentOutcome purchase(const std::string&,int64_t)override{purchase_calls++;commands.push_back("BLUEPAD_BLE_PURCHASE");return next_purchase;}
 PaymentOutcome payment_lookup(const std::string&)override{lookup_calls++;commands.push_back("BLUEPAD_BLE_LOOKUP");return next_lookup;}
 PaymentOutcome reverse(const std::string&,int64_t)override{reverse_calls++;commands.push_back("BLUEPAD_BLE_VOID");return next_reverse;}
 ExecutionResult recover(const RecoveryItem&)override{lookup_calls++;commands.push_back(profile==EdgeProfile::DaisyCompactS01?"DAISY_USB_LOOKUP":"DATECS_UART_LOOKUP");return next_fiscal;}
};
ReceiptPlan plan(const std::string&kind,const std::string&payment="CASH") {ReceiptPlan p;p.operation_id="11111111-1111-4111-8111-111111111111";p.receipt_session_id="22222222-2222-4222-8222-222222222222";p.unp="DY000600-OP01-0000001";p.command_type=kind;p.payments.push_back({"pay",payment,"10.00",1000});return p;}
CompositeBinding binding(EdgeProfile profile){CompositeBinding b;b.profile=profile;b.payment.present=profile==EdgeProfile::DatecsDp150BluePad50;return b;}

 void protocol_corruption_matrix(EdgeProfile profile){ProtocolFaithfulIo io(profile);std::vector<uint8_t>payload{'O','K'};assert(io.roundtrip(69,payload));const uint8_t seq=0x20;auto frame=profile==EdgeProfile::DaisyCompactS01?bee::DaisyCodec::encode(seq,69,payload):bee::DatecsCodec::encode(seq,69,payload);assert(!frame.empty());frame[frame.size()-2]^=1;bee::ParsedFrame parsed;std::string error;assert(!(profile==EdgeProfile::DaisyCompactS01?bee::DaisyCodec::decode(frame,parsed,error):bee::DatecsCodec::decode(frame,parsed,error)));}
void fiscal_matrix(EdgeProfile profile){auto b=binding(profile);ProtocolFaithfulIo io(profile);ProfileExecutor x(b,io);
 for(const auto&kind:{"REPORT_X","REPORT_Z","CASH_IN","CASH_OUT","SALE_REVERSE"}){auto r=x.execute(plan(kind));if(r.certainty!=ExecutionCertainty::Committed)std::fprintf(stderr,"profile command failed: %s %s\n",kind,r.error_code.c_str());assert(r.certainty==ExecutionCertainty::Committed);}
 assert(io.commands.size()==5);
}
void receipt_fault_matrix(EdgeProfile profile){auto b=binding(profile);ProtocolFaithfulIo io(profile);ProfileExecutor x(b,io);
 assert(x.execute(plan("SALE_FINALIZE")).certainty==ExecutionCertainty::Committed);
 if(profile==EdgeProfile::DaisyCompactS01){assert(x.execute(plan("SALE_FINALIZE","CARD")).certainty==ExecutionCertainty::DeviceUnavailable);return;}
 assert(x.execute(plan("SALE_FINALIZE","CARD")).certainty==ExecutionCertainty::Committed);
 auto split=plan("SALE_FINALIZE","CASH");split.payments.push_back({"card","CARD","5.00",500});assert(x.execute(split).certainty==ExecutionCertainty::Committed);
 io.next_purchase.certainty=PaymentCertainty::Declined;assert(x.execute(plan("SALE_FINALIZE","CARD")).certainty==ExecutionCertainty::Rejected);io.next_purchase.certainty=PaymentCertainty::Unknown;assert(x.execute(plan("SALE_FINALIZE","CARD")).certainty==ExecutionCertainty::Unknown);
 io.next_purchase.certainty=PaymentCertainty::Approved;io.next_fiscal.certainty=ExecutionCertainty::Rejected;assert(x.execute(plan("SALE_FINALIZE","CARD")).certainty==ExecutionCertainty::Compensated);
 io.next_reverse.certainty=PaymentCertainty::Unknown;assert(x.execute(plan("SALE_FINALIZE","CARD")).certainty==ExecutionCertainty::Unknown);
 io.next_reverse.certainty=PaymentCertainty::Approved;io.payment_up=false;assert(x.execute(plan("SALE_FINALIZE","CARD")).certainty==ExecutionCertainty::DeviceUnavailable);io.payment_up=true;
 RecoveryItem recovery{};recovery.operation_id="11111111-1111-4111-8111-111111111111";assert(x.recover(recovery).certainty==io.next_fiscal.certainty);
 io.fiscal_up=false;assert(x.execute(plan("SALE_FINALIZE")).certainty==ExecutionCertainty::DeviceUnavailable);
}
int main(){for(auto profile:{EdgeProfile::DatecsDp150BluePad50,EdgeProfile::DaisyCompactS01}){protocol_corruption_matrix(profile);fiscal_matrix(profile);receipt_fault_matrix(profile);}return 0;}
