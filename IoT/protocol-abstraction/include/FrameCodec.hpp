#pragma once
#include <cstdint>
#include <string>
#include <vector>
namespace bee {
struct ParsedFrame{uint16_t command{};uint8_t sequence{};std::vector<uint8_t> data;std::vector<uint8_t> status;};
class DaisyCodec{public:static std::vector<uint8_t> encode(uint8_t sequence,uint8_t command,const std::vector<uint8_t>&data);static bool decode(const std::vector<uint8_t>&frame,ParsedFrame&out,std::string&error);};
class DatecsCodec{public:static std::vector<uint8_t> encode(uint8_t sequence,uint16_t command,const std::vector<uint8_t>&data);static bool decode(const std::vector<uint8_t>&frame,ParsedFrame&out,std::string&error);};
}
