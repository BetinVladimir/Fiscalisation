#include "execution_authorization.h"
#include <cassert>
#include <fstream>
#include <iterator>
#include <iostream>
using namespace beefiscal;
std::string read(const std::string&p){std::ifstream f(p);assert(f);return {std::istreambuf_iterator<char>(f),{}};}
int main(int argc,char**argv){assert(argc==2);const std::string d=argv[1],pem=read(d+"/trust.pem");idf::CompositeBinding b;b.tenant_id="tenant-1";b.location_id="location-1";b.register_id="register-1";b.edge_device_id="edge-1";b.fiscal.device_id="fiscal-1";b.fiscal.fiscal_device_number="DT636497";b.fiscal.fiscal_memory_number="12345678";b.payload_sha256=read(d+"/binding.sha256");b.generation=7;b.authority.unp_prefix="DT636497";b.authority.unp_range_start=1;b.authority.unp_range_end=100;
 for(const char*mode:{"valid","expired","future","long-ttl","wrong-tenant","wrong-binding","wrong-range","wrong-device-number","wrong-memory-number","wrong-operator","wrong-command","wrong-client","tampered-issuer","tampered-proof","tampered-payload","foreign-alias","noncanonical","online-valid","online-wrong-unp","online-wrong-server","online-wrong-serial","online-wrong-operator","online-missing-authority","online-wrong-source","offline-caller-unp","online-wrong-hash","online-wrong-operation","online-wrong-receipt","online-altered-lines"}){auto raw=read(d+"/"+mode+".json");go_json::Value command;std::string error;assert(go_json::parse(raw.data(),raw.size(),command,error));idf::VerifiedExecutionAuthorization out;auto rc=idf::verify_execution_authorization(command,b,{pem.c_str(),"root-1",1700000400},out);if(std::string(mode)=="valid"||std::string(mode)=="online-valid"){if(rc!=ESP_OK)std::cerr<<"valid rejected "<<rc<<"\n";assert(rc==ESP_OK);if(std::string(mode)=="online-valid")assert(out.unp_source=="SERVER_ASSIGNED"&&out.assigned_unp=="DT636497-OP01-0000500"&&out.server_sale_id=="server-sale-1");assert(out.operator_code=="OP01"&&out.actor_id=="subject-1"&&out.client_proof_sha256.size()==64);}else{assert(rc!=ESP_OK);assert(out.authorization_id.empty());}std::cout<<mode<<" PASS\n";}
 // Verify the same cryptographically valid grant against live clock/binding changes.
 // Reuse a previously accepted output to prove failure cannot retain authority.
 auto raw=read(d+"/valid.json");go_json::Value command;std::string error;
 assert(go_json::parse(raw.data(),raw.size(),command,error));
 idf::VerifiedExecutionAuthorization out;
 auto verify=[&](int64_t now,const idf::CompositeBinding&binding,const char*key){
  return idf::verify_execution_authorization(command,binding,{pem.c_str(),key,now},out);
 };
 assert(verify(1700000001,b,"root-1")==ESP_OK); // nbf inclusive
 assert(verify(1700000800,b,"root-1")==ESP_OK); // final valid second
 assert(verify(1700000801,b,"root-1")!=ESP_OK); // exp exclusive
 assert(out.authorization_id.empty()&&out.actor_id.empty()&&out.client_proof_sha256.empty());
 assert(verify(1700000000,b,"root-1")!=ESP_OK);
 assert(verify(0,b,"root-1")!=ESP_OK); // no trusted wall-clock
 assert(verify(1700000400,b,"root-1")==ESP_OK);
 auto changed=b;changed.generation++;
 assert(verify(1700000400,changed,"root-1")!=ESP_OK);
 assert(out.authorization_id.empty()&&out.operator_id.empty());
 changed=b;changed.generation--;
 assert(verify(1700000400,changed,"root-1")!=ESP_OK);
 changed=b;changed.authority.unp_range_start++;
 assert(verify(1700000400,changed,"root-1")!=ESP_OK);
 changed=b;changed.authority.unp_range_end--;
 assert(verify(1700000400,changed,"root-1")!=ESP_OK);
 assert(verify(1700000400,b,"retired-root-key")!=ESP_OK);
 assert(out.authorization_id.empty()&&out.authorization_sha256.empty());
 std::cout<<"live clock boundaries / binding generation / lease change / issuer key fencing PASS\n";

}
