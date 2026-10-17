#include "profile_executor.h"
#include "money_plan.h"
#include <ctime>
#include "cJSON.h"
namespace beefiscal::idf {
namespace {
ExecutionResult uncertain(const char*error,const PaymentOutcome&payment={}){return{ExecutionCertainty::Unknown,"",error,payment.reference,payment.rrn,payment.auth_code};}
PaymentRecord record(const ReceiptPlan&p,const IntentPayment&payment,const PaymentOutcome&outcome={}){PaymentRecord r;r.payment_id=payment.id;r.operation_id=p.operation_id;r.receipt_id=p.receipt_session_id;r.amount_minor=payment.amount_minor;r.terminal_reference=outcome.reference;r.rrn=outcome.rrn;r.auth_code=outcome.auth_code;return r;}
}
ProfileExecutor::ProfileExecutor(const CompositeBinding&b,DeviceIo&i,DurableStorage*s):binding_(b),io_(i),storage_(s){}
bool ProfileExecutor::ready(){return io_.fiscal_ready();}
ExecutionResult ProfileExecutor::execute(const ReceiptPlan&p){
 const bool monetary=p.command_type=="SALE_FINALIZE"||p.command_type=="SALE_REVERSE"||p.command_type=="CASH_IN"||p.command_type=="CASH_OUT";
 if(monetary){auto error=receipt_plan_error(binding_,p);if(error.empty())error=io_.preflight(p);if(!error.empty())return{ExecutionCertainty::Rejected,"",error,"","",""};}
 if(!io_.fiscal_ready())return{ExecutionCertainty::DeviceUnavailable,"","FISCAL_DEVICE_UNAVAILABLE","","",""};
 std::vector<InitialPayment>cards;for(const auto&payment:p.payments)if(payment.type=="CARD")cards.push_back({payment.id,payment.amount_minor});
 // The current refund contract contains a new tender UUID, not its original
 // approved RRN. Do not issue VOID_PURCHASE using an invented reference.
 if(p.command_type=="SALE_REVERSE"&&!cards.empty())return{ExecutionCertainty::Rejected,"","CARD_REFUND_REFERENCE_REQUIRED","","",""};
 if(p.command_type!="SALE_FINALIZE")return io_.fiscal_command(p);
 if(cards.empty())return io_.fiscalize(p);
 if(!storage_)return{ExecutionCertainty::Rejected,"","DURABLE_PAYMENT_JOURNAL_REQUIRED","","",""};
 std::string phase;const auto prior=storage_->payment_saga_phase(p.operation_id.c_str(),phase);
 if(prior==ESP_OK)return uncertain("PAYMENT_SAGA_RECONCILIATION_REQUIRED");
 if(prior!=ESP_ERR_NOT_FOUND)return uncertain("PAYMENT_SAGA_STORAGE_FAILED");
 if(!binding_.payment.present||!io_.payment_ready())return{ExecutionCertainty::DeviceUnavailable,"","PAYMENT_TERMINAL_UNAVAILABLE","","",""};
 if(storage_->begin_payment_saga(p.operation_id.c_str(),p.receipt_session_id.c_str(),cards,std::time(nullptr))!=ESP_OK)return uncertain("PAYMENT_SAGA_BEGIN_FAILED");
 std::vector<std::pair<IntentPayment,PaymentOutcome>>approved;
 auto compensate=[&](const char*reason)->ExecutionResult{
  const auto&last=approved.back().second;
  bool proven=io_.verified_reversal_profile();for(const auto&v:approved)proven=proven&&!v.second.rrn.empty();
  if(!proven){std::string current;if(storage_->payment_saga_phase(p.operation_id.c_str(),current)!=ESP_OK||storage_->checkpoint_saga_phase(p.operation_id.c_str(),current.c_str(),"COMPENSATION_REQUIRED",std::time(nullptr))!=ESP_OK)return uncertain("COMPENSATION_CHECKPOINT_FAILED",last);return uncertain("COMPENSATION_PROFILE_OR_RRN_REQUIRED",last);}
  for(auto i=approved.rbegin();i!=approved.rend();++i){
   auto saved=record(p,i->first,i->second);
   if(storage_->checkpoint_payment(saved,"CARD_APPROVED","REVERSAL_SENDING","COMPENSATING",std::time(nullptr))!=ESP_OK)return uncertain("REVERSAL_INTENT_CHECKPOINT_FAILED",i->second);
   // The independently verified profile must define VOID by original RRN.
   const auto reversed=io_.reverse(i->second.rrn,i->first.amount_minor);
   saved.reversal_reference=reversed.reference;saved.reversal_rrn=reversed.rrn;saved.reversal_auth_code=reversed.auth_code;
   const char*state=reversed.certainty==PaymentCertainty::Approved?"REVERSED":reversed.certainty==PaymentCertainty::Unknown?"REVERSAL_UNKNOWN":"REVERSAL_FAILED";
   // Keep original purchase references; reversal acknowledgements are not replacements.
   if(storage_->checkpoint_payment(saved,"REVERSAL_SENDING",state,"COMPENSATING",std::time(nullptr))!=ESP_OK)return uncertain("REVERSAL_OUTCOME_CHECKPOINT_FAILED",i->second);
   if(reversed.certainty!=PaymentCertainty::Approved)return uncertain("COMPENSATION_REQUIRED",i->second);
  }
  if(storage_->checkpoint_saga_phase(p.operation_id.c_str(),"COMPENSATING","COMPENSATED",std::time(nullptr))!=ESP_OK)return uncertain("COMPENSATION_FINAL_CHECKPOINT_FAILED",last);
  return{ExecutionCertainty::Compensated,"",reason,last.reference,last.rrn,last.auth_code};
 };
 for(const auto&payment:p.payments)if(payment.type=="CARD"){
  if(!io_.payment_ready()){if(approved.empty())return uncertain("PAYMENT_TERMINAL_UNAVAILABLE");return compensate("PAYMENT_TERMINAL_UNAVAILABLE");}
  if(storage_->checkpoint_payment(record(p,payment),"RESERVED","CARD_SENDING","PAYMENTS",std::time(nullptr))!=ESP_OK)return uncertain("CARD_INTENT_CHECKPOINT_FAILED");
  const auto card=io_.purchase(payment.id,payment.amount_minor);
  const char*state=card.certainty==PaymentCertainty::Approved?"CARD_APPROVED":card.certainty==PaymentCertainty::Unknown?"CARD_UNKNOWN":card.certainty==PaymentCertainty::Declined?"CARD_DECLINED":"CARD_NOT_SENT";
  if(storage_->checkpoint_payment(record(p,payment,card),"CARD_SENDING",state,"PAYMENTS",std::time(nullptr))!=ESP_OK)return uncertain("CARD_OUTCOME_CHECKPOINT_FAILED",card);
  // Current uncertain payment must be reconciled before any compensation.
  if(card.certainty==PaymentCertainty::Unknown)return uncertain("CARD_UNKNOWN",card);
  if(card.certainty!=PaymentCertainty::Approved){if(!approved.empty())return compensate(card.error_code.empty()?"CARD_NOT_APPROVED":card.error_code.c_str());return{ExecutionCertainty::Rejected,"",card.error_code.empty()?"CARD_NOT_APPROVED":card.error_code,card.reference,card.rrn,card.auth_code};}
  approved.push_back({payment,card});
  if(card.reference.empty())return uncertain("CARD_REFERENCE_REQUIRED",card);
 }
 if(storage_->checkpoint_saga_phase(p.operation_id.c_str(),"PAYMENTS","FISCAL_SENDING",std::time(nullptr))!=ESP_OK)return uncertain("FISCAL_INTENT_CHECKPOINT_FAILED",approved.back().second);
 auto fiscal=io_.fiscalize(p);const auto&last=approved.back().second;fiscal.terminal_reference=last.reference;fiscal.rrn=last.rrn;fiscal.auth_code=last.auth_code;
 if(fiscal.certainty==ExecutionCertainty::Committed||fiscal.certainty==ExecutionCertainty::Unknown)return fiscal;
 return compensate(fiscal.error_code.empty()?"FISCAL_NOT_COMMITTED":fiscal.error_code.c_str());
}
ExecutionResult ProfileExecutor::recover(const RecoveryItem&i){
 if(!storage_)return uncertain("RECOVERY_JOURNAL_REQUIRED");
 PaymentSagaSnapshot saga;const auto scanned=storage_->payment_saga_snapshot(i.operation_id.c_str(),i.receipt_id.c_str(),saga);
 if(scanned==ESP_ERR_NOT_FOUND){
  // Legacy recovery selects one arbitrary payment; it cannot authorize CARD IO.
  bool card=!i.payment_id.empty()&&(i.payment_state.find("CARD")!=std::string::npos||!i.terminal_reference.empty()||!i.rrn.empty());
  auto*root=cJSON_Parse(i.canonical_payload.c_str());if(!cJSON_IsObject(root)){cJSON_Delete(root);return uncertain("LEGACY_RECOVERY_PAYLOAD_INVALID");}auto*payments=cJSON_GetObjectItemCaseSensitive(root,"payments");cJSON*payment=nullptr;cJSON_ArrayForEach(payment,payments){auto*type=cJSON_GetObjectItemCaseSensitive(payment,"type");if(cJSON_IsString(type)&&std::string(type->valuestring)=="CARD")card=true;}cJSON_Delete(root);
  if(card)return uncertain("LEGACY_CARD_INVENTORY_REQUIRED");
  return io_.recover(i);
 }
 if(scanned!=ESP_OK)return uncertain("PAYMENT_SAGA_STORAGE_FAILED");
 auto terminal=[&](ExecutionCertainty certainty,const char*error)->ExecutionResult{const auto&p=saga.payments.back();return{certainty,"",error,p.terminal_reference,p.rrn,p.auth_code};};
 // A completed compensation is durable evidence even if final outbox projection crashed.
 if(saga.phase=="COMPENSATED"){for(const auto&p:saga.payments)if(p.state!="REVERSED"&&p.state!="CARD_DECLINED"&&p.state!="CARD_NOT_SENT"&&p.state!="RESERVED")return uncertain("COMPENSATION_EVIDENCE_CONFLICT");return terminal(ExecutionCertainty::Compensated,"");}
 bool unresolved=false;
 for(auto&p:saga.payments){
  const bool purchase=p.state=="CARD_SENDING"||p.state=="CARD_UNKNOWN";
  const bool reversal=p.state=="REVERSAL_SENDING"||p.state=="REVERSAL_UNKNOWN";
  if(!purchase&&!reversal)continue;
  if(!io_.verified_payment_query_profile()){unresolved=true;continue;}
  const auto queried=io_.query_payment({reversal?PaymentQueryKind::Reversal:PaymentQueryKind::Purchase,p});
  if(queried.operation_id!=p.operation_id||queried.payment_id!=p.payment_id||queried.amount_minor!=p.amount_minor||queried.outcome.certainty==PaymentCertainty::Unknown){unresolved=true;continue;}
  auto observed=p;const auto&outcome=queried.outcome;
  if(reversal){observed.reversal_reference=outcome.reference;observed.reversal_rrn=outcome.rrn;observed.reversal_auth_code=outcome.auth_code;}
  else{if(!outcome.reference.empty())observed.terminal_reference=outcome.reference;if(!outcome.rrn.empty())observed.rrn=outcome.rrn;if(!outcome.auth_code.empty())observed.auth_code=outcome.auth_code;}
  if(!reversal&&outcome.certainty==PaymentCertainty::Approved&&observed.terminal_reference.empty())return uncertain("CARD_REFERENCE_REQUIRED");
  const char*state=reversal?(outcome.certainty==PaymentCertainty::Approved?"REVERSED":"REVERSAL_FAILED"):(outcome.certainty==PaymentCertainty::Approved?"CARD_APPROVED":outcome.certainty==PaymentCertainty::Declined?"CARD_DECLINED":"CARD_NOT_SENT");
  if(storage_->reconcile_payment(p,observed,state,std::time(nullptr))!=ESP_OK)return uncertain("PAYMENT_RECONCILIATION_CHECKPOINT_FAILED");
  observed.state=state;p=observed;
 }
 if(unresolved)return uncertain("PAYMENT_RECONCILIATION_REQUIRED");
 bool approved=false,reversed=false,negative=false,failedReversal=false;
 for(const auto&p:saga.payments){if(p.state=="CARD_APPROVED")approved=true;else if(p.state=="REVERSED")reversed=true;else if(p.state=="CARD_DECLINED"||p.state=="CARD_NOT_SENT"||p.state=="RESERVED")negative=true;else if(p.state=="REVERSAL_FAILED")failedReversal=true;else return uncertain("PAYMENT_STATE_UNSUPPORTED");}
 if(failedReversal)return uncertain("REVERSAL_MANUAL_RECONCILIATION_REQUIRED");
 if(saga.phase=="FISCAL_SENDING"){
  // Read-only physical proof, never the legacy cancel/reverse recovery helper.
  const auto proof=io_.fiscal_receipt_proof(i);
  if(proof.certainty==ExecutionCertainty::Committed){if(!approved||negative||reversed)return uncertain("FISCAL_PAYMENT_EVIDENCE_CONFLICT");return proof;}
  if(proof.certainty!=ExecutionCertainty::Rejected)return uncertain("FISCAL_DOCUMENT_NOT_PROVEN");
  // Rejected here is reserved for verified absence of this exact operation.
  negative=true;
 }
 if(!approved){
  if(reversed){if(saga.phase!="COMPENSATING"||storage_->checkpoint_saga_phase(i.operation_id.c_str(),"COMPENSATING","COMPENSATED",std::time(nullptr))!=ESP_OK)return uncertain("COMPENSATION_FINAL_CHECKPOINT_FAILED");return terminal(ExecutionCertainty::Compensated,"");}
  if(saga.phase=="PAYMENTS")return terminal(ExecutionCertainty::Rejected,"CARD_NOT_APPROVED");
  return uncertain("PAYMENT_SAGA_RECONCILIATION_REQUIRED");
 }
 // All-approved PAYMENTS never resends fiscal commands: an explicit recovery
 // policy can add safe continuation later. Compensate only a proven failed path.
 if(!negative&&saga.phase!="COMPENSATING"&&saga.phase!="COMPENSATION_REQUIRED")return uncertain("FISCAL_RECOVERY_POLICY_REQUIRED");
 if(!io_.verified_reversal_profile())return uncertain("COMPENSATION_PROFILE_OR_RRN_REQUIRED");
 for(const auto&p:saga.payments)if(p.state=="CARD_APPROVED"&&p.rrn.empty())return uncertain("COMPENSATION_PROFILE_OR_RRN_REQUIRED");
 for(auto p=saga.payments.rbegin();p!=saga.payments.rend();++p)if(p->state=="CARD_APPROVED"){
  if(storage_->checkpoint_payment(*p,"CARD_APPROVED","REVERSAL_SENDING","COMPENSATING",std::time(nullptr))!=ESP_OK)return uncertain("REVERSAL_INTENT_CHECKPOINT_FAILED");
  const auto reversedOutcome=io_.reverse(p->rrn,p->amount_minor);auto evidence=*p;evidence.reversal_reference=reversedOutcome.reference;evidence.reversal_rrn=reversedOutcome.rrn;evidence.reversal_auth_code=reversedOutcome.auth_code;
  const char*next=reversedOutcome.certainty==PaymentCertainty::Approved?"REVERSED":reversedOutcome.certainty==PaymentCertainty::Unknown?"REVERSAL_UNKNOWN":"REVERSAL_FAILED";
  if(storage_->checkpoint_payment(evidence,"REVERSAL_SENDING",next,"COMPENSATING",std::time(nullptr))!=ESP_OK)return uncertain("REVERSAL_OUTCOME_CHECKPOINT_FAILED");
  if(reversedOutcome.certainty!=PaymentCertainty::Approved)return uncertain("COMPENSATION_REQUIRED");
 }
 if(storage_->checkpoint_saga_phase(i.operation_id.c_str(),"COMPENSATING","COMPENSATED",std::time(nullptr))!=ESP_OK)return uncertain("COMPENSATION_FINAL_CHECKPOINT_FAILED");
 return terminal(ExecutionCertainty::Compensated,"");
}
}
