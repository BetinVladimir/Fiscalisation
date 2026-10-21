// Include the real implementation to inspect its private exact-int64 parser;
// production API has no parser/sign-digest test hooks.
#include "../../main/ble_certificate_proof.cpp"
#include "startup_entropy.h"
#include "nvs.h"
#include <atomic>
#include <cassert>
#include <chrono>
#include <condition_variable>
#include <fstream>
#include <iostream>
#include <iterator>
#include <thread>
using namespace beefiscal::idf;
using V=beefiscal::go_json::Value;
static std::string read(const std::string& path){std::ifstream f(path,std::ios::binary);assert(f);return {std::istreambuf_iterator<char>(f),{}};}
static const V& member(const V& value,const char* key){for(const auto& p:value.fields)if(p.first==key)return p.second;assert(false);return value;}
static V json(const std::string& raw){V out;std::string error;assert(beefiscal::go_json::parse(raw.data(),raw.size(),out,error));return out;}
static BLECertificateChallengeEnvelope envelope(const V& value){return {member(value,"canonical_json").text,member(value,"signature").text,member(value,"key_id").text};}
struct Clock final:TrustedBLECertificateTime {int64_t value{};bool ready=true;bool now(int64_t& out)const override {out=value;return ready;}};
struct System {
 std::string pem,kid;DeviceIdentity identity;ProvisionedBindingStore binding;EncryptedProvisioning encrypted;Clock clock;
 System(const std::string& dir,int64_t now):pem(read(dir+"/issuer.pem")),kid(read(dir+"/issuer.kid")),binding(pem.c_str(),kid.c_str()),encrypted(identity,binding,pem.c_str(),kid.c_str()){
  const auto scalar=read(dir+"/device.scalar");assert(scalar.size()==32);std::copy(scalar.begin(),scalar.end(),factory_scalar.begin());
  assert(with_startup_entropy([&]{return identity.open();})==ESP_OK);assert(binding.open()==ESP_OK);clock.value=now;
 }
 BLEServerCertificateProofSigner signer(){return {identity,encrypted,binding,clock,pem.c_str(),kid.c_str()};}
 void install(const std::string& packet){uint8_t nonce[32];memset(nonce,3,32);ProvisionedRuntime runtime;assert(encrypted.install(reinterpret_cast<const uint8_t*>(packet.data()),packet.size(),nonce,runtime)==ESP_OK);runtime.clear();}
};
static void reset_faults(){proof_read_error=false;proof_shrink_read=false;proof_commit_mode=0;commit_hook={};nvs_fail_commit=false;}
static void clear_cache(){for(auto i=nvs_durable.begin();i!=nvs_durable.end();){if(i->first.rfind("ble-cert-pop/",0)==0)i=nvs_durable.erase(i);else ++i;}reset_faults();}
int main(int argc,char** argv){assert(argc==2);const std::string dir=argv[1];const auto original=envelope(json(read(dir+"/challenge-0.json")));const auto now=std::stoll(read(dir+"/now"));
 System system(dir,now);const auto scalar=factory_scalar;system.install(read(dir+"/installer.bin"));auto signer=system.signer();BLECertificateProofEnvelope proof;
 assert(signer.prove(original,proof)==ESP_OK);const auto expected=proof;assert(proof.signature.size()==86);assert(identity_writes==0&&factory_scalar==scalar);
 assert(signer.prove(original,proof)==ESP_OK&&proof.canonical_json==expected.canonical_json&&proof.signature==expected.signature);
 auto restarted=system.signer();assert(restarted.prove(original,proof)==ESP_OK&&proof.signature==expected.signature);
 DeviceIdentity rebooted;assert(with_startup_entropy([&]{return rebooted.open();})==ESP_OK);assert(rebooted.public_key_der_base64url()==system.identity.public_key_der_base64url());
 EncryptedProvisioning rebooted_config(rebooted,system.binding,system.pem.c_str(),system.kid.c_str());BLEServerCertificateProofSigner coldboot(rebooted,rebooted_config,system.binding,system.clock,system.pem.c_str(),system.kid.c_str());assert(coldboot.prove(original,proof)==ESP_OK&&proof.signature==expected.signature);
 auto installed_snapshot=nvs_durable;nvs_durable.erase("edge-provision/active");assert(signer.prove(original,proof)!=ESP_OK&&proof.signature.empty());nvs_durable=installed_snapshot;
 factory_scalar.fill(0);factory_scalar[31]=3;DeviceIdentity foreign_identity;assert(with_startup_entropy([&]{return foreign_identity.open();})==ESP_OK);EncryptedProvisioning foreign_config(foreign_identity,system.binding,system.pem.c_str(),system.kid.c_str());BLEServerCertificateProofSigner foreign_signer(foreign_identity,foreign_config,system.binding,system.clock,system.pem.c_str(),system.kid.c_str());assert(foreign_signer.prove(original,proof)!=ESP_OK&&proof.signature.empty());factory_scalar=scalar;
 const auto entropy_after_reboots=test_hardware_entropy_calls;test_rf_active=true;
 assert(signer.prove(original,proof)==ESP_OK&&proof.signature==expected.signature);

 std::ofstream output(dir+"/firmware-proof.json");output<<"{\"canonical_json\":";std::string encoded,error;assert(beefiscal::go_json::encode(V::string(proof.canonical_json),encoded,error));output<<encoded<<",\"signature\":\""<<proof.signature<<"\"}";output.close();
 const auto negatives=json(read(dir+"/negative-vectors.json"));for(const auto& v:negatives.fields){const auto commits=proof_commits;proof={"stale","stale"};assert(signer.prove(envelope(v.second),proof)!=ESP_OK&&proof.canonical_json.empty()&&proof.signature.empty()&&proof_commits==commits);}
 // Actual parser preserves exact int64 beyond binary64, independently of the
 // installed legacy decoder's documented INT32 generation cap.
 for(const auto& token: {"9007199254740991","9007199254740992","9007199254740993","9223372036854775807"}){auto raw=original.canonical_json;const auto at=raw.find("\"binding_generation\":7");assert(at!=std::string::npos);raw.replace(at,22,"\"binding_generation\":"+std::string(token));Challenge c;assert(parse_challenge(raw,c)&&c.generation==std::stoll(token));}
 Clock unknown;unknown.ready=false;BLEServerCertificateProofSigner no_time(system.identity,system.encrypted,system.binding,unknown,system.pem.c_str(),system.kid.c_str());assert(no_time.prove(original,proof)!=ESP_OK&&proof.signature.empty());
 BLEServerCertificateProofSigner production_default(system.identity,system.encrypted,system.binding);assert(production_default.prove(original,proof)!=ESP_OK&&proof.signature.empty());
 system.clock.ready=false;assert(signer.prove(original,proof)!=ESP_OK&&proof.signature.empty());system.clock.ready=true;
 system.clock.value=now-1;assert(signer.prove(original,proof)!=ESP_OK);system.clock.value=now+120;assert(signer.prove(original,proof)!=ESP_OK);system.clock.value=now;
 // Fixed cache has no unexpired eviction. Replays remain exact when full.
 for(int i=1;i<4;i++)assert(signer.prove(envelope(json(read(dir+"/challenge-"+std::to_string(i)+".json"))),proof)==ESP_OK);
 const auto commits=proof_commits;assert(signer.prove(envelope(json(read(dir+"/challenge-4.json"))),proof)==ESP_ERR_NO_MEM&&proof.signature.empty()&&proof_commits==commits);
 assert(signer.prove(original,proof)==ESP_OK&&proof.signature==expected.signature);
 // Basic persisted-cache corruption never publishes authority.
 auto durable=nvs_durable;for(size_t offset:{size_t(0),size_t(4+117)}){nvs_durable=durable;nvs_durable["ble-cert-pop/cache-v1"][offset]^=1;auto damaged=system.signer();assert(damaged.prove(original,proof)!=ESP_OK&&proof.signature.empty());}nvs_durable=durable;
 proof_shrink_read=true;auto truncated=system.signer();assert(truncated.prove(original,proof)!=ESP_OK&&proof.signature.empty());proof_shrink_read=false;
 for(bool warm:{false,true})for(int mode:{1,2,3}){clear_cache();auto faulted=system.signer();BLECertificateProofEnvelope acknowledged;const auto target=warm?envelope(json(read(dir+"/challenge-1.json"))):original;if(warm)assert(faulted.prove(original,acknowledged)==ESP_OK);proof_commit_mode=mode;const auto before=proof_commits;assert(faulted.prove(target,proof)!=ESP_OK&&proof.signature.empty());
  proof_read_error=true;assert(faulted.prove(target,proof)!=ESP_OK&&proof.signature.empty()&&proof_commits==before+1);
  reset_faults();assert(faulted.prove(target,proof)!=ESP_OK&&proof.signature.empty());assert(faulted.prove(target,proof)==ESP_OK);
  assert(proof_commits==before+(mode==1?2:1));auto retry=system.signer();BLECertificateProofEnvelope again;assert(retry.prove(target,again)==ESP_OK&&again.signature==proof.signature);if(warm){assert(retry.prove(original,again)==ESP_OK&&again.signature==acknowledged.signature);}
 }
 // Installation and publication share the actual guard, including both current
 // encrypted and legacy stores. Pause proof at real NVS commit to make overlap.
 for(bool encrypted:{true,false}){clear_cache();nvs_durable.erase("edge-provision/active");nvs_durable.erase("edge-binding/active");nvs_durable.erase("edge-binding/generation");system.binding.close();assert(system.binding.open()==ESP_OK);
  if(encrypted)system.install(read(dir+"/installer.bin"));else{const auto b=read(dir+"/binding.json"),sig=read(dir+"/binding.signature");SignedBindingEnvelope e{b.data(),b.size(),sig.c_str(),system.kid.c_str()};assert(system.binding.install(e)==ESP_OK);}
  auto concurrent=system.signer();std::mutex mutex;std::condition_variable cv;bool paused=false,release=false;std::atomic<bool> installer_started{false},installer_done{false};
  commit_hook=[&](const std::string& name){if(name!="ble-cert-pop")return;std::unique_lock<std::mutex> lock(mutex);paused=true;cv.notify_all();cv.wait(lock,[&]{return release;});};
  BLECertificateProofEnvelope concurrent_proof;esp_err_t proof_rc=ESP_FAIL,install_rc=ESP_FAIL;
  std::thread worker([&]{proof_rc=concurrent.prove(original,concurrent_proof);});
  {std::unique_lock<std::mutex> lock(mutex);assert(cv.wait_for(lock,std::chrono::seconds(5),[&]{return paused;}));}
  std::thread installer([&]{installer_started=true;if(encrypted){const auto packet=read(dir+"/installer-next.bin");uint8_t nonce[32];memset(nonce,3,32);ProvisionedRuntime out;install_rc=system.encrypted.install(reinterpret_cast<const uint8_t*>(packet.data()),packet.size(),nonce,out);out.clear();}else{const auto b=read(dir+"/binding-next.json"),sig=read(dir+"/binding-next.signature");install_rc=system.binding.install({b.data(),b.size(),sig.c_str(),system.kid.c_str()});}installer_done=true;});
  while(!installer_started)std::this_thread::yield();
  assert(!installed_authority_mutex().try_lock());assert(!installer_done);
  {std::lock_guard<std::mutex> lock(mutex);release=true;}cv.notify_all();worker.join();installer.join();commit_hook={};
  assert(proof_rc==ESP_OK&&install_rc==ESP_OK&&concurrent_proof.signature.size()==86);
  assert(concurrent.prove(original,proof)!=ESP_OK&&proof.signature.empty());
 }
 assert(identity_writes==0&&factory_scalar==scalar&&test_hardware_entropy_calls==entropy_after_reboots&&!test_entropy_enabled);
 std::cout<<"PASS actual DeviceIdentity/NVS singlehash proof, strict root/context/int64/time/cache/fault/restart and both concurrent installers\n";
}
