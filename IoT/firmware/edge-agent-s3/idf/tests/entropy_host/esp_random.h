#pragma once
#include "bootloader_random.h"
#include <cstddef>
#include <random>
inline void esp_fill_random(void*out,size_t n){assert(test_entropy_enabled&&!test_rf_active);++test_hardware_entropy_calls;test_startup_events.push_back("hardware.entropy");std::random_device rng;auto*p=static_cast<unsigned char*>(out);while(n--)*p++=static_cast<unsigned char>(rng());}
