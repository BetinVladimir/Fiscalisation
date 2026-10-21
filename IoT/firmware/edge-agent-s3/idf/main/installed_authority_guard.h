#pragma once
#include <mutex>
namespace beefiscal::idf {
// Configuration installation and purpose-bound identity publication share one
// lock. Recursive because encrypted installation validates/loads legacy state.
inline std::recursive_mutex& installed_authority_mutex() {
  static std::recursive_mutex mutex;
  return mutex;
}
using InstalledAuthorityGuard = std::lock_guard<std::recursive_mutex>;
}
