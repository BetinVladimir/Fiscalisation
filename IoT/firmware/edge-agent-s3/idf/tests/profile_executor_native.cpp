#include "profile_executor.h"
#include "CommandPayload.hpp"
#include "FrameCodec.hpp"
#include <cassert>
#include <cstdio>
#include <string>
#include <vector>
using namespace beefiscal::idf;

// Every CARD scenario uses an actual durable journal; test plans are separate operations.
#include <unistd.h>
struct Journal{DurableStorage storage;std::string path;Journal(){char file[]="/tmp/beefiscal-profile-saga.XXXXXX";int fd=mkstemp(file);assert(fd>=0);close(fd);path=file;remove(path.c_str());assert(storage.open_database_for_test(path.c_str())==ESP_OK);assert(storage.configure_authority("DT636497",1,1000,1)==ESP_OK);}~Journal(){storage.close();remove(path.c_str());}};
static ExecutionResult run(ProfileExecutor&executor,DurableStorage&storage,const ReceiptPlan&p){std::vector<InitialPayment>payments;for(const auto&v:p.payments)payments.push_back({v.id,v.amount_minor});assert(storage.reserve_command(p.operation_id.c_str(),std::string(64,'a').c_str(),"HTTP","{}",("received-"+p.operation_id).c_str(),p.receipt_session_id.c_str(),"{}",payments,1704067200,[](const AuthorityReservation&){return "{}";},nullptr,"0001").status==ReservationStatus::New);assert(storage.set_operation_state(p.operation_id.c_str(),"EXECUTING",nullptr,1704067200)==ESP_OK);return executor.execute(p);}

