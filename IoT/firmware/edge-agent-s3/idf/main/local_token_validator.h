#pragma once
#include <string>
#include "esp_err.h"
#include "provisioned_binding.h"

namespace beefiscal::idf {
struct LocalTokenClaims { std::string subject, operator_id, shift_id, jti; int64_t expires_at{}; };
esp_err_t validate_local_token(const char* jwt,const char* required_scope,
  const CompositeBinding&,LocalTokenClaims&);
}
