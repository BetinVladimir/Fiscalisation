// White-box boundary fixture only: expose the existing private pointer in this
// translation unit, without adding a production setter or changing shipped code.
#include <cstddef>
#include <cstdint>
#include <string>
#define private public
#include "device_identity.h"
#undef private
#include "device_identity.cpp"
#include "startup_entropy.h"
#include "bootloader_random.h"
#include "nvs.h"
#include <cassert>
#include <iostream>
using namespace beefiscal::idf;
int main(){
 DeviceIdentity identity;assert(with_startup_entropy([&]{return identity.open();})==ESP_OK);
 const auto durable=test_identity_scalar;const auto pub=identity.public_key_der_base64url();const auto writes=test_nvs_writes;const auto hardware=test_hardware_entropy_calls;
 auto* rng=identity.impl_;rng->random_requests=DeviceIdentity::Impl::maximum_random_requests-1;
 unsigned char bytes[32]{};assert(DeviceIdentity::Impl::random(rng,bytes,sizeof(bytes))==0);
 assert(rng->random_requests==DeviceIdentity::Impl::maximum_random_requests);
 assert(DeviceIdentity::Impl::random(rng,bytes,sizeof(bytes))==MBEDTLS_ERR_CTR_DRBG_ENTROPY_SOURCE_FAILED);
 std::string signature="stale";assert(identity.sign_hash_hex(std::string(64,'a').c_str(),signature)!=ESP_OK&&signature.empty());
 std::string proof="stale";assert(identity.sign_provisioning_install(std::string(64,'a'),1,std::string(64,'b'),proof)!=ESP_OK&&proof.empty());
 assert(identity.open()==ESP_OK);assert(rng->random_requests==DeviceIdentity::Impl::maximum_random_requests);
 assert(test_identity_scalar==durable&&test_nvs_writes==writes&&test_hardware_entropy_calls==hardware&&identity.public_key_der_base64url()==pub);
 DeviceIdentity rebooted;assert(with_startup_entropy([&]{return rebooted.open();})==ESP_OK);
 assert(rebooted.public_key_der_base64url()==pub&&test_identity_scalar==durable&&test_nvs_writes==writes);
 assert(rebooted.sign_hash_hex(std::string(64,'a').c_str(),signature)==ESP_OK&&!signature.empty());
 std::cout<<"Actual DRBG callback final allowed request/exhaustion + public sign fail-closed/no reseed/no rekey + fresh-boot recovery PASS\n";
}