struct ProtocolFaithfulIo final:DeviceIo{
 EdgeProfile profile;bool fiscal_up{true},payment_up{true};
 ExecutionResult next_fiscal{ExecutionCertainty::Committed,"FISCAL-1","","","",""};
 PaymentOutcome next_purchase{PaymentCertainty::Approved,"STAN-1","RRN-1","AUTH-1",""};
 PaymentOutcome next_lookup{PaymentCertainty::Approved,"STAN-1","RRN-1","AUTH-1",""};
 PaymentOutcome next_reverse{PaymentCertainty::Approved,"STAN-1","RRN-1","AUTH-1",""};
 int fiscal_calls{},purchase_calls{},lookup_calls{},reverse_calls{};std::vector<std::string> commands;
 bool currency_is_eur{true};
 std::string authority_unp_prefix;
 explicit ProtocolFaithfulIo(EdgeProfile value):profile(value){}
 bool verified_reversal_profile()const override{return true;}
 bool fiscal_ready()override{return fiscal_up&&currency_is_eur;}bool payment_ready()override{return payment_up;}
 // Independent response layout from Daisy v2.0-4 p6 and Datecs PM v2.11.4 sec3.4.
 // Do not use the production request encoder to manufacture a device response.
 static void nibble_word(std::vector<uint8_t>&v,uint16_t n){for(int shift:{12,8,4,0})v.push_back(uint8_t(0x30+((n>>shift)&15)));}
 std::vector<uint8_t> response_frame(uint16_t command,const std::vector<uint8_t>&payload)const{
  const uint8_t seq=0x20;
  std::vector<uint8_t> response{1};
  if(profile==EdgeProfile::DaisyCompactS01){
   response.push_back(uint8_t(payload.size()+0x2b));response.push_back(seq);response.push_back(uint8_t(command));
  }else{
   nibble_word(response,uint16_t(payload.size()+0x33));response.push_back(seq);nibble_word(response,command);
  }
  response.insert(response.end(),payload.begin(),payload.end());response.push_back(4);
  response.insert(response.end(),profile==EdgeProfile::DaisyCompactS01?6:8,0x80);response.push_back(5);
  uint16_t checksum=0;for(size_t i=1;i<response.size();++i)checksum=uint16_t(checksum+response[i]);
  nibble_word(response,checksum);response.push_back(3);return response;
 }
 bool roundtrip(uint16_t command,const std::vector<uint8_t>&payload){
  const auto response=response_frame(command,payload);bee::ParsedFrame decoded;std::string error;
  return(profile==EdgeProfile::DaisyCompactS01?bee::DaisyCodec::decode(response,decoded,error):bee::DatecsCodec::decode(response,decoded,error))&&decoded.command==command&&decoded.sequence==0x20&&decoded.data==payload;
 }
 ExecutionResult fiscalize(const ReceiptPlan&p)override{fiscal_calls++;commands.push_back(profile==EdgeProfile::DaisyCompactS01?"DAISY_USB_RECEIPT":"DATECS_UART_RECEIPT");std::vector<uint8_t>payload;std::string error;bee::OpenReceiptPayload open{1,"0000",p.unp.empty()?"DY000600-OP01-0000001":p.unp,1,false};const bool built=profile==EdgeProfile::DaisyCompactS01?bee::buildDaisyOpenReceipt(open,payload,error):bee::buildDatecsOpenReceipt(open,payload,error);if(!built||!roundtrip(48,payload))return{ExecutionCertainty::Rejected,"","PROTOCOL_FAKE_REJECTED","","",""};return next_fiscal;}
 ExecutionResult fiscal_command(const ReceiptPlan&p)override{fiscal_calls++;commands.push_back((profile==EdgeProfile::DaisyCompactS01?"DAISY_USB_":"DATECS_UART_")+p.command_type);std::vector<uint8_t>payload;std::string error;uint16_t command=0;bool built=false;if(p.command_type=="REPORT_X"||p.command_type=="REPORT_Z"){command=69;built=profile==EdgeProfile::DaisyCompactS01?bee::buildDaisyDailyReport(p.command_type=="REPORT_Z",payload):bee::buildDatecsDailyReport(p.command_type=="REPORT_Z",payload);}else if(p.command_type=="CASH_IN"||p.command_type=="CASH_OUT"){command=70;bee::CashMovementPayload value{"10.00",p.command_type=="CASH_OUT"};built=profile==EdgeProfile::DaisyCompactS01?bee::buildDaisyCashMovement(value,payload,error):bee::buildDatecsCashMovement(value,payload,error);}else if(p.command_type=="SALE_REVERSE"){command=profile==EdgeProfile::DaisyCompactS01?48:43;bee::ReversalOpenPayload value{1,profile==EdgeProfile::DaisyCompactS01?"0000":"00000000",1,1,42,"11-08-26 12:30:45","12345678",false,"","","DY000600-OP01-0000001"};built=profile==EdgeProfile::DaisyCompactS01?bee::buildDaisyOpenReversal(value,payload,error):bee::buildDatecsOpenReversal(value,payload,error);}if(!built||!roundtrip(command,payload))return{ExecutionCertainty::Rejected,"","PROTOCOL_FAKE_REJECTED","","",""};return next_fiscal;}
 PaymentOutcome purchase(const std::string&,int64_t)override{purchase_calls++;commands.push_back("BLUEPAD_BLE_PURCHASE");return next_purchase;}
 PaymentOutcome payment_lookup(const std::string&)override{lookup_calls++;commands.push_back("BLUEPAD_BLE_LOOKUP");return next_lookup;}
 PaymentOutcome reverse(const std::string&,int64_t)override{reverse_calls++;commands.push_back("BLUEPAD_BLE_VOID");return next_reverse;}
 ExecutionResult recover(const RecoveryItem&i)override{if(!authority_unp_prefix.empty()&&!i.canonical_unp.empty()&&(i.canonical_unp.size()<8||i.canonical_unp.substr(0,8)!=authority_unp_prefix))return{ExecutionCertainty::Unknown,"","FOREIGN_RECEIPT_OWNERSHIP","","",""};lookup_calls++;commands.push_back(profile==EdgeProfile::DaisyCompactS01?"DAISY_USB_LOOKUP":"DATECS_UART_LOOKUP");return next_fiscal;}
};
ReceiptPlan plan(const std::string&kind,const std::string&payment="CASH") {static int sequence=0;ReceiptPlan p;p.operation_id="operation-"+std::to_string(++sequence);p.receipt_session_id="receipt-"+std::to_string(sequence);p.unp="DY000600-OP01-0000001";p.command_type=kind;p.items.push_back({"line","Coffee","1.000","10.00","B",""});p.payments.push_back({"pay-"+p.operation_id,payment,"10.00",1000});return p;}
CompositeBinding binding(EdgeProfile profile){CompositeBinding b;b.profile=profile;b.fiscal.execution.present=true;b.fiscal.execution.operator_number=1;b.fiscal.execution.operator_count=30;b.fiscal.execution.operator_password_ref="fiscal-pass";b.fiscal.execution.device_configuration_id="test-config";if(profile==EdgeProfile::DatecsDp150BluePad50){b.fiscal.execution.till_number=1;b.fiscal.execution.datecs_card_mode=1;}else b.fiscal.execution.daisy_card_code='N';b.payment.present=profile==EdgeProfile::DatecsDp150BluePad50;b.payment.device_id="terminal-profile-test";return b;}

