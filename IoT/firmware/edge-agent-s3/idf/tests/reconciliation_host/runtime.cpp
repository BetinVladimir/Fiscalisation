#include "intent_processor.h"
#include "cJSON.h"
#include <cassert>
#include <fstream>
#include <sstream>
#include <iostream>
#include <cstring>
#include <sqlite3.h>
using namespace beefiscal::idf;
namespace beefiscal::idf{esp_err_t canonical_cbor_to_json(const uint8_t*,size_t,cJSON**){return ESP_ERR_NOT_SUPPORTED;}}
std::string read(const char*p){std::ifstream f(p);std::stringstream s;s<<f.rdbuf();return s.str();}
std::string text(cJSON*p,const char*k){auto*v=cJSON_GetObjectItem(p,k);assert(cJSON_IsString(v));return v->valuestring;}
struct Completion{int calls{};std::string operation,result;};
esp_err_t completed(const char*operation,const char*result,Ingress,void*context){auto*c=static_cast<Completion*>(context);++c->calls;c->operation=operation;c->result=result;return ESP_OK;}
struct NoIo:ReceiptExecutor{int calls{};bool ready()override{return true;}ExecutionResult execute(const ReceiptPlan&)override{++calls;return{};}ExecutionResult recover(const RecoveryItem&)override{++calls;return{};}};
int main(int argc,char**argv){assert(argc==6);auto*f=cJSON_Parse(read(argv[1]).c_str());assert(f);auto*c=cJSON_GetObjectItem(f,"command");auto*g=cJSON_GetObjectItem(f,"grant");auto*e=cJSON_GetObjectItem(g,"expected");auto*env=cJSON_GetObjectItem(c,"reconciliation_authorization");const std::string mode=argv[4];CompositeBinding binding;binding.tenant_id=text(g,"tenant_id");binding.location_id=text(g,"location_id");binding.register_id=text(g,"register_id");binding.edge_device_id=text(g,"edge_device_id");binding.payload_sha256=text(g,"binding_sha256");binding.generation=cJSON_GetObjectItem(g,"binding_generation")->valueint;binding.payment.present=true;binding.payment.device_id=text(g,"terminal_id");std::string pem=read(argv[2]),kid=text(env,"key_id");ExecutionAuthorizationTrust trust{pem.c_str(),kid.c_str(),(int64_t)cJSON_GetObjectItem(g,"nbf")->valuedouble};
if(mode=="signature")cJSON_SetValuestring(cJSON_GetObjectItem(env,"signature"),"AAAA");
if(mode=="tenant")binding.tenant_id="foreign";
if(mode=="terminal")binding.payment.device_id="foreign";
if(mode=="binding")binding.payload_sha256=std::string(64,'a');
if(mode=="expired")trust.now=cJSON_GetObjectItem(g,"exp")->valuedouble;
if(mode=="future")--trust.now;
if(mode=="alias")cJSON_AddStringToObject(c,"intent_id","foreign");
const auto oid=text(e,"operation_id"),rid=text(e,"receipt_id"),pid=text(e,"payment_id");remove(argv[3]);DurableStorage s;assert(s.open_database_for_test(argv[3])==ESP_OK);assert(s.configure_authority("AB123456",1,1000,7)==ESP_OK);assert(s.reserve_command(oid.c_str(),std::string(64,'a').c_str(),"HTTP","{\"payment_device_id\":\"terminal-1\"}","received",rid.c_str(),"{}",{{pid,250}},trust.now,[](const AuthorityReservation&){return "{}";},nullptr,"A001").status==ReservationStatus::New);assert(s.set_operation_state(oid.c_str(),"EXECUTING",nullptr,trust.now)==ESP_OK);assert(s.begin_payment_saga(oid.c_str(),rid.c_str(),{{pid,250}},trust.now,"terminal-1")==ESP_OK);PaymentRecord row;assert(s.payment(pid.c_str(),row)==ESP_OK);assert(s.checkpoint_payment(row,"RESERVED","CARD_SENDING","PAYMENTS",trust.now)==ESP_OK);assert(s.checkpoint_payment(row,"CARD_SENDING","CARD_UNKNOWN","PAYMENTS",trust.now)==ESP_OK);assert(s.set_operation_state(oid.c_str(),"RECOVERY_REQUIRED",nullptr,trust.now)==ESP_OK);
if(mode=="row"){sqlite3*db{};assert(sqlite3_open(argv[3],&db)==SQLITE_OK);assert(sqlite3_exec(db,"UPDATE payments SET rrn='already-known'",nullptr,nullptr,nullptr)==SQLITE_OK);sqlite3_close(db);}if(mode=="admin-fault"||mode=="outbox-fault"){sqlite3*db{};assert(sqlite3_open(argv[3],&db)==SQLITE_OK);const char*trigger=mode=="admin-fault"?"CREATE TRIGGER admin_fault BEFORE INSERT ON operations WHEN NEW.first_transport='ADMIN' BEGIN SELECT RAISE(ABORT,'fault');END;":"CREATE TRIGGER audit_fault BEFORE INSERT ON event_outbox WHEN NEW.kind='PAYMENT_RECONCILED' BEGIN SELECT RAISE(ABORT,'fault');END;";assert(sqlite3_exec(db,trigger,nullptr,nullptr,nullptr)==SQLITE_OK);sqlite3_close(db);}

s.close();assert(s.open_database_for_test(argv[3])==ESP_OK);NoIo io;Completion completion;IntentProcessor processor(s,binding,io,"",completed,&completion,&trust);char*wire=cJSON_PrintUnformatted(c);const auto rc=processor.accept({(const uint8_t*)wire,strlen(wire),Ingress::Http});const bool valid=mode=="valid";if((rc==ESP_OK)!=valid)std::cerr<<mode<<" rc="<<rc<<"\n";assert((rc==ESP_OK)==valid);assert(io.calls==0);assert(completion.calls==(valid?1:0));if(valid){assert(completion.operation==text(c,"client_operation_id"));assert(!completion.result.empty());}assert(s.payment(pid.c_str(),row)==ESP_OK);assert(row.state==(valid?"CARD_APPROVED":"CARD_UNKNOWN"));OperationRecord original;assert(s.operation(oid.c_str(),original)==ESP_OK&&original.state=="RECOVERY_REQUIRED");if(valid){OperationRecord admin;assert(s.operation(text(c,"client_operation_id").c_str(),admin)==ESP_OK&&admin.state=="COMMITTED");std::ofstream out(argv[5]);bool exported=false;assert(s.pending(100,[&](const PendingEvent&event){if(event.kind=="PAYMENT_RECONCILED"){out<<event.payload;exported=true;}return true;})==ESP_OK);assert(exported);assert(processor.accept({(const uint8_t*)wire,strlen(wire),Ingress::Http})!=ESP_OK);s.close();assert(s.open_database_for_test(argv[3])==ESP_OK);assert(s.payment(pid.c_str(),row)==ESP_OK&&row.state=="CARD_APPROVED");}
cJSON_free(wire);cJSON_Delete(f);std::cout<<mode<<" actual Go grant→mbedTLS→IntentProcessor→SQLite PASS\n";}
