#if defined(BEEFISCAL_TIME_ANCHOR_HOST_TEST)
#error actual crypto profile must not enable synthetic core permit factory
#endif
#include "../../main/device_time_anchor.cpp"
#include "../../main/device_identity.cpp"
#include "startup_entropy.h"
#include "nvs.h"
#include <cassert>
#include <fstream>
#include <iostream>
#include <iterator>
#include <type_traits>
#include <thread>
#include <condition_variable>
#include <atomic>
using namespace beefiscal::idf;namespace go_json=beefiscal::go_json;
namespace beefiscal::idf {class DeviceTimeAnchorCryptoTestFaults {public:static void exhaust(DeviceIdentity& i){std::lock_guard<std::mutex> lock(i.impl_->mutex);i.impl_->random_requests=DeviceIdentity::Impl::maximum_random_requests;}};}
// Constructed before first installed guard use; destructor exercises real static exit.
static DeviceIdentity static_exit_identity;
static std::string read(const std::string& p){std::ifstream f(p,std::ios::binary);assert(f);return {std::istreambuf_iterator<char>(f),{}};}
struct Clock final:TimeAnchorMonotonicClock{int64_t value{};bool read_us(int64_t& out)const override{out=value;return true;}};
struct Nonces final:TimeAnchorNonceSource{uint64_t n{};bool fill(TimeNonce& out)override{out={};auto v=++n;for(size_t i=0;i<8;++i)out[i]=uint8_t(v>>(i*8));return true;}};
struct System{std::string pem,kid;DeviceIdentity identity;ProvisionedBindingStore binding;EncryptedProvisioning encrypted;Clock clock;Nonces nonces;TimeAnchorCore core;DeviceTimeAnchorCoordinator coordinator;DeviceTimeAnchorRequestSigner signer;DeviceTimeAnchorVerifier verifier;
 System(const std::string& dir):pem(read(dir+"/issuer.pem")),kid(read(dir+"/issuer.kid")),binding(pem.c_str(),kid.c_str()),encrypted(identity,binding,pem.c_str(),kid.c_str()),core(clock,nonces),coordinator(identity,encrypted,binding,core,pem.c_str(),kid.c_str()),signer(coordinator),verifier(coordinator){auto scalar=read(dir+"/device.scalar");assert(scalar.size()==32);std::copy(scalar.begin(),scalar.end(),factory_scalar.begin());assert(with_startup_entropy([&]{return identity.open();})==ESP_OK);assert(binding.open()==ESP_OK);const auto packet=read(dir+"/installer.bin");uint8_t nonce[32];memset(nonce,3,32);ProvisionedRuntime out;assert(encrypted.install(reinterpret_cast<const uint8_t*>(packet.data()),packet.size(),nonce,out)==ESP_OK);assert(coordinator.trusted_start_boot()==ESP_OK);}
};
int main(int argc,char** argv){assert(argc==3);const std::string dir=argv[1],mode=argv[2];System s(dir);TimeAnchorEnvelope a,b;ExpectedTimeAnchorExchange ea,eb;
 assert(s.signer.begin(a,ea)==ESP_OK); // Returning prepared bytes is NOT publication/admission.
 assert(s.signer.begin(b,eb)==ESP_OK); // Replacement after return makes a stale.
 if(mode=="request"){TimeAnchorEnvelope c,d;ExpectedTimeAnchorExchange ec,ed;std::thread left([&]{assert(s.signer.begin(c,ec)==ESP_OK);}),right([&]{assert(s.signer.begin(d,ed)==ESP_OK);});left.join();right.join();for(const auto& item:{std::make_pair("a",a),std::make_pair("b",b),std::make_pair("c",c),std::make_pair("d",d)}){std::string raw;assert(encode_time_anchor_envelope(item.second,raw));std::ofstream(dir+"/request-"+item.first+".json")<<raw;}std::cout<<"PASS actual DeviceIdentity purpose-bound request singlehash signing\n";return 0;}
 assert(mode=="verify"||mode.rfind("life-",0)==0);go_json::Value responses;std::string error;const auto raw=read(dir+"/responses.json");assert(go_json::parse(raw.data(),raw.size(),responses,error));TimeAnchorEnvelope response_a,response_b,high;std::vector<TimeAnchorEnvelope> concurrent_responses;
 for(auto& item:responses.fields){std::string envelope_json;assert(go_json::encode(item.second,envelope_json,error));TimeAnchorEnvelope e;assert(parse_time_anchor_envelope(envelope_json,e));if(item.first=="a")response_a=e;else if(item.first=="b")response_b=e;else if(item.first=="b-highS")high=e;else if(item.first=="c"||item.first=="d")concurrent_responses.push_back(e);else if(item.first.rfind("parser-wide-",0)==0){assert(verify_root(e,s.pem.c_str(),s.kid.c_str()));Value body;assert(go_json::parse(e.canonical_json.data(),e.canonical_json.size(),body,error));TimeAnchorRequest r;assert(text(body,"device_id",r.scope.device_id)&&text(body,"device_kid",r.scope.device_kid)&&text(body,"device_spki_sha256",r.scope.device_spki_sha256)&&text(body,"tenant_id",r.scope.tenant_id)&&text(body,"location_id",r.scope.location_id)&&text(body,"register_id",r.scope.register_id)&&text(body,"binding_sha256",r.scope.binding_sha256)&&integer(body,"binding_generation",r.scope.binding_generation)&&integer(body,"request_sequence",r.sequence));std::string boot,nonce,digest;assert(text(body,"boot_nonce",boot)&&text(body,"request_nonce",nonce)&&text(body,"request_sha256",digest));std::vector<uint8_t> decoded;assert(decode64(boot,32,decoded)&&decoded.size()==32);std::copy(decoded.begin(),decoded.end(),r.boot_nonce.begin());assert(decode64(nonce,32,decoded)&&decoded.size()==32);std::copy(decoded.begin(),decoded.end(),r.request_nonce.begin());TimeAnchorClaims claims;assert(response(e.canonical_json,r,digest,s.kid,claims));assert(claims.sequence==r.sequence&&claims.scope.binding_generation==r.scope.binding_generation&&claims.server_unix_ms==r.sequence);}else{assert(item.first.rfind("bad-",0)==0);assert(s.verifier.verify_and_accept(eb,e)==TimeAnchorError::InvalidSample);}}
 if(mode.rfind("life-",0)==0){
  const auto packet=read(dir+"/installer-next.bin"),legacy=read(dir+"/binding-next.json"),sig=read(dir+"/binding-next.signature");
  uint8_t nonce[32];memset(nonce,3,32);ProvisionedRuntime out;TimeAnchorBounds observed;
  const bool pending=mode=="life-legacy-aba"||mode=="life-commit-race"||mode=="life-invalid"||mode=="life-identity-aba"||mode=="life-verified-aba"||mode=="life-owner-aba";
  if(!pending)assert(s.verifier.verify_and_accept(eb,response_b)==TimeAnchorError::Ok);
  if(mode=="life-invalid"){
   auto bad=packet;bad[0]^=1;assert(s.encrypted.install(reinterpret_cast<const uint8_t*>(bad.data()),bad.size(),nonce,out)!=ESP_OK);
   assert(s.verifier.verify_and_accept(eb,response_b)==TimeAnchorError::Ok);
  }else if(mode=="life-legacy-aba"||mode=="life-legacy-anchor"){
   s.binding.close();assert(s.core.bounds(observed)==TimeAnchorError::Unavailable);assert(s.binding.open()==ESP_OK);
   assert(s.verifier.verify_and_accept(eb,response_b)==TimeAnchorError::StaleTicket);
   assert(s.coordinator.trusted_start_boot()==ESP_OK);TimeAnchorEnvelope fresh;ExpectedTimeAnchorExchange owner;assert(s.signer.begin(fresh,owner)==ESP_OK&&fresh.canonical_json!=b.canonical_json);
  }else if(mode=="life-identity-open"||mode=="life-identity-aba"){
   {DeviceIdentity reopened;assert(with_startup_entropy([&]{return reopened.open();})==ESP_OK);assert(s.core.bounds(observed)==TimeAnchorError::Unavailable);assert(s.coordinator.trusted_start_boot()==ESP_OK);}
   assert(s.core.bounds(observed)==TimeAnchorError::Unavailable);assert(s.verifier.verify_and_accept(eb,response_b)==TimeAnchorError::StaleTicket);
  }else if(mode=="life-identity-commit"||mode=="life-identity-readback"){
   lifecycle_generate_identity=true;nvs_durable.erase("edge-identity/p256");nvs_fail_commit=mode=="life-identity-commit";
   if(mode=="life-identity-readback")commit_hook=[](const std::string& name){if(name=="edge-identity")lifecycle_read_error=name;};
   {DeviceIdentity replacement;assert(with_startup_entropy([&]{return replacement.open();})!=ESP_OK);assert(!replacement.ready());}
   nvs_fail_commit=false;lifecycle_generate_identity=false;lifecycle_read_error.clear();commit_hook={};assert(s.core.bounds(observed)==TimeAnchorError::Unavailable);assert(s.coordinator.trusted_start_boot()==ESP_ERR_INVALID_STATE);
  }else if(mode=="life-static-exit"){
   assert(with_startup_entropy([]{return static_exit_identity.open();})==ESP_OK);
   assert(s.core.bounds(observed)==TimeAnchorError::Unavailable);
  }else if(mode=="life-owner-aba"){
   Clock clock;Nonces nonces;TimeAnchorCore core(clock,nonces);
   typename std::aligned_storage<sizeof(DeviceTimeAnchorCoordinator),alignof(DeviceTimeAnchorCoordinator)>::type memory;
   auto* old=new (&memory) DeviceTimeAnchorCoordinator(s.identity,s.encrypted,s.binding,core,s.pem.c_str(),s.kid.c_str());
   assert(old->trusted_start_boot()==ESP_OK);TimeAnchorEnvelope packet;ExpectedTimeAnchorExchange owner;DeviceTimeAnchorRequestSigner original(*old);assert(original.begin(packet,owner)==ESP_OK&&packet.canonical_json==a.canonical_json);
   old->~DeviceTimeAnchorCoordinator();assert(core.bounds(observed)==TimeAnchorError::Unavailable);
   auto* replacement=new (&memory) DeviceTimeAnchorCoordinator(s.identity,s.encrypted,s.binding,core,s.pem.c_str(),s.kid.c_str());DeviceTimeAnchorVerifier current(*replacement);assert(current.verify_and_accept(owner,response_a)==TimeAnchorError::StaleTicket);replacement->~DeviceTimeAnchorCoordinator();
  }else if(mode=="life-registry-race"){
   std::atomic<bool> go{false};std::thread nodes([&]{while(!go.load())std::this_thread::yield();for(int i=0;i<1000;++i){Clock clock;Nonces nonces;TimeAnchorCore core(clock,nonces);DeviceTimeAnchorCoordinator c(s.identity,s.encrypted,s.binding,core,s.pem.c_str(),s.kid.c_str());}});
   std::thread changes([&]{while(!go.load())std::this_thread::yield();for(int i=0;i<1000;++i){s.binding.close();assert(s.binding.open()==ESP_OK);}});go=true;nodes.join();changes.join();assert(s.core.bounds(observed)==TimeAnchorError::Unavailable);
  }else if(mode=="life-verified-aba"){
   std::mutex gate;std::condition_variable cv;bool verified=false,release=false;TimeAnchorError result=TimeAnchorError::Ok;
   time_anchor_verified_test_barrier=[&]{std::unique_lock<std::mutex> lock(gate);verified=true;cv.notify_all();cv.wait(lock,[&]{return release;});};
   std::thread completion([&]{result=s.verifier.verify_and_accept(eb,response_b);});
   {std::unique_lock<std::mutex> lock(gate);cv.wait(lock,[&]{return verified;});}
   s.binding.close();assert(s.binding.open()==ESP_OK);assert(s.coordinator.trusted_start_boot()==ESP_OK);
   {std::lock_guard<std::mutex> lock(gate);release=true;}cv.notify_all();completion.join();time_anchor_verified_test_barrier={};assert(result==TimeAnchorError::StaleTicket);
  }else if(mode=="life-commit-race"){
   std::mutex gate;std::condition_variable cv;bool entered=false,release=false;esp_err_t installed=ESP_FAIL;TimeAnchorError result=TimeAnchorError::Ok;
   commit_hook=[&](const std::string& name){if(name!="edge-provision")return;assert(s.core.bounds(observed)==TimeAnchorError::Unavailable);std::unique_lock<std::mutex> lock(gate);entered=true;cv.notify_all();cv.wait(lock,[&]{return release;});};
   std::thread mutation([&]{installed=s.encrypted.install(reinterpret_cast<const uint8_t*>(packet.data()),packet.size(),nonce,out);});
   {std::unique_lock<std::mutex> lock(gate);cv.wait(lock,[&]{return entered;});}
   std::thread completion([&]{result=s.verifier.verify_and_accept(eb,response_b);});
   {std::lock_guard<std::mutex> lock(gate);release=true;}cv.notify_all();mutation.join();completion.join();commit_hook={};assert(installed==ESP_OK&&result==TimeAnchorError::StaleTicket);
  }else{
   const bool legacy_mode=mode=="life-legacy-commit"||mode=="life-legacy-readback";
   const std::string space=legacy_mode?"edge-binding":"edge-provision";
   if(mode=="life-write")lifecycle_write_error=space;
   if(mode=="life-commit"||mode=="life-legacy-commit")nvs_fail_commit=true;
   if(mode=="life-readback"||mode=="life-legacy-readback")commit_hook=[&](const std::string& name){if(name==space)lifecycle_read_error=space;};
   const auto result=legacy_mode?s.binding.install({legacy.data(),legacy.size(),sig.c_str(),s.kid.c_str()}):s.encrypted.install(reinterpret_cast<const uint8_t*>(packet.data()),packet.size(),nonce,out);
   assert(s.core.bounds(observed)==TimeAnchorError::Unavailable);
   if(mode=="life-success")assert(result==ESP_OK);else assert(result!=ESP_OK);
   nvs_fail_commit=false;lifecycle_read_error.clear();lifecycle_write_error.clear();commit_hook={};
   if(mode=="life-success")assert(s.coordinator.trusted_start_boot()==ESP_OK);
   else {assert(s.coordinator.trusted_start_boot()==ESP_ERR_INVALID_STATE);s.binding.close();assert(s.binding.open()==ESP_OK);assert(s.coordinator.trusted_binding_changed()==ESP_ERR_INVALID_STATE);TimeAnchorEnvelope denied;ExpectedTimeAnchorExchange owner;assert(s.signer.begin(denied,owner)==ESP_ERR_INVALID_STATE&&denied.signature.empty());}
  }
  std::cout<<"PASS actual installed lifecycle "<<mode<<"\n";return 0;
 }
 // Outer framing rejects unknown/duplicate keys, NUL, invalid UTF8/surrogates.
 for(const auto& bad:{std::string("{\"algorithm\":\"x\",\"algorithm\":\"x\"}"),std::string("{\"x\":1}"),std::string("\"\\ud800\""),std::string("\"\\u0000\""),std::string(1,char(0xff))}){TimeAnchorEnvelope rejected;assert(!parse_time_anchor_envelope(bad,rejected));}
 TimeAnchorBounds bounds;assert(s.core.bounds(bounds)==TimeAnchorError::Unavailable);
 s.coordinator.retire_prepared(ea);assert(s.verifier.verify_and_accept(ea,response_a)==TimeAnchorError::StaleTicket); // Actual root response cannot adopt old returned packet.
 assert(s.verifier.verify_and_accept(eb,response_b)==TimeAnchorError::Ok);assert(s.core.bounds(bounds)==TimeAnchorError::Ok);
 assert(s.verifier.verify_and_accept(eb,high)==TimeAnchorError::StaleTicket); // High-S root DER passed crypto before stale check.
 // Simultaneous requests serialize actual identity signing without holding core mutex.
 TimeAnchorEnvelope c,d;ExpectedTimeAnchorExchange ec,ed;std::thread left([&]{assert(s.signer.begin(c,ec)==ESP_OK);}),right([&]{assert(s.signer.begin(d,ed)==ESP_OK);});left.join();right.join();int adopted=0,stale=0;for(const auto& e:concurrent_responses){Value body;assert(go_json::parse(e.canonical_json.data(),e.canonical_json.size(),body,error));std::string digest;assert(text(body,"request_sha256",digest));const auto& owner=digest==hash(std::string(kRequestDomain)+c.canonical_json)?ec:ed;const auto result=s.verifier.verify_and_accept(owner,e);if(result==TimeAnchorError::Ok)++adopted;else{assert(result==TimeAnchorError::StaleTicket);++stale;}}assert(adopted==1&&stale==1);
 auto malformed=response_b;malformed.signature+="=";assert(s.verifier.verify_and_accept(eb,malformed)==TimeAnchorError::InvalidSample);std::vector<uint8_t> der;assert(decode64(response_b.signature,72,der));der.push_back(0);malformed=response_b;malformed.signature=encode64(der.data(),der.size());assert(s.verifier.verify_and_accept(eb,malformed)==TimeAnchorError::InvalidSample);
 s.clock.value=30000000;assert(s.verifier.verify_and_accept(eb,response_b)==TimeAnchorError::StaleTicket);assert(s.core.bounds(bounds)==TimeAnchorError::Expired);
 assert(s.coordinator.trusted_binding_changed()==ESP_OK);assert(s.verifier.verify_and_accept(eb,response_b)==TimeAnchorError::StaleTicket);assert(s.coordinator.trusted_start_boot()==ESP_OK);assert(s.verifier.verify_and_accept(eb,response_b)==TimeAnchorError::StaleTicket);
 // Verification/body delay is included before core m1; raw5s also exceeds
 // conservative real5s budget once drift is accounted for.
 Clock late_clock;Nonces late_nonces;TimeAnchorCore late_core(late_clock,late_nonces);DeviceTimeAnchorCoordinator late_coordinator(s.identity,s.encrypted,s.binding,late_core,s.pem.c_str(),s.kid.c_str());DeviceTimeAnchorRequestSigner late_signer(late_coordinator);DeviceTimeAnchorVerifier late_verifier(late_coordinator);assert(late_coordinator.trusted_start_boot()==ESP_OK);assert(late_verifier.verify_and_accept(ea,response_a)==TimeAnchorError::InvalidSample);TimeAnchorEnvelope late_packet;ExpectedTimeAnchorExchange late_expected;assert(late_signer.begin(late_packet,late_expected)==ESP_OK&&late_packet.canonical_json==a.canonical_json);late_clock.value=5000000;assert(late_verifier.verify_and_accept(late_expected,response_a)==TimeAnchorError::RoundTripExceeded);assert(late_core.bounds(bounds)==TimeAnchorError::Unavailable);assert(late_signer.begin(late_packet,late_expected)==ESP_OK&&late_packet.canonical_json==b.canonical_json);late_coordinator.retire_prepared(late_expected);assert(late_verifier.verify_and_accept(late_expected,response_b)==TimeAnchorError::StaleTicket);
 // Signer compares actual installed scope and never auto-rebinds a core.
 CompositeBinding installed;assert(s.encrypted.load_binding(installed)==ESP_OK);std::vector<uint8_t> spki;assert(decode64(s.identity.public_key_der_base64url(),160,spki));TimeAnchorScope foreign_scope{installed.edge_device_id,s.identity.kid(),hash(std::string(reinterpret_cast<char*>(spki.data()),spki.size())),installed.tenant_id,installed.location_id,"foreign-register",installed.payload_sha256,installed.generation};assert(s.core.start_boot(foreign_scope)==TimeAnchorError::Ok);TimeAnchorEnvelope wrong_scope;ExpectedTimeAnchorExchange wrong_expected;assert(s.signer.begin(wrong_scope,wrong_expected)!=ESP_OK&&wrong_scope.signature.empty());assert(s.signer.begin(wrong_scope,wrong_expected)!=ESP_OK);assert(s.coordinator.trusted_start_boot()==ESP_OK);
 DeviceTimeAnchorCoordinator unavailable(s.identity,s.encrypted,s.binding,s.core);DeviceTimeAnchorRequestSigner no_root(unavailable);TimeAnchorEnvelope empty;ExpectedTimeAnchorExchange missing;assert(no_root.begin(empty,missing)!=ESP_OK&&empty.signature.empty());
 // Actual mbedTLS ECDSA failure through the real identity DRBG budget.
 nvs_durable.erase("edge-provision/active");const auto legacy=read(dir+"/binding.json"),legacy_sig=read(dir+"/binding.signature");assert(s.binding.install({legacy.data(),legacy.size(),legacy_sig.c_str(),s.kid.c_str()})==ESP_OK);assert(s.coordinator.trusted_start_boot()==ESP_OK);DeviceTimeAnchorCryptoTestFaults::exhaust(s.identity);assert(s.signer.begin(empty,missing)==ESP_FAIL&&empty.signature.empty()&&empty.canonical_json.empty());
 // No signing auto-boot: a healthy installed device with invalidated core stays unavailable.
 s.coordinator.trusted_invalidate();assert(s.signer.begin(empty,missing)!=ESP_OK);assert(s.core.bounds(bounds)==TimeAnchorError::Unavailable);
 static_assert(!std::is_constructible<VerifiedTimeAnchorPermit,const TimeAnchorRequest&,const TimeAnchorClaims&>::value);
 std::cout<<"PASS actual root DER low/highS, signed hostile context/integer vectors, unsigned preservation, returned-packet replacement, replay/expiry/boot/binding, default-no-root/no-autoboot\n";
}
