#pragma once
#include <string>
#include "esp_err.h"
namespace beefiscal::idf {
// Station remains disconnected on bad credentials/timeout; no READY is inferred
// from a successful esp_wifi_start call.
esp_err_t provisioned_network_start(const std::string&ssid,const std::string&password);
}
