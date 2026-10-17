#pragma once
#include <cstdint>
#include <string>
#include <utility>
#include <vector>
struct cJSON;
namespace beefiscal::go_json {
// Objects decoded into Go map[string]any sort keys recursively; Go structs retain
// declaration order. Integer is separate: int64 envelope fields must never pass
// through cJSON's double representation.
struct Value {
 enum class Kind { Null, Boolean, Integer, Number, String, Array, Map, Struct };
 Kind kind=Kind::Null; bool boolean=false; int64_t integer=0; double number=0;
 std::string text; std::vector<Value> elements;
 std::vector<std::pair<std::string,Value>> fields;
 static Value null(); static Value boolean_value(bool);
 static Value integer_value(int64_t); static Value number_value(double);
 static Value string(std::string); static Value array(std::vector<Value>);
 static Value object(std::vector<std::pair<std::string,Value>>, bool ordered=false);
};
// cJSON strings are NUL-terminated: reject escaped NUL at parse boundary before
// cJSON, or construct Value strings directly (which preserve embedded NUL).
bool from_cjson(const cJSON*, Value&, std::string& error);
bool encode(const Value&, std::string& output, std::string& error);
} // namespace beefiscal::go_json
