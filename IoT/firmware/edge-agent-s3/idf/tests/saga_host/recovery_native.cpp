#include "profile_executor.h"
#include <cassert>
#include <cstdio>
#include <iostream>
#include <sqlite3.h>
using namespace beefiscal::idf;
static void sql(const std::string&p,const std::string&q){sqlite3*d{};assert(sqlite3_open(p.c_str(),&d)==0);char*e{};int rc=sqlite3_exec(d,q.c_str(),nullptr,nullptr,&e);if(rc)std::cerr<<(e?e:"")<<'\n';assert(rc==0);sqlite3_free(e);sqlite3_close(d);}
struct Io final:DeviceIo{
 std::function<void()> during_query;int purchase_calls{},reverse_calls{},query_calls{},proof_calls{},legacy_calls{};bool verified{true},foreign{},conflict{};PaymentCertainty answer{PaymentCertainty::Approved},reverse_answer{PaymentCertainty::Approved};ExecutionCertainty proof{ExecutionCertainty::Unknown};
 bool fiscal_ready()override{return true;}bool payment_ready()override{return true;}
 bool verified_reversal_profile()const override{return verified;}bool verified_payment_query_profile()const override{return verified;}
 ExecutionResult fiscalize(const ReceiptPlan&)override{assert(false);return{};}ExecutionResult fiscal_command(const ReceiptPlan&)override{assert(false);return{};}
 PaymentOutcome purchase(const std::string&,int64_t)override{++purchase_calls;assert(false);return{};}PaymentOutcome payment_lookup(const std::string&)override{assert(false);return{};}
 PaymentOutcome reverse(const std::string&rrn,int64_t amount)override{++reverse_calls;assert(rrn=="RRN-1"&&amount==500);return{reverse_answer,"reverse-stan","reverse-rrn","reverse-auth",""};}
 PaymentQueryResult query_payment(const PaymentQuery&q)override{++query_calls;if(during_query)during_query();assert(q.payment.operation_id=="op"&&q.payment.amount_minor==500);PaymentOutcome out{answer,conflict?"conflict":q.payment.terminal_reference,q.payment.rrn,q.payment.auth_code,""};if(q.kind==PaymentQueryKind::Reversal){out.reference="reverse-stan";out.rrn="reverse-rrn";out.auth_code="reverse-auth";}return{out,foreign?"foreign":"op",q.payment.payment_id,500};}
 ExecutionResult fiscal_receipt_proof(const RecoveryItem&)override{++proof_calls;return{proof,proof==ExecutionCertainty::Committed?"proven-document":"","","","",""};}
 ExecutionResult recover(const RecoveryItem&)override{++legacy_calls;return{};}
};
int main(int argc,char**argv){assert(argc==3);std::string mode=argv[1],path=argv[2];remove(path.c_str());DurableStorage s;assert(s.open_database_for_test(path.c_str())==ESP_OK);assert(s.configure_authority("DT636497",1,100,1)==ESP_OK);assert(s.reserve_command("op",std::string(64,'a').c_str(),"HTTP","{}","received","receipt","{}",{{"card-1",500},{"card-2",500}},1704067200,[](const AuthorityReservation&){return "{}";},nullptr,"0001").status==ReservationStatus::New);assert(s.set_operation_state("op","EXECUTING",nullptr,1)==ESP_OK);assert(s.begin_payment_saga("op","receipt",{{"card-1",500},{"card-2",500}},2)==ESP_OK);
 sql(path,"UPDATE payments SET state='CARD_DECLINED',terminal_reference='STAN-'||substr(payment_id,6),rrn='RRN-'||substr(payment_id,6),auth_code='auth'; UPDATE operations SET state='RECOVERY_REQUIRED';");
 auto set=[&](const char*state,const char*phase){sql(path,"UPDATE payments SET state='"+std::string(state)+"' WHERE payment_id='card-1';UPDATE payment_saga SET phase='"+phase+"';");};
 Io io;ExecutionCertainty want=ExecutionCertainty::Unknown;int reversals=0,queries=0,proofs=0;
 if(mode=="compensated"){set("REVERSED","COMPENSATED");want=ExecutionCertainty::Compensated;}
 if(mode=="declined")want=ExecutionCertainty::Rejected;
 if(mode=="reserved"){set("RESERVED","PAYMENTS");want=ExecutionCertainty::Rejected;}
 if(mode.find("query-")==0||mode=="foreign-query"||mode=="conflict-query"||mode=="operation-fault"){set("CARD_UNKNOWN","PAYMENTS");queries=1;}
 if(mode=="query-approved"){want=ExecutionCertainty::Compensated;reversals=1;}
 if(mode=="second-unknown-resolved"){set("CARD_APPROVED","PAYMENTS");sql(path,"UPDATE payments SET state='CARD_UNKNOWN' WHERE payment_id='card-2';");io.answer=PaymentCertainty::Declined;want=ExecutionCertainty::Compensated;queries=1;reversals=1;}
 if(mode=="query-declined"){io.answer=PaymentCertainty::Declined;want=ExecutionCertainty::Rejected;}
 if(mode=="query-race")io.during_query=[&](){sql(path,"UPDATE payments SET rrn='concurrent-rrn' WHERE payment_id='card-1';");};
 if(mode=="query-unknown")io.answer=PaymentCertainty::Unknown;
 if(mode=="foreign-query")io.foreign=true;if(mode=="conflict-query")io.conflict=true;
 if(mode=="query-fault")sql(path,"CREATE TRIGGER fault BEFORE UPDATE ON payments BEGIN SELECT RAISE(ABORT,'fault');END;");
 if(mode=="operation-fault")sql(path,"CREATE TRIGGER fault BEFORE UPDATE ON operations BEGIN SELECT RAISE(ABORT,'fault');END;");
 if(mode.find("reversal-")==0){set("REVERSAL_UNKNOWN","COMPENSATING");queries=1;}
 if(mode=="reversal-query"){want=ExecutionCertainty::Compensated;}
 if(mode=="reversal-unknown")io.answer=PaymentCertainty::Unknown;
 if(mode=="reversal-fault")sql(path,"CREATE TRIGGER fault BEFORE UPDATE ON payment_saga_members BEGIN SELECT RAISE(ABORT,'fault');END;");
 if(mode.find("comp-")==0){set("CARD_APPROVED","PAYMENTS");reversals=1;}
 if(mode=="partial-reversed-resume"){set("CARD_APPROVED","COMPENSATING");sql(path,"UPDATE payments SET state='REVERSED' WHERE payment_id='card-2';");want=ExecutionCertainty::Compensated;reversals=1;}
 if(mode=="comp-resume")want=ExecutionCertainty::Compensated;
 if(mode=="comp-unknown")io.reverse_answer=PaymentCertainty::Unknown;
 if(mode=="comp-unverified"){io.verified=false;reversals=0;}
 if(mode=="comp-intent-fault"){reversals=0;sql(path,"CREATE TRIGGER fault BEFORE UPDATE ON payments WHEN NEW.state='REVERSAL_SENDING' BEGIN SELECT RAISE(ABORT,'fault');END;");}
 if(mode=="comp-outcome-fault")sql(path,"CREATE TRIGGER fault BEFORE UPDATE ON payments WHEN NEW.state='REVERSED' BEGIN SELECT RAISE(ABORT,'fault');END;");
 if(mode.find("fiscal-")==0){sql(path,"UPDATE payments SET state='CARD_APPROVED'; UPDATE payment_saga SET phase='FISCAL_SENDING';");proofs=1;}
 if(mode=="fiscal-proven"){io.proof=ExecutionCertainty::Committed;want=ExecutionCertainty::Committed;}
 if(mode=="fiscal-conflict"){sql(path,"UPDATE payments SET state='CARD_DECLINED' WHERE payment_id='card-2';");io.proof=ExecutionCertainty::Committed;}
 RecoveryItem item;item.operation_id="op";item.receipt_id="receipt";item.canonical_payload="{}";
 if(mode=="legacy-card"){sql(path,"DELETE FROM payment_saga_members;DELETE FROM payment_saga;");item.canonical_payload="{\"payments\":[{\"type\":\"CARD\"}]}";}
 if(mode=="terminal-owner"){set("CARD_UNKNOWN","PAYMENTS");sql(path,"UPDATE operations SET state='COMMITTED';");queries=0;}
 if(mode=="snapshot-fault")sql(path,"DROP TABLE payment_saga_members;");
 if(mode=="stale-cas"){set("CARD_UNKNOWN","PAYMENTS");PaymentRecord old;assert(s.payment("card-1",old)==ESP_OK);sql(path,"UPDATE payments SET rrn='concurrent-rrn' WHERE payment_id='card-1';");auto observed=old;assert(s.reconcile_payment(old,observed,"CARD_APPROVED",3)!=ESP_OK);set("CARD_UNKNOWN","PAYMENTS");io.answer=PaymentCertainty::Unknown;queries=1;}
 // A genuine process lifetime boundary precedes every recovery.
 s.close();assert(s.open_database_for_test(path.c_str())==ESP_OK);CompositeBinding b;ProfileExecutor executor(b,io,&s);auto result=executor.recover(item);assert(result.certainty==want);assert(io.purchase_calls==0&&io.reverse_calls==reversals&&io.query_calls==queries&&io.proof_calls==proofs&&io.legacy_calls==0);
 if(mode=="query-fault"||mode=="operation-fault"||mode=="reversal-fault"){PaymentRecord row;assert(s.payment("card-1",row)==ESP_OK);assert(row.state==(mode=="reversal-fault"?"REVERSAL_UNKNOWN":"CARD_UNKNOWN"));}
 // Remove injected fault, restart again; uncertain sent outcomes may only query.
 if(mode=="comp-unknown"||mode=="comp-outcome-fault"){sql(path,"DROP TRIGGER IF EXISTS fault;UPDATE operations SET state='RECOVERY_REQUIRED';");s.close();assert(s.open_database_for_test(path.c_str())==ESP_OK);Io again;again.answer=PaymentCertainty::Unknown;ProfileExecutor retry(b,again,&s);assert(retry.recover(item).certainty==ExecutionCertainty::Unknown);assert(again.purchase_calls==0&&again.reverse_calls==0&&again.query_calls==1);}
 std::cout<<mode<<" PASS (actual SQLite per-tender recovery)\n";
}
