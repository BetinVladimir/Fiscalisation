#include "profile_executor.h"
#include <cassert>
#include <cstdio>
#include <iostream>
#include <sqlite3.h>
using namespace beefiscal::idf;
static void sql(const std::string&path,const std::string&query){sqlite3*db{};assert(sqlite3_open(path.c_str(),&db)==0);char*error{};auto rc=sqlite3_exec(db,query.c_str(),nullptr,nullptr,&error);if(rc)std::cerr<<(error?error:"")<<"\n";assert(rc==0);sqlite3_free(error);sqlite3_close(db);}
static std::string scalar(const std::string&path,const std::string&query){sqlite3*db{};sqlite3_stmt*s{};assert(sqlite3_open(path.c_str(),&db)==0);assert(sqlite3_prepare_v2(db,query.c_str(),-1,&s,nullptr)==0);assert(sqlite3_step(s)==SQLITE_ROW);auto*p=sqlite3_column_text(s,0);std::string out=p?(const char*)p:"";sqlite3_finalize(s);sqlite3_close(db);return out;}
struct Io final:DeviceIo{
 int purchases{},reversals{},fiscal{},recoveries{};bool verified{true};std::vector<PaymentCertainty>purchase_results{PaymentCertainty::Approved,PaymentCertainty::Declined};PaymentCertainty reversal_result{PaymentCertainty::Approved};ExecutionCertainty fiscal_result{ExecutionCertainty::Committed};std::vector<std::string>reversal_rrns;
 bool fiscal_ready()override{return true;}bool payment_ready()override{return true;}bool verified_reversal_profile()const override{return verified;}
 ExecutionResult fiscalize(const ReceiptPlan&)override{++fiscal;return{fiscal_result,"fiscal","failure","","",""};}ExecutionResult fiscal_command(const ReceiptPlan&p)override{return fiscalize(p);}
 PaymentOutcome purchase(const std::string&,int64_t)override{const auto index=purchases++;return{purchase_results.at(index),"STAN-"+std::to_string(index),"RRN-"+std::to_string(index),"AUTH-"+std::to_string(index),"declined"};}
 PaymentOutcome payment_lookup(const std::string&)override{return{};}
 PaymentOutcome reverse(const std::string&original_rrn,int64_t)override{++reversals;reversal_rrns.push_back(original_rrn);return{reversal_result,"REVERSAL-STAN","REVERSAL-RRN","REVERSAL-AUTH",""};}
 ExecutionResult recover(const RecoveryItem&)override{++recoveries;return{};}
};
static CompositeBinding binding(){CompositeBinding b;b.profile=EdgeProfile::DatecsDp150BluePad50;b.payment.present=true;b.payment.device_id="terminal-A";auto&c=b.fiscal.execution;c.present=true;c.operator_number=1;c.operator_count=30;c.operator_password_ref="pass";c.device_configuration_id="declared";c.till_number=1;c.datecs_card_mode=2;return b;}
static ReceiptPlan plan(){ReceiptPlan p;p.operation_id="op";p.receipt_session_id="receipt";p.command_type="SALE_FINALIZE";p.items.push_back({"line","Coffee","1.000","10.00","B",""});p.payments={{"card-1","CARD","5.00",500},{"card-2","CARD","5.00",500}};return p;}
static void trigger(const std::string&path,const std::string&table,const std::string&condition,const char*event="UPDATE"){sql(path,"CREATE TRIGGER fault BEFORE "+std::string(event)+" ON "+table+" WHEN "+condition+" BEGIN SELECT RAISE(ABORT,'injected checkpoint failure');END;");}
int main(int argc,char**argv){assert(argc==3);const std::string mode=argv[1],path=argv[2];remove(path.c_str());DurableStorage storage;assert(storage.open_database_for_test(path.c_str())==ESP_OK);assert(storage.configure_authority("DT636497",1,100,1)==ESP_OK);assert(storage.reserve_command("op",std::string(64,'a').c_str(),"HTTP","{}","received","receipt","{}",{{"card-1",500},{"card-2",500}},1704067200,[](const AuthorityReservation&){return "{}";},nullptr,"0001").status==ReservationStatus::New);assert(storage.set_operation_state("op","EXECUTING",nullptr,1704067200)==ESP_OK);auto b=binding();auto p=plan();Io io;
 if(mode=="all-approved"||mode=="fiscal-unknown"||mode=="fiscal-rejected"||mode.find("fiscal-")==0)io.purchase_results={PaymentCertainty::Approved,PaymentCertainty::Approved};
 if(mode=="second-unknown")io.purchase_results[1]=PaymentCertainty::Unknown;
 if(mode=="second-not-sent")io.purchase_results[1]=PaymentCertainty::NotSent;
 if(mode=="unverified")io.verified=false;
 if(mode=="reversal-unknown")io.reversal_result=PaymentCertainty::Unknown;
 if(mode=="fiscal-unknown")io.fiscal_result=ExecutionCertainty::Unknown;
 if(mode=="fiscal-rejected")io.fiscal_result=ExecutionCertainty::Rejected;
 if(mode=="first-declined")io.purchase_results[0]=PaymentCertainty::Declined;
 if(mode=="refund-no-original")p.command_type="SALE_REVERSE";
 if(mode=="begin-member")trigger(path,"payment_saga_members","1","INSERT");
 if(mode=="begin-table")trigger(path,"payment_saga","1","INSERT");
 if(mode=="begin-operation")trigger(path,"operations","NEW.state='PAYMENT_SAGA_PENDING' AND NOT EXISTS(SELECT 1 FROM payments WHERE state!='RESERVED')");
 if(mode=="intent-payment")trigger(path,"payments","NEW.state='CARD_SENDING'");
 if(mode=="intent-operation")trigger(path,"operations","EXISTS(SELECT 1 FROM payments WHERE state='CARD_SENDING')");
 if(mode=="outcome-payment")trigger(path,"payments","NEW.state='CARD_APPROVED'");
 if(mode=="outcome-operation")trigger(path,"operations","EXISTS(SELECT 1 FROM payments WHERE state='CARD_APPROVED')");
 if(mode=="fiscal-phase")trigger(path,"payment_saga","NEW.phase='FISCAL_SENDING'");
 if(mode=="fiscal-operation")trigger(path,"operations","EXISTS(SELECT 1 FROM payment_saga WHERE phase='FISCAL_SENDING')");
 if(mode=="comp-intent-payment")trigger(path,"payments","NEW.state='REVERSAL_SENDING'");
 if(mode=="comp-intent-operation")trigger(path,"operations","NEW.state='COMPENSATION_REQUIRED'");
 if(mode=="comp-outcome-member")trigger(path,"payment_saga_members","1");
 if(mode=="comp-outcome-payment")trigger(path,"payments","NEW.state='REVERSED'");
 if(mode=="comp-outcome-operation")trigger(path,"operations","EXISTS(SELECT 1 FROM payments WHERE state='REVERSED')");
 if(mode=="comp-final-phase")trigger(path,"payment_saga","NEW.phase='COMPENSATED'");
 if(mode=="comp-final-operation")trigger(path,"operations","EXISTS(SELECT 1 FROM payment_saga WHERE phase='COMPENSATED')");
 ProfileExecutor executor(b,io,&storage);
 if(mode=="cas-ownership"){
  assert(storage.begin_payment_saga("op","receipt",{{"card-1",500},{"card-1",500}},1704067201)!=ESP_OK);std::string phase;assert(storage.payment_saga_phase("op",phase)==ESP_ERR_NOT_FOUND);
  assert(storage.begin_payment_saga("op","receipt",{{"card-1",500}},1704067201)==ESP_OK);PaymentRecord row;assert(storage.payment("card-1",row)==ESP_OK);auto wrong=row;wrong.operation_id="foreign";assert(storage.checkpoint_payment(wrong,"RESERVED","CARD_SENDING","PAYMENTS",1704067202)!=ESP_OK);wrong=row;wrong.receipt_id="foreign";assert(storage.checkpoint_payment(wrong,"RESERVED","CARD_SENDING","PAYMENTS",1704067202)!=ESP_OK);wrong=row;wrong.amount_minor=501;assert(storage.checkpoint_payment(wrong,"RESERVED","CARD_SENDING","PAYMENTS",1704067202)!=ESP_OK);assert(storage.payment("card-2",wrong)==ESP_OK);assert(storage.checkpoint_payment(wrong,"RESERVED","CARD_SENDING","PAYMENTS",1704067202)!=ESP_OK);assert(storage.checkpoint_saga_phase("op","PAYMENTS","FISCAL_SENDING",1704067202)!=ESP_OK);
  assert(storage.payment("card-1",row)==ESP_OK&&row.state=="RESERVED");assert(storage.checkpoint_payment(row,"RESERVED","CARD_SENDING","PAYMENTS",1704067202)==ESP_OK);row.rrn="original-rrn";assert(storage.checkpoint_payment(row,"CARD_SENDING","CARD_APPROVED","PAYMENTS",1704067203)==ESP_OK);assert(storage.set_operation_state("op","COMMITTED",nullptr,1704067204)==ESP_OK);assert(storage.checkpoint_payment(row,"CARD_APPROVED","REVERSAL_SENDING","COMPENSATING",1704067205)!=ESP_OK);
 }
 auto result=executor.execute(p);
 if(mode=="all-approved"){assert(result.certainty==ExecutionCertainty::Committed&&io.fiscal==1&&io.purchases==2);assert(scalar(path,"SELECT phase FROM payment_saga")=="FISCAL_SENDING");}
 else if(mode=="decline-compensated"||mode=="second-not-sent"||mode=="fiscal-rejected"){assert(result.certainty==ExecutionCertainty::Compensated);assert(io.reversals==(mode=="fiscal-rejected"?2:1));assert(io.reversal_rrns[0]==(mode=="fiscal-rejected"?"RRN-1":"RRN-0"));assert(scalar(path,"SELECT state FROM payments WHERE payment_id='card-1'")=="REVERSED");assert(scalar(path,"SELECT reversal_reference||'|'||reversal_rrn||'|'||reversal_auth_code FROM payment_saga_members WHERE payment_id='card-1'")=="REVERSAL-STAN|REVERSAL-RRN|REVERSAL-AUTH");}
 else if(mode=="first-declined"||mode=="refund-no-original"){assert(result.certainty==ExecutionCertainty::Rejected&&io.reversals==0&&io.fiscal==0);if(mode=="refund-no-original")assert(io.purchases==0);}
 else {assert(result.certainty==ExecutionCertainty::Unknown);if(mode!="fiscal-unknown")assert(io.fiscal==0);}
 if(mode=="second-unknown"){assert(io.reversals==0);assert(scalar(path,"SELECT state FROM payments WHERE payment_id='card-1'")=="CARD_APPROVED");assert(scalar(path,"SELECT state FROM payments WHERE payment_id='card-2'")=="CARD_UNKNOWN");}
 if(mode=="unverified"){assert(io.reversals==0);assert(scalar(path,"SELECT phase FROM payment_saga")=="COMPENSATION_REQUIRED");}
 if(mode=="outcome-payment"||mode=="outcome-operation"){assert(io.purchases==1&&io.reversals==0);assert(scalar(path,"SELECT state FROM payments WHERE payment_id='card-1'")=="CARD_SENDING");}
 if(mode=="begin-member"||mode=="begin-table"||mode=="begin-operation"||mode=="intent-payment"||mode=="intent-operation")assert(io.purchases==0&&io.reversals==0);
 if(mode=="comp-outcome-member"||mode=="comp-outcome-payment"||mode=="comp-outcome-operation"){assert(io.reversals==1);assert(scalar(path,"SELECT state FROM payments WHERE payment_id='card-1'")=="REVERSAL_SENDING");}
 if(io.reversals){assert(scalar(path,"SELECT terminal_reference||'|'||rrn||'|'||auth_code FROM payments WHERE payment_id='card-1'")=="STAN-0|RRN-0|AUTH-0");}
 // Simulate IntentProcessor's UNKNOWN projection and a power-cycle. Never infer a
 // retry-safe payment from an operation-level result or lose its durable phase.
 sql(path,"DROP TRIGGER IF EXISTS fault;");assert(storage.set_operation_state("op","RECOVERY_REQUIRED",nullptr,1704067201)==ESP_OK);storage.close();assert(storage.open_database_for_test(path.c_str())==ESP_OK);Io restarted;ProfileExecutor after(b,restarted,&storage);assert(after.execute(p).certainty!=ExecutionCertainty::Committed);assert(restarted.purchases==0&&restarted.reversals==0&&restarted.fiscal==0);
 RecoveryItem item;item.operation_id="op";item.receipt_id="receipt";std::string phase;if(storage.payment_saga_phase("op",phase)==ESP_OK){const bool completed=mode=="decline-compensated"||mode=="second-not-sent"||mode=="fiscal-rejected"||mode=="comp-final-phase"||mode=="comp-final-operation";
 const bool newCompensation=mode=="unverified"||mode=="comp-intent-payment"||mode=="comp-intent-operation";
 auto recovered=after.recover(item);
 assert(recovered.certainty==(completed||newCompensation?ExecutionCertainty::Compensated:(mode=="first-declined"||mode=="intent-payment"||mode=="intent-operation")?ExecutionCertainty::Rejected:ExecutionCertainty::Unknown));
 assert(restarted.recoveries==0&&restarted.purchases==0&&restarted.fiscal==0&&restarted.reversals==(newCompensation?1:0));}
 std::cout<<mode<<" PASS (actual SQLite checkpoints, power-cycle, no duplicate payment/reversal)\n";
}
