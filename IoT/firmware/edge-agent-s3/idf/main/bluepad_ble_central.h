#pragma once
#include "provisioned_binding.h"
#include "profile_executor.h"
namespace beefiscal::idf{
esp_err_t bluepad_ble_start(const PaymentEndpointBinding&);
// Called by the shared NimBLE host after controller synchronization.  The POS
// GATT peripheral and BluePad central deliberately share one NimBLE host.
void bluepad_ble_on_host_sync();
bool bluepad_ble_ready();
PaymentOutcome bluepad_ble_purchase(const std::string& operation_id,int64_t amount_minor);
PaymentOutcome bluepad_ble_lookup(const std::string& reference);
PaymentOutcome bluepad_ble_reverse(const std::string& reference,int64_t amount_minor);
}
