// stdin/stdout harness for FakeDatecsDevice — used by Python acceptance tests.
// Protocol (one command per line on stdin):
//   G <req_hex> <ans_hex>   — register golden pair
//   H <frame_hex>           — handle frame, prints hex response or "" on NAK/DROP
//   DROP <cmd_decimal>      — add DROP_AFTER fault for this command
//   C                       — print doc_counter
//   RESET                   — reset device state
#include "FakeDatecsDevice.hpp"
#include <cstdlib>
#include <iomanip>
#include <iostream>
#include <sstream>
#include <string>

static std::vector<uint8_t> fromHex(const std::string& s) {
    std::vector<uint8_t> v;
    v.reserve(s.size() / 2);
    for (size_t i = 0; i + 1 < s.size(); i += 2)
        v.push_back(static_cast<uint8_t>(std::stoul(s.substr(i, 2), nullptr, 16)));
    return v;
}

static std::string toHex(const std::vector<uint8_t>& v) {
    std::ostringstream oss;
    for (uint8_t b : v) oss << std::hex << std::setw(2) << std::setfill('0') << (int)b;
    return oss.str();
}

int main() {
    bee::FakeDatecsDevice device;
    bee::FakeDatecsDevice::FaultPlan fp;
    std::string line;
    while (std::getline(std::cin, line)) {
        if (line.empty()) continue;
        if (line.rfind("G ", 0) == 0) {
            auto sp = line.find(' ', 2);
            if (sp == std::string::npos) continue;
            device.addGoldenPair(fromHex(line.substr(2, sp - 2)), fromHex(line.substr(sp + 1)));
        } else if (line.rfind("H ", 0) == 0) {
            auto resp = device.handle(fromHex(line.substr(2)));
            std::cout << toHex(resp) << '\n';
            std::cout.flush();
        } else if (line.rfind("DROP ", 0) == 0) {
            fp.drop_after.push_back(static_cast<uint16_t>(std::stoul(line.substr(5))));
            device.setFaultPlan(fp);
        } else if (line == "C") {
            std::cout << device.doc_counter() << '\n';
            std::cout.flush();
        } else if (line == "RESET") {
            device.reset();
            fp = {};
        }
    }
    return 0;
}
