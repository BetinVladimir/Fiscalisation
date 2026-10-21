#include "canonical_cbor.h"
#include <cassert>
#include <cmath>
#include <cstring>
#include <cstdlib>
#include <iostream>
#include <limits>
using namespace beefiscal::idf;using V=beefiscal::go_json::Value;
static std::string encode(const V&v){uint8_t*p=reinterpret_cast<uint8_t*>(1);size_t n=99;assert(canonical_value_to_cbor(v,&p,&n)==ESP_OK);static const char h[]="0123456789abcdef";std::string s;for(size_t i=0;i<n;++i){s+=h[p[i]>>4];s+=h[p[i]&15];}free(p);return s;}
static void bad(const V&v){uint8_t*p=reinterpret_cast<uint8_t*>(1);size_t n=99;assert(canonical_value_to_cbor(v,&p,&n)==ESP_ERR_INVALID_ARG&&!p&&!n);}
int main(){
 const std::pair<int64_t,const char*>cases[]={{0,"00"},{23,"17"},{24,"1818"},{255,"18ff"},{256,"190100"},{65535,"19ffff"},{65536,"1a00010000"},{INT32_MAX,"1a7fffffff"},{2147483648LL,"1a80000000"},{4294967295LL,"1affffffff"},{4294967296LL,"1b0000000100000000"},{9007199254740991LL,"1b001fffffffffffff"},{9007199254740992LL,"1b0020000000000000"},{9007199254740993LL,"1b0020000000000001"},{INT64_MAX,"1b7fffffffffffffff"},{INT64_MIN,"3b7fffffffffffffff"},{-24,"37"},{-25,"3818"},{-256,"38ff"},{-257,"390100"},{-65536,"39ffff"},{-65537,"3a00010000"},{-4294967296LL,"3affffffff"},{-4294967297LL,"3b0000000100000000"}};
 for(const auto&c:cases){assert(encode(V::integer_value(c.first))==c.second);std::cout<<c.first<<" "<<c.second<<"\n";}
 assert(encode(V::object({{"z",V::integer_value(2)},{"a",V::string("first")}}))=="a26161656669727374617a02");
 assert(encode(V::array({V::null(),V::boolean_value(true),V::boolean_value(false),V::array({}),V::object({})}))=="85f6f5f480a0");
 assert(encode(V::object({{"é",V::integer_value(2)},{"aa",V::integer_value(1)}}))=="a26261610162c3a902");
 for(double n:{0.0,7.0,7.5,std::numeric_limits<double>::quiet_NaN(),std::numeric_limits<double>::infinity(),-std::numeric_limits<double>::infinity()})bad(V::number_value(n));
 bad(V::object({{"a",V::null()},{"a",V::null()}}));bad(V::object({{"a",V::null()},{"a",V::null()}},true));
 for(const auto&s:{std::string("\xc0\x80",2),std::string("\xed\xa0\x80",3),std::string("\xf4\x90\x80\x80",4),std::string("\xe2\x82",2),std::string("\x80",1),std::string("a\0b",3)}){bad(V::string(s));bad(V::object({{s,V::null()}}));}
 assert(encode(V::string("\xf0\x9f\x98\x80"))=="64f09f9880");
 V depth=V::null();for(int i=0;i<15;++i)depth=V::array({depth});assert(!encode(depth).empty());bad(V::array({depth}));
 assert(encode(V::string(std::string(8189,'x'))).size()==16384);bad(V::string(std::string(8190,'x')));bad(V::string(std::string(8193,'x')));bad(V::array(std::vector<V>(8193,V::null())));
 for(double n:{2147483648.0,9007199254740991.0,-9007199254740991.0}){cJSON*v=cJSON_CreateNumber(n);uint8_t*p{};size_t z{};assert(canonical_json_to_cbor(v,&p,&z)==ESP_OK);free(p);cJSON_Delete(v);}
 for(double n:{9007199254740992.0,-9007199254740992.0,1.5,std::numeric_limits<double>::quiet_NaN(),std::numeric_limits<double>::infinity()}){cJSON*v=cJSON_CreateNumber(n);uint8_t*p=reinterpret_cast<uint8_t*>(1);size_t z=99;assert(canonical_json_to_cbor(v,&p,&z)==ESP_ERR_INVALID_ARG&&!p&&!z);cJSON_Delete(v);}
 cJSON*dup=cJSON_CreateObject();cJSON_AddNumberToObject(dup,"a",1);cJSON_AddNumberToObject(dup,"a",2);uint8_t*p=reinterpret_cast<uint8_t*>(1);size_t z=99;assert(canonical_json_to_cbor(dup,&p,&z)==ESP_ERR_INVALID_ARG&&!p&&!z);cJSON_Delete(dup);
 assert(canonical_json_to_cbor(nullptr,&p,&z)==ESP_ERR_INVALID_ARG&&!p&&!z);p=reinterpret_cast<uint8_t*>(1);assert(canonical_value_to_cbor(V::null(),&p,nullptr)==ESP_ERR_INVALID_ARG&&!p);z=99;assert(canonical_value_to_cbor(V::null(),nullptr,&z)==ESP_ERR_INVALID_ARG&&!z);
 std::string error;V out;for(const auto&s:{std::string("\"\xc0\x80\"",4),std::string("\"\\ud800\""),std::string("\"\\udc00\"")}){assert(!beefiscal::go_json::parse_strict_unicode(s.data(),s.size(),out,error));assert(beefiscal::go_json::parse(s.data(),s.size(),out,error));}
 const std::string paired="\"\\ud83d\\ude00\"";assert(beefiscal::go_json::parse_strict_unicode(paired.data(),paired.size(),out,error)&&out.text=="\xf0\x9f\x98\x80");
 std::cout<<"typed encoder independent hex/bridge/Unicode/limits PASS\n";
}
