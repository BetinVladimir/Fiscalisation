#include <cassert>
#include <cstdio>
#include <cstring>
#include <iostream>
#include <sqlite3.h>
#include <openssl/sha.h>
#include "intent_processor.h"
#include "canonical_cbor.h"
#include "cJSON.h"
#include <openssl/hmac.h>
#include <openssl/evp.h>
#include "go_json.h"
static std::string mqtt_issued="2024-01-01T00:00:00Z",mqtt_expires="2024-01-01T00:02:00Z";
static bool expired_command=false;static bool adversarial_result=false;static bool preassigned=false;static int assigned_negative=-1;
static std::string escaped_reference(){return std::string(2048,'x')+"\"\n\\end";}
using namespace beefiscal::idf;
namespace beefiscal::idf {esp_err_t canonical_cbor_to_json(const uint8_t*,size_t,cJSON**){return ESP_ERR_NOT_SUPPORTED;}}
static void sql(const char*path,const char*query){sqlite3*d{};assert(sqlite3_open(path,&d)==SQLITE_OK);char*err{};int rc=sqlite3_exec(d,query,nullptr,nullptr,&err);if(rc)std::cerr<<(err?err:"")<<"\n";assert(rc==SQLITE_OK);sqlite3_free(err);sqlite3_close(d);}
static std::string scalar(const char*path,const char*query){sqlite3*d{};sqlite3_stmt*s{};assert(sqlite3_open(path,&d)==0);assert(sqlite3_prepare_v2(d,query,-1,&s,nullptr)==0);assert(sqlite3_step(s)==SQLITE_ROW);auto*t=sqlite3_column_text(s,0);std::string r=t?(const char*)t:"";sqlite3_finalize(s);sqlite3_close(d);return r;}
static std::string digest(const std::string&s){unsigned char out[32];SHA256((const unsigned char*)s.data(),s.size(),out);char b[65];for(int n=0;n<32;n++)snprintf(b+n*2,3,"%02x",out[n]);return b;}
static std::string command(bool sale,bool cash=false){std::string payload=sale?R"({"items":[{"line_id":"00000000-0000-4000-8000-000000000003","name":"Item","quantity":"1","unit_price":{"amount":"2.00","currency":"EUR"},"tax_group":"A"}],"payments":[{"payment_id":"00000000-0000-4000-8000-000000000004","type":"CARD","amount":{"amount":"1.00","currency":"EUR"}},{"payment_id":"00000000-0000-4000-8000-000000000005","type":"CARD","amount":{"amount":"1.00","currency":"EUR"}}]})":"{}";
// Both payment IDs are UUIDs, intentionally distinct from operation/session IDs.

if(cash){size_t pos=0;while((pos=payload.find("CARD",pos))!=std::string::npos){payload.replace(pos,4,"CASH");pos+=4;}}
{auto*j=cJSON_Parse(payload.c_str());beefiscal::go_json::Value v;std::string error;assert(beefiscal::go_json::from_cjson(j,v,error));assert(beefiscal::go_json::encode(v,payload,error));cJSON_Delete(j);}

return "{\"intent_id\":\"00000000-0000-4000-8000-000000000001\",\"receipt_session_id\":\"00000000-0000-4000-8000-000000000002\",\"action\":\""+std::string(sale?"SALE_FINALIZE":"DEVICE_IDENTITY")+"\",\"tenant_id\":\"t\",\"register_id\":\"r\",\"edge_device_id\":\"e\",\"binding_generation\":1,\"payload_sha256\":\""+digest(payload)+"\",\"canonical_payload\":"+payload+"}";}
static std::string mqtt_command(const std::string&body){auto*j=cJSON_Parse(body.c_str());assert(j);const std::pair<const char*,const char*>renames[]={{"intent_id","client_operation_id"},{"action","command_type"},{"edge_device_id","device_id"},{"binding_generation","fencing_token"},{"canonical_payload","payload"}};for(auto&entry:renames){auto*v=cJSON_DetachItemFromObject(j,entry.first);assert(v);cJSON_AddItemToObject(j,entry.second,v);}if(mqtt_issued!="MISSING")cJSON_AddStringToObject(j,"issued_at",mqtt_issued.c_str());if(mqtt_expires!="MISSING")cJSON_AddStringToObject(j,"expires_at",mqtt_expires.c_str());auto*payload=cJSON_GetObjectItem(j,"payload");auto*type=cJSON_GetObjectItem(j,"command_type");if(cJSON_IsString(type)&&std::string(type->valuestring)=="SALE_FINALIZE"){cJSON_AddStringToObject(payload,"server_sale_id","server-sale");cJSON_AddStringToObject(payload,"operator_code","0001");cJSON_AddStringToObject(payload,"unp_source","SERVER_ASSIGNED");cJSON_AddStringToObject(payload,"unp","DT636497-0001-0000500");if(assigned_negative==0)cJSON_DeleteItemFromObject(payload,"unp_source");if(assigned_negative==1)cJSON_DeleteItemFromObject(payload,"unp");if(assigned_negative==2)cJSON_SetValuestring(cJSON_GetObjectItem(payload,"unp_source"),"LEASED");if(assigned_negative>=3){const char*values[]={"XX636497-0001-0000500","DT636497-0002-0000500","DT636497-0001-0000050","DT636497-0001-0000000","DT636497-0001-000500"};cJSON_SetValuestring(cJSON_GetObjectItem(payload,"unp"),values[assigned_negative-3]);}beefiscal::go_json::Value v;std::string bytes,error;assert(beefiscal::go_json::from_cjson(payload,v,error)&&beefiscal::go_json::encode(v,bytes,error));cJSON_SetValuestring(cJSON_GetObjectItem(j,"payload_sha256"),digest(bytes).c_str());}beefiscal::go_json::Value value;std::string wire,error;assert(beefiscal::go_json::from_cjson(j,value,error)&&beefiscal::go_json::encode(value,wire,error));unsigned n{};unsigned char mac[32];const std::string key(32,'k');assert(HMAC(EVP_sha256(),key.data(),key.size(),(const unsigned char*)wire.data(),wire.size(),mac,&n)&&n==32);unsigned char encoded[48]{};int length=EVP_EncodeBlock(encoded,mac,32);std::string signature((char*)encoded,length);while(!signature.empty()&&signature.back()=='=')signature.pop_back();for(char&c:signature){if(c=='+')c='-';else if(c=='/')c='_';}cJSON_AddStringToObject(j,"signature",signature.c_str());assert(beefiscal::go_json::from_cjson(j,value,error)&&beefiscal::go_json::encode(value,wire,error));cJSON_Delete(j);return wire;}
struct TestClockState{CommandInstant instant{1704067201,0};int reads{};int expire_on{};bool failed{};};
static bool test_clock(CommandInstant&out,void*context){auto&s=*static_cast<TestClockState*>(context);++s.reads;out=s.expire_on&&s.reads>=s.expire_on?CommandInstant{1704067320,0}:s.instant;return !s.failed;}
struct Executor:ReceiptExecutor{DurableStorage&s;int calls{};bool cards;Executor(DurableStorage&v,bool c):s(v),cards(c){}bool ready()override{return true;}ExecutionResult execute(const ReceiptPlan&p)override{++calls;if(preassigned)assert(p.unp=="DT636497-0001-0000500");if(cards)for(size_t n=0;n<p.payments.size();n++){auto&v=p.payments[n];assert(s.upsert_payment(v.id.c_str(),p.operation_id.c_str(),p.receipt_session_id.c_str(),"CARD_APPROVED",v.amount_minor,("terminal"+std::to_string(n)).c_str(),("rrn"+std::to_string(n)).c_str(),("auth"+std::to_string(n)).c_str(),1704067201)==ESP_OK);}return{ExecutionCertainty::Committed,adversarial_result?escaped_reference():"fiscal-1","","aggregate","aggregate-rrn","aggregate-auth"};}ExecutionResult recover(const RecoveryItem&)override{return{};}};
static int recovery_case(const std::string&mode,const char*path){
remove(path);DurableStorage storage;assert(storage.open_database_for_test(path)==ESP_OK);assert(storage.configure_authority("DT636497",1,100,1)==ESP_OK);
const char*op="recover-op";const char*rid="recover-receipt";
assert(storage.reserve_command(op,std::string(64,'a').c_str(),"HTTP","{}","received",rid,"{}",{},1704067200,[](const AuthorityReservation&){return "{}";},nullptr,"0001").status==ReservationStatus::New);
assert(storage.set_operation_state(op,"EXECUTING",nullptr,1704067200)==ESP_OK);
assert(storage.upsert_payment("card-1",op,rid,"CARD_APPROVED",100,"term-1","rrn-1","auth-1",1704067200)==ESP_OK);
assert(storage.upsert_payment("card-2",op,rid,"CARD_APPROVED",100,"term-2","rrn-2","auth-2",1704067200)==ESP_OK);
if(mode=="recovery-outbox")sql(path,"CREATE TRIGGER fault BEFORE INSERT ON event_outbox WHEN NEW.kind='RECOVERY_RESULT' BEGIN SELECT RAISE(ABORT,'recovery outbox fault'); END;");
if(mode=="recovery-receipt")sql(path,"CREATE TRIGGER fault BEFORE UPDATE ON receipts BEGIN SELECT RAISE(ABORT,'recovery receipt fault'); END;");
struct RecoveryExecutor:ReceiptExecutor{int calls{};bool unknown{};bool ready()override{return true;}ExecutionResult execute(const ReceiptPlan&)override{assert(false);return{};}ExecutionResult recover(const RecoveryItem&)override{++calls;return{unknown?ExecutionCertainty::Unknown:ExecutionCertainty::Committed,adversarial_result?escaped_reference():"receipt-1","","aggregate","aggregate-rrn","aggregate-auth"};}}executor;
CompositeBinding binding;IntentProcessor processor(storage,binding,executor,"");
if(mode=="recovery-storage-error"){storage.close();assert(processor.recover_pending()!=ESP_OK);assert(executor.calls==0);std::cout<<mode<<" PASS (closed SQLite fails explicitly)\n";return 0;}
if(mode=="recovery-missing-receipt"){sql(path,"PRAGMA foreign_keys=OFF;DELETE FROM receipts;");assert(processor.recover_pending()!=ESP_OK);assert(executor.calls==0);assert(scalar(path,"SELECT state FROM operations")=="EXECUTING");std::cout<<mode<<" PASS (legacy incomplete context fails closed)\n";return 0;}
if(mode=="recovery-progress"){executor.unknown=true;assert(processor.recover_pending()==ESP_OK);executor.unknown=false;}
auto rc=processor.recover_pending();if(mode=="recovery-json"){auto payload=scalar(path,"SELECT result_payload FROM operations");auto* json=cJSON_Parse(payload.c_str());assert(json);auto* ref=cJSON_GetObjectItemCaseSensitive(json,"fiscal_reference");assert(cJSON_IsString(ref)&&escaped_reference()==ref->valuestring);cJSON_Delete(json);assert(scalar(path,"SELECT payload FROM event_outbox WHERE kind='RECOVERY_RESULT'")==payload);}
assert(executor.calls==(mode=="recovery-progress"?2:1));
if(mode=="recovery-outbox"||mode=="recovery-receipt"){
assert(rc!=ESP_OK);assert(scalar(path,"SELECT state FROM operations")=="EXECUTING");assert(scalar(path,"SELECT state FROM receipts")=="RESERVED");assert(scalar(path,"SELECT count(*) FROM event_outbox WHERE kind='RECOVERY_RESULT'")=="0");
}else{assert(rc==ESP_OK);assert(scalar(path,"SELECT state FROM operations")=="COMMITTED");assert(scalar(path,"SELECT count(*) FROM event_outbox WHERE kind='RECOVERY_RESULT'")==(mode=="recovery-progress"?"2":"1"));assert(scalar(path,"SELECT count(*) FROM payments WHERE rrn IN ('rrn-1','rrn-2') AND state='CARD_APPROVED'")=="2");assert(processor.recover_pending()==ESP_OK);assert(executor.calls==(mode=="recovery-progress"?2:1));}
storage.close();assert(storage.open_database_for_test(path)==ESP_OK);assert(scalar(path,"SELECT state FROM operations")==((mode=="recovery-outbox"||mode=="recovery-receipt")?"EXECUTING":"COMMITTED"));std::cout<<mode<<" PASS (actual recovery + SQLite, reopened)\n";return 0;
}
static int validity_case(const std::string&mode,const char*path){
 // Independent Gregorian/Unix vectors include the 2038 boundary on ESP32.
 CommandInstant parsed;
 assert(parse_command_instant("1970-01-01T00:00:00Z",parsed)&&parsed.seconds==0);
 assert(parse_command_instant("2000-02-29T00:00:00Z",parsed)&&parsed.seconds==951782400);
 assert(parse_command_instant("2024-02-29T00:00:00.1Z",parsed)&&parsed.seconds==1709164800&&parsed.nanoseconds==100000000);
 assert(parse_command_instant("2038-01-19T03:14:08.000000001Z",parsed)&&parsed.seconds==2147483648LL&&parsed.nanoseconds==1);
 assert(!parse_command_instant("1900-02-29T00:00:00Z",parsed));
 assert(!parse_command_instant("2024-04-31T00:00:00Z",parsed));
 assert(!parse_command_instant("2024-01-01T24:00:00Z",parsed));
 assert(!parse_command_instant("2024-01-01T00:00:00.Z",parsed));
 remove(path);DurableStorage storage;assert(storage.open_database_for_test(path)==ESP_OK);assert(storage.configure_authority("DT636497",1,100,1)==ESP_OK);
 CompositeBinding binding;binding.tenant_id="t";binding.register_id="r";binding.edge_device_id="e";binding.generation=1;binding.authority.unp_prefix="DT636497";binding.fiscal.fiscal_device_number="DT636497";binding.authority.unp_range_start=1;binding.authority.unp_range_end=100;
 TestClockState state;bool success=false,terminal=false;
 if(mode=="validity-issued-boundary"){state.instant={1704067200,0};success=true;}
 else if(mode=="validity-fraction"){mqtt_issued="2024-01-01T00:00:00.123456789Z";mqtt_expires="2024-01-01T00:00:00.123456791Z";state.instant={1704067200,123456790};success=true;}
 else if(mode=="validity-nano-expired"){mqtt_expires="2024-01-01T00:00:01.000000001Z";state.instant={1704067201,1};}
 else if(mode=="validity-nano-future"){mqtt_issued="2024-01-01T00:00:01.000000001Z";state.instant={1704067201,0};}
 else if(mode=="validity-expired")state.instant={1704067320,0};
 else if(mode=="validity-future")state.instant={1704067199,999999999};
 else if(mode=="validity-missing-issued")mqtt_issued="MISSING";
 else if(mode=="validity-missing-expires")mqtt_expires="MISSING";
 else if(mode=="validity-reversed")mqtt_expires="2023-12-31T23:59:59Z";
 else if(mode=="validity-equal")mqtt_expires=mqtt_issued;
 else if(mode=="validity-calendar")mqtt_issued="2023-02-29T00:00:00Z";
 else if(mode=="validity-overflow")mqtt_issued="2024-01-01T00:00:00.1234567890Z";
 else if(mode=="validity-offset")mqtt_issued="2024-01-01T00:00:00+00:00";
 else if(mode=="validity-leap-second")mqtt_issued="2024-01-01T00:00:60Z";
 else if(mode=="validity-clock-zero")state.instant={0,0};
 else if(mode=="validity-clock-failed")state.failed=true;
 else if(mode=="validity-clock-nanos")state.instant.nanoseconds=1000000000;
 else if(mode=="validity-reservation-expired")state.expire_on=2;
 else if(mode=="validity-transaction-expired")state.expire_on=3;
 else if(mode=="validity-admitted-expired"){state.expire_on=4;terminal=true;}
 else if(mode=="validity-before-io-expired"){state.expire_on=5;terminal=true;}
 else assert(false);
 CommandClock clock{test_clock,&state};Executor executor(storage,false);IntentProcessor processor(storage,binding,executor,"a2tra2tra2tra2tra2tra2tra2tra2tra2tra2tra2s=",nullptr,nullptr,nullptr,&clock);
 auto wire=mqtt_command(command(true,true));auto result=processor.accept({(const uint8_t*)wire.data(),wire.size(),Ingress::Mqtt});
 assert(executor.calls==(success?1:0));assert(scalar(path,"SELECT next_unp FROM authority_state")=="1");
 if(success){assert(result==ESP_OK);assert(scalar(path,"SELECT state FROM operations")=="COMMITTED");}
 else if(terminal){assert(result==ESP_OK);assert(scalar(path,"SELECT state FROM operations")=="REJECTED");assert(scalar(path,"SELECT json_extract(result_payload,'$.error_code') FROM operations")=="COMMAND_EXPIRED");assert(processor.recover_pending()==ESP_OK&&executor.calls==0);}
 else {assert(result!=ESP_OK);assert(scalar(path,"SELECT count(*) FROM operations")=="0");assert(scalar(path,"SELECT count(*) FROM event_outbox")=="0");}
 std::cout<<mode<<" actual HMAC+IntentProcessor+SQLite PASS\n";return 0;
}
int main(int argc,char**argv){assert(argc==3);std::string mode=argv[1];if(mode.find("validity-")==0)return validity_case(mode,argv[2]);expired_command=mode=="expired-command";preassigned=mode=="preassigned";adversarial_result=mode=="json-result"||mode=="recovery-json";const char*path=argv[2];if(mode.find("recovery-")==0)return recovery_case(mode,path);remove(path);DurableStorage storage;assert(storage.open_database_for_test(path)==ESP_OK);assert(storage.configure_authority("DT636497",1,100,1)==ESP_OK);if(mode=="checkpoint")sql(path,"CREATE TRIGGER fault BEFORE UPDATE OF state ON operations WHEN NEW.state='EXECUTING' BEGIN SELECT RAISE(ABORT,'checkpoint fault'); END;");if(mode=="outbox")sql(path,"CREATE TRIGGER fault BEFORE INSERT ON event_outbox WHEN NEW.kind='RECEIPT_RESULT' BEGIN SELECT RAISE(ABORT,'outbox fault'); END;");if(mode=="payment")sql(path,"CREATE TRIGGER fault BEFORE UPDATE ON payments BEGIN SELECT RAISE(ABORT,'payment fault'); END;");if(mode=="collision")sql(path,"CREATE TRIGGER collide AFTER INSERT ON operations BEGIN INSERT INTO event_outbox(event_id,operation_id,kind,payload,payload_digest,created_at) VALUES('result-'||NEW.operation_id,NEW.operation_id,'RECEIPT_RESULT','{}','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',1704067201); END;");if(mode=="receipt")sql(path,"CREATE TRIGGER fault BEFORE UPDATE ON receipts BEGIN SELECT RAISE(ABORT,'receipt fault'); END;");bool cards=mode=="cards";Executor executor(storage,cards);CompositeBinding binding;binding.tenant_id="t";binding.register_id="r";binding.edge_device_id="e";binding.generation=1;binding.authority.unp_prefix="DT636497";binding.fiscal.fiscal_device_number="DT636497";binding.authority.unp_range_start=1;binding.authority.unp_range_end=100;TestClockState fixed_clock;CommandClock clock{test_clock,&fixed_clock};IntentProcessor processor(storage,binding,executor,"a2tra2tra2tra2tra2tra2tra2tra2tra2tra2tra2s=",nullptr,nullptr,nullptr,expired_command?nullptr:&clock);if(expired_command){auto cmd=mqtt_command(command(false));assert(processor.accept({(const uint8_t*)cmd.data(),cmd.size(),Ingress::Mqtt})!=ESP_OK);assert(executor.calls==0);assert(scalar(path,"SELECT count(*) FROM operations")=="0");assert(scalar(path,"SELECT next_unp FROM authority_state")=="1");std::cout<<"expired signed command rejected before IO/reservation\n";return 0;}if(mode=="currency-validation"){
 for(int variant=0;variant<5;variant++){
  auto good=command(true);auto*root=cJSON_Parse(good.c_str());cJSON_AddStringToObject(root,"client_sale_surrogate_id","00000000-0000-4000-8000-000000000009");auto*payload=cJSON_GetObjectItem(root,"canonical_payload");auto*line=cJSON_GetArrayItem(cJSON_GetObjectItem(payload,"items"),0);auto*price=cJSON_GetObjectItem(line,"unit_price");
  if(variant==0)cJSON_SetValuestring(cJSON_GetObjectItem(price,"currency"),"BGN");
  if(variant==1)cJSON_DeleteItemFromObject(price,"currency");
  if(variant==2)cJSON_AddStringToObject(line,"discount","0.20");
  if(variant>=3){auto*discount=cJSON_AddObjectToObject(line,"discount");cJSON_AddStringToObject(discount,"amount","0.20");if(variant==3)cJSON_AddStringToObject(discount,"currency","BGN");}
  beefiscal::go_json::Value value;std::string body,error;assert(beefiscal::go_json::from_cjson(payload,value,error));assert(beefiscal::go_json::encode(value,body,error));auto hash=digest(body);cJSON_SetValuestring(cJSON_GetObjectItem(root,"payload_sha256"),hash.c_str());char*wire=cJSON_PrintUnformatted(root);assert(processor.accept({(const uint8_t*)wire,strlen(wire),Ingress::Http})==ESP_ERR_INVALID_ARG);cJSON_free(wire);cJSON_Delete(root);assert(executor.calls==0);assert(scalar(path,"SELECT count(*) FROM operations")=="0");
 }
 std::cout<<mode<<" PASS (actual decoder, no reservation/executor)\n";return 0;
}if(mode=="preassigned-negative"){for(assigned_negative=0;assigned_negative<8;++assigned_negative){auto negative=mqtt_command(command(true,true));assert(processor.accept({(const uint8_t*)negative.data(),negative.size(),Ingress::Mqtt})!=ESP_OK);assert(executor.calls==0);assert(scalar(path,"SELECT count(*) FROM operations")=="0");assert(scalar(path,"SELECT next_unp FROM authority_state")=="1");}std::cout<<mode<<" PASS eight authenticated negativecommands beforeI/O\n";return 0;}auto cmd=mqtt_command(command(cards||mode=="payment"||preassigned,mode=="payment"||preassigned));auto result=processor.accept({(const uint8_t*)cmd.data(),cmd.size(),Ingress::Mqtt});
if(mode=="checkpoint"){assert(result!=ESP_OK);assert(executor.calls==0);assert(scalar(path,"SELECT state FROM operations")=="RESERVED");}
else if(mode=="outbox"||mode=="receipt"||mode=="payment"||mode=="collision"){assert(result!=ESP_OK);assert(executor.calls==1);assert(scalar(path,"SELECT state FROM operations")=="EXECUTING");assert(scalar(path,"SELECT state FROM receipts")=="RESERVED");assert(scalar(path,"SELECT count(*) FROM event_outbox WHERE kind='RECEIPT_RESULT'")==(mode=="collision"?"1":"0"));}
else {assert(result==ESP_OK);assert(executor.calls==1);assert(scalar(path,"SELECT state FROM operations")=="COMMITTED");assert(scalar(path,"SELECT count(*) FROM event_outbox WHERE kind='RECEIPT_RESULT'")=="1");if(cards){assert(scalar(path,"SELECT count(DISTINCT rrn) FROM payments")=="2");assert(scalar(path,"SELECT count(*) FROM payments WHERE terminal_reference IN ('terminal0','terminal1')")=="2");}}
if(preassigned){assert(scalar(path,"SELECT next_unp FROM authority_state")=="1");assert(scalar(path,"SELECT unp_sequence FROM receipt_authority")=="0");assert(scalar(path,"SELECT execution_unp FROM receipt_authority")=="DT636497-0001-0000500");assert(processor.accept({(const uint8_t*)cmd.data(),cmd.size(),Ingress::Mqtt})==ESP_OK&&executor.calls==1);}if(mode=="json-result"){auto payload=scalar(path,"SELECT result_payload FROM operations");auto* json=cJSON_Parse(payload.c_str());assert(json);auto* ref=cJSON_GetObjectItemCaseSensitive(json,"fiscal_reference");assert(cJSON_IsString(ref)&&escaped_reference()==ref->valuestring);cJSON_Delete(json);assert(scalar(path,"SELECT payload FROM event_outbox WHERE kind='RECEIPT_RESULT'")==payload);}
if(mode=="normal"||mode=="cards"){assert(processor.accept({(const uint8_t*)cmd.data(),cmd.size(),Ingress::Mqtt})==ESP_OK);assert(executor.calls==1);}
storage.close();assert(storage.open_database_for_test(path)==ESP_OK);assert(!scalar(path,"SELECT state FROM operations").empty());std::cout<<mode<<" PASS (actual IntentProcessor + SQLite, reopened)\n";}
