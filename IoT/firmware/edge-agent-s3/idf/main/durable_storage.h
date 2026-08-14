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
  int64_t updated_at;};
struct AuthorityReservation{int64_t operation_sequence{},unp_sequence{};std::string unp;};
struct InitialPayment{std::string payment_id;int64_t amount_minor{};};
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
    const std::function<std::string(const AuthorityReservation&)>& event_payload);
  esp_err_t set_operation_state(const char* operation_id,const char* state,
    const char* result_payload,int64_t now);
  esp_err_t upsert_receipt(const char* receipt_id,const char* operation_id,
    const char* state,const char* canonical_payload,int64_t now);
  esp_err_t upsert_payment(const char* payment_id,const char* operation_id,
    const char* receipt_id,const char* state,int64_t amount_minor,const char* terminal_reference,
    const char* rrn,const char* auth_code,int64_t now);
  esp_err_t append_event(const char* event_id,const char* operation_id,const char* kind,
    const char* payload,const char* digest,int64_t now);
  esp_err_t pending(size_t limit,const std::function<bool(const PendingEvent&)>&);
  esp_err_t record_publish_attempt(const char* event_id,const char* error,int64_t now);
  esp_err_t acknowledge_through(int64_t sequence,const char* ack_id,
    const char* committed_event_hash,int64_t now);
  esp_err_t recovery(size_t limit,const std::function<bool(const RecoveryItem&)>&);
  esp_err_t prune_acknowledged(int64_t now,uint32_t retention_days=93);
  int64_t acknowledged_cursor()const;
  std::string acknowledged_event_hash()const;
  esp_err_t configure_authority(const char*prefix,int64_t first,int64_t last,
    int64_t generation);
  esp_err_t reserve_authority(AuthorityReservation&);
  esp_err_t load_local_sale(const char* surrogate_id,std::string& payload,
    int64_t& version,std::string& state)const;
  esp_err_t save_local_sale(const char* surrogate_id,int64_t version,
    const char* state,const char* payload,int64_t now);
private:struct Impl;Impl*impl_;
};
}
