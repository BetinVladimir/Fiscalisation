#pragma once
#include <cstdio>
#define ESP_LOGE(tag,format,...) std::fprintf(stderr,format "\n", ##__VA_ARGS__)
