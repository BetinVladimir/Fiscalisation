#pragma once
#include <cstdint>
inline int64_t esp_timer_get_time(){static int64_t tick=0;return ++tick*1000;}
