#pragma once
#include "edge_runtime_port.h"
namespace beefiscal::idf{
class CommandQueue final{
 public:struct Impl;esp_err_t start(CommandSink,void*,size_t depth=8);
  esp_err_t enqueue(const CommandView&);
  esp_err_t enqueue_and_wait(const CommandView&,uint32_t timeout_ms=120000);
 private:Impl*impl_{};
};
esp_err_t queued_command_sink(const CommandView&,void*);
}
