#pragma once
#include "device_identity.h"
#include "durable_storage.h"
#include "provisioned_binding.h"
#include "esp_err.h"
namespace beefiscal::idf{
esp_err_t sync_runtime_start(DurableStorage&,const CompositeBinding&,DeviceIdentity&,
  const char*sync_ack_hmac_key);
esp_err_t sync_runtime_accept_ack(const uint8_t*,size_t,void*);
}
