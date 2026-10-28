#pragma once
#include <mutex>
#include <new>
namespace beefiscal::idf {
// Configuration installation and purpose-bound identity publication share one
// lock. Recursive because encrypted installation validates/loads legacy state.
inline std::recursive_mutex& installed_authority_mutex() {
  // Firmware authority has boot lifetime. Host/global Identity and binding
  // destructors may run after first-use statics would normally be destroyed.
  // Fixed in-place storage intentionally never destroys/frees this mutex.
  alignas(std::recursive_mutex) static unsigned char storage[sizeof(std::recursive_mutex)];
  static auto* mutex=new (storage) std::recursive_mutex;
  return *mutex;
}
using InstalledAuthorityGuard = std::lock_guard<std::recursive_mutex>;
}
