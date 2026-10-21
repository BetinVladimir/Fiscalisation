#include "canonical_cbor.h"
#include "go_wire.h"
#include <cassert>
#include <iostream>
#include <string>
#include <vector>
using namespace beefiscal;
static std::vector<uint8_t>hex(const std::string&s){assert(s.size()%2==0);auto digit=[](char c){return c<='9'?c-'0':c-'a'+10;};std::vector<uint8_t>v;for(size_t i=0;i<s.size();i+=2)v.push_back(uint8_t((digit(s[i])<<4)|digit(s[i+1])));return v;}
static go_json::Value decode(const std::vector<uint8_t>&bytes){go_json::Value v;assert(idf::canonical_cbor_to_value(bytes.data(),bytes.size(),v)==ESP_OK);return v;}
static void reject(const std::vector<uint8_t>&bytes){auto v=go_json::Value::string("unchanged");assert(idf::canonical_cbor_to_value(bytes.data(),bytes.size(),v)!=ESP_OK);assert(v.kind==go_json::Value::Kind::String&&v.text=="unchanged");}
int main(){
 struct Integer{const char*wire;int64_t expected;const char*json;};
 for(const auto&v:std::vector<Integer>{{"00",0,"0"},{"20",-1,"-1"},{"192000",8192,"8192"},{"192001",8193,"8193"},{"1a7fffffff",INT32_MAX,"2147483647"},{"1a80000000",int64_t(INT32_MAX)+1,"2147483648"},{"3a7fffffff",INT32_MIN,"-2147483648"},{"3a80000000",int64_t(INT32_MIN)-1,"-2147483649"},{"1b7fffffffffffffff",INT64_MAX,"9223372036854775807"},{"3b7fffffffffffffff",INT64_MIN,"-9223372036854775808"}}){
  auto value=decode(hex(v.wire));int64_t integer=0;assert(value.kind==go_json::Value::Kind::Integer&&go_json::integer(value,integer)&&integer==v.expected);std::string json,error;assert(go_json::encode(value,json,error)&&json==v.json);
 }
 for(const char*bad:{"1b8000000000000000","3b8000000000000000","1bffffffffffffffff","3bffffffffffffffff","1817","1900ff","1a0000ffff","1b00000000ffffffff","780161","9800","9fff","c001","f93c00","fa3f800000","fb3ff0000000000000","f7","0000","630102","6100","a2616101616102","a241010162303102","622000","992001","592001","792001"})reject(hex(bad));
 assert(decode(hex("430001ff")).text=="0001ff");
 for(const char*bad_utf8:{"62c080","63eda080","64f4908080","61c2","6180","62e180","64f0808080","62c1bf","64f5808080"})reject(hex(bad_utf8));
 assert(decode(hex("63e29895")).text=="\xe2\x98\x95");assert(decode(hex("64f09f9880")).text=="\xf0\x9f\x98\x80");
 std::vector<uint8_t>depth(15,0x81);depth.push_back(0);decode(depth);depth.insert(depth.begin(),0x81);reject(depth);
 std::vector<uint8_t>maximum{0x79,0x1f,0xfd};maximum.insert(maximum.end(),8189,'a');assert(maximum.size()==8192&&decode(maximum).text.size()==8189);
 std::vector<uint8_t>too_long{0x79,0x1f,0xfe};too_long.insert(too_long.end(),8190,'a');assert(too_long.size()==8193);reject(too_long);
 assert(go_json::safe_cjson_numbers(decode(hex("1b001fffffffffffff"))));assert(!go_json::safe_cjson_numbers(decode(hex("1b0020000000000000"))));
 assert(go_json::safe_cjson_numbers(decode(hex("3b001ffffffffffffe"))));assert(!go_json::safe_cjson_numbers(decode(hex("3b001fffffffffffff"))));
 assert(!go_json::safe_cjson_numbers(go_json::Value::object({{"nested",go_json::Value::array({go_json::Value::integer_value(INT64_MIN)})}})));
 assert(!go_json::safe_cjson_numbers(go_json::Value::number_value(9007199254740992.0)));
 // JSON lexical strictness remains separate from the typed CBOR integer path.
 go_json::Value json;std::string error;int64_t integer{};
 for(const char*number:{"7.0","7e0"}){assert(go_json::parse(number,std::char_traits<char>::length(number),json,error));assert(!go_json::integer(json,integer));}
 assert(go_json::parse("7",1,json,error)&&go_json::integer(json,integer)&&integer==7);
 std::cout<<"direct CBOR int64/UTF8/length/depth/minimal/duplicate/boundary vectors PASS\n";
}
