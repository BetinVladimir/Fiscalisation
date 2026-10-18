#include "profile_executor.h"
#include <cassert>
#include <cstdio>
#include <iostream>
#include <sqlite3.h>
#include <thread>
#include <atomic>
using namespace beefiscal::idf;
struct Io:DeviceIo{int purchases{},fiscal{};PaymentCertainty outcome{PaymentCertainty::Unknown};ExecutionCertainty fiscal_outcome{ExecutionCertainty::Committed};bool fiscal_ready()override{return true;}bool payment_ready()override{return true;}ExecutionResult fiscalize(const ReceiptPlan&)override{++fiscal;return{fiscal_outcome,"real-doc","","","",""};}PaymentOutcome purchase(const std::string&,int64_t)override{++purchases;return{outcome,"STAN","RRN","AUTH",""};}ExecutionResult fiscal_command(const ReceiptPlan&)override{return{};}PaymentOutcome payment_lookup(const std::string&)override{return{};}PaymentOutcome reverse(const std::string&,int64_t)override{return{};}ExecutionResult recover(const RecoveryItem&)override{return{};}};
CompositeBinding binding(const char*terminal){CompositeBinding b;b.profile=EdgeProfile::DatecsDp150BluePad50;b.payment.present=true;b.payment.device_id=terminal;auto&c=b.fiscal.execution;c.present=true;c.operator_number=1;c.operator_count=30;c.operator_password_ref="pass";c.device_configuration_id="declared";c.till_number=1;c.datecs_card_mode=2;return b;}
ReceiptPlan plan(const std::string&id){ReceiptPlan p;p.operation_id=id;p.receipt_session_id=id+"-receipt";p.command_type="SALE_FINALIZE";p.items.push_back({id+"-line","Coffee","1.000","10.00","B",""});p.payments={{id+"-card","CARD","10.00",1000}};return p;}
void reserve(DurableStorage&s,const ReceiptPlan&p,const char*terminal){std::string route="{\"payment_device_id\":\""+std::string(terminal)+"\"}";assert(s.reserve_command(p.operation_id.c_str(),std::string(64,'a').c_str(),"HTTP",route.c_str(),(p.operation_id+"-received").c_str(),p.receipt_session_id.c_str(),"{}",{{p.payments[0].id,1000}},1704067200,[](const AuthorityReservation&){return "{}";},nullptr,"0001").status==ReservationStatus::New);assert(s.set_operation_state(p.operation_id.c_str(),"EXECUTING",nullptr,1704067200)==ESP_OK);}
void sql(const char*path,const char*query){sqlite3*d{};assert(sqlite3_open(path,&d)==SQLITE_OK);assert(sqlite3_exec(d,query,nullptr,nullptr,nullptr)==SQLITE_OK);sqlite3_close(d);}
int count(const char*path,const char*query){sqlite3*d{};sqlite3_stmt*q{};assert(sqlite3_open(path,&d)==SQLITE_OK);assert(sqlite3_prepare_v2(d,query,-1,&q,nullptr)==SQLITE_OK);assert(sqlite3_step(q)==SQLITE_ROW);int n=sqlite3_column_int(q,0);sqlite3_finalize(q);sqlite3_close(d);return n;}
void complete(DurableStorage&s,const ReceiptPlan&p,const char*state){assert(s.complete_operation(p.operation_id.c_str(),p.receipt_session_id.c_str(),state,"{}","{}",{},(p.operation_id+"-result").c_str(),"COMMAND_RESULT",std::string(64,'a').c_str(),1704067300)==ESP_OK);}
int main(int argc,char**argv){
 assert(argc==3);const std::string mode=argv[1];std::remove(argv[2]);DurableStorage s;assert(s.open_database_for_test(argv[2])==ESP_OK);assert(s.configure_authority("DT636497",1,100,1)==ESP_OK);auto b=binding("terminal-A");auto p=plan("old");reserve(s,p,"terminal-A");Io first;
 if(mode=="approved-complete"||mode=="approved-open"){first.outcome=PaymentCertainty::Approved;if(mode=="approved-open")first.fiscal_outcome=ExecutionCertainty::Unknown;}
 if(mode=="concurrent"){
  auto secondplan=plan("parallel");reserve(s,secondplan,"terminal-A");DurableStorage other;assert(other.open_database_for_test(argv[2])==ESP_OK);Io second;ProfileExecutor left(b,first,&s),right(b,second,&other);std::atomic<int>ready{0};std::thread a([&]{++ready;while(ready<2){}left.execute(p);});std::thread z([&]{++ready;while(ready<2){}right.execute(secondplan);});a.join();z.join();assert(first.purchases+second.purchases==1);assert(count(argv[2],"SELECT count(*) FROM terminal_payment_guard")==1);std::cout<<mode<<" PASS concurrent actual SQLite connections\n";return 0;
 }
 if(mode=="claim-fault")sql(argv[2],"CREATE TRIGGER fault BEFORE INSERT ON payment_saga_members BEGIN SELECT RAISE(ABORT,'fault');END;");
 if(mode=="outcome-fault")sql(argv[2],"CREATE TRIGGER fault BEFORE UPDATE ON payments WHEN NEW.state='CARD_UNKNOWN' BEGIN SELECT RAISE(ABORT,'fault');END;");
 ProfileExecutor x(b,first,&s);auto initial=x.execute(p);assert(initial.certainty==(mode=="approved-complete"?ExecutionCertainty::Committed:ExecutionCertainty::Unknown));assert(first.purchases==(mode=="claim-fault"?0:1));sql(argv[2],"DROP TRIGGER IF EXISTS fault");if(mode=="claim-fault")assert(count(argv[2],"SELECT count(*) FROM terminal_payment_guard")==0);
 if(mode=="approved-complete"||mode=="contradictory-committed")complete(s,p,"COMMITTED");
 s.close();assert(s.open_database_for_test(argv[2])==ESP_OK);
 if(mode=="sending")sql(argv[2],"UPDATE payments SET state='CARD_SENDING' WHERE operation_id='old'");
 if(mode=="legacy"||mode=="legacy-unknown-route"){sql(argv[2],"DELETE FROM terminal_payment_guard");if(mode=="legacy-unknown-route")sql(argv[2],"UPDATE operations SET route_snapshot='{}' WHERE operation_id='old'");}
 if(mode.find("resolve-")==0){
  PaymentRecord expected;assert(s.payment(p.payments[0].id.c_str(),expected)==ESP_OK);auto observed=expected;PaymentReconciliationEvidence evidence{"reconcile-1","terminal-A","admin-1","acquirer-case-1",std::string(64,'b'),std::string(64,'c')};
  if(mode=="resolve-wrong-terminal")evidence.terminal_id="terminal-B";
  if(mode=="resolve-wrong-amount")observed.amount_minor++;
  if(mode=="resolve-bad-evidence")evidence.evidence_sha256="bad";
  if(mode=="resolve-event-collision")assert(s.append_event("reconcile-1-reconciled","old","WRONG_KIND","{}",std::string(64,'a').c_str(),1704067201)==ESP_OK);
  if(mode=="resolve-outbox-fault")sql(argv[2],"CREATE TRIGGER fail_evidence BEFORE INSERT ON event_outbox WHEN NEW.kind='PAYMENT_RECONCILED' BEGIN SELECT RAISE(ABORT,'fault');END;");
  const char*next=mode=="resolve-declined"?"CARD_DECLINED":"CARD_APPROVED";
  const auto rc=s.reconcile_payment(expected,observed,next,1704067250,&evidence);const bool valid=mode=="resolve-approved"||mode=="resolve-declined";assert((rc==ESP_OK)==valid);
  PaymentRecord saved;assert(s.payment(expected.payment_id.c_str(),saved)==ESP_OK);assert(saved.state==(valid?next:"CARD_UNKNOWN"));assert(count(argv[2],"SELECT count(*) FROM payment_reconciliations")==(valid?1:0));assert(count(argv[2],"SELECT count(*) FROM event_outbox WHERE kind='PAYMENT_RECONCILED'")==(valid?1:0));
  if(valid)assert(s.reconcile_payment(expected,observed,next,1704067251,&evidence)!=ESP_OK);
  if(mode=="resolve-declined")complete(s,p,"REJECTED");
  if(mode=="resolve-approved")complete(s,p,"REJECTED"); // A rejected fiscal projection cannot erase an approved bank payment.
 }
 auto n=plan("new");const bool other=mode=="other-terminal";reserve(s,n,other?"terminal-B":"terminal-A");if(mode=="cash")n.payments[0].type="CASH";auto nb=binding(other?"terminal-B":"terminal-A");Io second;ProfileExecutor y(nb,second,&s);y.execute(n);
 const bool allowed=other||mode=="approved-complete"||mode=="resolve-declined"||mode=="claim-fault";assert(second.purchases==(allowed?1:0));if(mode=="cash")assert(second.fiscal==1);
 std::cout<<mode<<" PASS (actual SQLite restart, scoped guard, no duplicate payment)\n";
}
