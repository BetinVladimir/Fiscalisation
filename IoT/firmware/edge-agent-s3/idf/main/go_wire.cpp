#include "go_wire.h"
#include <cmath>
namespace beefiscal::go_json {
Value* field(Value& v,const char* name){for(auto& f:v.fields)if(f.first==name)return &f.second;return nullptr;}
const Value* field(const Value& v,const char* name){for(const auto& f:v.fields)if(f.first==name)return &f.second;return nullptr;}
namespace {
bool order(Value& v,std::initializer_list<const char*> names,std::string& e){if((v.kind!=Value::Kind::Map&&v.kind!=Value::Kind::Struct)||v.fields.size()!=names.size()){e="typed JSON field set mismatch";return false;}std::vector<std::pair<std::string,Value>> ordered;for(auto n:names){auto*f=field(v,n);if(!f){e="missing typed JSON field";return false;}ordered.emplace_back(n,*f);}v=Value::object(std::move(ordered),true);return true;}
bool seq(Value& v,const char* key,std::string&e,bool positive=true){auto*f=field(v,key);int64_t n;if(!f||!integer(*f,n)||(positive&&n<1)){e="invalid typed int64";return false;}*f=Value::integer_value(n);return true;}
}
bool event_envelope(Value& v,std::string& e){return order(v,{"event_id","operation_id","device_id","journal_seq","event_type","occurred_at","payload","prev_hash","event_hash","signature"},e)&&seq(v,"journal_seq",e);}
bool sync_batch(Value& v,std::string& e){if(!order(v,{"edge_id","schema_version","first_seq","last_seq","previous_acknowledged_hash","events","batch_sha256","signature"},e)||!seq(v,"first_seq",e)||!seq(v,"last_seq",e))return false;auto*a=field(v,"events");if(!a||a->kind!=Value::Kind::Array){e="invalid events array";return false;}for(auto& x:a->elements)if(!event_envelope(x,e))return false;return true;}
bool sync_ack(Value& v,std::string& e){if(!order(v,{"ack_id","edge_id","committed_through_seq","committed_event_hash","committed_at","operation_results","rejected","signature"},e)||!seq(v,"committed_through_seq",e))return false;auto*a=field(v,"operation_results");if(a->kind!=Value::Kind::Null&&a->kind!=Value::Kind::Array){e="invalid operation results";return false;}for(auto& x:a->elements)if(!order(x,{"operation_id","state","version"},e)||!seq(x,"version",e,false))return false;auto*r=field(v,"rejected");if(r->kind!=Value::Kind::Null&&r->kind!=Value::Kind::Array){e="invalid rejected array";return false;}return true;}
bool contains_nul(const Value& v){if(v.kind==Value::Kind::String&&v.text.find('\0')!=std::string::npos)return true;for(const auto& f:v.fields)if(f.first.find('\0')!=std::string::npos||contains_nul(f.second))return true;for(const auto& x:v.elements)if(contains_nul(x))return true;return false;}
bool safe_cjson_numbers(const Value& v){if(v.kind==Value::Kind::Integer&&(v.integer < -9007199254740991LL||v.integer > 9007199254740991LL))return false;if(v.kind==Value::Kind::Number&&(!std::isfinite(v.number)||std::fabs(v.number)>9007199254740991.0))return false;for(const auto& f:v.fields)if(!safe_cjson_numbers(f.second))return false;for(const auto& x:v.elements)if(!safe_cjson_numbers(x))return false;return true;}
}
