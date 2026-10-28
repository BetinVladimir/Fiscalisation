#pragma once
#include "durable_storage.h"
#include "execution_authorization.h"
namespace beefiscal::idf {
struct VerifiedPaymentReconciliation {PaymentRecord expected,observed;PaymentReconciliationEvidence evidence;};
esp_err_t verify_payment_reconciliation(const go_json::Value&,const CompositeBinding&,const ExecutionAuthorizationTrust&,VerifiedPaymentReconciliation&);
esp_err_t apply_payment_reconciliation(const go_json::Value&,const CompositeBinding&,DurableStorage&,const ExecutionAuthorizationTrust* =nullptr);
}
