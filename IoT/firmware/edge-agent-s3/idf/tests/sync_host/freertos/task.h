#pragma once
#include <cstdint>
inline int xTaskCreate(void(*)(void*),const char*,int,void*,int,void*){return 1;}
inline void vTaskDelay(uint32_t){}
