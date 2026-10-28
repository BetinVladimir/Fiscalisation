#include "canonical_cbor.h"
#include <algorithm>
#include <cstdlib>
#include <cmath>
#include <cstring>
#include <string>
#include <vector>
namespace beefiscal::idf{namespace{
class Decoder{const uint8_t*p_;const uint8_t*end_;int depth_{};
 bool take(uint8_t&v){if(p_==end_)return false;v=*p_++;return true;}
 bool length(uint8_t ai,uint64_t&v){if(ai<24){v=ai;return true;}int n=ai==24?1:ai==25?2:ai==26?4:ai==27?8:0;if(!n||end_-p_<n)return false;v=0;for(int i=0;i<n;i++)v=(v<<8)|*p_++;return(ai==24&&v>=24)||(ai==25&&v>255)||(ai==26&&v>65535)||(ai==27&&v>0xffffffffULL);}
 cJSON*read(){if(++depth_>16)return nullptr;uint8_t first;if(!take(first)){depth_--;return nullptr;}uint8_t major=first>>5,ai=first&31;uint64_t n=0;cJSON*out=nullptr;
  if(major<=5&&!length(ai,n)){depth_--;return nullptr;}if(n>8192){depth_--;return nullptr;}
  if(major==0)out=cJSON_CreateNumber((double)n);else if(major==1&&n<=INT64_MAX)out=cJSON_CreateNumber((double)(-1-(int64_t)n));
  else if(major==2||major==3){if((uint64_t)(end_-p_)<n){depth_--;return nullptr;}if(major==3){if(memchr(p_,0,(size_t)n)){depth_--;return nullptr;}std::string s((const char*)p_,(size_t)n);out=cJSON_CreateString(s.c_str());}else{std::string hex;static const char h[]="0123456789abcdef";hex.resize(n*2);for(size_t i=0;i<n;i++){hex[i*2]=h[p_[i]>>4];hex[i*2+1]=h[p_[i]&15];}out=cJSON_CreateString(hex.c_str());}p_+=n;}
  else if(major==4){out=cJSON_CreateArray();for(uint64_t i=0;out&&i<n;i++){cJSON*v=read();if(!v){cJSON_Delete(out);out=nullptr;break;}cJSON_AddItemToArray(out,v);}}
  else if(major==5){out=cJSON_CreateObject();for(uint64_t i=0;out&&i<n;i++){cJSON*k=read();if(!cJSON_IsString(k)||cJSON_GetObjectItemCaseSensitive(out,k->valuestring)){cJSON_Delete(k);cJSON_Delete(out);out=nullptr;break;}std::string key=k->valuestring;cJSON_Delete(k);cJSON*v=read();if(!v){cJSON_Delete(out);out=nullptr;break;}cJSON_AddItemToObject(out,key.c_str(),v);}}
  else if(major==7&&ai==20)out=cJSON_CreateFalse();
  else if(major==7&&ai==21)out=cJSON_CreateTrue();
  else if(major==7&&ai==22)out=cJSON_CreateNull();
  depth_--;return out;}
 public:Decoder(const uint8_t*p,size_t n):p_(p),end_(p+n){}cJSON*decode(){cJSON*v=read();if(!v||p_!=end_){cJSON_Delete(v);return nullptr;}return v;}};
}
esp_err_t canonical_cbor_to_json(const uint8_t*p,size_t n,cJSON**out){if(!p||!n||n>8192||!out)return ESP_ERR_INVALID_ARG;*out=Decoder(p,n).decode();return*out?ESP_OK:ESP_ERR_INVALID_ARG;}
namespace {
using V=go_json::Value;
constexpr size_t kLimit=8192;
bool valid_text(const std::string&s){
 for(size_t i=0;i<s.size();){const auto c=uint8_t(s[i]);if(!c)return false;if(c<0x80){++i;continue;}
  const size_t n=c>=0xc2&&c<=0xdf?2:c>=0xe0&&c<=0xef?3:c>=0xf0&&c<=0xf4?4:0;
  if(!n||n>s.size()-i)return false;
  for(size_t j=1;j<n;++j)if((uint8_t(s[i+j])&0xc0)!=0x80)return false;
  const auto b=uint8_t(s[i+1]);if((c==0xe0&&b<0xa0)||(c==0xed&&b>=0xa0)||(c==0xf0&&b<0x90)||(c==0xf4&&b>=0x90))return false;i+=n;
 }return true;
}
class Writer {
 std::vector<uint8_t>bytes;
 bool byte(uint8_t v){if(bytes.size()==kLimit)return false;bytes.push_back(v);return true;}
 bool text(const std::string&s){if(s.size()>kLimit||!valid_text(s)||!head(3,s.size())||s.size()>kLimit-bytes.size())return false;bytes.insert(bytes.end(),s.begin(),s.end());return true;}
 bool head(uint8_t major,uint64_t n){
  if(n<24)return byte(uint8_t(major<<5)|uint8_t(n));
  const unsigned width=n<=UINT8_MAX?1:n<=UINT16_MAX?2:n<=UINT32_MAX?4:8;
  if(width+1>kLimit-bytes.size())return false;
  byte(uint8_t(major<<5)|(width==1?24:width==2?25:width==4?26:27));
  for(unsigned i=width;i>0;--i)byte(uint8_t(n>>((i-1)*8)));
  return true;
 }
 bool write(const V&v,unsigned depth){
  if(depth>16)return false;
  switch(v.kind){
   case V::Kind::Null:return byte(0xf6);
   case V::Kind::Boolean:return byte(v.boolean?0xf5:0xf4);
   case V::Kind::Integer:return v.integer>=0?head(0,uint64_t(v.integer)):head(1,uint64_t(-(v.integer+1)));
   case V::Kind::Number:return false;
   case V::Kind::String:return text(v.text);
   case V::Kind::Array:
    if(v.elements.size()>kLimit||!head(4,v.elements.size()))return false;
    for(const auto&item:v.elements)if(!write(item,depth+1))return false;
    return true;
   case V::Kind::Map:case V::Kind::Struct:{
    if(v.fields.size()>kLimit||!head(5,v.fields.size()))return false;
    std::vector<const std::pair<std::string,V>*>fields;fields.reserve(v.fields.size());
    for(const auto&field:v.fields){if(!valid_text(field.first))return false;fields.push_back(&field);}
    std::sort(fields.begin(),fields.end(),[](const auto*a,const auto*b){const auto&x=a->first;const auto&y=b->first;return x.size()!=y.size()?x.size()<y.size():x<y;});
    const std::string*prior=nullptr;
    for(const auto*field:fields){if(prior&&*prior==field->first)return false;prior=&field->first;if(!text(field->first)||!write(field->second,depth+1))return false;}
    return true;
   }
  }return false;
 }
 public:
 bool encode(const V&v){return write(v,1)&&!bytes.empty();}
 const std::vector<uint8_t>&result()const{return bytes;}
};
size_t head_size(uint64_t n){return n<24?1:n<=UINT8_MAX?2:n<=UINT16_MAX?3:n<=UINT32_MAX?5:9;}
bool charge(size_t amount,size_t&remaining){if(amount>remaining)return false;remaining-=amount;return true;}
bool from_json(const cJSON*p,V&v,unsigned depth,size_t&remaining){
 if(!p||depth>16)return false;
 if(cJSON_IsNull(p)){v=V::null();return charge(1,remaining);}
 if(cJSON_IsBool(p)){v=V::boolean_value(cJSON_IsTrue(p));return charge(1,remaining);}
 if(cJSON_IsString(p)){if(!p->valuestring)return false;const size_t n=strnlen(p->valuestring,kLimit+1);if(n>kLimit||!charge(head_size(n)+n,remaining))return false;v=V::string(std::string(p->valuestring,n));return valid_text(v.text);}
 if(cJSON_IsNumber(p)){const double n=p->valuedouble;if(!std::isfinite(n)||std::fabs(n)>9007199254740991.0||std::trunc(n)!=n)return false;const auto integer=int64_t(n);v=V::integer_value(integer);return charge(head_size(integer>=0?uint64_t(integer):uint64_t(-(integer+1))),remaining);}
 const bool array=cJSON_IsArray(p),object=cJSON_IsObject(p);
 if(!array&&!object)return false;
 size_t count=0;for(const cJSON*e=p->child;e;e=e->next)if(++count>kLimit)return false;
 if(!charge(head_size(count),remaining))return false;
 if(array){v=V::array({});for(const cJSON*e=p->child;e;e=e->next){V item;if(!from_json(e,item,depth+1,remaining))return false;v.elements.push_back(std::move(item));}return true;}
 v=V::object({});for(const cJSON*e=p->child;e;e=e->next){if(!e->string)return false;const size_t n=strnlen(e->string,kLimit+1);if(n>kLimit||!charge(head_size(n)+n,remaining))return false;std::string key(e->string,n);if(!valid_text(key))return false;for(const auto&field:v.fields)if(field.first==key)return false;V item;if(!from_json(e,item,depth+1,remaining))return false;v.fields.emplace_back(std::move(key),std::move(item));}return true;
}
}
esp_err_t canonical_value_to_cbor(const V&v,uint8_t**data,size_t*size){
 if(data)*data=nullptr;
 if(size)*size=0;
 if(!data||!size)return ESP_ERR_INVALID_ARG;
 Writer writer;if(!writer.encode(v))return ESP_ERR_INVALID_ARG;
 const auto&o=writer.result();auto*next=static_cast<uint8_t*>(malloc(o.size()));if(!next)return ESP_ERR_NO_MEM;
 memcpy(next,o.data(),o.size());*data=next;*size=o.size();return ESP_OK;
}
esp_err_t canonical_json_to_cbor(const cJSON*v,uint8_t**data,size_t*size){
 if(data)*data=nullptr;
 if(size)*size=0;
 if(!data||!size)return ESP_ERR_INVALID_ARG;
 V value;size_t remaining=kLimit;if(!from_json(v,value,1,remaining))return ESP_ERR_INVALID_ARG;return canonical_value_to_cbor(value,data,size);
}
}

