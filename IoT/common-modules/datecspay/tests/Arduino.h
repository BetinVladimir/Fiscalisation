#pragma once

#include <stddef.h>
#include <stdint.h>
#include <chrono>
#include <thread>

inline unsigned long millis() {
    using namespace std::chrono;
    return (unsigned long)duration_cast<milliseconds>(steady_clock::now().time_since_epoch()).count();
}
inline void yield() { std::this_thread::yield(); }
inline void delay(unsigned long ms) { std::this_thread::sleep_for(std::chrono::milliseconds(ms)); }

class Stream {
public:
    virtual ~Stream() = default;
    virtual int available() = 0;
    virtual int read() = 0;
    virtual size_t write(const uint8_t* buffer, size_t size) = 0;
};
