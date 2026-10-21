#pragma once
#include <cstddef>
#include <cstdint>
#include "cJSON.h"
#include "esp_err.h"
#include "go_json.h"
namespace beefiscal::idf{
// Lossless typed command decoder; no double/cJSON bridge.
esp_err_t canonical_cbor_to_value(const uint8_t*,size_t,go_json::Value&);
esp_err_t canonical_cbor_to_json(const uint8_t*,size_t,cJSON**);
esp_err_t canonical_json_to_cbor(const cJSON*,uint8_t**,size_t*);
}
