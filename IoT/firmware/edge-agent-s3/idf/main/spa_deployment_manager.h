#pragma once
#include <string>
#include "esp_err.h"
#include "provisioned_binding.h"

namespace beefiscal::idf {
struct DeploymentState { std::string application_id{"com.beeloy.miniposweb"},version{"none"},build_id{"none"},state{"FAILED"},error_code; };
class SpaDeploymentManager final {
 public:
  explicit SpaDeploymentManager(const CompositeBinding&,const char* root="/sdcard/beeloy/spa");
  esp_err_t check_and_activate();
  DeploymentState state()const;
  std::string active_root()const;
 private:
  const CompositeBinding& binding_;std::string root_;DeploymentState state_;
  esp_err_t load_state();esp_err_t persist_state(const char* slot);
};
}
