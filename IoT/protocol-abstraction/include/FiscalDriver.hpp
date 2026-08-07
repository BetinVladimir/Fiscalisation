#pragma once
#include <cstdint>
#include <string>
#include <vector>
namespace bee { enum class RetryClass{Read,PreAccept,LookupThenDecide,Never}; enum class Disposition{Supported,Optional,Privileged,Excluded}; struct CommandSpec{uint16_t code;const char* name;const char* canonical;Disposition disposition;RetryClass retry;}; class FiscalDriver{public:virtual ~FiscalDriver()=default;virtual bool probe()=0;virtual std::vector<uint8_t> encode(uint16_t,const std::vector<uint8_t>&)=0;virtual bool supports(uint16_t)const=0;}; }
namespace bee { extern const uint16_t DatecsAllCommands[73]; extern const uint16_t DaisyAllCommands[88]; bool isDatecsDocumented(uint16_t); bool isDaisyDocumented(uint16_t); bool datecsCommandSpec(uint16_t,CommandSpec&); bool daisyCommandSpec(uint16_t,CommandSpec&); }