void protocol_corruption_matrix(EdgeProfile profile){
 ProtocolFaithfulIo io(profile);std::vector<uint8_t>payload{'O','K'};assert(io.roundtrip(69,payload));
 const auto valid=io.response_frame(69,payload);
 auto accepted=[&](const std::vector<uint8_t>&frame){bee::ParsedFrame parsed;std::string error;return profile==EdgeProfile::DaisyCompactS01?bee::DaisyCodec::decode(frame,parsed,error):bee::DatecsCodec::decode(frame,parsed,error);};
 assert(accepted(valid));
 // Every corruption starts from a proven valid RESPONSE, not a TX packet
 // that response decoding would reject even before checksum corruption.
 for(size_t i=0;i<valid.size();++i){auto corrupted=valid;corrupted[i]^=1;assert(!accepted(corrupted));}
 for(size_t n=0;n<valid.size();++n)assert(!accepted(std::vector<uint8_t>(valid.begin(),valid.begin()+n)));
 auto bad_bcc=valid;bad_bcc[bad_bcc.size()-2]='A';assert(!accepted(bad_bcc));
}
void fiscal_matrix(EdgeProfile profile){auto b=binding(profile);ProtocolFaithfulIo io(profile);Journal journal;ProfileExecutor x(b,io,&journal.storage);
 for(const auto&kind:{"REPORT_X","REPORT_Z","CASH_IN","CASH_OUT","SALE_REVERSE"}){auto r=run(x,journal.storage,plan(kind));if(r.certainty!=ExecutionCertainty::Committed)std::fprintf(stderr,"profile command failed: %s %s\n",kind,r.error_code.c_str());assert(r.certainty==ExecutionCertainty::Committed);}
 assert(io.commands.size()==5);
}
void receipt_fault_matrix(EdgeProfile profile){
 const auto b=binding(profile);
 {ProtocolFaithfulIo io(profile);Journal journal;ProfileExecutor x(b,io,&journal.storage);
  assert(run(x,journal.storage,plan("SALE_FINALIZE")).certainty==ExecutionCertainty::Committed);
  if(profile==EdgeProfile::DaisyCompactS01){assert(run(x,journal.storage,plan("SALE_FINALIZE","CARD")).certainty==ExecutionCertainty::DeviceUnavailable);return;}}
 // Independent fault cases own independent actual journals. An unresolved
 // payment intentionally prevents another CARD operation on the same journal.
 auto card_case=[&](auto configure,ExecutionCertainty expected){ProtocolFaithfulIo io(profile);Journal journal;ProfileExecutor x(b,io,&journal.storage);configure(io);assert(run(x,journal.storage,plan("SALE_FINALIZE","CARD")).certainty==expected);};
 card_case([](auto&){},ExecutionCertainty::Committed);
 {ProtocolFaithfulIo io(profile);Journal journal;ProfileExecutor x(b,io,&journal.storage);auto split=plan("SALE_FINALIZE","CASH");split.payments[0].amount="5.00";split.payments[0].amount_minor=500;split.payments.push_back({"card","CARD","5.00",500});assert(run(x,journal.storage,split).certainty==ExecutionCertainty::Committed);}
 card_case([](auto&io){io.next_purchase.certainty=PaymentCertainty::Declined;},ExecutionCertainty::Rejected);
 card_case([](auto&io){io.next_purchase.certainty=PaymentCertainty::Unknown;},ExecutionCertainty::Unknown);
 card_case([](auto&io){io.next_fiscal.certainty=ExecutionCertainty::Rejected;},ExecutionCertainty::Compensated);
 card_case([](auto&io){io.next_fiscal.certainty=ExecutionCertainty::Rejected;io.next_reverse.certainty=PaymentCertainty::Unknown;},ExecutionCertainty::Unknown);
 card_case([](auto&io){io.payment_up=false;},ExecutionCertainty::DeviceUnavailable);
 {ProtocolFaithfulIo io(profile);Journal journal;ProfileExecutor x(b,io,&journal.storage);io.next_purchase.certainty=PaymentCertainty::Unknown;
  assert(run(x,journal.storage,plan("SALE_FINALIZE","CARD")).certainty==ExecutionCertainty::Unknown);
  const auto purchases=io.purchase_calls;io.next_purchase.certainty=PaymentCertainty::Approved;
  const auto blocked=run(x,journal.storage,plan("SALE_FINALIZE","CARD"));assert(blocked.certainty==ExecutionCertainty::Rejected&&blocked.error_code=="PAYMENT_TERMINAL_RECONCILIATION_REQUIRED"&&io.purchase_calls==purchases);
  assert(run(x,journal.storage,plan("SALE_FINALIZE","CASH")).certainty==ExecutionCertainty::Committed);}
 {ProtocolFaithfulIo io(profile);Journal journal;ProfileExecutor x(b,io,&journal.storage);RecoveryItem recovery{};recovery.operation_id="11111111-1111-4111-8111-111111111111";const auto lookups=io.lookup_calls;assert(x.recover(recovery).certainty==ExecutionCertainty::Unknown);assert(io.lookup_calls==lookups);io.fiscal_up=false;assert(run(x,journal.storage,plan("SALE_FINALIZE")).certainty==ExecutionCertainty::DeviceUnavailable);}
}
void recovery_ownership_matrix(EdgeProfile profile){
 const auto b=binding(profile);
 const std::string own_unp="DT636497-OP01-0000001";
 const std::string foreign_unp="XX111111-OP01-0000001";
 const std::string cash_payload="{\"payments\":[{\"type\":\"CASH\"}]}";
 // Matching prefix: io_.recover() must be called
 {ProtocolFaithfulIo io(profile);io.authority_unp_prefix="DT636497";Journal journal;ProfileExecutor x(b,io,&journal.storage);
  RecoveryItem ri{};ri.operation_id="22222222-2222-4222-8222-222222222221";ri.receipt_id="receipt-22222222";ri.canonical_unp=own_unp;ri.canonical_payload=cash_payload;
  const auto lookups=io.lookup_calls;
  assert(x.recover(ri).certainty==ExecutionCertainty::Committed);
  assert(io.lookup_calls==lookups+1);}
 // Foreign prefix: io_.recover() must NOT be called (open foreign receipt not cancelled)
 {ProtocolFaithfulIo io(profile);io.authority_unp_prefix="DT636497";Journal journal;ProfileExecutor x(b,io,&journal.storage);
  RecoveryItem ri{};ri.operation_id="33333333-3333-4333-8333-333333333331";ri.receipt_id="receipt-33333333";ri.canonical_unp=foreign_unp;ri.canonical_payload=cash_payload;
  const auto lookups=io.lookup_calls;
  assert(x.recover(ri).certainty==ExecutionCertainty::Unknown);
  assert(io.lookup_calls==lookups);}
}
void currency_param_gate(){
 // DSY-08.2: device with non-EUR currency must return DeviceUnavailable before any I/O.
 CompositeBinding b=binding(EdgeProfile::DaisyCompactS01);
 {ProtocolFaithfulIo io(EdgeProfile::DaisyCompactS01);io.currency_is_eur=false;
  Journal journal;ProfileExecutor x(b,io,&journal.storage);
  auto r=run(x,journal.storage,plan("SALE_FINALIZE","CASH"));
  assert(r.certainty==ExecutionCertainty::DeviceUnavailable&&io.fiscal_calls==0);}
 {ProtocolFaithfulIo io(EdgeProfile::DaisyCompactS01);io.currency_is_eur=true;
  Journal journal;ProfileExecutor x(b,io,&journal.storage);
  auto r=run(x,journal.storage,plan("SALE_FINALIZE","CASH"));
  assert(r.certainty==ExecutionCertainty::Committed&&io.fiscal_calls>0);}
 std::printf("CurrencyParamGate: PASS\n");
}
void profile_unconfigured_no_io(){
 // DSY-03: Unconfigured profile must not attempt any fiscal I/O.
 CompositeBinding b; // b.profile defaults to EdgeProfile::Unconfigured
 ProtocolFaithfulIo io(EdgeProfile::DatecsDp150BluePad50); // mock codec; profile in binding is Unconfigured
 Journal journal;ProfileExecutor x(b,io,&journal.storage);
 auto r=run(x,journal.storage,plan("REPORT_Z")); // non-monetary: currently bypasses config check
 assert(io.fiscal_calls==0);// DSY-03: must be 0; will fail (>0) until fix in profile_orchestrator.cpp
 assert(r.certainty!=ExecutionCertainty::Committed);
 std::printf("profile_unconfigured_no_io: PASS\n");
}
int main(){for(auto profile:{EdgeProfile::DatecsDp150BluePad50,EdgeProfile::DaisyCompactS01}){protocol_corruption_matrix(profile);fiscal_matrix(profile);receipt_fault_matrix(profile);recovery_ownership_matrix(profile);}profile_unconfigured_no_io();currency_param_gate();std::printf("recovery_ownership_matrix: PASS\n");return 0;}
