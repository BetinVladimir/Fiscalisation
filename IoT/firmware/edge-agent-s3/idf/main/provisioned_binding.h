#pragma once
#include <cstddef>
#include <cstdint>
#include <string>
#include "esp_err.h"

namespace beefiscal::idf {
enum class EdgeProfile:uint8_t{Unconfigured,DatecsDp150BluePad50,DaisyCompactS01};
struct FiscalEndpointBinding{std::string device_id,vendor,model,transport,usb_serial;
  uint32_t uart_baud{};uint8_t uart_data_bits{};char uart_parity{'N'};uint8_t uart_stop_bits{};
  int uart_tx_pin{-1},uart_rx_pin{-1};uint16_t usb_vid{},usb_pid{};uint8_t usb_interface{};};
struct PaymentEndpointBinding{bool present{};std::string device_id,vendor,model,transport,
  ble_identity,service_uuid,tx_characteristic_uuid,rx_characteristic_uuid;};
struct MqttBinding{std::string uri,client_id,command_topic,sync_topic,ack_topic,
  root_ca_ref,client_certificate_ref,client_key_ref;};
struct OperationalAuthority{std::string command_hmac_ref,sync_ack_hmac_ref,
  transaction_signing_kid,unp_prefix;int64_t unp_range_start{},unp_range_end{};};
struct CompositeBinding{uint32_t schema_version{};int64_t generation{};
  std::string tenant_id,location_id,register_id,edge_device_id,ble_advertising_identity;
  EdgeProfile profile{EdgeProfile::Unconfigured};FiscalEndpointBinding fiscal;
  PaymentEndpointBinding payment;MqttBinding mqtt;OperationalAuthority authority;
  std::string payload_sha256;};
struct SignedBindingEnvelope{const char* canonical_json{};size_t canonical_json_size{};
  const char* signature_base64url{};const char* key_id{};};

class ProvisionedBindingStore final{
 public:
  explicit ProvisionedBindingStore(const char* trusted_signing_public_key_pem,
                                    const char* trusted_key_id);
  ~ProvisionedBindingStore();ProvisionedBindingStore(const ProvisionedBindingStore&)=delete;
  ProvisionedBindingStore&operator=(const ProvisionedBindingStore&)=delete;
  esp_err_t open(const char* nvs_namespace="edge-binding");void close();
  esp_err_t load(CompositeBinding&);esp_err_t install(const SignedBindingEnvelope&,
    CompositeBinding* installed=nullptr);int64_t generation()const{return generation_;}
  static esp_err_t decode_and_validate(const char*,size_t,CompositeBinding&);
 private:struct Impl;Impl*impl_;const char*public_key_;const char*key_id_;int64_t generation_{};
  esp_err_t verify(const SignedBindingEnvelope&)const;
};
const char*edge_profile_name(EdgeProfile);
}
