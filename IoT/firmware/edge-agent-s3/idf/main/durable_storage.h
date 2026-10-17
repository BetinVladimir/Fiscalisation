#pragma once
#include <cstddef>
#include <cstdint>
#include <functional>
#include <string>
#include <vector>
#include "esp_err.h"

namespace beefiscal::idf {
enum class ReservationStatus:uint8_t{New,Duplicate,PayloadConflict,Invalid,StorageFailure};
struct ReservationResult{ReservationStatus status;std::string prior_result;};
struct PendingEvent{int64_t sequence;std::string event_id;std::string operation_id;
  std::string kind;std::string payload;std::string payload_digest;int64_t created_at;
  uint32_t attempts;};
struct RecoveryItem{std::string operation_id;std::string receipt_id;std::string state;
  std::string route_snapshot;std::string canonical_payload;std::string canonical_unp;
  std::string payment_id;
  int64_t payment_amount_minor{};std::string payment_state,terminal_reference,rrn,auth_code;
  int64_t updated_at;std::string payload_digest;};
struct FrozenSyncBatch{std::string batch_id,body,last_event_hash,previous_hash;int64_t first_sequence{},last_sequence{};};
struct AuthorityReservation{int64_t operation_sequence{},unp_sequence{};std::string unp;std::string prefix;int64_t range_start{},range_end{},generation{};};
struct InitialPayment{std::string payment_id;int64_t amount_minor{};};
struct PaymentRecord{std::string payment_id,operation_id,receipt_id,state,terminal_reference,rrn,auth_code,reversal_reference,reversal_rrn,reversal_auth_code;int64_t amount_minor{};};
struct PaymentSagaSnapshot{std::string phase;std::vector<PaymentRecord> payments;};
struct OperationRecord{std::string operation_id,payload_digest,state,result_payload;int64_t updated_at{};};
struct AtomicReservationResult{ReservationStatus status;std::string prior_result;
  AuthorityReservation authority;};
struct StorageConfig{const char* mount_point;const char* database_relative_path;
  int clk_pin;int cmd_pin;int d0_pin;bool one_bit;};

class DurableStorage final{
public:
  DurableStorage();~DurableStorage();DurableStorage(const DurableStorage&)=delete;
  DurableStorage&operator=(const DurableStorage&)=delete;
  esp_err_t open(const StorageConfig&);esp_err_t open_database_for_test(const char* path);
  void close();bool ready()const;
  bool can_accept_operation(int64_t reserve_bytes=262144)const;
  ReservationResult reserve(const char* operation_id,const char* payload_digest,
    const char* transport,const char* route_snapshot,int64_t now);
  ReservationResult reserve_with_received_event(const char* operation_id,
    const char* payload_digest,const char* transport,const char* route_snapshot,
    const char* event_id,const char* event_payload,int64_t now);
  AtomicReservationResult reserve_command(const char* operation_id,
    const char* payload_digest,const char* transport,const char* route_snapshot,
    const char* event_id,const char* receipt_id,const char* canonical_payload,
    const std::vector<InitialPayment>& payments,int64_t now,
    const std::function<std::string(const AuthorityReservation&)>& event_payload,const char* execution_unp=nullptr,const char* operator_code=nullptr,bool claim_sale=false);
  esp_err_t set_operation_state(const char* operation_id,const char* state,
    const char* result_payload,int64_t now);
  // Atomically project a result and durable outbox evidence. Only explicitly
  // supplied payments are updated; executor-owned CARD records stay untouched.
  esp_err_t complete_operation(const char* operation_id,const char* receipt_id,
    const char* state,const char* canonical_payload,const char* result_payload,
    const std::vector<InitialPayment>& payments,const char* event_id,
    const char* event_kind,const char* payload_digest,int64_t now);
  esp_err_t operation(const char* operation_id,OperationRecord&)const;
  esp_err_t upsert_receipt(const char* receipt_id,const char* operation_id,
    const char* state,const char* canonical_payload,int64_t now);
  esp_err_t upsert_payment(const char* payment_id,const char* operation_id,
    const char* receipt_id,const char* state,int64_t amount_minor,const char* terminal_reference,
    const char* rrn,const char* auth_code,int64_t now);
  esp_err_t payment(const char* payment_id,PaymentRecord&)const;
  esp_err_t payment_saga_phase(const char* operation_id,std::string&)const;
  esp_err_t payment_saga_snapshot(const char* operation_id,const char* receipt_id,PaymentSagaSnapshot&)const;
  // Recovery CAS compares the complete observed row; it never purchases again.
  esp_err_t reconcile_payment(const PaymentRecord& expected,const PaymentRecord& observed,
    const char* next_state,int64_t now);
  esp_err_t begin_payment_saga(const char* operation_id,const char* receipt_id,
    const std::vector<InitialPayment>& card_payments,int64_t now);
  esp_err_t checkpoint_payment(const PaymentRecord&,const char* expected_state,
    const char* next_state,const char* saga_phase,int64_t now);
  esp_err_t checkpoint_saga_phase(const char* operation_id,const char* expected_phase,
    const char* next_phase,int64_t now);
  esp_err_t append_event(const char* event_id,const char* operation_id,const char* kind,
    const char* payload,const char* digest,int64_t now);
  esp_err_t pending(size_t limit,const std::function<bool(const PendingEvent&)>&);
  esp_err_t record_publish_attempt(const char* event_id,const char* error,int64_t now);
  esp_err_t frozen_sync_batch(FrozenSyncBatch&)const;
  esp_err_t freeze_sync_batch(const FrozenSyncBatch&,const std::vector<PendingEvent>&);
  esp_err_t acknowledge_through(int64_t sequence,const char* ack_id,
    const char* committed_event_hash,int64_t now);
  esp_err_t recovery(size_t limit,const std::function<bool(const RecoveryItem&)>&);
  esp_err_t prune_acknowledged(int64_t now,uint32_t retention_days=93);
  int64_t acknowledged_cursor()const;
  std::string acknowledged_event_hash()const;
  esp_err_t configure_authority(const char*prefix,int64_t first,int64_t last,
    int64_t generation);
  esp_err_t reserve_authority(AuthorityReservation&,const char* operator_code=nullptr);
  esp_err_t load_local_sale(const char* surrogate_id,std::string& payload,
    int64_t& version,std::string& state)const;
  esp_err_t save_local_sale(const char* surrogate_id,int64_t version,
    const char* state,const char* payload,int64_t now);
private:struct Impl;Impl*impl_;
};
}
