#pragma once
#include <cstddef>
#include <cstdint>
#include <vector>
namespace bee { struct ParsedFrame; }
namespace beefiscal::idf {
enum class SerialProfile { Datecs, Daisy };
struct SerialTransport {
 void* context;
 int (*write)(void*,const uint8_t*,size_t,uint32_t);
 int (*read)(void*,uint8_t*,size_t,uint32_t);
 int64_t (*now_ms)(void*);
};
// One owner per physical port; session survives logical operations, not power loss.
class SerialSession {
 public:
 bool exchange(SerialProfile,const SerialTransport&,uint16_t,const std::vector<uint8_t>&,bee::ParsedFrame* = nullptr);
 private:uint8_t next_sequence_{0x20};
};
}
