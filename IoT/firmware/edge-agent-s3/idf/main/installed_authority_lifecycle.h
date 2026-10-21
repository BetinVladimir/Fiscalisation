#pragma once
#include "installed_authority_guard.h"
#include <cstdint>
#include <cstddef>
#include <limits>
namespace beefiscal::idf {
class DeviceTimeAnchorCoordinator;
class DeviceIdentity;
class ProvisionedBindingStore;
class EncryptedProvisioning;
class InstalledAuthorityMutation;
// Internal synchronous registry. Every access holds the installed guard. It
// owns no callbacks supplied by transports and never outlives a registered node.
class InstalledAuthorityRegistration final {
 private:
  friend class DeviceTimeAnchorCoordinator;
  friend class DeviceTimeAnchorRequestSigner;friend class DeviceTimeAnchorVerifier;
  friend class InstalledAuthorityMutation;
  struct State { InstalledAuthorityRegistration* head{};int64_t epoch{};size_t mutations{};bool quarantined{}; };
  static State& state(){alignas(State) static unsigned char storage[sizeof(State)];static auto* value=new (storage) State;return *value;}
  InstalledAuthorityRegistration* next_{};InstalledAuthorityRegistration* previous_{};
  void* owner_;void(*revoke_)(void*);bool registered_{true};
  InstalledAuthorityRegistration(void* owner,void(*revoke)(void*)):owner_(owner),revoke_(revoke){
    InstalledAuthorityGuard guard(installed_authority_mutex());auto& s=state();next_=s.head;if(next_)next_->previous_=this;s.head=this;
  }
  void remove(){InstalledAuthorityGuard guard(installed_authority_mutex());if(!registered_)return;auto& s=state();if(previous_)previous_->next_=next_;else s.head=next_;if(next_)next_->previous_=previous_;registered_=false;}
  ~InstalledAuthorityRegistration(){remove();}
  InstalledAuthorityRegistration(const InstalledAuthorityRegistration&)=delete;
  InstalledAuthorityRegistration& operator=(const InstalledAuthorityRegistration&)=delete;
  static void revoke_all(){for(auto* node=state().head;node;node=node->next_)node->revoke_(node->owner_);}
  static bool healthy(){const auto& s=state();return !s.quarantined&&s.mutations==0;}
  static int64_t epoch(){return state().epoch;}
};
// Constructed only by actual storage/key lifecycle paths, after validation and
// before possible mutation. Failure is time-only sticky until verified reboot;
// confirm never clears an earlier quarantine. It mints no authority.
class InstalledAuthorityMutation final {
 private:
  friend class DeviceIdentity;friend class ProvisionedBindingStore;friend class EncryptedProvisioning;
  InstalledAuthorityGuard guard_;bool confirmed_{},counted_{};
  InstalledAuthorityMutation():guard_(installed_authority_mutex()){
    auto& s=InstalledAuthorityRegistration::state();
    if(s.epoch==std::numeric_limits<int64_t>::max())s.quarantined=true;else ++s.epoch;
    if(s.mutations==std::numeric_limits<size_t>::max())s.quarantined=true;else {++s.mutations;counted_=true;}
    InstalledAuthorityRegistration::revoke_all();
  }
  void confirm(){confirmed_=true;}
  ~InstalledAuthorityMutation(){auto& s=InstalledAuthorityRegistration::state();if(!confirmed_)s.quarantined=true;if(counted_)--s.mutations;}
  InstalledAuthorityMutation(const InstalledAuthorityMutation&)=delete;
  InstalledAuthorityMutation& operator=(const InstalledAuthorityMutation&)=delete;
};
}
