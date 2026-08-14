#include "profile_executor.h"
#include <ctime>
namespace beefiscal::idf{
ProfileExecutor::ProfileExecutor(const CompositeBinding&b,DeviceIo&i,DurableStorage*s):binding_(b),io_(i),storage_(s){}
bool ProfileExecutor::ready(){return io_.fiscal_ready();}
ExecutionResult ProfileExecutor::execute(const ReceiptPlan&p){
 if(!io_.fiscal_ready())return{ExecutionCertainty::DeviceUnavailable,"","FISCAL_DEVICE_UNAVAILABLE","","",""};
 if(p.command_type!="SALE_FINALIZE")return io_.fiscal_command(p);
 int64_t card_amount=0;for(const auto&payment:p.payments)if(payment.type=="CARD")card_amount+=payment.amount_minor;
 PaymentOutcome card;
 if(card_amount){
  if(!binding_.payment.present||!io_.payment_ready())return{ExecutionCertainty::DeviceUnavailable,"","PAYMENT_TERMINAL_UNAVAILABLE","","",""};
  card=io_.purchase(p.operation_id,card_amount);
  if(storage_){const int64_t at=std::time(nullptr);const char*payment_state=card.certainty==PaymentCertainty::Approved?"CARD_APPROVED":card.certainty==PaymentCertainty::Unknown?"CARD_UNKNOWN":card.certainty==PaymentCertainty::Declined?"CARD_DECLINED":"CARD_NOT_SENT";for(const auto&payment:p.payments)if(payment.type=="CARD")storage_->upsert_payment(payment.id.c_str(),p.operation_id.c_str(),p.receipt_session_id.c_str(),payment_state,payment.amount_minor,card.reference.empty()?nullptr:card.reference.c_str(),card.rrn.empty()?nullptr:card.rrn.c_str(),card.auth_code.empty()?nullptr:card.auth_code.c_str(),at);storage_->set_operation_state(p.operation_id.c_str(),payment_state,nullptr,at);}
  if(card.certainty==PaymentCertainty::Declined)return{ExecutionCertainty::Rejected,"",card.error_code,card.reference,card.rrn,card.auth_code};
  if(card.certainty==PaymentCertainty::Unknown)return{ExecutionCertainty::Unknown,"","CARD_UNKNOWN",card.reference,card.rrn,card.auth_code};
  if(card.certainty!=PaymentCertainty::Approved)return{ExecutionCertainty::Rejected,"","CARD_NOT_SENT","","",""};
 }
 auto fiscal=io_.fiscalize(p);fiscal.terminal_reference=card.reference;fiscal.rrn=card.rrn;fiscal.auth_code=card.auth_code;
 if(card_amount&&fiscal.certainty!=ExecutionCertainty::Committed){
  if(fiscal.certainty==ExecutionCertainty::Unknown)return fiscal;
  auto reversed=io_.reverse(card.reference.empty()?p.operation_id:card.reference,card_amount);
  if(reversed.certainty==PaymentCertainty::Approved)return{ExecutionCertainty::Compensated,"",fiscal.error_code,card.reference,card.rrn,card.auth_code};
  return{ExecutionCertainty::Unknown,"","COMPENSATION_REQUIRED",card.reference,card.rrn,card.auth_code};
 }
 return fiscal;
}
ExecutionResult ProfileExecutor::recover(const RecoveryItem&i){return io_.recover(i);}
}
