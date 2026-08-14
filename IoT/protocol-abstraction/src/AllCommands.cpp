#include "FiscalDriver.hpp"
#include <algorithm>
#include <iterator>
namespace bee {
const uint16_t DatecsAllCommands[73]={33,35,38,39,42,43,44,45,46,47,48,49,50,51,53,54,55,56,57,58,60,61,62,63,64,65,66,68,69,70,71,72,74,76,80,83,84,86,87,88,89,90,91,92,94,95,96,98,99,100,101,103,105,106,107,109,110,111,112,115,116,122,123,124,125,126,127,135,140,202,203,253,255};
const uint16_t DaisyAllCommands[88]={33,35,38,39,42,43,44,45,46,47,48,49,50,51,52,53,54,55,56,57,58,61,62,63,64,65,66,68,69,70,71,73,74,76,79,84,85,90,94,95,96,97,99,100,101,102,103,104,105,106,107,108,109,110,111,112,113,114,115,116,117,118,119,128,130,131,132,133,134,138,146,149,150,151,152,153,154,155,156,157,165,166,173,174,176,194,195,201};
bool isDatecsDocumented(uint16_t c){return std::binary_search(std::begin(DatecsAllCommands),std::end(DatecsAllCommands),c);}
bool isDaisyDocumented(uint16_t c){return std::binary_search(std::begin(DaisyAllCommands),std::end(DaisyAllCommands),c);}
}
