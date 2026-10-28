#pragma once
#include "bootloader_random.h"
#include <utility>
namespace beefiscal::idf {
// Startup only: call before initializing RF or ADC users. Never toggle the SAR
// entropy source while WiFi/Bluetooth/ADC are active.
template<class Action> auto with_startup_entropy(Action&& action) -> decltype(action()) {
  struct Scope { Scope(){bootloader_random_enable();} ~Scope(){bootloader_random_disable();} } scope;
  return std::forward<Action>(action)();
}
}
