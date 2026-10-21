#include "startup_entropy.h"
#include "device_identity.h"
#include "bootloader_random.h"
#include "nvs.h"
#include <cassert>
#include <iostream>
#include <thread>
#include <vector>
using namespace beefiscal::idf;
int main(){
 DeviceIdentity first;
 assert(with_startup_entropy([&]{assert(test_entropy_enabled);return first.open();})==ESP_OK);
 assert(first.ready()&&test_identity_present&&test_nvs_writes==1&&!test_entropy_enabled);
 assert(test_startup_events.front()=="entropy.enable"&&test_startup_events.back()=="entropy.disable");
 assert(test_hardware_entropy_calls>0);
 const auto scalar=test_identity_scalar;const auto pub=first.public_key_der_base64url();
 const auto entropy_calls=test_hardware_entropy_calls;
 // Signing happens before RF startup too (sync can start before BLE). Crossing
 // the default DRBG reseed interval must never read an inactive hardware source.
 for(int i=0;i<11000;i++){std::string signature;assert(first.sign_hash_hex(std::string(64,'a').c_str(),signature)==ESP_OK&&!signature.empty());}
 assert(test_hardware_entropy_calls==entropy_calls);
 std::vector<std::thread> workers;
 for(int worker=0;worker<4;worker++)workers.emplace_back([&]{for(int n=0;n<25;n++){std::string signature;assert(first.sign_hash_hex(std::string(64,'d').c_str(),signature)==ESP_OK&&!signature.empty());}});
 for(auto& worker:workers)worker.join();
 assert(test_hardware_entropy_calls==entropy_calls);
 std::string proof;assert(first.sign_provisioning_install(std::string(64,'a'),1,std::string(64,'b'),proof)==ESP_OK);
 assert(test_hardware_entropy_calls==entropy_calls&&test_identity_scalar==scalar&&test_nvs_writes==1);
 // Reload also needs random scalar-multiplication blinding under the guard,
 // while preserving the exact enrolled identity rather than regenerating it.
 DeviceIdentity reload;assert(with_startup_entropy([&]{return reload.open();})==ESP_OK);
 assert(reload.public_key_der_base64url()==pub&&test_identity_scalar==scalar&&test_nvs_writes==1);
 test_identity_scalar.fill(0);DeviceIdentity corrupted;const auto writes=test_nvs_writes;
 assert(with_startup_entropy([&]{return corrupted.open();})!=ESP_OK&&!corrupted.ready());
 assert(!test_entropy_enabled&&test_nvs_writes==writes);
 test_identity_scalar=scalar;test_identity_present=false;test_fail_commit=true;DeviceIdentity failed;
 assert(with_startup_entropy([&]{return failed.open();})!=ESP_OK&&!failed.ready());
 assert(!test_entropy_enabled&&test_entropy_enable_count==test_entropy_disable_count);
 test_fail_commit=false;test_identity_present=true;test_identity_scalar=scalar;
 const auto seeded_calls=test_hardware_entropy_calls;test_rf_active=true;const auto before=test_entropy_enable_count;std::string signature;
 assert(first.sign_hash_hex(std::string(64,'c').c_str(),signature)==ESP_OK);
 assert(test_entropy_enable_count==before&&test_hardware_entropy_calls==seeded_calls);
 std::cout<<"Actual mbedTLS startup first-boot/reload + entropy cleanup/corrupt-NVS + non-RF DRBG signing PASS\n";
}
