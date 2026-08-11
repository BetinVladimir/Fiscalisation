#pragma once

#include <stddef.h>
#include <stdint.h>
#include <string.h>
#include <chrono>

inline unsigned long millis() {
    using namespace std::chrono;
    return (unsigned long)duration_cast<milliseconds>(steady_clock::now().time_since_epoch()).count();
}

template <typename T> inline T max(T a, T b) { return a > b ? a : b; }

class Stream {
public:
    virtual ~Stream() = default;
    virtual int available() = 0;
    virtual int read() = 0;
    virtual size_t write(const uint8_t* buffer, size_t size) = 0;
};
