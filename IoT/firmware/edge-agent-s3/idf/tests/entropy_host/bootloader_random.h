#pragma once
#include <cassert>
#include <vector>
#include <string>
inline bool test_entropy_enabled=false,test_rf_active=false;
inline int test_entropy_enable_count=0,test_entropy_disable_count=0,test_hardware_entropy_calls=0;
inline std::vector<std::string>test_startup_events;
inline void bootloader_random_enable(){assert(!test_rf_active&&!test_entropy_enabled);test_entropy_enabled=true;++test_entropy_enable_count;test_startup_events.push_back("entropy.enable");}
inline void bootloader_random_disable(){assert(!test_rf_active&&test_entropy_enabled);test_entropy_enabled=false;++test_entropy_disable_count;test_startup_events.push_back("entropy.disable");}
