#include <cassert>
#include <cstdio>
#include <iostream>
#include <thread>
#include <atomic>
#include <sqlite3.h>
#include <openssl/evp.h>
#include "../../main/sync_runtime.cpp"
using namespace beefiscal::idf;
static int signs{},publishes{};static bool failed_publish{};static std::string sent_id,sent_body;
namespace beefiscal::idf{DeviceIdentity::DeviceIdentity(){}DeviceIdentity::~DeviceIdentity(){}bool DeviceIdentity::ready()const{return true;}esp_err_t DeviceIdentity::sign_hash_hex(const char*h,std::string&s)const{s="fixture-signature-"+std::to_string(++signs)+h;return ESP_OK;}int mqtt_publish_sync(const char*id,const uint8_t*p,size_t n){++publishes;sent_id=id;sent_body.assign((const char*)p,n);return failed_publish?-1:1;}}
static void sql(const char*path,const char*query){sqlite3*d{};assert(sqlite3_open(path,&d)==0);char*err{};int rc=sqlite3_exec(d,query,nullptr,nullptr,&err);if(rc)std::cerr<<(err?err:"")<<"\n";assert(rc==0);sqlite3_free(err);sqlite3_close(d);}
static std::string ack(int64_t sequence,const std::string&event_hash,const std::string&edge="edge"){
 Json root(cJSON_CreateObject());cJSON_AddStringToObject(root.get(),"ack_id","ack-1");cJSON_AddStringToObject(root.get(),"edge_id",edge.c_str());cJSON_AddNumberToObject(root.get(),"committed_through_seq",sequence);cJSON_AddStringToObject(root.get(),"committed_event_hash",event_hash.c_str());cJSON_AddStringToObject(root.get(),"committed_at","2026-10-17T00:00:00Z");cJSON_AddArrayToObject(root.get(),"operation_results");cJSON_AddArrayToObject(root.get(),"rejected");cJSON_AddStringToObject(root.get(),"signature","");auto body=print(root.get());unsigned char mac[32],encoded[64];std::string key(32,'k');HMAC(EVP_sha256(),key.data(),key.size(),(const unsigned char*)body.data(),body.size(),mac,nullptr);int n=EVP_EncodeBlock(encoded,mac,32);std::string signature((char*)encoded,n);cJSON_SetValuestring(cJSON_GetObjectItem(root.get(),"signature"),signature.c_str());return print(root.get());}
