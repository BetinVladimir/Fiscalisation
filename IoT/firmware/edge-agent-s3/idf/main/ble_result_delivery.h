#pragma once
#include "edge_runtime_port.h"
namespace beefiscal::idf {
// Best-effort notification result, not the financial operation outcome.
esp_err_t deliver_ble_result(const char*,const char*,Ingress,BleReplyOwner,void*);
}
