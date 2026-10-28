#pragma once
#include "go_json.h"
#include "provisioned_binding.h"
#include <cstdint>
#include <string>
namespace beefiscal::idf {
struct VerifiedExecutionAuthorization {
 std::string authorization_id,authorization_sha256,client_proof_sha256,
 workstation_session_id,actor_id,app_instance_id,operator_id,operator_code,unp_source,server_sale_id,assigned_unp;
};
struct ExecutionAuthorizationTrust {
 const char* public_key_pem{};
 const char* key_id{};
 int64_t now{};
};
// Production overload reads the compiled binding issuer trust anchor and clock.
esp_err_t verify_execution_authorization(const go_json::Value& command,
 const CompositeBinding&,VerifiedExecutionAuthorization&);
// Explicit trust/clock overload for deterministic actual-crypto tests.
esp_err_t verify_execution_authorization(const go_json::Value& command,
 const CompositeBinding&,const ExecutionAuthorizationTrust&,
 VerifiedExecutionAuthorization&);
}
