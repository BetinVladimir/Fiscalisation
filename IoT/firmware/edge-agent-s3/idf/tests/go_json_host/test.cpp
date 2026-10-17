#include "go_json.h"
#include "cJSON.h"
#include <openssl/sha.h>
#include <cassert>
#include <cstring>
#include <fstream>
#include <iostream>
#include <limits>
using namespace beefiscal::go_json;
std::string hash(const std::string& s){unsigned char h[32];SHA256((const unsigned char*)s.data(),s.size(),h);std::string o;const char*x="0123456789abcdef";for(auto b:h){o+=x[b>>4];o+=x[b&15];}return o;}
Value parse(const std::string&s){auto*p=cJSON_Parse(s.c_str());assert(p);Value v;std::string e;assert(from_cjson(p,v,e));cJSON_Delete(p);return v;}
void typed(Value&v,bool batch){v.kind=Value::Kind::Struct;for(auto&f:v.fields){if(f.first=="journal_seq"||f.first=="first_seq"||f.first=="last_seq")f.second=Value::integer_value(INT64_MAX);if(batch&&f.first=="events")for(auto&e:f.second.elements)typed(e,false);}}
int main(int argc,char**argv){assert(argc==2);std::ifstream f(argv[1]);std::string line,o,e;size_t count=0;
 while(std::getline(f,line)){auto p=line.find('\t'),q=line.find('\t',p+1);assert(p!=std::string::npos&&q!=std::string::npos);auto input=line.substr(p+1,q-p-1),expected=line.substr(q+1);Value v;
 if(line[0]=='F'){uint64_t bits=std::stoull(input,nullptr,16);double n;std::memcpy(&n,&bits,8);v=Value::number_value(n);Value parsed;std::string roundtrip;assert(beefiscal::go_json::parse(expected.data(),expected.size(),parsed,e));assert(encode(parsed,roundtrip,e)&&roundtrip==expected);}
 else{v=parse(input);if(line[0]=='E'||line[0]=='B')typed(v,line[0]=='B');}
 assert(encode(v,o,e));if(line[0]=='E'||line[0]=='B'||line[0]=='P'||line[0]=='C'){if(hash(o)!=expected){std::cerr<<"hash mismatch "<<line[0]<<" "<<o<<"\n";return 1;}}
 else if(o!=expected){std::cerr<<"mismatch "<<input<<" expected "<<expected<<" got "<<o<<"\n";return 1;}++count;}
 assert(count>2000);
 assert(encode(Value::string(std::string("a\0b",3)),o,e)&&o=="\"a\\u0000b\"");
 assert(encode(Value::string(std::string("\xff\xc0\xaf",3)),o,e)&&o=="\"\\ufffd\\ufffd\\ufffd\"");
 const std::string unchanged="unchanged";o=unchanged;assert(!encode(Value::number_value(std::numeric_limits<double>::infinity()),o,e)&&o==unchanged);
 assert(!encode(Value::object({{"a",Value::null()},{"a",Value::null()}}),o,e));
 std::cout<<"Go oracle corpus PASS "<<count<<" cases\n";
}
