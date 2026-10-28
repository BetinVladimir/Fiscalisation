#include "profile_executor.h"
#include "uart_runtime.h"
#include "edge_runtime_port.h"
#include "bluepad_ble_central.h"
#include "mbedtls/sha256.h"
#include "CommandPayload.hpp"
#include "FrameCodec.hpp"
#include "esp_timer.h"
#include <cstdio>
#include <ctime>
#include <algorithm>
#include "cJSON.h"
#include "money_plan.h"
namespace beefiscal::idf{namespace{
int write_bytes(void*ctx,const uint8_t*p,size_t n,uint32_t timeout){auto&b=*static_cast<const CompositeBinding*>(ctx);if(b.profile==EdgeProfile::DatecsDp150BluePad50)return uart_runtime_write(p,n,timeout);if(b.profile==EdgeProfile::DaisyCompactS01)return usb_cdc_write(p,n,timeout);return -1;}
int read_bytes(void*ctx,uint8_t*p,size_t n,uint32_t timeout){auto&b=*static_cast<const CompositeBinding*>(ctx);if(b.profile==EdgeProfile::DatecsDp150BluePad50)return uart_runtime_read(p,n,timeout);if(b.profile==EdgeProfile::DaisyCompactS01)return usb_cdc_read(p,n,timeout);return -1;}
int64_t monotonic_ms(void*){return esp_timer_get_time()/1000;}
bool exchange(const CompositeBinding&b,uint16_t command,const std::vector<uint8_t>&payload,SerialSession&sequence,bee::ParsedFrame*out=nullptr){
 if(b.profile==EdgeProfile::Unconfigured)return false;
 SerialTransport transport{const_cast<CompositeBinding*>(&b),write_bytes,read_bytes,monotonic_ms};
 const SerialProfile sp=b.profile==EdgeProfile::DatecsDp150BluePad50?SerialProfile::Datecs:SerialProfile::Daisy;
 return sequence.exchange(sp,transport,command,payload,out);
}
// Vendor-issued document evidence. These queries never print/cancel/refund.
bool exact_unp_token(const std::string& text,const std::string& unp){
 auto identifier=[](unsigned char c){return(c>='A'&&c<='Z')||(c>='a'&&c<='z')||(c>='0'&&c<='9')||c=='-';};
 if(unp.size()!=21)return false;
 for(size_t pos=text.find(unp);pos!=std::string::npos;pos=text.find(unp,pos+1))
  if((pos==0||!identifier(text[pos-1]))&&(pos+unp.size()==text.size()||!identifier(text[pos+unp.size()])))return true;
 return false;
}
bool issued_time(const std::string& value){
 // Both manuals permit a two/four-digit year; separators differ by vendor.
 const bool short_year=value.size()==17;const size_t offset=short_year?0:2;
 if(value.size()!=17&&value.size()!=19)return false;
 for(size_t i=0;i<value.size();++i)if(i!=2&&i!=5&&i!=8+offset&&i!=11+offset&&i!=14+offset&&(value[i]<'0'||value[i]>'9'))return false;
 if((value[2]!='-'&&value[2]!='.')||value[5]!=value[2]||value[8+offset]!=' '||(value[11+offset]!=':'&&value[11+offset]!='.')||value[14+offset]!=value[11+offset])return false;
 auto n=[&](size_t at){return(value[at]-'0')*10+value[at+1]-'0';};
 int year=short_year?2000+n(6):n(6)*100+n(8),month=n(3),day=n(0);
 if(month<1||month>12||day<1||n(9+offset)>23||n(12+offset)>59||n(15+offset)>59)return false;
 const int days[]={31,28,31,30,31,30,31,31,30,31,30,31};
 return day<=days[month-1]+(month==2&&year%4==0&&(year%100!=0||year%400==0));
}
bool device_identity(const CompositeBinding& b,SerialSession& sequence){
 std::vector<uint8_t>payload;bee::ParsedFrame frame;std::string error,serial;
 if(b.authority.unp_prefix.size()!=8)return false;
 if(b.profile==EdgeProfile::DatecsDp150BluePad50){bee::buildDatecsDeviceIdentity(payload);if(!exchange(b,123,payload,sequence,&frame))return false;bee::DeviceIdentityResult identity;if(!bee::parseDatecsDeviceIdentity(frame.data,identity,error))return false;serial=identity.serialNumber;}
 else{bee::buildDaisyDiagnostic(false,payload);if(!exchange(b,90,payload,sequence,&frame))return false;bee::DaisyDiagnosticResult identity;if(!bee::parseDaisyDiagnostic(frame.data,identity,error)||identity.country!=6)return false;serial=identity.serialNumber;}
 return serial.size()==8&&serial==b.authority.unp_prefix.substr(0,8);
}
bool datecs_document_proof(const CompositeBinding&b,uint32_t document,const ReceiptPlan&p,SerialSession&sequence){
 std::vector<uint8_t>payload;std::string error;bee::EjDocumentSelector selector{0,document,0};
 if(p.unp.empty()||!bee::buildDatecsEjDocumentSelector(selector,payload,error))return false;
 bee::ParsedFrame frame;if(!exchange(b,125,payload,sequence,&frame))return false;
 bee::EjDocumentInfoResult info;const uint8_t wanted=p.command_type=="SALE_REVERSE"?6:1;
 if(!bee::parseDatecsEjDocumentInfo(frame.data,info,error)||info.globalDocumentNumber!=document||info.documentType!=wanted||!issued_time(info.dateTime))return false;
 // Bounded text scan; UNP must be a whole token, not a substring of another ID.
 const int64_t deadline=esp_timer_get_time()+8000000;
 for(int line=0;line<96&&esp_timer_get_time()<deadline;line++){
  selector={1,0,0};if(!bee::buildDatecsEjDocumentSelector(selector,payload,error)||!exchange(b,125,payload,sequence,&frame))return false;
  std::string text;if(!bee::parseDatecsEjTextLine(frame.data,text,error)||text.empty())return false;
  if(exact_unp_token(text,p.unp))return device_identity(b,sequence);
 }
 return false;
}
bool daisy_document_proof(const CompositeBinding&b,const ReceiptPlan&p,SerialSession&sequence,uint32_t&document){
 std::vector<uint8_t>payload;bee::ParsedFrame frame;std::string error;
 if(p.unp.empty()||!exchange(b,113,payload,sequence,&frame)||!bee::parseDaisyLastDocumentNumber(frame.data,document,error)||!document||!bee::buildDaisyIssuedDocument(document,false,payload,error)||!exchange(b,119,payload,sequence,&frame))return false;
 bee::DaisyIssuedDocumentResult issued;
 if(!bee::parseDaisyIssuedDocument(frame.data,false,issued,error)||!issued.found||issued.number!=document||issued.unp!=p.unp||!issued_time(issued.issuedAt)||(issued.description&0x3f)!=1||(issued.description&0xc0)!=0xc0)return false;
 if(p.command_type=="SALE_REVERSE"){
  // Daisy reason0/1/2 corresponds to issued document type1/2/3.
  if(issued.type!=p.reversal_reason+1||issued.linkedDocument!=p.original_document||issued.linkedFiscalMemoryNumber!=p.original_fmin)return false;
 }else if(issued.type!=0)return false;
 return device_identity(b,sequence);
}
ExecutionResult finish_receipt(const CompositeBinding&b,const ReceiptPlan&p,SerialSession&sequence){std::vector<uint8_t>payload;std::string error;for(auto&line:p.items){bee::SaleItemPayload item;item.name=line.name;item.taxGroup=line.tax_group[0];item.unitPrice=line.unit_price;item.quantity=line.quantity;if(!line.discount.empty()&&line.discount!="0.00"){item.discountType=4;item.discountValue=line.discount;}bool built=b.profile==EdgeProfile::DatecsDp150BluePad50?bee::buildDatecsSaleItem(item,payload,error):bee::buildDaisySaleItem(item,payload,error);if(!built||!exchange(b,49,payload,sequence))return{ExecutionCertainty::Unknown,"","FISCAL_LINE_UNKNOWN","","",""};}bee::SubtotalPayload request{false,false,0,""};bee::ParsedFrame subtotal_frame;
 const bool datecs=b.profile==EdgeProfile::DatecsDp150BluePad50;
 if(!(datecs?bee::buildDatecsSubtotal(request,payload,error):bee::buildDaisySubtotal(request,payload,error))||!exchange(b,51,payload,sequence,&subtotal_frame))return{ExecutionCertainty::Unknown,"","FISCAL_SUBTOTAL_UNKNOWN","","",""};
 std::string observed;if(datecs){bee::SubtotalResult result;if(!bee::parseDatecsSubtotal(subtotal_frame.data,result,error))return{ExecutionCertainty::Unknown,"","FISCAL_SUBTOTAL_INVALID","","",""};observed=result.subtotal;}else{bee::DaisyFiscalTotalsResult result;if(!bee::parseDaisySubtotal(subtotal_frame.data,result,observed,error))return{ExecutionCertainty::Unknown,"","FISCAL_SUBTOTAL_INVALID","","",""};}
 int64_t observed_minor{},expected_minor=0;for(const auto&payment:p.payments)expected_minor+=payment.amount_minor;
 if(!fixed_unsigned(observed,2,observed_minor)||observed_minor!=expected_minor)return{ExecutionCertainty::Unknown,"","FISCAL_SUBTOTAL_MISMATCH","","",""};
 for(auto&payment:p.payments){bee::PaymentPayload value{payment.amount,payment.type=="CASH"?'P':b.fiscal.execution.daisy_card_code,(uint8_t)(payment.type=="CASH"?0:b.fiscal.execution.datecs_card_mode)};bool built=b.profile==EdgeProfile::DatecsDp150BluePad50?bee::buildDatecsPayment(value,payload,error):bee::buildDaisyPayment(value,payload,error);if(!built||!exchange(b,53,payload,sequence))return{ExecutionCertainty::Unknown,"","FISCAL_PAYMENT_UNKNOWN","","",""};}payload.clear();bee::ParsedFrame closed;if(!exchange(b,56,payload,sequence,&closed))return{ExecutionCertainty::Unknown,"","FISCAL_CLOSE_UNKNOWN","","",""};
 uint32_t document{};bool proven=false;
 if(datecs){bee::DatecsResult result;if(bee::parseDatecsResult(closed.data,result,error)&&result.fields.size()==1){const auto&number=result.fields[0];if(!number.empty()&&number.size()<=7&&std::all_of(number.begin(),number.end(),[](char c){return c>='0'&&c<='9';})){document=static_cast<uint32_t>(std::stoul(number));if(document)proven=datecs_document_proof(b,document,p,sequence);}}}
 else{uint32_t all{},fiscal{};if(bee::parseDaisyOpenReceiptResult(closed.data,all,fiscal,error))proven=daisy_document_proof(b,p,sequence,document);}
 if(!proven)return{ExecutionCertainty::Unknown,"","FISCAL_DOCUMENT_NOT_PROVEN","","",""};
 return{ExecutionCertainty::Committed,std::to_string(document),"","","",""};}
}
EspIdfDeviceIo::EspIdfDeviceIo(const CompositeBinding&b,const std::string&password):binding_(b),fiscal_password_(password){}
std::string EspIdfDeviceIo::preflight(const ReceiptPlan&p){
 // S3D-03.1: reject unsupported payment types before general plan validation
 // (payment_slots_bc50.json: CASH=0,CARD=1,CHEQUE=3,VOUCHER=4,COUPON=5;
 //  NHIF/DEFERRED/INTERNAL_CONSUMPTION are not mapped)
 if(p.command_type=="SALE_FINALIZE"||p.command_type=="SALE_REVERSE"){
  for(const auto&payment:p.payments){
   if(payment.type=="NHIF"||payment.type=="DEFERRED"||payment.type=="INTERNAL_CONSUMPTION")
    return "PAYMENT_TYPE_UNSUPPORTED";
  }
 }
 auto error=receipt_plan_error(binding_,p);if(!error.empty())return error;
 if(!valid_fiscal_password(binding_.profile,fiscal_password_))return "FISCAL_CREDENTIAL_REQUIRED";
 std::vector<uint8_t>payload;std::string message;const bool datecs=binding_.profile==EdgeProfile::DatecsDp150BluePad50;
 if(p.command_type=="SALE_FINALIZE"||p.command_type=="SALE_REVERSE"){
  if(p.command_type=="SALE_FINALIZE"){bee::OpenReceiptPayload open{binding_.fiscal.execution.operator_number,fiscal_password_,p.unp,binding_.fiscal.execution.till_number,false};if(!(datecs?bee::buildDatecsOpenReceipt(open,payload,message):bee::buildDaisyOpenReceipt(open,payload,message)))return "FISCAL_OPEN_INVALID";}
  else{bee::ReversalOpenPayload reversal{binding_.fiscal.execution.operator_number,fiscal_password_,binding_.fiscal.execution.till_number,p.reversal_reason,p.original_document,p.original_datetime,p.original_fmin,false,"","",p.unp};if(!(datecs?bee::buildDatecsOpenReversal(reversal,payload,message):bee::buildDaisyOpenReversal(reversal,payload,message)))return "FISCAL_REVERSAL_INVALID";}
  for(const auto&line:p.items){bee::SaleItemPayload item;item.name=line.name;item.taxGroup=line.tax_group[0];item.unitPrice=line.unit_price;item.quantity=line.quantity;if(!line.discount.empty()&&line.discount!="0.00"){item.discountType=4;item.discountValue=line.discount;}if(!(datecs?bee::buildDatecsSaleItem(item,payload,message):bee::buildDaisySaleItem(item,payload,message)))return "FISCAL_LINE_INVALID";}
  for(const auto&payment:p.payments){bee::PaymentPayload value{payment.amount,payment.type=="CASH"?'P':binding_.fiscal.execution.daisy_card_code,(uint8_t)(payment.type=="CASH"?0:binding_.fiscal.execution.datecs_card_mode)};if(!(datecs?bee::buildDatecsPayment(value,payload,message):bee::buildDaisyPayment(value,payload,message)))return "FISCAL_PAYMENT_INVALID";}
 }else if(p.command_type=="CASH_IN"||p.command_type=="CASH_OUT"){bee::CashMovementPayload value{p.amount,p.command_type=="CASH_OUT"};if(!(datecs?bee::buildDatecsCashMovement(value,payload,message):bee::buildDaisyCashMovement(value,payload,message)))return "CASH_MOVEMENT_INVALID";}
 return {};
}
bool EspIdfDeviceIo::begin(){if(binding_.profile==EdgeProfile::DatecsDp150BluePad50)return uart_runtime_start(binding_.fiscal)==ESP_OK&&(!binding_.payment.present||bluepad_ble_start(binding_.payment)==ESP_OK);return binding_.profile==EdgeProfile::DaisyCompactS01;}
bool EspIdfDeviceIo::fiscal_ready(){return binding_.profile==EdgeProfile::DatecsDp150BluePad50?uart_runtime_ready():usb_cdc_ready();}
bool EspIdfDeviceIo::payment_ready(){return binding_.payment.present&&bluepad_ble_ready();}
ExecutionResult EspIdfDeviceIo::fiscalize(const ReceiptPlan&p){auto validation=preflight(p);if(!validation.empty())return{ExecutionCertainty::Rejected,"",validation,"","",""};if(!fiscal_ready())return{ExecutionCertainty::DeviceUnavailable,"","FISCAL_DEVICE_UNAVAILABLE","","",""};auto& sequence=serial_;std::vector<uint8_t>payload;std::string error;bee::OpenReceiptPayload open{binding_.fiscal.execution.operator_number,fiscal_password_,p.unp,binding_.fiscal.execution.till_number,false};bool built=binding_.profile==EdgeProfile::DatecsDp150BluePad50?bee::buildDatecsOpenReceipt(open,payload,error):bee::buildDaisyOpenReceipt(open,payload,error);if(!built||!exchange(binding_,48,payload,sequence))return{ExecutionCertainty::Unknown,"","FISCAL_OPEN_UNKNOWN","","",""};return finish_receipt(binding_,p,sequence);}
ExecutionResult EspIdfDeviceIo::fiscal_command(const ReceiptPlan&p){
 if(p.command_type=="SALE_REVERSE"||p.command_type=="CASH_IN"||p.command_type=="CASH_OUT"){auto validation=preflight(p);if(!validation.empty())return{ExecutionCertainty::Rejected,"",validation,"","",""};}
 if(!fiscal_ready())return{ExecutionCertainty::DeviceUnavailable,"","FISCAL_DEVICE_UNAVAILABLE","","",""};
 auto& sequence=serial_;std::vector<uint8_t>payload;std::string error;uint16_t command=0;
 if(p.command_type=="REPORT_X"||p.command_type=="REPORT_Z"){
  command=69;const bool z=p.command_type=="REPORT_Z";
  if(!(binding_.profile==EdgeProfile::DatecsDp150BluePad50?bee::buildDatecsDailyReport(z,payload):bee::buildDaisyDailyReport(z,payload)))return{ExecutionCertainty::Rejected,"","REPORT_PAYLOAD_INVALID","","",""};
 }else if(p.command_type=="CASH_IN"||p.command_type=="CASH_OUT"){
  command=70;bee::CashMovementPayload movement{p.amount,p.command_type=="CASH_OUT"};
  if(!(binding_.profile==EdgeProfile::DatecsDp150BluePad50?bee::buildDatecsCashMovement(movement,payload,error):bee::buildDaisyCashMovement(movement,payload,error)))return{ExecutionCertainty::Rejected,"","CASH_MOVEMENT_INVALID","","",""};
 }else if(p.command_type=="SALE_REVERSE"){
  command=binding_.profile==EdgeProfile::DatecsDp150BluePad50?43:48;bee::ReversalOpenPayload reversal{binding_.fiscal.execution.operator_number,fiscal_password_,binding_.fiscal.execution.till_number,p.reversal_reason,p.original_document,p.original_datetime,p.original_fmin,false,"","",p.unp};
  if(!(binding_.profile==EdgeProfile::DatecsDp150BluePad50?bee::buildDatecsOpenReversal(reversal,payload,error):bee::buildDaisyOpenReversal(reversal,payload,error))||!exchange(binding_,command,payload,sequence))return{ExecutionCertainty::Unknown,"","REVERSAL_OPEN_UNKNOWN","","",""};
  return finish_receipt(binding_,p,sequence);
 }else if(p.command_type=="PRINTER_TEST"){
  if(!exchange(binding_,38,payload,sequence))return{ExecutionCertainty::Unknown,"","PRINTER_TEST_OPEN_UNKNOWN","","",""};
  std::string text="BEELOY PRINTER TEST";if(binding_.profile==EdgeProfile::DatecsDp150BluePad50)text+="\t\t\t\t\t\t";payload.assign(text.begin(),text.end());
  if(!exchange(binding_,42,payload,sequence))return{ExecutionCertainty::Unknown,"","PRINTER_TEST_TEXT_UNKNOWN","","",""};
  payload.clear();if(!exchange(binding_,39,payload,sequence))return{ExecutionCertainty::Unknown,"","PRINTER_TEST_CLOSE_UNKNOWN","","",""};
  return{ExecutionCertainty::Committed,"","","","",""};
 }else if(p.command_type=="DEVICE_IDENTITY"){
  command=binding_.profile==EdgeProfile::DatecsDp150BluePad50?123:90;if(binding_.profile==EdgeProfile::DatecsDp150BluePad50)payload={'1','\t'};
 }else if(p.command_type=="DEVICE_TIME")command=62;
 else return{ExecutionCertainty::Rejected,"","UNSUPPORTED_FISCAL_COMMAND","","",""};
 if(!exchange(binding_,command,payload,sequence))return{ExecutionCertainty::Unknown,"","FISCAL_COMMAND_UNKNOWN","","",""};
 return{ExecutionCertainty::Committed,"","","","",""};
}
PaymentOutcome EspIdfDeviceIo::purchase(const std::string&id,int64_t amount){return bluepad_ble_purchase(id,amount);}
PaymentOutcome EspIdfDeviceIo::payment_lookup(const std::string&id){return bluepad_ble_lookup(id);}
PaymentOutcome EspIdfDeviceIo::reverse(const std::string&id,int64_t amount){return bluepad_ble_reverse(id,amount);}
ExecutionResult EspIdfDeviceIo::fiscal_receipt_proof(const RecoveryItem&i){
 if(!fiscal_ready())return{ExecutionCertainty::DeviceUnavailable,"","FISCAL_DEVICE_UNAVAILABLE",i.terminal_reference,i.rrn,i.auth_code};
 // A refund repeats the original UNP; no persisted pre-print document watermark
 // exists yet. Latest matching refund is not proof of this exact operation.
 cJSON*route=cJSON_ParseWithLength(i.route_snapshot.data(),i.route_snapshot.size());
 cJSON*type=route?cJSON_GetObjectItemCaseSensitive(route,"command_type"):nullptr;
 const bool sale=cJSON_IsString(type)&&std::string(type->valuestring)=="SALE_FINALIZE";cJSON_Delete(route);
 if(!sale)return{ExecutionCertainty::Unknown,"","FISCAL_OPERATION_IDENTITY_NOT_PROVEN",i.terminal_reference,i.rrn,i.auth_code};
 ReceiptPlan p;p.command_type="SALE_FINALIZE";p.unp=i.canonical_unp;
 bee::ParsedFrame frame;std::vector<uint8_t>empty;std::string error;bee::ReceiptStatusResult status;
 if(!exchange(binding_,76,empty,serial_,&frame)||!(binding_.profile==EdgeProfile::DatecsDp150BluePad50?bee::parseDatecsReceiptStatus(frame.data,status,error):bee::parseDaisyReceiptStatus(frame.data,status,error))||status.openState!=0)return{ExecutionCertainty::Unknown,"","FISCAL_DOCUMENT_NOT_PROVEN",i.terminal_reference,i.rrn,i.auth_code};
 uint32_t document=status.receiptNumber;const bool proven=binding_.profile==EdgeProfile::DatecsDp150BluePad50?document&&datecs_document_proof(binding_,document,p,serial_):daisy_document_proof(binding_,p,serial_,document);
 return{proven?ExecutionCertainty::Committed:ExecutionCertainty::Unknown,proven?std::to_string(document):"",proven?"":"FISCAL_DOCUMENT_NOT_PROVEN",i.terminal_reference,i.rrn,i.auth_code};
}
ExecutionResult EspIdfDeviceIo::recover(const RecoveryItem&i){
 if(!i.canonical_unp.empty()&&binding_.authority.unp_prefix.size()==8&&
    (i.canonical_unp.size()<8||i.canonical_unp.substr(0,8)!=binding_.authority.unp_prefix))
  return{ExecutionCertainty::Unknown,"","FOREIGN_RECEIPT_OWNERSHIP",i.terminal_reference,i.rrn,i.auth_code};
 return fiscal_receipt_proof(i);
}
}
