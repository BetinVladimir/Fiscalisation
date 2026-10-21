#include "go_json.h"
#include "cJSON.h"
#define FMT_HEADER_ONLY
#include "vendor/fmt/format.h"
#include <algorithm>
#include <cmath>
#include <unordered_set>
#include <cstdlib>
#include <cerrno>
#include <climits>
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
namespace {
class Reader {
 const char* p;const char* end;std::string& error;bool strict;
 void ws(){while(p<end&&(*p==' '||*p=='\t'||*p=='\n'||*p=='\r'))++p;}
 bool bad(){error="invalid JSON";return false;}
 bool hex(unsigned& n){n=0;for(int i=0;i<4;++i){if(p==end)return false;char c=*p++;unsigned d=c>='0'&&c<='9'?unsigned(c-'0'):c>='a'&&c<='f'?unsigned(c-'a'+10):c>='A'&&c<='F'?unsigned(c-'A'+10):16;if(d>15)return false;n=(n<<4)|d;}return true;}
 static void utf8(unsigned c,std::string& s){if(c<0x80)s+=char(c);else if(c<0x800){s+=char(0xc0|(c>>6));s+=char(0x80|(c&63));}else if(c<0x10000){s+=char(0xe0|(c>>12));s+=char(0x80|((c>>6)&63));s+=char(0x80|(c&63));}else{s+=char(0xf0|(c>>18));s+=char(0x80|((c>>12)&63));s+=char(0x80|((c>>6)&63));s+=char(0x80|(c&63));}}
 bool string(std::string& s){if(p==end||*p++!='"')return bad();while(p<end){unsigned char c=*p++;if(c=='"')return true;if(c<32)return bad();if(c!='\\'){if(c<0x80){s+=char(c);continue;}const char* first=p-1;size_t n=c>=0xc2&&c<=0xdf?2:c>=0xe0&&c<=0xef?3:c>=0xf0&&c<=0xf4?4:0;bool valid=n&&size_t(end-first)>=n;for(size_t j=1;valid&&j<n;++j)valid=(static_cast<unsigned char>(first[j])&0xc0)==0x80;if(valid&&n>=3){unsigned char b=first[1];valid=!(c==0xe0&&b<0xa0)&&!(c==0xed&&b>=0xa0)&&!(c==0xf0&&b<0x90)&&!(c==0xf4&&b>=0x90);}if(valid){s.append(first,n);p=first+n;}else {if(strict)return bad();utf8(0xfffd,s);}continue;}if(p==end)return bad();c=*p++;switch(c){case '"':case '/':case '\\':s+=char(c);break;case 'b':s+='\b';break;case 'f':s+='\f';break;case 'n':s+='\n';break;case 'r':s+='\r';break;case 't':s+='\t';break;case 'u':{unsigned u;if(!hex(u))return bad();if(u>=0xd800&&u<=0xdbff){const char* save=p;unsigned low;if(end-p>=6&&p[0]=='\\'&&p[1]=='u'){p+=2;if(hex(low)&&low>=0xdc00&&low<=0xdfff){utf8(0x10000+((u-0xd800)<<10)+(low-0xdc00),s);break;}}p=save;if(strict)return bad();u=0xfffd;}else if(u>=0xdc00&&u<=0xdfff){if(strict)return bad();u=0xfffd;}utf8(u,s);break;}default:return bad();}}return bad();}
 bool value(Value& v,unsigned depth){ws();if(p==end||depth>16)return bad();char c=*p;
 if(c=='"'){v=Value::string("");return string(v.text);}
 if(c=='{'||c=='['){++p;bool map=c=='{';v=map?Value::object({}):Value::array({});std::unordered_set<std::string>seen;ws();if(p<end&&*p==(map?'}':']')){++p;return true;}for(;;){std::string key;if(map){ws();if(!string(key)||!seen.insert(key).second)return bad();ws();if(p==end||*p++!=':')return bad();}Value child;if(!value(child,depth+1))return false;if(map)v.fields.emplace_back(std::move(key),std::move(child));else v.elements.push_back(std::move(child));ws();if(p==end)return bad();c=*p++;if(c==(map?'}':']'))return true;if(c!=',')return bad();}}
 if(c=='n'&&end-p>=4&&std::string(p,4)=="null"){p+=4;v=Value::null();return true;}
 if(c=='t'&&end-p>=4&&std::string(p,4)=="true"){p+=4;v=Value::boolean_value(true);return true;}
 if(c=='f'&&end-p>=5&&std::string(p,5)=="false"){p+=5;v=Value::boolean_value(false);return true;}
 const char* start=p;if(c=='-')++p;if(p==end)return bad();if(*p=='0')++p;else{if(*p<'1'||*p>'9')return bad();while(p<end&&*p>='0'&&*p<='9')++p;}
 if(p<end&&*p=='.'){++p;const char* digits=p;while(p<end&&*p>='0'&&*p<='9')++p;if(digits==p)return bad();}
 if(p<end&&(*p=='e'||*p=='E')){++p;if(p<end&&(*p=='+'||*p=='-'))++p;const char* digits=p;while(p<end&&*p>='0'&&*p<='9')++p;if(digits==p)return bad();}
 std::string token(start,p);char* tail=nullptr;double n=std::strtod(token.c_str(),&tail);if(!tail||*tail||!std::isfinite(n))return bad();v=Value::number_value(n);v.text=std::move(token);return true;
 }
public:Reader(const char* data,size_t n,std::string& e,bool strict_unicode=false):p(data),end(data+n),error(e),strict(strict_unicode){}
 bool read(Value& v){if(!value(v,0))return false;ws();return p==end||bad();}
};
}
bool parse(const char* data,size_t n,Value& v,std::string& error){error.clear();if(!data||!n){error="empty JSON";return false;}Value next;Reader reader(data,n,error);if(!reader.read(next))return false;v=std::move(next);return true;}
bool parse_strict_unicode(const char* data,size_t n,Value& v,std::string& error){error.clear();if(!data||!n){error="empty JSON";return false;}Value next;Reader reader(data,n,error,true);if(!reader.read(next))return false;v=std::move(next);return true;}
bool integer(const Value& v,int64_t& out){if(v.kind==Value::Kind::Integer){out=v.integer;return true;}if(v.kind!=Value::Kind::Number||v.text.empty()||v.text.find_first_of(".eE")!=std::string::npos)return false;errno=0;char* tail=nullptr;long long n=std::strtoll(v.text.c_str(),&tail,10);if(errno==ERANGE||!tail||*tail)return false;out=static_cast<int64_t>(n);return true;}

}
