#pragma once
#include <cstddef>
#include <random>
inline void esp_fill_random(void*out,size_t n){std::random_device r;auto*p=static_cast<unsigned char*>(out);while(n--)*p++=static_cast<unsigned char>(r());}