namespace beefiscal::idf { namespace {
class ValueDecoder {
  using V=go_json::Value;
  const uint8_t* cursor_;const uint8_t* end_;
  bool argument(uint8_t ai,uint64_t&value){
    if(ai<24){value=ai;return true;}
    const size_t width=ai==24?1:ai==25?2:ai==26?4:ai==27?8:0;
    if(!width||size_t(end_-cursor_)<width)return false;
    value=0;for(size_t i=0;i<width;i++)value=(value<<8)|*cursor_++;
    return (width==1&&value>=24)||(width==2&&value>255)||
      (width==4&&value>65535)||(width==8&&value>UINT32_MAX);
  }
  static bool utf8(const uint8_t*data,size_t size){
    for(size_t at=0;at<size;){
      const uint8_t c=data[at];if(c==0)return false;
      if(c<0x80){++at;continue;}
      const size_t width=c>=0xc2&&c<=0xdf?2:c>=0xe0&&c<=0xef?3:c>=0xf0&&c<=0xf4?4:0;
      if(!width||width>size-at)return false;
      for(size_t i=1;i<width;i++)if((data[at+i]&0xc0)!=0x80)return false;
      const uint8_t next=data[at+1];
      if((c==0xe0&&next<0xa0)||(c==0xed&&next>=0xa0)||
         (c==0xf0&&next<0x90)||(c==0xf4&&next>=0x90))return false;
      at+=width;
    }
    return true;
  }
  bool read(V&out,unsigned depth){
    if(depth>16||cursor_==end_)return false;
    const uint8_t first=*cursor_++,major=first>>5,ai=first&31;
    if(major==7){
      if(ai==20||ai==21){out=V::boolean_value(ai==21);return true;}
      if(ai==22){out=V::null();return true;}
      return false;
    }
    if(major>5)return false;
    uint64_t value;if(!argument(ai,value))return false;
    if(major==0||major==1){
      if(value>uint64_t(INT64_MAX))return false;
      out=V::integer_value(major==0?int64_t(value):-int64_t(value)-1);return true;
    }
    // Length caps apply to containers/bytes/text, not integer magnitude.
    if(value>8192)return false;
    if(major==2||major==3){
      if(value>uint64_t(end_-cursor_))return false;
      std::string text;
      if(major==3){if(!utf8(cursor_,size_t(value)))return false;text.assign(reinterpret_cast<const char*>(cursor_),size_t(value));}
      else{static const char hex[]="0123456789abcdef";text.resize(size_t(value)*2);for(size_t i=0;i<value;i++){text[2*i]=hex[cursor_[i]>>4];text[2*i+1]=hex[cursor_[i]&15];}}
      cursor_+=size_t(value);out=V::string(std::move(text));return true;
    }
    out=major==4?V::array({}):V::object({});
    for(uint64_t i=0;i<value;i++){
      V key,item;
      if(major==5){
        if(!read(key,depth+1)||key.kind!=V::Kind::String)return false;
        for(const auto&field:out.fields)if(field.first==key.text)return false;
      }
      if(!read(item,depth+1))return false;
      if(major==4)out.elements.push_back(std::move(item));
      else out.fields.emplace_back(std::move(key.text),std::move(item));
    }
    return true;
  }
 public:
  ValueDecoder(const uint8_t*data,size_t size):cursor_(data),end_(data+size){}
  bool decode(V&out){V next;if(!read(next,1)||cursor_!=end_)return false;out=std::move(next);return true;}
};
}
esp_err_t canonical_cbor_to_value(const uint8_t*data,size_t size,go_json::Value&out){
  if(!data||!size||size>8192)return ESP_ERR_INVALID_ARG;
  return ValueDecoder(data,size).decode(out)?ESP_OK:ESP_ERR_INVALID_ARG;
}
}
