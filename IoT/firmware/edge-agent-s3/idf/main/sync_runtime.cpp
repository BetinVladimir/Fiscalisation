#include "sync_runtime.h"
#include "edge_runtime_port.h"
#include "cJSON.h"
#include "go_wire.h"
#include "esp_log.h"
#include "esp_random.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "mbedtls/base64.h"
#include "mbedtls/md.h"
#include "mbedtls/sha256.h"
#include <ctime>
#include <cstring>
#include <memory>
#include <algorithm>
#include <vector>
namespace beefiscal::idf{namespace{
struct State{DurableStorage*s{};const CompositeBinding*b{};DeviceIdentity*identity{};std::vector<uint8_t>ack_key;};State state;
struct Del{void operator()(cJSON*v)const{cJSON_Delete(v);}};using Json=std::unique_ptr<cJSON,Del>;
std::string hash(const char*p,size_t n){uint8_t d[32];mbedtls_sha256((const uint8_t*)p,n,d,0);static const char h[]="0123456789abcdef";std::string o(64,'0');for(size_t i=0;i<32;i++){o[i*2]=h[d[i]>>4];o[i*2+1]=h[d[i]&15];}return o;}
[[maybe_unused]] std::string print(cJSON*v){char*p=cJSON_PrintUnformatted(v);if(!p)return{};std::string o=p;cJSON_free(p);return o;}
std::string iso(int64_t t){char b[32];time_t x=t;tm value{};gmtime_r(&x,&value);strftime(b,sizeof(b),"%Y-%m-%dT%H:%M:%SZ",&value);return b;}
bool b64decode(const char*s,std::vector<uint8_t>&o){if(!s)return false;std::string v=s;for(char&c:v){if(c=='-')c='+';else if(c=='_')c='/';}while(v.size()%4)v+='=';o.resize(v.size());size_t n=0;if(mbedtls_base64_decode(o.data(),o.size(),&n,(uint8_t*)v.data(),v.size()))return false;o.resize(n);return n>=32;}
bool constant_equal(const uint8_t*a,const uint8_t*b,size_t n){uint8_t d=0;for(size_t i=0;i<n;i++)d|=a[i]^b[i];return d==0;}
std::string uuid(){uint8_t v[16];esp_fill_random(v,sizeof(v));v[6]=(v[6]&15)|0x40;v[8]=(v[8]&63)|0x80;char b[37];snprintf(b,sizeof(b),"%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",v[0],v[1],v[2],v[3],v[4],v[5],v[6],v[7],v[8],v[9],v[10],v[11],v[12],v[13],v[14],v[15]);return b;}
using V=go_json::Value;
std::string canonical(const V& v){std::string out,error;if(!go_json::encode(v,out,error))return {};return out;}
bool event(const PendingEvent&e,const std::string&previous,std::string&event_hash,DeviceIdentity&identity,V& v){
 V payload;std::string error;if(!go_json::parse(e.payload.data(),e.payload.size(),payload,error)||payload.kind!=V::Kind::Map)return false;
 const char*kind=e.kind=="COMMAND_RECEIVED"?"ACCEPTED":e.kind=="RECEIPT_RESULT"?"FISCALIZED":"SNAPSHOT";
 auto*st=go_json::field(payload,"state");if(st&&st->kind==V::Kind::String){auto*operation=go_json::field(payload,"operation_type");if(st->text=="COMMITTED"&&operation&&operation->kind==V::Kind::String&&operation->text=="REVERSAL")kind="REVERSED";if(st->text=="RECOVERY_REQUIRED")kind="RECOVERY_REQUIRED";else if(st->text=="REJECTED")kind="FAILED";else if(st->text=="COMPENSATED")kind="COMPENSATED";}
 v=V::object({{"event_id",V::string(e.event_id)},{"operation_id",V::string(e.operation_id)},{"device_id",V::string(state.b->edge_device_id)},{"journal_seq",V::integer_value(e.sequence)},{"event_type",V::string(kind)},{"occurred_at",V::string(iso(e.created_at))},{"payload",std::move(payload)},{"prev_hash",previous.empty()?V::null():V::string(previous)},{"event_hash",V::string("")},{"signature",V::null()}},true);
 auto body=canonical(v);if(body.empty())return false;event_hash=hash(body.data(),body.size());*go_json::field(v,"event_hash")=V::string(event_hash);std::string signature;if(identity.sign_hash_hex(event_hash.c_str(),signature)!=ESP_OK)return false;*go_json::field(v,"signature")=V::string(signature);return true;
}
bool frozen_compatible(const FrozenSyncBatch& frozen){
 V batch;std::string error;if(!go_json::parse(frozen.body.data(),frozen.body.size(),batch,error)||!go_json::sync_batch(batch,error))return false;
 auto*events=go_json::field(batch,"events");if(events->elements.empty()||go_json::field(batch,"first_seq")->integer!=frozen.first_sequence||go_json::field(batch,"last_seq")->integer!=frozen.last_sequence)return false;
 std::string previous=frozen.previous_hash;
 for(auto original:events->elements){auto*eh=go_json::field(original,"event_hash");auto*prev=go_json::field(original,"prev_hash");if(!eh||eh->kind!=V::Kind::String||!prev||(previous.empty()?prev->kind!=V::Kind::Null:prev->kind!=V::Kind::String||prev->text!=previous))return false;std::string expected=eh->text;*eh=V::string("");*go_json::field(original,"signature")=V::null();auto b=canonical(original);if(b.empty()||hash(b.data(),b.size())!=expected)return false;previous=expected;}
 if(previous!=frozen.last_event_hash)return false;
 auto*bh=go_json::field(batch,"batch_sha256");if(bh->kind!=V::Kind::String)return false;std::string expected=bh->text;*bh=V::string("");*go_json::field(batch,"signature")=V::string("");auto b=canonical(batch);return !b.empty()&&hash(b.data(),b.size())==expected;
}
esp_err_t publish_once(){
 FrozenSyncBatch frozen;auto loaded=state.s->frozen_sync_batch(frozen);
 if(loaded==ESP_ERR_NOT_FOUND){
  std::vector<PendingEvent>pending;
  if(state.s->pending(50,[&](const PendingEvent&e){pending.push_back(e);return true;})!=ESP_OK)return ESP_FAIL;
  if(pending.empty())return ESP_ERR_NOT_FOUND;
  std::string previous=state.s->acknowledged_event_hash();const std::string initial_hash=previous;std::vector<V>events;
  for(auto&e:pending){std::string eh;V v;if(!event(e,previous,eh,*state.identity,v))return ESP_FAIL;events.push_back(std::move(v));previous=eh;}
  V batch=V::object({{"edge_id",V::string(state.b->edge_device_id)},{"schema_version",V::string("2026-08-07")},{"first_seq",V::integer_value(pending.front().sequence)},{"last_seq",V::integer_value(pending.back().sequence)},{"previous_acknowledged_hash",initial_hash.empty()?V::null():V::string(initial_hash)},{"events",V::array(std::move(events))},{"batch_sha256",V::string("")},{"signature",V::string("")}},true);
  auto body=canonical(batch);if(body.empty())return ESP_FAIL;auto batch_hash=hash(body.data(),body.size());std::string signature;
  if(state.identity->sign_hash_hex(batch_hash.c_str(),signature)!=ESP_OK)return ESP_FAIL;
  *go_json::field(batch,"batch_sha256")=V::string(batch_hash);*go_json::field(batch,"signature")=V::string(signature);
  FrozenSyncBatch candidate{uuid(),canonical(batch),previous,initial_hash,pending.front().sequence,pending.back().sequence};
  if(state.s->freeze_sync_batch(candidate,pending)!=ESP_OK)return ESP_FAIL;
  if(state.s->frozen_sync_batch(frozen)!=ESP_OK)return ESP_FAIL;
 }else if(loaded!=ESP_OK)return loaded;
 if(!frozen_compatible(frozen)){
  ESP_LOGE("edge-sync","FROZEN_SYNC_CANONICAL_MISMATCH: retain signed batch %s for operator reconciliation; automatic rewrite forbidden",frozen.batch_id.c_str());
  state.s->pending(50,[&](const PendingEvent&e){if(e.sequence>=frozen.first_sequence&&e.sequence<=frozen.last_sequence)state.s->record_publish_attempt(e.event_id.c_str(),"FROZEN_SYNC_CANONICAL_MISMATCH",time(nullptr));return true;});
  return ESP_ERR_INVALID_STATE;
 }
 const int id=mqtt_publish_sync(frozen.batch_id.c_str(),(const uint8_t*)frozen.body.data(),frozen.body.size());
 state.s->pending(50,[&](const PendingEvent&e){if(e.sequence>=frozen.first_sequence&&e.sequence<=frozen.last_sequence)state.s->record_publish_attempt(e.event_id.c_str(),id<0?"MQTT_PUBLISH_FAILED":nullptr,time(nullptr));return true;});
 return id<0?ESP_FAIL:ESP_OK;
}
void worker(void*){uint32_t delay_ms=1000;for(;;){const auto result=publish_once();if(result==ESP_ERR_NOT_FOUND){state.s->prune_acknowledged(time(nullptr),93);delay_ms=1000;}else if(result==ESP_OK)delay_ms=1000;else delay_ms=delay_ms<15000?delay_ms*2:30000;vTaskDelay(pdMS_TO_TICKS(delay_ms+(esp_random()%251)));}}

}
esp_err_t sync_runtime_start(DurableStorage&s,const CompositeBinding&b,DeviceIdentity&i,const char*key){if(!s.ready()||!i.ready()||!b64decode(key,state.ack_key))return ESP_ERR_INVALID_ARG;state.s=&s;state.b=&b;state.identity=&i;return xTaskCreate(worker,"edge-sync",12288,nullptr,7,nullptr)==pdPASS?ESP_OK:ESP_ERR_NO_MEM;}
esp_err_t sync_runtime_accept_ack(const uint8_t*data,size_t n,void*){
 if(!state.s||!data||!n||n>8192)return ESP_ERR_INVALID_ARG;
 V root;std::string error;if(!go_json::parse((const char*)data,n,root,error)||!go_json::sync_ack(root,error))return ESP_ERR_INVALID_ARG;
 auto*sig=go_json::field(root,"signature");auto*edge=go_json::field(root,"edge_id");auto*id=go_json::field(root,"ack_id");auto*eh=go_json::field(root,"committed_event_hash");auto*seq=go_json::field(root,"committed_through_seq");
 if(sig->kind!=V::Kind::String||edge->kind!=V::Kind::String||id->kind!=V::Kind::String||eh->kind!=V::Kind::String||sig->text.find('\0')!=std::string::npos||id->text.find('\0')!=std::string::npos||eh->text.find('\0')!=std::string::npos||state.b->edge_device_id!=edge->text)return ESP_ERR_INVALID_ARG;
 std::vector<uint8_t>provided;if(!b64decode(sig->text.c_str(),provided))return ESP_ERR_INVALID_CRC;*sig=V::string("");auto unsigned_body=canonical(root);if(unsigned_body.empty())return ESP_ERR_INVALID_ARG;
 uint8_t mac[32];mbedtls_md_context_t ctx;mbedtls_md_init(&ctx);auto*info=mbedtls_md_info_from_type(MBEDTLS_MD_SHA256);int e=mbedtls_md_setup(&ctx,info,1);if(!e)e=mbedtls_md_hmac_starts(&ctx,state.ack_key.data(),state.ack_key.size());if(!e)e=mbedtls_md_hmac_update(&ctx,(uint8_t*)unsigned_body.data(),unsigned_body.size());if(!e)e=mbedtls_md_hmac_finish(&ctx,mac);mbedtls_md_free(&ctx);
 if(e||provided.size()!=32||!constant_equal(mac,provided.data(),32))return ESP_ERR_INVALID_CRC;
 return state.s->acknowledge_through(seq->integer,id->text.c_str(),eh->text.c_str(),time(nullptr));
}
}
