#pragma once
#include "intent_processor.h"
#include "provisioned_binding.h"
#include "serial_exchange.h"
namespace beefiscal::idf{
enum class PaymentCertainty:uint8_t{Approved,Declined,NotSent,Unknown};
struct PaymentOutcome{PaymentCertainty certainty{PaymentCertainty::NotSent};std::string reference,rrn,auth_code,error_code;};
enum class PaymentQueryKind:uint8_t{Purchase,Reversal};
struct PaymentQuery{PaymentQueryKind kind;PaymentRecord payment;};
struct PaymentQueryResult{PaymentOutcome outcome;std::string operation_id,payment_id;int64_t amount_minor{};};
class DeviceIo{public:virtual~DeviceIo()=default;virtual bool fiscal_ready()=0;
 // reverse() accepts the proven original RRN, never STAN or a payment UUID.
 virtual bool verified_payment_query_profile()const{return false;}
 virtual PaymentQueryResult query_payment(const PaymentQuery&){return{{PaymentCertainty::Unknown,"","","","QUERY_UNAVAILABLE"},"","",0};}
 virtual ExecutionResult fiscal_receipt_proof(const RecoveryItem&){return{ExecutionCertainty::Unknown,"","FISCAL_PROOF_UNAVAILABLE","","",""};}
 virtual bool verified_reversal_profile()const{return false;}
 virtual bool payment_ready()=0;virtual std::string preflight(const ReceiptPlan&){return {};};virtual ExecutionResult fiscalize(const ReceiptPlan&)=0;
 virtual ExecutionResult fiscal_command(const ReceiptPlan&)=0;
 virtual PaymentOutcome purchase(const std::string&,int64_t)=0;
 virtual PaymentOutcome payment_lookup(const std::string&)=0;
 virtual PaymentOutcome reverse(const std::string&,int64_t)=0;
 virtual ExecutionResult recover(const RecoveryItem&)=0;};
class PaymentPort{public:virtual~PaymentPort()=default;virtual bool ready()=0;
 virtual PaymentOutcome purchase(const std::string&,int64_t)=0;
 virtual PaymentOutcome lookup(const std::string&)=0;
 virtual PaymentOutcome reverse(const std::string&,int64_t)=0;};
class ProfileExecutor final:public ReceiptExecutor{
 public:ProfileExecutor(const CompositeBinding&,DeviceIo&,DurableStorage* storage=nullptr);bool ready()override;
 ExecutionResult execute(const ReceiptPlan&)override;ExecutionResult recover(const RecoveryItem&)override;
 private:const CompositeBinding&binding_;DeviceIo&io_;DurableStorage*storage_{};
};
class EspIdfDeviceIo final:public DeviceIo{
 public:explicit EspIdfDeviceIo(const CompositeBinding&,const std::string& fiscal_password="");bool begin();bool fiscal_ready()override;
 bool payment_ready()override;std::string preflight(const ReceiptPlan&)override;ExecutionResult fiscalize(const ReceiptPlan&)override;
 ExecutionResult fiscal_command(const ReceiptPlan&)override;
 PaymentOutcome purchase(const std::string&,int64_t)override;
 PaymentOutcome payment_lookup(const std::string&)override;
 PaymentOutcome reverse(const std::string&,int64_t)override;
 ExecutionResult recover(const RecoveryItem&)override;
 ExecutionResult fiscal_receipt_proof(const RecoveryItem&)override;
 private:const CompositeBinding&binding_;std::string fiscal_password_;SerialSession serial_;
};
}
