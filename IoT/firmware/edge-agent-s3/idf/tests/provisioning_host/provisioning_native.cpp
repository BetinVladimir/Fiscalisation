#include "provisioning_crypto.h"
#include "provisioning_frames.h"
#include "device_identity.h"
#include "nvs.h"
#include <fstream>
#include <iterator>
#include <cassert>
#include <iostream>
#include <algorithm>
using namespace beefiscal::idf;
static std::vector<uint8_t>read(const std::string&p){std::ifstream f(p,std::ios::binary);assert(f);return{std::istreambuf_iterator<char>(f),{}};}
static void hex(const std::vector<uint8_t>&s,uint8_t*out){assert(s.size()==64);for(size_t i=0;i<32;i++)out[i]=static_cast<uint8_t>(std::stoi(std::string(s.begin()+i*2,s.begin()+i*2+2),nullptr,16));}
struct Recipient{DeviceIdentity identity;int calls{};static esp_err_t derive(void*ctx,const uint8_t peer[65],uint8_t shared[32]){auto*r=static_cast<Recipient*>(ctx);r->calls++;return r->identity.provisioning_shared_secret(peer,shared);}};
static void number(uint8_t*p,uint32_t n){for(int i=3;i>=0;i--){p[i]=n&255;n>>=8;}}
static std::vector<std::vector<uint8_t>>frames(const std::vector<uint8_t>&p){uint8_t d[32];assert(!mbedtls_sha256(p.data(),p.size(),d,0));std::vector<std::vector<uint8_t>>out;for(size_t offset=0;offset<p.size();offset+=124){const auto body=std::min(size_t(124),p.size()-offset);std::vector<uint8_t>f(61+body);memcpy(f.data(),"BFPF",4);memset(f.data()+4,7,16);number(f.data()+20,p.size());number(f.data()+24,offset);memcpy(f.data()+28,d,32);f[60]=(offset+body==p.size());memcpy(f.data()+61,p.data()+offset,body);out.push_back(f);}return out;}
int main(int argc,char**argv){assert(argc==2);const std::string dir=argv[1];auto packet=read(dir+"/package.bin"),plain=read(dir+"/plaintext.bin"),pem=read(dir+"/issuer.pem"),scalar=read(dir+"/device.scalar");assert(scalar.size()==32);std::copy(scalar.begin(),scalar.end(),factory_scalar.begin());Recipient device;assert(device.identity.open()==ESP_OK);
 ProvisioningSubject expected;expected.device_kid="device-1";expected.issuer_kid="issuer-1";expected.generation_floor=6;hex(read(dir+"/nonce.hex"),expected.setup_nonce);uint8_t binding[32];hex(read(dir+"/binding.hex"),binding);std::string issuer(pem.begin(),pem.end());OpenedProvisioning opened;
 assert(open_provisioning_package(packet.data(),packet.size(),issuer.c_str(),expected,Recipient::derive,&device,opened)==ESP_OK);assert(opened.plaintext==plain&&opened.generation==7&&!memcmp(opened.binding_sha256,binding,32)&&device.calls==1);
 assert(std::search(packet.begin(),packet.end(),plain.begin(),plain.begin()+20)==packet.end());
 auto reject=[&](const std::vector<uint8_t>&p,const ProvisioningSubject&s){const auto before=device.calls;assert(open_provisioning_package(p.data(),p.size(),issuer.c_str(),s,Recipient::derive,&device,opened)!=ESP_OK);assert(opened.plaintext.empty()&&opened.generation==0&&device.calls==before);};
 auto wrong=expected;wrong.device_kid="foreign";reject(packet,wrong);wrong=expected;wrong.issuer_kid="foreign";reject(packet,wrong);wrong=expected;wrong.generation_floor=7;reject(packet,wrong);wrong=expected;wrong.setup_nonce[0]^=1;reject(packet,wrong);
 for(size_t i:{size_t(0),size_t(44),size_t(76),size_t(141),size_t(159),packet.size()-65,packet.size()-1}){auto bad=packet;bad[i]^=1;reject(bad,expected);}auto extra=packet;extra.push_back(0);reject(extra,expected);auto short_packet=packet;short_packet.resize(180);reject(short_packet,expected);
 // Correct issuer signature but modified ciphertext proves actual GCM tag check.
 auto bad_tag=read(dir+"/signed-bad-tag.bin");const auto calls=device.calls;assert(open_provisioning_package(bad_tag.data(),bad_tag.size(),issuer.c_str(),expected,Recipient::derive,&device,opened)!=ESP_OK&&device.calls==calls+1&&opened.plaintext.empty());
 // A valid signed package for another private key never decrypts.
 factory_scalar.fill(0);factory_scalar[31]=3;Recipient foreign;assert(foreign.identity.open()==ESP_OK);assert(open_provisioning_package(packet.data(),packet.size(),issuer.c_str(),expected,Recipient::derive,&foreign,opened)!=ESP_OK&&opened.plaintext.empty());
 // Corrupt stored private scalars must fail closed without replacing identity.
 const auto durable_before=nvs_durable;
 for(unsigned char invalid:{static_cast<unsigned char>(0),static_cast<unsigned char>(255)}){
  factory_scalar.fill(invalid);DeviceIdentity corrupted;
  assert(corrupted.open()!=ESP_OK&&!corrupted.ready());
  std::string rejected_signature="stale";
  assert(corrupted.sign_hash_hex(std::string(64,'a').c_str(),rejected_signature)!=ESP_OK&&rejected_signature.empty());
  assert(nvs_durable==durable_before&&nvs_pending.empty());
 }
 std::copy(scalar.begin(),scalar.end(),factory_scalar.begin());
 DeviceIdentity reloaded;assert(reloaded.open()==ESP_OK);
 assert(reloaded.public_key_der_base64url()==device.identity.public_key_der_base64url());
 uint8_t bad_point[65]{},shared[32];bad_point[0]=4;assert(device.identity.provisioning_shared_secret(bad_point,shared)!=ESP_OK);
 auto chunks=frames(packet);assert(chunks.size()>2);ProvisioningFrames assembly;std::vector<uint8_t>completed;uint64_t tick=100;
 for(const auto&f:chunks){assert(assembly.accept(f.data(),f.size(),++tick,completed)==ESP_OK);}assert(completed==packet);
 assert(assembly.accept(chunks[1].data(),chunks[1].size(),++tick,completed)!=ESP_OK);
 assert(assembly.accept(chunks[0].data(),chunks[0].size(),++tick,completed)==ESP_OK);assert(assembly.accept(chunks[0].data(),chunks[0].size(),++tick,completed)!=ESP_OK);
 assert(assembly.accept(chunks[0].data(),chunks[0].size(),++tick,completed)==ESP_OK);assembly.reset();assert(assembly.accept(chunks[1].data(),chunks[1].size(),++tick,completed)!=ESP_OK);
 assert(assembly.accept(chunks[0].data(),chunks[0].size(),++tick,completed)==ESP_OK);assert(assembly.accept(chunks[1].data(),chunks[1].size(),tick+10001,completed)!=ESP_OK);
 auto altered=chunks;altered.back().back()^=1;for(size_t i=0;i<altered.size();i++){auto status=assembly.accept(altered[i].data(),altered[i].size(),++tick,completed);assert(status==(i+1==altered.size()?ESP_ERR_INVALID_CRC:ESP_OK));}assert(completed.empty());
 auto early=chunks[0];early[60]=1;assert(assembly.accept(early.data(),early.size(),++tick,completed)!=ESP_OK);
 std::cout<<"Go issuer -> actual mbedTLS/device identity decrypt; subject/signature/GCM/device/replay guards + ordered framed transfer PASS\n";
}
