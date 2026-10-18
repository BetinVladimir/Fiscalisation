#include "encrypted_provisioning.h"
#include "nvs.h"
#include <fstream>
#include <iterator>
#include <cassert>
#include <iostream>
#include <algorithm>
using namespace beefiscal::idf;
static std::vector<uint8_t>read(const std::string&p){std::ifstream f(p,std::ios::binary);assert(f);return{std::istreambuf_iterator<char>(f),{}};}
int main(int argc,char**argv){assert(argc==2);std::string dir=argv[1];auto packet=read(dir+"/installer.bin"),pem=read(dir+"/issuer.pem"),kid=read(dir+"/issuer.kid"),scalar=read(dir+"/device.scalar");assert(scalar.size()==32);std::copy(scalar.begin(),scalar.end(),factory_scalar.begin());DeviceIdentity identity;assert(identity.open()==ESP_OK);std::string issuer(pem.begin(),pem.end()),issuerkid(kid.begin(),kid.end());ProvisionedBindingStore binding(issuer.c_str(),issuerkid.c_str());EncryptedProvisioning store(identity,binding,issuer.c_str(),issuerkid.c_str());ProvisionedRuntime out;uint8_t nonce[32];memset(nonce,3,32);
 assert(store.load(out)==ESP_ERR_NVS_NOT_FOUND);
 auto wrong=packet;wrong.back()^=1;assert(store.install(wrong.data(),wrong.size(),nonce,out)!=ESP_OK&&out.secrets.client_key_pem.empty()&&nvs_durable.empty());nonce[0]^=1;assert(store.install(packet.data(),packet.size(),nonce,out)!=ESP_OK&&nvs_durable.empty());nonce[0]^=1;
 for(auto name:{"wrong-binding.bin","wrong-secret.bin"}){auto invalid=read(dir+"/"+name);assert(store.install(invalid.data(),invalid.size(),nonce,out)!=ESP_OK&&out.secrets.client_key_pem.empty()&&nvs_durable.empty());}
 nvs_fail_commit=true;assert(store.install(packet.data(),packet.size(),nonce,out)!=ESP_OK&&out.secrets.client_key_pem.empty()&&nvs_durable.empty());nvs_fail_commit=false;
 assert(store.install(packet.data(),packet.size(),nonce,out)==ESP_OK);assert(out.binding.generation==7&&out.binding.schema_version==3&&out.binding.authority.transaction_signing_kid==identity.kid()&&out.wifi_ssid=="test-network"&&out.wifi_password=="test-password"&&out.secrets.fiscal_operator_password=="12345678"&&out.secrets.client_key_pem.find("PRIVATE KEY")!=std::string::npos);
 assert(nvs_durable.size()==1&&nvs_durable.begin()->second==packet);auto hash=out.binding.payload_sha256;out.clear();assert(out.secrets.fiscal_operator_password.empty());assert(store.install(packet.data(),packet.size(),nonce,out)!=ESP_OK);
 // Recreate controller after committed blob / lost installation ACK: no resend
 // or second mutable binding/secrets write is needed to recover exact proof.
 EncryptedProvisioning reload(identity,binding,issuer.c_str(),issuerkid.c_str());assert(reload.load(out)==ESP_OK&&out.binding.payload_sha256==hash&&nvs_durable.size()==1);
 std::string signature;assert(identity.sign_provisioning_install(out.binding.payload_sha256,out.binding.generation,out.package_sha256,signature)==ESP_OK);std::ofstream proof(dir+"/firmware-proof.json");proof<<"{\"state\":\"INSTALLED\",\"device_kid\":\""<<identity.kid()<<"\",\"binding_sha256\":\""<<out.binding.payload_sha256<<"\",\"package_sha256\":\""<<out.package_sha256<<"\",\"generation\":7,\"signature\":\""<<signature<<"\"}";proof.close();
 nvs_durable.begin()->second.back()^=1;assert(reload.load(out)!=ESP_OK&&out.secrets.client_key_pem.empty());
 std::cout<<"Actual issuer -> mbedTLS signed schema3 + encrypted NVS transaction / commit failure / lost-ACK reload / replay / subject and secret rejection PASS\n";
}
