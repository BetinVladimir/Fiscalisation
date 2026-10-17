#include "profile_executor.h"
#include "runtime_config.h"
#include "money_plan.h"
#include "nvs.h"
#include "FrameCodec.hpp"
#include <cassert>
#include <sqlite3.h>
#include <cstdio>
#include <iostream>
#include <fstream>
#include <iterator>
#include "cJSON.h"
using namespace beefiscal::idf;
static std::vector<std::pair<int,std::string>> commands;
static int purchases=0;
static int subtotal_mode=0;static bool storno=false;
namespace beefiscal::idf{
bool SerialSession::exchange(SerialProfile profile,const SerialTransport&,uint16_t c,const std::vector<uint8_t>&p,bee::ParsedFrame*out){commands.push_back({c,std::string(p.begin(),p.end())});if(c==43||c==48)storno=c==43||std::string(p.begin(),p.end()).find("\tR")!=std::string::npos;
 if(out&&c!=51){std::string wire;
 if(c==56)wire=profile==SerialProfile::Datecs?"0\t473\t":"000003,000002";
 if(c==113)wire="473";
 if(c==119){wire="P473\t17.10.2026 14.00.00\t193\t"+std::to_string(storno?2:0)+"\t1\t0\tDY000600-OP01-0000001\t0";if(storno)wire+="\t12345678\t42\t0";}
 if(c==125)wire=std::string(p.begin(),p.end()).rfind("0\t",0)==0?"0\t473\t473\t17-10-2026 14:00:00\t"+std::to_string(storno?6:1)+"\t1\t":"0\tUNP: DY000600-OP01-0000001\t";
 if(c==123)wire="0\tDY000600\t12345678\tCompany\tAddress\t123456789\tShop\tAddress\t";
 if(c==90)wire="VER123 17-10-2026 14:00,ABCD,0000,6,DY000600,12345678";
 out->data.assign(wire.begin(),wire.end());
 }if(c==51&&out){if(subtotal_mode==3)return false;std::string wire;if(subtotal_mode==2)wire="bad";else if(profile==SerialProfile::Datecs)wire=std::string("0\t473\t")+(subtotal_mode==1?"6.99":"7.00")+"\t0.00\t7.00\t0.00\t0.00\t0.00\t0.00\t0.00\t0.00\t";else wire=std::string(subtotal_mode==1?"6.99":"7.00")+",0.00,7.00,0.00,0.00,0.00,0.00,0.00,0.00";out->data.assign(wire.begin(),wire.end());}return true;}
esp_err_t uart_runtime_start(const FiscalEndpointBinding&){return ESP_OK;}bool uart_runtime_ready(){return true;}int uart_runtime_read(uint8_t*,size_t,uint32_t){return 0;}int uart_runtime_write(const uint8_t*,size_t,uint32_t){return 0;}
bool usb_cdc_ready(){return true;}int usb_cdc_read(uint8_t*,size_t,uint32_t){return 0;}int usb_cdc_write(const uint8_t*,size_t,uint32_t){return 0;}
esp_err_t bluepad_ble_start(const PaymentEndpointBinding&){return ESP_OK;}bool bluepad_ble_ready(){return true;}
PaymentOutcome bluepad_ble_purchase(const std::string&,int64_t){++purchases;return{PaymentCertainty::Approved,"ref","rrn","auth",""};}PaymentOutcome bluepad_ble_lookup(const std::string&){return{};}PaymentOutcome bluepad_ble_reverse(const std::string&,int64_t){return{};}

}
static CompositeBinding binding(EdgeProfile profile){CompositeBinding b;b.schema_version=3;b.authority.unp_prefix="DY000600";b.profile=profile;b.payment.present=true;auto&c=b.fiscal.execution;c.present=true;c.operator_number=2;c.operator_count=30;c.operator_password_ref="fiscal-pass";c.device_configuration_id="installed-device-config-7";if(profile==EdgeProfile::DatecsDp150BluePad50){c.till_number=24;c.datecs_card_mode=2;}else c.daisy_card_code='C';return b;}
static ReceiptPlan plan(){ReceiptPlan p;p.command_type="SALE_FINALIZE";p.operation_id="op";p.receipt_session_id="receipt";p.unp="DY000600-OP01-0000001";p.items.push_back({"line","Coffee","3.000","2.65","B","0.95"});p.payments.push_back({"pay","CARD","7.00",700});return p;}
static void binding_contract(const char*path){std::ifstream file(path);std::string wire((std::istreambuf_iterator<char>(file)),{});CompositeBinding b;assert(ProvisionedBindingStore::decode_and_validate(wire.data(),wire.size(),b)==ESP_OK&&b.schema_version==3&&b.fiscal.execution.present);
 auto*root=cJSON_Parse(wire.c_str());auto*fiscal=cJSON_GetObjectItem(root,"fiscal_endpoint");auto*x=cJSON_GetObjectItem(fiscal,"execution");
 auto verify=[&](bool valid){char*raw=cJSON_PrintUnformatted(root);const auto result=ProvisionedBindingStore::decode_and_validate(raw,strlen(raw),b);cJSON_free(raw);assert((result==ESP_OK)==valid);};
 cJSON_SetNumberValue(cJSON_GetObjectItem(x,"operator_number"),0);verify(false);cJSON_SetNumberValue(cJSON_GetObjectItem(x,"operator_number"),2);
 cJSON_SetNumberValue(cJSON_GetObjectItem(x,"operator_count"),1);verify(false);cJSON_SetNumberValue(cJSON_GetObjectItem(x,"operator_count"),30);
 cJSON_SetValuestring(cJSON_GetObjectItem(x,"operator_password_ref"),"too-long-secret-reference");verify(false);cJSON_SetValuestring(cJSON_GetObjectItem(x,"operator_password_ref"),"fiscal-pass");
 auto*mapping=cJSON_GetObjectItem(x,"payment_mapping");auto*cash=cJSON_GetObjectItem(mapping,"CASH");if(cJSON_IsNumber(cash)){cJSON_SetNumberValue(cash,1);verify(false);cJSON_SetNumberValue(cash,0);}else{cJSON_SetValuestring(cash,"N");verify(false);cJSON_SetValuestring(cash,"P");}verify(true);
 cJSON_DeleteItemFromObject(fiscal,"execution");verify(false);cJSON_SetNumberValue(cJSON_GetObjectItem(root,"schema_version"),2);verify(true);assert(!b.fiscal.execution.present);cJSON_Delete(root);
}
int main(int argc,char**argv){assert(argc==4);binding_contract(argv[1]);binding_contract(argv[2]);for(auto profile:{EdgeProfile::DatecsDp150BluePad50,EdgeProfile::DaisyCompactS01}){auto b=binding(profile);auto p=plan();const std::string password=profile==EdgeProfile::DatecsDp150BluePad50?"12345678":"1234";std::string database=std::string(argv[3])+std::to_string(int(profile));remove(database.c_str());DurableStorage storage;assert(storage.open_database_for_test(database.c_str())==ESP_OK);assert(storage.configure_authority("DT636497",1,100,1)==ESP_OK);assert(storage.reserve_command("op",std::string(64,'a').c_str(),"HTTP","{}","received","receipt","{}",{{"pay",700}},1704067200,[](const AuthorityReservation&){return "{}";},nullptr,"0001").status==ReservationStatus::New);EspIdfDeviceIo io(b,password);ProfileExecutor executor(b,io,&storage);auto run=[&](const ReceiptPlan&value){sqlite3*db{};assert(sqlite3_open(database.c_str(),&db)==SQLITE_OK);assert(sqlite3_exec(db,"DELETE FROM payment_saga_members;DELETE FROM payment_saga;UPDATE operations SET state='EXECUTING';UPDATE payments SET state='RESERVED',terminal_reference=NULL,rrn=NULL,auth_code=NULL",nullptr,nullptr,nullptr)==SQLITE_OK);sqlite3_close(db);return executor.execute(value);};commands.clear();purchases=0;assert(run(p).certainty==ExecutionCertainty::Committed);assert(purchases==1&&commands.size()==8);if(profile==EdgeProfile::DatecsDp150BluePad50){assert(commands[0]==std::make_pair(48,std::string("2\t12345678\tDY000600-OP01-0000001\t24\t\t")));assert(commands[1]==std::make_pair(49,std::string("Coffee\t2\t2.65\t3.000\t4\t0.95\t0\t\t")));assert(commands[3]==std::make_pair(53,std::string("2\t7.00\t")));b.fiscal.execution.datecs_card_mode=1;commands.clear();assert(run(p).certainty==ExecutionCertainty::Committed);assert(commands[3].second=="1\t7.00\t");}else{assert(commands[0].second=="2,1234,DY000600-OP01-0000001");std::string line="Coffee\t";line.push_back(char(0xc1));line+="2.65*3.000$-0.95";assert(commands[1].second==line);assert(commands[3].second=="\tC7.00");}
 assert(commands[2].first==51&&commands[2].second==(profile==EdgeProfile::DatecsDp150BluePad50?"0\t0\t\t\t":"00"));
 for(int mode=1;mode<=3;mode++){subtotal_mode=mode;commands.clear();auto result=run(p);assert(result.certainty==ExecutionCertainty::Unknown&&result.terminal_reference=="ref"&&result.rrn=="rrn"&&result.auth_code=="auth");assert(commands.size()==3&&commands.back().first==51);sqlite3*db{};sqlite3_stmt*row{};assert(sqlite3_open(database.c_str(),&db)==SQLITE_OK);assert(sqlite3_prepare_v2(db,"SELECT count(*) FROM payments WHERE payment_id='pay' AND state='CARD_APPROVED' AND terminal_reference='ref' AND rrn='rrn' AND auth_code='auth'",-1,&row,nullptr)==SQLITE_OK);assert(sqlite3_step(row)==SQLITE_ROW&&sqlite3_column_int(row,0)==1);sqlite3_finalize(row);sqlite3_close(db);}subtotal_mode=0;
 auto reversal=p;reversal.command_type="SALE_REVERSE";reversal.payments[0].type="CASH";reversal.original_document=42;reversal.original_datetime="11-08-26 12:30:45";reversal.original_fmin="12345678";reversal.reversal_reason=1;commands.clear();assert(run(reversal).certainty==ExecutionCertainty::Committed);assert(commands.front().first==(profile==EdgeProfile::DatecsDp150BluePad50?43:48));assert(commands.front().second.find(password)!=std::string::npos);
 const auto deny=[&](ReceiptPlan v){commands.clear();purchases=0;assert(run(v).certainty==ExecutionCertainty::Rejected);assert(commands.empty()&&purchases==0);};
 auto bad=p;bad.payments[0].amount="7.01";bad.payments[0].amount_minor=701;deny(bad);bad=p;bad.items[0].discount="8.00";deny(bad);bad=p;bad.items[0].quantity="9223372036854775807.000";deny(bad);bad=p;bad.items[0].name=std::string(73,'x');deny(bad);bad=p;bad.payments[0].amount_minor=1;deny(bad);bad=p;bad.items[0].tax_group="";deny(bad);
 b.fiscal.execution.present=false;deny(p);b.fiscal.execution.present=true;EspIdfDeviceIo nosecret(b);ProfileExecutor blocked(b,nosecret);purchases=0;assert(blocked.execute(p).error_code=="FISCAL_CREDENTIAL_REQUIRED"&&purchases==0);
 // Go lineTotalCents rounds half-up before subtracting the absolute line discount.
 p.items[0].quantity="0.500";p.items[0].unit_price="0.01";p.items[0].discount="0.00";p.payments[0].amount="0.01";p.payments[0].amount_minor=1;assert(receipt_plan_error(b,p).empty());
 assert(RuntimeConfigLoader::provision_fiscal_password(b,password)==ESP_OK);assert(secret_values["fiscal-pass"]==password);assert(RuntimeConfigLoader::provision_fiscal_password(b,"invalid")==ESP_ERR_INVALID_ARG);
 b.mqtt.root_ca_ref="ca";b.mqtt.client_certificate_ref="cert";b.mqtt.client_key_ref="key";b.authority.command_hmac_ref="cmd";b.authority.sync_ack_hmac_ref="ack";secret_values["ca"]="BEGIN CERTIFICATE";secret_values["cert"]="BEGIN CERTIFICATE";secret_values["key"]="PRIVATE KEY";secret_values["cmd"]=std::string(32,'c');secret_values["ack"]=std::string(32,'a');RuntimeSecrets secrets;assert(RuntimeConfigLoader{}.load_secrets(b,secrets)==ESP_OK&&secrets.fiscal_operator_password==password);secret_values.erase("fiscal-pass");assert(RuntimeConfigLoader{}.load_secrets(b,secrets)!=ESP_OK&&secrets.fiscal_operator_password.empty());
 }std::cout<<"actual profile execution vendor bytes / fixed money / fail-before-POS / secure-store path PASS\n";}
