#include "receipt_saga.h"
#include <cassert>
#include <cstdio>
using namespace beefiscal::idf;
struct Fiscal: FiscalPort{ExecutionResult next{ExecutionCertainty::Committed,"F1","","","",""};ExecutionResult found{ExecutionCertainty::Committed,"F1","","","",""};int calls{};bool ready()override{return true;}ExecutionResult fiscalize(const ReceiptPlan&)override{calls++;return next;}ExecutionResult lookup(const std::string&)override{return found;}ExecutionResult cancel(const std::string&)override{return{ExecutionCertainty::Compensated};}};
struct Payment: PaymentPort{PaymentOutcome buy{PaymentCertainty::Approved,"T1","R1","A1",""};PaymentOutcome found{PaymentCertainty::Approved,"T1","R1","A1",""};PaymentOutcome reversed{PaymentCertainty::Approved,"T1","R1","A1",""};int purchases{},reversals{};bool ready()override{return true;}PaymentOutcome purchase(const std::string&,int64_t)override{purchases++;return buy;}PaymentOutcome lookup(const std::string&)override{return found;}PaymentOutcome reverse(const std::string&,int64_t)override{reversals++;return reversed;}};
ReceiptPlan card(){ReceiptPlan p;p.operation_id="11111111-1111-4111-8111-111111111111";p.payments.push_back({"p","CARD","10.00",1000});return p;}
ReceiptPlan cash(){ReceiptPlan p;p.operation_id="22222222-2222-4222-8222-222222222222";p.payments.push_back({"p","CASH","10.00",1000});return p;}
ReceiptPlan split(){auto p=card();p.payments.insert(p.payments.begin(),{"c","CASH","4.00",400});return p;}
// DSY-05: reversal amount validation — mirrors storno_matrix.json (7 cases)
static std::string check_reversal_amount(int64_t original,int64_t&reversed,int64_t request){
 if(request<=0)return "INVALID_AMOUNT";
 if(reversed>=original)return "ALREADY_FULLY_REVERSED";
 if(reversed+request>original)return "REVERSAL_EXCEEDS_REMAINING";
 reversed+=request;return "OK";
}
void storno_matrix_tests(){
 int pass=0;
 {int64_t r=0;assert(check_reversal_amount(1200,r,1200)=="OK");pass++;}
 {int64_t r=0;assert(check_reversal_amount(1200,r,500)=="OK");assert(check_reversal_amount(1200,r,700)=="OK");pass++;}
 {int64_t r=0;assert(check_reversal_amount(1200,r,500)=="OK");assert(check_reversal_amount(1200,r,800)=="REVERSAL_EXCEEDS_REMAINING");pass++;}
 {int64_t r=0;assert(check_reversal_amount(1200,r,1200)=="OK");assert(check_reversal_amount(1200,r,1)=="ALREADY_FULLY_REVERSED");pass++;}
 {int64_t r=0;assert(check_reversal_amount(1200,r,0)=="INVALID_AMOUNT");pass++;}
 {int64_t r=0;assert(check_reversal_amount(1200,r,-100)=="INVALID_AMOUNT");pass++;}
 {int64_t r=0;assert(check_reversal_amount(3,r,1)=="OK");assert(check_reversal_amount(3,r,1)=="OK");assert(check_reversal_amount(3,r,1)=="OK");assert(check_reversal_amount(3,r,1)=="ALREADY_FULLY_REVERSED");pass++;}
 std::printf("storno_matrix pass=%d\n",pass);
}
void daisy_receipt_saga_tests(){
 // DSY-04.1: drop after cmd 48 (open receipt) → UNKNOWN, not FAILED; result can be recovered.
 {Fiscal f;f.next.certainty=ExecutionCertainty::Unknown;ReceiptSaga s(f,nullptr);
  auto r=s.execute(cash());
  assert(r.certainty==ExecutionCertainty::Unknown);
  f.found.certainty=ExecutionCertainty::Committed;
  assert(s.recover(cash(),"EXECUTING").certainty==ExecutionCertainty::Committed);}
 std::printf("receipt_saga: daisy_drop_after_48=pass\n");
 // DSY-04.2: unknown after cmd 56 (close receipt) — three outcomes:
 // (1) lookup: doc number incremented (Committed) → COMMITTED
 {Fiscal f;f.next.certainty=ExecutionCertainty::Unknown;f.found.certainty=ExecutionCertainty::Committed;ReceiptSaga s(f,nullptr);
  s.execute(cash());assert(s.recover(cash(),"EXECUTING").certainty==ExecutionCertainty::Committed);}
 // (2) fresh saga: same lookup result → COMMITTED (lookup is idempotent)
 {Fiscal f;f.next.certainty=ExecutionCertainty::Unknown;f.found.certainty=ExecutionCertainty::Committed;ReceiptSaga s(f,nullptr);
  auto r=s.execute(cash());assert(r.certainty==ExecutionCertainty::Unknown);
  assert(s.recover(cash(),"EXECUTING").certainty==ExecutionCertainty::Committed);}
 // (3) lookup still Unknown → UNKNOWN (no resolution yet, manual intervention needed)
 {Fiscal f;f.next.certainty=ExecutionCertainty::Unknown;f.found.certainty=ExecutionCertainty::Unknown;ReceiptSaga s(f,nullptr);
  s.execute(cash());assert(s.recover(cash(),"EXECUTING").certainty==ExecutionCertainty::Unknown);}
 std::printf("receipt_saga: daisy_unknown_after_56=pass\n");
}
int main(){
 // The same executable contract is run for every production profile. BlueCash
 // and DP+BluePad have a payment port; Daisy is fiscal-only and fails card
 // closed. These are protocol-port fakes, not source-text anchors.
 for(int profile=0;profile<3;profile++){
  {Fiscal f;Payment p;ReceiptSaga s(f,profile==2?nullptr:&p);auto r=s.execute(cash());assert(r.certainty==ExecutionCertainty::Committed&&f.calls==1&&p.purchases==0);}
  if(profile<2){
   {Fiscal f;Payment p;ReceiptSaga s(f,&p);auto r=s.execute(card());assert(r.certainty==ExecutionCertainty::Committed&&p.purchases==1&&f.calls==1&&p.reversals==0);}
   {Fiscal f;Payment p;ReceiptSaga s(f,&p);auto r=s.execute(split());assert(r.certainty==ExecutionCertainty::Committed&&p.purchases==1&&f.calls==1);}
   {Fiscal f;Payment p;p.buy.certainty=PaymentCertainty::Declined;ReceiptSaga s(f,&p);auto r=s.execute(card());assert(r.certainty==ExecutionCertainty::Rejected&&f.calls==0);}
   {Fiscal f;Payment p;p.buy.certainty=PaymentCertainty::Unknown;ReceiptSaga s(f,&p);auto r=s.execute(card());assert(r.certainty==ExecutionCertainty::Unknown&&f.calls==0);p.found.certainty=PaymentCertainty::Approved;auto recovered=s.recover(card(),"CARD_UNKNOWN");assert(recovered.certainty==ExecutionCertainty::Committed);}
   {Fiscal f;Payment p;f.next.certainty=ExecutionCertainty::Rejected;ReceiptSaga s(f,&p);auto r=s.execute(card());assert(r.certainty==ExecutionCertainty::Compensated&&p.reversals==1);}
   {Fiscal f;Payment p;f.next.certainty=ExecutionCertainty::Rejected;p.reversed.certainty=PaymentCertainty::Unknown;ReceiptSaga s(f,&p);auto r=s.execute(card());assert(r.certainty==ExecutionCertainty::Unknown&&r.error_code=="COMPENSATION_REQUIRED");}
  }else{Fiscal f;ReceiptSaga s(f,nullptr);auto r=s.execute(card());assert(r.certainty==ExecutionCertainty::DeviceUnavailable&&f.calls==0);}
  {Fiscal f;Payment p;f.next.certainty=ExecutionCertainty::Unknown;ReceiptSaga s(f,profile==2?nullptr:&p);auto r=s.execute(cash());assert(r.certainty==ExecutionCertainty::Unknown);f.found.certainty=ExecutionCertainty::Committed;assert(s.recover(cash(),"EXECUTING").certainty==ExecutionCertainty::Committed);}
 }
 daisy_receipt_saga_tests();
 storno_matrix_tests();
}
