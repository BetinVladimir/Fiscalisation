#include "go_json.h"
#include "cJSON.h"
#define FMT_HEADER_ONLY
#include "vendor/fmt/format.h"
#include <algorithm>
#include <cmath>
#include <unordered_set>
namespace beefiscal::go_json {
Value Value::null(){return {};}
Value Value::boolean_value(bool b){Value v;v.kind=Kind::Boolean;v.boolean=b;return v;}
Value Value::integer_value(int64_t i){Value v;v.kind=Kind::Integer;v.integer=i;return v;}
Value Value::number_value(double n){Value v;v.kind=Kind::Number;v.number=n;return v;}
Value Value::string(std::string s){Value v;v.kind=Kind::String;v.text=std::move(s);return v;}
Value Value::array(std::vector<Value> a){Value v;v.kind=Kind::Array;v.elements=std::move(a);return v;}
Value Value::object(std::vector<std::pair<std::string,Value>> f,bool ordered){Value v;v.kind=ordered?Kind::Struct:Kind::Map;v.fields=std::move(f);return v;}
namespace {
void quote(const std::string& s,std::string& out){
 static constexpr char hex[]="0123456789abcdef";out+='"';
 for(size_t i=0;i<s.size();){unsigned char c=s[i];
  if(c<0x80){++i;switch(c){case '"':out+="\\\"";break;case '\\':out+="\\\\";break;case '\n':out+="\\n";break;case '\r':out+="\\r";break;case '\t':out+="\\t";break;case '\b':out+="\\b";break;case '\f':out+="\\f";break;
   default:if(c<0x20||c=='<'||c=='>'||c=='&'){out+="\\u00";out+=hex[c>>4];out+=hex[c&15];}else out+=char(c);}continue;}
  size_t n=c>=0xc2&&c<=0xdf?2:c>=0xe0&&c<=0xef?3:c>=0xf0&&c<=0xf4?4:0;
  bool valid=n&&i+n<=s.size();for(size_t j=1;valid&&j<n;++j)valid=(static_cast<unsigned char>(s[i+j])&0xc0)==0x80;
  if(valid&&n>=3){unsigned char b=s[i+1];valid=!(c==0xe0&&b<0xa0)&&!(c==0xed&&b>=0xa0)&&!(c==0xf0&&b<0x90)&&!(c==0xf4&&b>=0x90);}
  if(!valid){out+="\\ufffd";++i;continue;}
  if(n==3&&c==0xe2&&static_cast<unsigned char>(s[i+1])==0x80&&(static_cast<unsigned char>(s[i+2])==0xa8||static_cast<unsigned char>(s[i+2])==0xa9))out+=static_cast<unsigned char>(s[i+2])==0xa8?"\\u2028":"\\u2029";
  else out.append(s,i,n);
  i+=n;
 }out+='"';
}
bool number(double x,std::string& out){
 if(!std::isfinite(x))return false;
 if(std::signbit(x))out+='-';
 const double a=std::fabs(x);if(a==0){out+='0';return true;}
 auto d=fmt::detail::dragonbox::to_decimal(a);
 std::string digits=std::to_string(d.significand);
 int point=static_cast<int>(digits.size())+d.exponent;
 if(a<1e-6||a>=1e21){out+=digits[0];if(digits.size()>1){out+='.';out+=digits.substr(1);}int e=point-1;out+=e<0?"e-":"e+";out+=std::to_string(e<0?-e:e);}
 else if(point<=0){out+="0.";out.append(static_cast<size_t>(-point),'0');out+=digits;}
 else if(point>=static_cast<int>(digits.size())){out+=digits;out.append(static_cast<size_t>(point-static_cast<int>(digits.size())),'0');}
 else{out+=digits.substr(0,point);out+='.';out+=digits.substr(point);}
 return true;
}
bool write(const Value& v,std::string& o,std::string& error,unsigned depth){
 if(depth>128){error="JSON nesting exceeds 128";return false;}
 switch(v.kind){
 case Value::Kind::Null:o+="null";return true;
 case Value::Kind::Boolean:o+=v.boolean?"true":"false";return true;
 case Value::Kind::Integer:o+=std::to_string(v.integer);return true;
 case Value::Kind::Number:if(number(v.number,o))return true;error="nonfinite JSON number";return false;
 case Value::Kind::String:quote(v.text,o);return true;
 case Value::Kind::Array:o+='[';for(size_t i=0;i<v.elements.size();++i){if(i)o+=',';if(!write(v.elements[i],o,error,depth+1))return false;}o+=']';return true;
 case Value::Kind::Map:case Value::Kind::Struct:{
 std::vector<const std::pair<std::string,Value>*> f;std::unordered_set<std::string> seen;
 for(const auto& p:v.fields){if(!seen.insert(p.first).second){error="duplicate JSON key";return false;}f.push_back(&p);}
 if(v.kind==Value::Kind::Map)std::sort(f.begin(),f.end(),[](auto a,auto b){return a->first<b->first;});
 o+='{';for(size_t i=0;i<f.size();++i){if(i)o+=',';quote(f[i]->first,o);o+=':';if(!write(f[i]->second,o,error,depth+1))return false;}o+='}';return true;}}
 error="invalid JSON kind";return false;
}
bool convert(const cJSON* p,Value& v,std::string& e,unsigned depth){
 if(!p||depth>128){e="invalid JSON or nesting exceeds 128";return false;}
 if(cJSON_IsNull(p))v=Value::null();else if(cJSON_IsBool(p))v=Value::boolean_value(cJSON_IsTrue(p));else if(cJSON_IsNumber(p))v=Value::number_value(p->valuedouble);else if(cJSON_IsString(p))v=Value::string(p->valuestring?p->valuestring:"");
 else if(cJSON_IsArray(p)||cJSON_IsObject(p)){v=Value::object({});if(cJSON_IsArray(p))v.kind=Value::Kind::Array;for(auto* child=p->child;child;child=child->next){Value c;if(!convert(child,c,e,depth+1))return false;if(v.kind==Value::Kind::Array)v.elements.push_back(std::move(c));else v.fields.emplace_back(child->string?child->string:"",std::move(c));}}
 else{e="unsupported cJSON type";return false;}return true;
}
}
bool from_cjson(const cJSON* p,Value& v,std::string& e){e.clear();return convert(p,v,e,0);}
bool encode(const Value& v,std::string& output,std::string& error){std::string next;error.clear();if(!write(v,next,error,0))return false;output=std::move(next);return true;}
}
