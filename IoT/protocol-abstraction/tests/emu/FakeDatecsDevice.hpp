#pragma once
// Datecs FU emulator for host-side tests. Request format (from DatecsCodec::encode):
// 01 LEN(4 nibble-ASCII, value=dataLen+0x2A) SEQ CMD(4 nibble-ASCII) DATA 05 BCC(4 nibble-ASCII) 03
// Response format (as DatecsCodec::decode expects):
// 01 LEN(4, value=dataLen+0x33) SEQ CMD(4) DATA 04 STATUS(8 raw) 05 BCC(4) 03
// SEQ range 0x20..0x7F (invariant 7, Datecs PM v2.11.4).
#include <array>
#include <cstdint>
#include <map>
#include <string>
#include <utility>
#include <vector>

namespace bee {

class FakeDatecsDevice {
public:
    struct FaultPlan {
        std::vector<uint16_t> drop_after; // cmds executed but no response sent
    };

    explicit FakeDatecsDevice(FaultPlan fp = {}) : fault_(std::move(fp)) {}

    // Register golden pair: exact incoming request bytes → exact answer bytes to return.
    void addGoldenPair(std::vector<uint8_t> req, std::vector<uint8_t> ans) {
        golden_[std::move(req)] = std::move(ans);
    }

    void setFaultPlan(FaultPlan fp) { fault_ = std::move(fp); }

    // Handle incoming Datecs request frame. Returns response bytes or {} on NAK/DROP.
    std::vector<uint8_t> handle(const std::vector<uint8_t>& frame) {
        uint8_t seq; uint16_t cmd; std::vector<uint8_t> data;
        if (!parseRequest(frame, seq, cmd, data)) return {0x15}; // NAK

        // SEQ cache: repeat SEQ with same CMD → return cached response, do not re-execute
        auto cacheKey = std::make_pair(cmd, seq);
        auto cit = seq_cache_.find(cacheKey);
        if (cit != seq_cache_.end()) return cit->second;

        // Golden match: exact request bytes → paired answer bytes
        auto git = golden_.find(frame);
        if (git != golden_.end()) {
            seq_cache_[cacheKey] = git->second;
            trackCall(cmd, seq);
            return git->second;
        }

        // Execute command
        executeCmd(cmd);
        trackCall(cmd, seq);

        // DROP_AFTER: command executed, no response; cache empty so retries also get nothing
        for (uint16_t drop : fault_.drop_after) {
            if (drop == cmd) { seq_cache_[cacheKey] = {}; return {}; }
        }

        auto resp = buildDefaultResponse(seq, cmd, {'0', '\t'});
        seq_cache_[cacheKey] = resp;
        return resp;
    }

    int doc_counter() const { return doc_counter_; }
    int z_counter()   const { return z_counter_; }
    const std::vector<std::pair<uint16_t,uint8_t>>& calls() const { return calls_; }

    void reset() {
        doc_counter_ = 0; z_counter_ = 0;
        seq_cache_.clear(); calls_.clear();
    }

private:
    FaultPlan fault_;
    int doc_counter_ = 0;
    int z_counter_   = 0;
    std::map<std::vector<uint8_t>, std::vector<uint8_t>> golden_;
    std::map<std::pair<uint16_t,uint8_t>, std::vector<uint8_t>> seq_cache_;
    std::vector<std::pair<uint16_t,uint8_t>> calls_;

    void executeCmd(uint16_t cmd) {
        if (cmd == 56) doc_counter_++;
        if (cmd == 69) z_counter_++;
    }
    void trackCall(uint16_t cmd, uint8_t seq) { calls_.emplace_back(cmd, seq); }

    static bool unNibble4(const std::vector<uint8_t>& v, size_t at, uint16_t& out) {
        if (at + 4 > v.size()) return false;
        out = 0;
        for (size_t i = 0; i < 4; ++i) {
            const uint8_t b = v[at + i];
            if (b < 0x30 || b > 0x3F) return false;
            out = uint16_t((out << 4) | (b - 0x30));
        }
        return true;
    }

    static void putNibble4(std::vector<uint8_t>& v, uint16_t x) {
        for (int s : {12, 8, 4, 0}) v.push_back(uint8_t(0x30 + ((x >> s) & 15)));
    }

    // Parse Datecs REQUEST frame: 01 LEN(4) SEQ CMD(4) DATA 05 BCC(4) 03
    static bool parseRequest(const std::vector<uint8_t>& v,
                              uint8_t& seq, uint16_t& cmd,
                              std::vector<uint8_t>& data) {
        if (v.size() < 16 || v[0] != 0x01 || v.back() != 0x03) return false;
        uint16_t lenEncoded;
        if (!unNibble4(v, 1, lenEncoded) || lenEncoded < 0x2A) return false;
        const size_t dataLen = lenEncoded - 0x2A;
        if (v.size() != 16 + dataLen) return false;
        if (v[10 + dataLen] != 0x05) return false;
        uint16_t bccStored;
        if (!unNibble4(v, 11 + dataLen, bccStored)) return false;
        uint16_t bccCalc = 0;
        for (size_t i = 1; i <= 10 + dataLen; ++i) bccCalc = uint16_t(bccCalc + v[i]);
        if (bccCalc != bccStored) return false;
        seq = v[5];
        if (!unNibble4(v, 6, cmd)) return false;
        data.assign(v.begin() + 10, v.begin() + 10 + dataLen);
        return true;
    }

    // Build Datecs RESPONSE frame: 01 LEN(4) SEQ CMD(4) DATA 04 STATUS(8,all 0x80) 05 BCC(4) 03
    static std::vector<uint8_t> buildDefaultResponse(uint8_t seq, uint16_t cmd,
                                                      const std::vector<uint8_t>& data) {
        std::vector<uint8_t> v{0x01};
        putNibble4(v, uint16_t(data.size() + 0x33));
        v.push_back(seq);
        putNibble4(v, cmd);
        v.insert(v.end(), data.begin(), data.end());
        v.push_back(0x04);
        v.insert(v.end(), 8, 0x80);
        v.push_back(0x05);
        uint16_t bcc = 0;
        for (size_t i = 1; i < v.size(); ++i) bcc = uint16_t(bcc + v[i]);
        putNibble4(v, bcc);
        v.push_back(0x03);
        return v;
    }
};

} // namespace bee
