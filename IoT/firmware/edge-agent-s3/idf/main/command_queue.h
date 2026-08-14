#pragma once
#include "edge_runtime_port.h"
namespace beefiscal::idf{
class CommandQueue final{
 public:struct Impl;esp_err_t start(CommandSink,void*,size_t depth=8);
  esp_err_t enqueue(const CommandView&);
 private:Impl*impl_{};
};
esp_err_t queued_command_sink(const CommandView&,void*);
}
