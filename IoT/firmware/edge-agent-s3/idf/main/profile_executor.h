#pragma once
#include "intent_processor.h"
#include "provisioned_binding.h"
namespace beefiscal::idf{
enum class PaymentCertainty:uint8_t{Approved,Declined,NotSent,Unknown};
struct PaymentOutcome{PaymentCertainty certainty{PaymentCertainty::NotSent};std::string reference,rrn,auth_code,error_code;};
class DeviceIo{public:virtual~DeviceIo()=default;virtual bool fiscal_ready()=0;
 virtual bool payment_ready()=0;virtual ExecutionResult fiscalize(const ReceiptPlan&)=0;
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
 public:explicit EspIdfDeviceIo(const CompositeBinding&);bool begin();bool fiscal_ready()override;
 bool payment_ready()override;ExecutionResult fiscalize(const ReceiptPlan&)override;
 ExecutionResult fiscal_command(const ReceiptPlan&)override;
 PaymentOutcome purchase(const std::string&,int64_t)override;
 PaymentOutcome payment_lookup(const std::string&)override;
 PaymentOutcome reverse(const std::string&,int64_t)override;
 ExecutionResult recover(const RecoveryItem&)override;
 private:const CompositeBinding&binding_;
};
}
