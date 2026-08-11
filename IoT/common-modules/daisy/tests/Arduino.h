#pragma once

#include <stddef.h>
#include <stdint.h>
#include <string.h>
#include <stdlib.h>
#include <stdio.h>
#include <chrono>
#include <thread>

inline unsigned long millis() {
    using namespace std::chrono;
    return (unsigned long)duration_cast<milliseconds>(steady_clock::now().time_since_epoch()).count();
}
inline void yield() { std::this_thread::yield(); }
template <typename T> inline T max(T a, T b) { return a > b ? a : b; }

class Stream {
public:
    virtual ~Stream() = default;
    virtual int available() = 0;
    virtual int read() = 0;
    virtual size_t write(const uint8_t* buffer, size_t size) = 0;
    size_t write(uint8_t value) { return write(&value, 1); }
};
