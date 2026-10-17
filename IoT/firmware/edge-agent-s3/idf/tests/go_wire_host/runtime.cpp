#include <cassert>
#include <fstream>
#include <iostream>
#include <algorithm>
#include <sqlite3.h>
#include "../../main/sync_runtime.cpp"
#include "intent_processor.h"
#include "canonical_cbor.h"
using namespace beefiscal::idf;
static std::string published;
namespace beefiscal::idf {DeviceIdentity::DeviceIdentity(){}DeviceIdentity::~DeviceIdentity(){}bool DeviceIdentity::ready()const{return true;}esp_err_t DeviceIdentity::sign_hash_hex(const char*h,std::string&s)const{s=std::string("test-key:")+h;return ESP_OK;}int mqtt_publish_sync(const char*,const uint8_t*p,size_t n){published.assign((const char*)p,n);return 1;}}
std::string read(const char*p){std::ifstream f(p);return std::string(std::istreambuf_iterator<char>(f),{});}
void sql(const char*path,const std::string&q){sqlite3*d{};assert(sqlite3_open(path,&d)==0);assert(sqlite3_exec(d,q.c_str(),nullptr,nullptr,nullptr)==0);sqlite3_close(d);}
struct Executor:ReceiptExecutor {int calls{};bool ready()override{return true;}ExecutionResult execute(const ReceiptPlan&p)override{++calls;assert(p.items.size()==1&&p.items[0].name=="Кафе & <чай>");return {ExecutionCertainty::Committed,"fiscal-1","","","",""};}ExecutionResult recover(const RecoveryItem&)override{return {};}};
int main(int argc,char**argv){assert(argc>=4);std::string mode=argv[1];DurableStorage storage;assert(storage.open_database_for_test(argv[2])==ESP_OK);CompositeBinding binding;binding.edge_device_id="edge";binding.tenant_id="tenant";binding.register_id="register";binding.generation=1;DeviceIdentity identity;assert(sync_runtime_start(storage,binding,identity,"a2tra2tra2tra2tra2tra2tra2tra2tra2tra2tra2s=")==ESP_OK);
 if(mode=="batch"){
 std::string payload=R"({"z":"<>&\u2028\u2029\u0000","a":{"z":1e21,"a":[1e-7,-0,0.000001]},"operation_sequence":5,"unp_sequence":3,"command":{"z":1,"a":2}})";
 assert(storage.reserve_with_received_event("op",std::string(64,'a').c_str(),"MQTT","{}","event-1",payload.c_str(),1704067200).status==ReservationStatus::New);
 if(argc==5){auto seq=std::stoll(argv[4]);sql(argv[2],"UPDATE event_outbox SET sequence="+std::to_string(seq)+";UPDATE sync_cursor SET acknowledged_sequence="+std::to_string(seq-1)+";");}
 assert(publish_once()==ESP_OK);std::ofstream(argv[3])<<published;return 0;}
 if(mode=="ack"){auto body=read(argv[3]);V ack_value;std::string error;assert(beefiscal::go_json::parse(body.data(),body.size(),ack_value,error));assert(beefiscal::go_json::sync_ack(ack_value,error));auto tampered=ack_value;auto* event_hash=beefiscal::go_json::field(tampered,"committed_event_hash");event_hash->text[0]=event_hash->text[0]=='0'?'1':'0';auto invalid=canonical(tampered);assert(sync_runtime_accept_ack((const uint8_t*)invalid.data(),invalid.size(),nullptr)==ESP_ERR_INVALID_CRC);
 std::reverse(ack_value.fields.begin(),ack_value.fields.end());ack_value.kind=V::Kind::Struct;for(auto&result:beefiscal::go_json::field(ack_value,"operation_results")->elements){std::reverse(result.fields.begin(),result.fields.end());result.kind=V::Kind::Struct;}auto reordered=canonical(ack_value);assert(sync_runtime_accept_ack((const uint8_t*)reordered.data(),reordered.size(),nullptr)==ESP_OK);assert(sync_runtime_accept_ack((const uint8_t*)body.data(),body.size(),nullptr)==ESP_OK);FrozenSyncBatch frozen;assert(storage.frozen_sync_batch(frozen)==ESP_ERR_NOT_FOUND);assert(storage.acknowledged_cursor()>0);std::cout<<"actual Go ACK HMAC accepted and durable cursor committed "<<storage.acknowledged_cursor()<<"\n";return 0;}
 if(mode=="migration"){FrozenSyncBatch before,after;assert(storage.frozen_sync_batch(before)==ESP_OK);Json legacy(cJSON_Parse(before.body.c_str()));assert(legacy);auto legacy_body=print(legacy.get());assert(legacy_body!=before.body);sql(argv[2],"UPDATE frozen_sync_batch SET body='"+legacy_body+"'");assert(storage.frozen_sync_batch(before)==ESP_OK);assert(publish_once()==ESP_ERR_INVALID_STATE&&published.empty());assert(storage.frozen_sync_batch(after)==ESP_OK&&after.body==before.body&&after.batch_id==before.batch_id);assert(storage.acknowledged_cursor()==0);std::cout<<"legacy incompatible frozen batch blocked unchanged\n";return 0;}
 cJSON* invalid=nullptr;const uint8_t nul_cbor[]={0x61,0x00};assert(canonical_cbor_to_json(nul_cbor,sizeof(nul_cbor),&invalid)==ESP_ERR_INVALID_ARG&&!invalid);
 assert(mode=="command");assert(storage.configure_authority("TEST-",1,100,1)==ESP_OK);Executor executor;IntentProcessor processor(storage,binding,executor,"a2tra2tra2tra2tra2tra2tra2tra2tra2tra2tra2s=");auto body=read(argv[3]);
 // Reorder the object after Go signs: semantic map canonicalization must survive.
 V v;std::string error;for(auto invalid:{std::string("01"),std::string("[1,]"),std::string("{\"a\":1,\"a\":2}"),std::string("1e9999"),std::string("\"\\uZZZZ\"")})assert(!beefiscal::go_json::parse(invalid.data(),invalid.size(),v,error));
 for(auto token:{std::string("9223372036854775807"),std::string("-9223372036854775808")}){assert(beefiscal::go_json::parse(token.data(),token.size(),v,error));int64_t exact;assert(beefiscal::go_json::integer(v,exact));}
 for(auto token:{std::string("9223372036854775808"),std::string("-9223372036854775809"),std::string("1.0")}){assert(beefiscal::go_json::parse(token.data(),token.size(),v,error));int64_t exact;assert(!beefiscal::go_json::integer(v,exact));}
 std::string deep(18,'[');deep+='0';deep.append(18,']');assert(!beefiscal::go_json::parse(deep.data(),deep.size(),v,error));
 std::string invalid_utf8="\"";invalid_utf8+=char(0xff);invalid_utf8+="\"";assert(beefiscal::go_json::parse(invalid_utf8.data(),invalid_utf8.size(),v,error));assert(canonical(v)=="\"\xef\xbf\xbd\"");
 assert(beefiscal::go_json::parse(body.data(),body.size(),v,error));std::reverse(v.fields.begin(),v.fields.end());v.kind=V::Kind::Struct;auto reordered=canonical(v);
 auto rc=processor.accept({(const uint8_t*)reordered.data(),reordered.size(),Ingress::Mqtt});assert(rc==ESP_OK);assert(executor.calls==1);assert(processor.accept({(const uint8_t*)body.data(),body.size(),Ingress::Mqtt})==ESP_OK&&executor.calls==1);
 std::string duplicate=body;duplicate.insert(duplicate.size()-1,",\"tenant_id\":\"tenant\"");
 std::string nul=body;auto tenantpos=nul.find("\"tenant_id\":\"tenant\"");assert(tenantpos!=std::string::npos);nul.replace(tenantpos,std::string("\"tenant_id\":\"tenant\"").size(),"\"tenant_id\":\"tenant\\u0000evil\"");
 std::string unsafe=body;auto fencepos=unsafe.find("\"fencing_token\":1");assert(fencepos!=std::string::npos);unsafe.replace(fencepos,std::string("\"fencing_token\":1").size(),"\"fencing_token\":9007199254740993");
 for(auto bad:{body+" trailing",duplicate,nul,unsafe})assert(processor.accept({(const uint8_t*)bad.data(),bad.size(),Ingress::Mqtt})==ESP_ERR_INVALID_ARG&&executor.calls==1);

 V original_command;assert(beefiscal::go_json::parse(body.data(),body.size(),original_command,error));
 for(auto bad:std::vector<std::pair<double,double>>{{1.5,1},{2147483648.0,1},{1,1.5}}){
  V payload=*beefiscal::go_json::field(original_command,"payload");
  payload.fields.emplace_back("reason",V::number_value(bad.second));
  payload.fields.emplace_back("original_document",V::object({{"document_number",V::number_value(bad.first)},{"document_datetime",V::string("17-10-26 01:02:03")},{"fiscal_memory_number",V::string("12345678")}}));
  auto bytes=canonical(payload);auto digest=hash(bytes.data(),bytes.size());
  V local=V::object({{"intent_id",V::string("00000000-0000-4000-8000-000000000009")},{"receipt_session_id",V::string("00000000-0000-4000-8000-000000000008")},{"action",V::string("SALE_REVERSE")},{"tenant_id",V::string("tenant")},{"register_id",V::string("register")},{"edge_device_id",V::string("edge")},{"binding_generation",V::integer_value(1)},{"canonical_payload",payload},{"payload_sha256",V::string(digest)}});
  auto invalid=canonical(local);assert(processor.accept({(const uint8_t*)invalid.data(),invalid.size(),Ingress::Http})==ESP_ERR_INVALID_ARG&&executor.calls==1);
 }
 std::cout<<"actual Go signed command/payload accepted with reordered keys; malformed and unsafeinteger ingress rejected\n";
}
