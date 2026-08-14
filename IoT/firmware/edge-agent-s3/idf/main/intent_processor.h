#pragma once
#include <cstddef>
#include <cstdint>
#include <string>
#include <vector>
#include "durable_storage.h"
#include "edge_runtime_port.h"
#include "provisioned_binding.h"

namespace beefiscal::idf{
struct IntentLine{std::string id,name,quantity,unit_price,tax_group,discount;};
struct IntentPayment{std::string id,type,amount;int64_t amount_minor{};};
struct ReceiptPlan{std::string operation_id,receipt_session_id,sale_id,unp,command_type,
  canonical_payload,payload_digest,route_snapshot,amount,original_datetime,original_fmin;
  uint32_t original_document{};uint8_t reversal_reason{};std::vector<IntentLine>items;
  std::vector<IntentPayment>payments;};
enum class ExecutionCertainty:uint8_t{Committed,Compensated,Rejected,Unknown,DeviceUnavailable};
struct ExecutionResult{ExecutionCertainty certainty{ExecutionCertainty::Rejected};
  std::string fiscal_reference,error_code,terminal_reference,rrn,auth_code;};
class ReceiptExecutor{public:virtual~ReceiptExecutor()=default;
  virtual bool ready()=0;virtual ExecutionResult execute(const ReceiptPlan&)=0;
  virtual ExecutionResult recover(const RecoveryItem&)=0;};
using ResultSink=esp_err_t(*)(const char* operation_id,const char* result_json,Ingress,void*);
class IntentProcessor final{
 public:IntentProcessor(DurableStorage&,const CompositeBinding&,ReceiptExecutor&,
  const std::string& command_hmac_key,ResultSink=nullptr,void* = nullptr);
  esp_err_t accept(const CommandView&);esp_err_t recover_pending();
 private:DurableStorage&storage_;const CompositeBinding&binding_;ReceiptExecutor&executor_;
  std::string command_hmac_key_;ResultSink result_sink_{};void*result_context_{};
  esp_err_t decode(const CommandView&,ReceiptPlan&);esp_err_t execute(ReceiptPlan&,Ingress);
};
}
