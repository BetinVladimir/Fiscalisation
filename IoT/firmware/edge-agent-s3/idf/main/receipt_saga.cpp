#include "receipt_saga.h"
namespace beefiscal::idf{namespace{int64_t card_total(const ReceiptPlan&p){int64_t n=0;for(auto&v:p.payments)if(v.type=="CARD")n+=v.amount_minor;return n;}}
ReceiptSaga::ReceiptSaga(FiscalPort&f,PaymentPort*p):fiscal_(f),payment_(p){}
ExecutionResult ReceiptSaga::execute(const ReceiptPlan&p){
 if(!fiscal_.ready())return{ExecutionCertainty::DeviceUnavailable,"","FISCAL_DEVICE_UNAVAILABLE","","",""};
 const auto amount=card_total(p);PaymentOutcome card;
 if(amount){if(!payment_||!payment_->ready())return{ExecutionCertainty::DeviceUnavailable,"","PAYMENT_TERMINAL_UNAVAILABLE","","",""};card=payment_->purchase(p.operation_id,amount);if(card.certainty==PaymentCertainty::Declined)return{ExecutionCertainty::Rejected,"",card.error_code,card.reference,card.rrn,card.auth_code};if(card.certainty==PaymentCertainty::Unknown)return{ExecutionCertainty::Unknown,"","CARD_UNKNOWN",card.reference,card.rrn,card.auth_code};if(card.certainty!=PaymentCertainty::Approved)return{ExecutionCertainty::Rejected,"","CARD_NOT_SENT","","",""};}
 auto fiscal=fiscal_.fiscalize(p);fiscal.terminal_reference=card.reference;fiscal.rrn=card.rrn;fiscal.auth_code=card.auth_code;
 if(amount&&fiscal.certainty!=ExecutionCertainty::Committed){if(fiscal.certainty==ExecutionCertainty::Unknown)return fiscal;auto reversed=payment_->reverse(card.reference,amount);if(reversed.certainty==PaymentCertainty::Approved)return{ExecutionCertainty::Compensated,"",fiscal.error_code,card.reference,card.rrn,card.auth_code};return{ExecutionCertainty::Unknown,"","COMPENSATION_REQUIRED",card.reference,card.rrn,card.auth_code};}return fiscal;
}
ExecutionResult ReceiptSaga::recover(const ReceiptPlan&p,const char*state){if(!state)return{ExecutionCertainty::Unknown,"","RECOVERY_STATE_MISSING","","",""};if(std::string(state)=="CARD_UNKNOWN"&&payment_){auto x=payment_->lookup(p.operation_id);if(x.certainty==PaymentCertainty::Declined||x.certainty==PaymentCertainty::NotSent)return{ExecutionCertainty::Rejected,"",x.error_code,x.reference,x.rrn,x.auth_code};if(x.certainty!=PaymentCertainty::Approved)return{ExecutionCertainty::Unknown,"","CARD_UNKNOWN",x.reference,x.rrn,x.auth_code};}auto f=fiscal_.lookup(p.operation_id);return f;}
}
