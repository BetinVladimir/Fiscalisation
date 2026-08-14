#pragma once
#include "command_queue.h"
#include "durable_storage.h"
#include "provisioned_binding.h"
#include "spa_deployment_manager.h"

namespace beefiscal::idf {
struct LocalHttpRuntime {
  const CompositeBinding* binding{};
  DurableStorage* storage{};
  CommandQueue* queue{};
  bool (*fiscal_ready)(void*){};
  bool (*payment_ready)(void*){};
  void* readiness_context{};
  const char* spa_root{"/sdcard/beeloy/spa/active"};
  SpaDeploymentManager* deployments{};
};
esp_err_t local_http_server_start(const LocalHttpRuntime&);
}
