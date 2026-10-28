#pragma once
#include <cstddef>
#include <cstdint>
inline uint32_t esp_random(){static uint32_t v=0;return ++v;}
inline void esp_fill_random(void*p,size_t n){auto*b=(uint8_t*)p;for(size_t i=0;i<n;i++)b[i]=(uint8_t)esp_random();}