static int accept(const std::string&s){return sync_runtime_accept_ack((const uint8_t*)s.data(),s.size(),nullptr);}
int main(int argc,char**argv){assert(argc==3);std::string mode=argv[1];const char*path=argv[2];remove(path);DurableStorage storage;assert(storage.open_database_for_test(path)==ESP_OK);CompositeBinding binding;binding.edge_device_id="edge";DeviceIdentity identity;std::string key(32,'k');unsigned char encoded[64];int n=EVP_EncodeBlock(encoded,(const unsigned char*)key.data(),key.size());std::string key64((char*)encoded,n);assert(sync_runtime_start(storage,binding,identity,key64.c_str())==ESP_OK);
assert(storage.reserve_with_received_event("operation",std::string(64,'a').c_str(),"MQTT","{}","event-1","{}",1704067200).status==ReservationStatus::New);
if(mode=="duplicate"){
 const std::string digest(64,'a');
 assert(storage.append_event("event-1","operation","COMMAND_RECEIVED","{}",digest.c_str(),1704067200)==ESP_OK);
 assert(storage.append_event("event-1","operation","COMMAND_RECEIVED","{changed}",digest.c_str(),1704067200)==ESP_ERR_INVALID_STATE);
 assert(storage.append_event("event-1","operation","COMMAND_RECEIVED","{}",digest.c_str(),1704067201)==ESP_ERR_INVALID_STATE);
 std::thread a([&](){for(int i=0;i<100;i++)assert(storage.append_event("event-1","operation","COMMAND_RECEIVED","{}",digest.c_str(),1704067200)==ESP_OK);});
 std::thread b([&](){for(int i=0;i<100;i++)assert(storage.append_event("event-1","operation","COMMAND_RECEIVED","{}",digest.c_str(),1704067200)==ESP_OK);});a.join();b.join();
}
if(mode=="large-sequence")sql(path,"UPDATE event_outbox SET sequence=2147483649;UPDATE sync_cursor SET acknowledged_sequence=2147483648;");
if(mode=="freeze-fault")sql(path,"CREATE TRIGGER fault BEFORE INSERT ON frozen_sync_batch BEGIN SELECT RAISE(ABORT,'freeze fault'); END;");
failed_publish=mode=="publish-failure";
const auto first=publish_once();
if(mode=="freeze-fault"){assert(first!=ESP_OK&&publishes==0);FrozenSyncBatch no;assert(storage.frozen_sync_batch(no)==ESP_ERR_NOT_FOUND);std::cout<<mode<<" PASS\n";return 0;}
assert(first==(failed_publish?ESP_FAIL:ESP_OK));FrozenSyncBatch frozen;assert(storage.frozen_sync_batch(frozen)==ESP_OK);assert(frozen.body==sent_body&&frozen.batch_id==sent_id);const auto initial_signs=signs;
if(mode=="large-sequence"){
 assert(frozen.last_sequence==2147483649LL);assert(accept(ack(2147483649LL,frozen.last_event_hash))==ESP_OK);assert(storage.acknowledged_cursor()==2147483649LL);
 auto invalid=ack(3,frozen.last_event_hash);auto invalidpos=invalid.find("committed_through_seq\":3");assert(invalidpos!=std::string::npos);invalid.replace(invalidpos,std::string("committed_through_seq\":3").size(),"committed_through_seq\":9007199254740992");assert(accept(invalid)==ESP_ERR_INVALID_ARG);
 assert(accept(ack(-1,frozen.last_event_hash))==ESP_ERR_INVALID_ARG);
 auto fraction=ack(3,frozen.last_event_hash);auto pos=fraction.find("committed_through_seq\":3");assert(pos!=std::string::npos);fraction.replace(pos,std::string("committed_through_seq\":3").size(),"committed_through_seq\":1.5");assert(accept(fraction)==ESP_ERR_INVALID_ARG);
 std::cout<<mode<<" PASS\n";return 0;
}
assert(storage.append_event("event-2","operation","RECEIPT_RESULT","{\"late\":true}",std::string(64,'b').c_str(),1704067201)==ESP_OK);
storage.close();assert(storage.open_database_for_test(path)==ESP_OK);failed_publish=false;assert(publish_once()==ESP_OK);assert(sent_body==frozen.body&&sent_id==frozen.batch_id&&signs==initial_signs);
if(mode=="changed-row")sql(path,"UPDATE event_outbox SET payload='{\"changed\":true}' WHERE sequence=1;");
if(mode=="ack-fault")sql(path,"CREATE TRIGGER fault BEFORE UPDATE ON sync_cursor BEGIN SELECT RAISE(ABORT,'cursor fault'); END;");
if(mode=="delete-fault")sql(path,"CREATE TRIGGER fault BEFORE DELETE ON frozen_sync_batch BEGIN SELECT RAISE(ABORT,'delete fault'); END;");
assert(accept(ack(frozen.last_sequence,std::string(64,'c')))!=ESP_OK);assert(accept(ack(frozen.last_sequence,frozen.last_event_hash,"foreign"))!=ESP_OK);assert(accept(ack(2,frozen.last_event_hash))!=ESP_OK);auto tampered=ack(frozen.last_sequence,frozen.last_event_hash);tampered[tampered.find("ack-1")]='X';assert(accept(tampered)!=ESP_OK);
if(mode=="interleaving"){
 std::atomic<bool>attempted{false};std::thread writer;
 storage.pending(1,[&](const PendingEvent&){writer=std::thread([&](){attempted=true;assert(storage.append_event("event-3","operation","RECEIPT_RESULT","{}",std::string(64,'b').c_str(),1704067202)==ESP_OK);});while(!attempted.load())std::this_thread::yield();assert(accept(ack(frozen.last_sequence,frozen.last_event_hash))==ESP_OK);return false;});
 writer.join();size_t count=0;storage.pending(10,[&](const PendingEvent&e){assert(e.sequence>1);++count;return true;});assert(count==2);std::cout<<mode<<" PASS (writer queued while ACK holds storage mutex)\n";return 0;
}
if(mode=="concurrent-append"){
 std::thread writer([&](){for(int n=3;n<=102;n++)assert(storage.append_event(("event-"+std::to_string(n)).c_str(),"operation","RECEIPT_RESULT","{}",std::string(64,'b').c_str(),1704067201+n)==ESP_OK);});
 assert(accept(ack(frozen.last_sequence,frozen.last_event_hash))==ESP_OK);writer.join();assert(storage.acknowledged_cursor()==1);size_t count=0;storage.pending(200,[&](const PendingEvent&e){assert(e.sequence>1);++count;return true;});assert(count==101);std::cout<<mode<<" PASS (concurrent actual SQLite writer/ACK)\n";return 0;
}
auto result=accept(ack(frozen.last_sequence,frozen.last_event_hash));
if(mode=="changed-row"||mode=="ack-fault"||mode=="delete-fault"){assert(result!=ESP_OK);assert(storage.acknowledged_cursor()==0);assert(storage.frozen_sync_batch(frozen)==ESP_OK);size_t count=0;storage.pending(10,[&](const PendingEvent&){++count;return true;});assert(count==2);}
else{assert(result==ESP_OK&&storage.acknowledged_cursor()==1);FrozenSyncBatch no;assert(storage.frozen_sync_batch(no)==ESP_ERR_NOT_FOUND);assert(accept(ack(1,frozen.last_event_hash))==ESP_OK);assert(accept(ack(1,std::string(64,'d')))!=ESP_OK);size_t count=0;storage.pending(10,[&](const PendingEvent&e){++count;assert(e.event_id=="event-2");return true;});assert(count==1);assert(publish_once()==ESP_OK);FrozenSyncBatch second;assert(storage.frozen_sync_batch(second)==ESP_OK&&second.first_sequence==2);assert(accept(ack(1,frozen.last_event_hash))==ESP_OK);FrozenSyncBatch unchanged;assert(storage.frozen_sync_batch(unchanged)==ESP_OK&&unchanged.body==second.body);assert(accept(ack(2,second.last_event_hash))==ESP_OK);assert(accept(ack(1,frozen.last_event_hash))!=ESP_OK);}
storage.close();assert(storage.open_database_for_test(path)==ESP_OK);assert(storage.acknowledged_cursor()==((mode=="changed-row"||mode=="ack-fault"||mode=="delete-fault")?0:2));std::cout<<mode<<" PASS (actual sync runtime, durable SQLite restart)\n";}
