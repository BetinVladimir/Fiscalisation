#pragma once
#include "profile_executor.h"
namespace beefiscal::idf{
class FiscalPort{public:virtual~FiscalPort()=default;virtual bool ready()=0;
 virtual ExecutionResult fiscalize(const ReceiptPlan&)=0;virtual ExecutionResult lookup(const std::string&)=0;
 virtual ExecutionResult cancel(const std::string&)=0;};
class ReceiptSaga final{public:ReceiptSaga(FiscalPort&,PaymentPort*);
 ExecutionResult execute(const ReceiptPlan&);ExecutionResult recover(const ReceiptPlan&,const char*state);
private:FiscalPort&fiscal_;PaymentPort*payment_{};};
}
