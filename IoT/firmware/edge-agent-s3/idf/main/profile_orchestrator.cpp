#include "profile_executor.h"
#include <ctime>
namespace beefiscal::idf{
ProfileExecutor::ProfileExecutor(const CompositeBinding&b,DeviceIo&i,DurableStorage*s):binding_(b),io_(i),storage_(s){}
bool ProfileExecutor::ready(){return io_.fiscal_ready();}
ExecutionResult ProfileExecutor::execute(const ReceiptPlan&p){
 if(!io_.fiscal_ready())return{ExecutionCertainty::DeviceUnavailable,"","FISCAL_DEVICE_UNAVAILABLE","","",""};
 if(p.command_type=="SALE_REVERSE"){
  std::vector<std::pair<IntentPayment,PaymentOutcome>> refunds;
  for(const auto&payment:p.payments)if(payment.type=="CARD"){
   if(!binding_.payment.present||!io_.payment_ready())return{ExecutionCertainty::DeviceUnavailable,"","PAYMENT_TERMINAL_UNAVAILABLE","","",""};
   auto refund=io_.reverse(payment.id,payment.amount_minor);
   const int64_t at=std::time(nullptr);
   if(storage_)storage_->upsert_payment(payment.id.c_str(),p.operation_id.c_str(),p.receipt_session_id.c_str(),refund.certainty==PaymentCertainty::Approved?"REFUND_APPROVED":refund.certainty==PaymentCertainty::Unknown?"REFUND_UNKNOWN":"REFUND_DECLINED",payment.amount_minor,refund.reference.empty()?nullptr:refund.reference.c_str(),refund.rrn.empty()?nullptr:refund.rrn.c_str(),refund.auth_code.empty()?nullptr:refund.auth_code.c_str(),at);
   if(refund.certainty==PaymentCertainty::Unknown)return{ExecutionCertainty::Unknown,"","CARD_REFUND_UNKNOWN",refund.reference,refund.rrn,refund.auth_code};
   if(refund.certainty!=PaymentCertainty::Approved)return{ExecutionCertainty::Rejected,"",refund.error_code.empty()?"CARD_REFUND_DECLINED":refund.error_code,refund.reference,refund.rrn,refund.auth_code};
   refunds.push_back({payment,refund});
  }
  auto fiscal=io_.fiscal_command(p);
  if(fiscal.certainty!=ExecutionCertainty::Committed&&!refunds.empty())return{ExecutionCertainty::Unknown,"",fiscal.certainty==ExecutionCertainty::Unknown?fiscal.error_code:"REFUND_COMPLETED_FISCAL_STORNO_FAILED",refunds.back().second.reference,refunds.back().second.rrn,refunds.back().second.auth_code};
  if(!refunds.empty()){fiscal.terminal_reference=refunds.back().second.reference;fiscal.rrn=refunds.back().second.rrn;fiscal.auth_code=refunds.back().second.auth_code;}
  return fiscal;
 }
 if(p.command_type!="SALE_FINALIZE")return io_.fiscal_command(p);
 std::vector<std::pair<IntentPayment,PaymentOutcome>> approved;
 for(const auto&payment:p.payments)if(payment.type=="CARD"){
  if(!binding_.payment.present||!io_.payment_ready())return{ExecutionCertainty::DeviceUnavailable,"","PAYMENT_TERMINAL_UNAVAILABLE","","",""};
  auto card=io_.purchase(payment.id,payment.amount_minor);
  if(storage_){const int64_t at=std::time(nullptr);const char*payment_state=card.certainty==PaymentCertainty::Approved?"CARD_APPROVED":card.certainty==PaymentCertainty::Unknown?"CARD_UNKNOWN":card.certainty==PaymentCertainty::Declined?"CARD_DECLINED":"CARD_NOT_SENT";storage_->upsert_payment(payment.id.c_str(),p.operation_id.c_str(),p.receipt_session_id.c_str(),payment_state,payment.amount_minor,card.reference.empty()?nullptr:card.reference.c_str(),card.rrn.empty()?nullptr:card.rrn.c_str(),card.auth_code.empty()?nullptr:card.auth_code.c_str(),at);storage_->set_operation_state(p.operation_id.c_str(),payment_state,nullptr,at);}
  if(card.certainty==PaymentCertainty::Declined)return{ExecutionCertainty::Rejected,"",card.error_code,card.reference,card.rrn,card.auth_code};
  if(card.certainty==PaymentCertainty::Unknown)return{ExecutionCertainty::Unknown,"","CARD_UNKNOWN",card.reference,card.rrn,card.auth_code};
  if(card.certainty!=PaymentCertainty::Approved)return{ExecutionCertainty::Rejected,"","CARD_NOT_SENT","","",""};
  approved.push_back({payment,card});
 }
 auto fiscal=io_.fiscalize(p);if(!approved.empty()){fiscal.terminal_reference=approved.back().second.reference;fiscal.rrn=approved.back().second.rrn;fiscal.auth_code=approved.back().second.auth_code;}
 if(!approved.empty()&&fiscal.certainty!=ExecutionCertainty::Committed){
  if(fiscal.certainty==ExecutionCertainty::Unknown)return fiscal;
  for(auto it=approved.rbegin();it!=approved.rend();++it){auto reversed=io_.reverse(it->second.reference.empty()?it->first.id:it->second.reference,it->first.amount_minor);if(reversed.certainty!=PaymentCertainty::Approved)return{ExecutionCertainty::Unknown,"","COMPENSATION_REQUIRED",it->second.reference,it->second.rrn,it->second.auth_code};}
  return{ExecutionCertainty::Compensated,"",fiscal.error_code,approved.back().second.reference,approved.back().second.rrn,approved.back().second.auth_code};
 }
 return fiscal;
}
ExecutionResult ProfileExecutor::recover(const RecoveryItem&i){return io_.recover(i);}
}
