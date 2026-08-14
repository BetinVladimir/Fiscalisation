#include "canonical_cbor.h"
#include <algorithm>
#include <cstdlib>
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
  else if(major==2||major==3){if((uint64_t)(end_-p_)<n){depth_--;return nullptr;}if(major==3){std::string s((const char*)p_,(size_t)n);out=cJSON_CreateString(s.c_str());}else{std::string hex;static const char h[]="0123456789abcdef";hex.resize(n*2);for(size_t i=0;i<n;i++){hex[i*2]=h[p_[i]>>4];hex[i*2+1]=h[p_[i]&15];}out=cJSON_CreateString(hex.c_str());}p_+=n;}
  else if(major==4){out=cJSON_CreateArray();for(uint64_t i=0;out&&i<n;i++){cJSON*v=read();if(!v){cJSON_Delete(out);out=nullptr;break;}cJSON_AddItemToArray(out,v);}}
  else if(major==5){out=cJSON_CreateObject();for(uint64_t i=0;out&&i<n;i++){cJSON*k=read();if(!cJSON_IsString(k)||cJSON_GetObjectItemCaseSensitive(out,k->valuestring)){cJSON_Delete(k);cJSON_Delete(out);out=nullptr;break;}std::string key=k->valuestring;cJSON_Delete(k);cJSON*v=read();if(!v){cJSON_Delete(out);out=nullptr;break;}cJSON_AddItemToObject(out,key.c_str(),v);}}
  else if(major==7&&ai==20)out=cJSON_CreateFalse();
  else if(major==7&&ai==21)out=cJSON_CreateTrue();
  else if(major==7&&ai==22)out=cJSON_CreateNull();
  depth_--;return out;}
 public:Decoder(const uint8_t*p,size_t n):p_(p),end_(p+n){}cJSON*decode(){cJSON*v=read();if(!v||p_!=end_){cJSON_Delete(v);return nullptr;}return v;}};
}
esp_err_t canonical_cbor_to_json(const uint8_t*p,size_t n,cJSON**out){if(!p||!n||n>8192||!out)return ESP_ERR_INVALID_ARG;*out=Decoder(p,n).decode();return*out?ESP_OK:ESP_ERR_INVALID_ARG;}
namespace{
void head(std::vector<uint8_t>&o,uint8_t major,uint64_t n){if(n<24)o.push_back(uint8_t(major<<5)|n);else if(n<=0xff){o.push_back(uint8_t(major<<5)|24);o.push_back(n);}else if(n<=0xffff){o.push_back(uint8_t(major<<5)|25);o.push_back(n>>8);o.push_back(n);}else{o.push_back(uint8_t(major<<5)|26);for(int i=3;i>=0;i--)o.push_back(n>>(i*8));}}
bool encode(const cJSON*v,std::vector<uint8_t>&o,int depth){if(!v||depth>16)return false;if(cJSON_IsString(v)){size_t n=strlen(v->valuestring);head(o,3,n);o.insert(o.end(),v->valuestring,v->valuestring+n);return true;}if(cJSON_IsBool(v)){o.push_back(cJSON_IsTrue(v)?0xf5:0xf4);return true;}if(cJSON_IsNull(v)){o.push_back(0xf6);return true;}if(cJSON_IsNumber(v)&&v->valuedouble==v->valueint){int64_t n=v->valueint;if(n>=0)head(o,0,n);else head(o,1,uint64_t(-1-n));return true;}if(cJSON_IsArray(v)){head(o,4,cJSON_GetArraySize(v));cJSON*e=nullptr;cJSON_ArrayForEach(e,v)if(!encode(e,o,depth+1))return false;return true;}if(cJSON_IsObject(v)){std::vector<cJSON*>fields;for(cJSON*e=v->child;e;e=e->next)fields.push_back(e);std::sort(fields.begin(),fields.end(),[](cJSON*a,cJSON*b){size_t x=strlen(a->string),y=strlen(b->string);return x==y?strcmp(a->string,b->string)<0:x<y;});head(o,5,fields.size());for(auto*e:fields){size_t n=strlen(e->string);head(o,3,n);o.insert(o.end(),e->string,e->string+n);if(!encode(e,o,depth+1))return false;}return true;}return false;}
}
esp_err_t canonical_json_to_cbor(const cJSON*v,uint8_t**data,size_t*size){if(!v||!data||!size)return ESP_ERR_INVALID_ARG;std::vector<uint8_t>o;if(!encode(v,o,0)||o.empty()||o.size()>8192)return ESP_ERR_INVALID_ARG;*data=(uint8_t*)malloc(o.size());if(!*data)return ESP_ERR_NO_MEM;memcpy(*data,o.data(),o.size());*size=o.size();return ESP_OK;}
}
