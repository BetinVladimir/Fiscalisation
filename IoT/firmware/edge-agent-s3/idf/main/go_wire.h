#pragma once
#include "go_json.h"
namespace beefiscal::go_json {
Value* field(Value&, const char*);
const Value* field(const Value&, const char*);
// Typed structs used by the actual Go domain; strict field set and int64 tokens.
bool event_envelope(Value&, std::string& error);
bool sync_batch(Value&, std::string& error);
bool sync_ack(Value&, std::string& error);
bool contains_nul(const Value&);
// Only for business decode via cJSON; prevents loss at cJSON integer consumers.
bool safe_cjson_numbers(const Value&);
}
