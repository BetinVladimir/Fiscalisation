#pragma once
// Daisy FU emulator for host-side tests. Request format (from DaisyCodec::encode):
// 01 LEN(raw=dataLen+0x24) SEQ CMD DATA 05 BCC(4 nibble-ASCII) 03
// Response format (as DaisyCodec::decode expects):
// 01 LEN(raw=dataLen+0x2B) SEQ CMD DATA 04 STATUS(6 raw) 05 BCC(4 nibble-ASCII) 03
// SEQ range 0x20..0xFF (invariant 7, Daisy protocol v2.0-4).
// Validation rules: non-CASH payment → UNSUPPORTED; Latin tax letter → NAK (TAX_GROUP_LATIN).
#include <array>
#include <cstdint>
#include <map>
#include <string>
#include <utility>
#include <vector>

namespace bee {

class FakeDaisyDevice {
public:
    struct FaultPlan {
        std::vector<uint8_t> drop_after; // cmds executed but no response sent
        int delay_ms;                    // SYN bytes 0x16 every 60 ms when non-zero
        FaultPlan() : delay_ms(0) {}
    };

    explicit FakeDaisyDevice() {}
    explicit FakeDaisyDevice(FaultPlan fp) : fault_(std::move(fp)) {}

    void addGoldenPair(std::vector<uint8_t> req, std::vector<uint8_t> ans) {
        golden_[std::move(req)] = std::move(ans);
    }

    void setFaultPlan(FaultPlan fp) { fault_ = std::move(fp); }

    // Handle incoming Daisy request frame. Returns response bytes or {} on NAK/DROP.
    // Returns {0x15} on bad BCC/LEN.
    std::vector<uint8_t> handle(const std::vector<uint8_t>& frame) {
        uint8_t seq, cmd; std::vector<uint8_t> data;
        if (!parseRequest(frame, seq, cmd, data)) return {0x15}; // NAK

        // Validate Daisy-specific rules before SEQ cache
        if (isLatinTaxLetter(data, cmd)) return {0x15}; // TAX_GROUP_LATIN

        // SEQ cache
        auto cacheKey = std::make_pair(cmd, seq);
        auto cit = seq_cache_.find(cacheKey);
        if (cit != seq_cache_.end()) return cit->second;

        // Golden match
        auto git = golden_.find(frame);
        if (git != golden_.end()) {
            seq_cache_[cacheKey] = git->second;
            trackCall(cmd, seq);
            return git->second;
        }

        // Validate non-CASH payment type for cmd 53 (payment)
        if (isNonCashPayment(data, cmd)) {
            auto err = buildErrorResponse(seq, cmd, "UNSUPPORTED");
            seq_cache_[cacheKey] = err;
            return err;
        }

        executeCmd(cmd);
        trackCall(cmd, seq);

        for (uint8_t drop : fault_.drop_after) {
            if (drop == cmd) { seq_cache_[cacheKey] = {}; return {}; }
        }

        auto resp = buildDefaultResponse(seq, cmd, {'0', '\t'});
        seq_cache_[cacheKey] = resp;
        return resp;
    }

    int doc_counter() const { return doc_counter_; }
    int z_counter()   const { return z_counter_; }
    const std::vector<std::pair<uint8_t,uint8_t>>& calls() const { return calls_; }

    void reset() {
        doc_counter_ = 0; z_counter_ = 0;
        seq_cache_.clear(); calls_.clear();
    }

private:
    FaultPlan fault_;
    int doc_counter_ = 0;
    int z_counter_   = 0;
    std::map<std::vector<uint8_t>, std::vector<uint8_t>> golden_;
    std::map<std::pair<uint8_t,uint8_t>, std::vector<uint8_t>> seq_cache_;
    std::vector<std::pair<uint8_t,uint8_t>> calls_;

    void executeCmd(uint8_t cmd) {
        if (cmd == 56) doc_counter_++;
        if (cmd == 69) z_counter_++;
    }
    void trackCall(uint8_t cmd, uint8_t seq) { calls_.emplace_back(cmd, seq); }

    // CMD 49 (sale): first DATA field is tax group letter (e.g. "B" or "б")
    // Latin ASCII uppercase letters are invalid — must be Cyrillic in CP1251
    static bool isLatinTaxLetter(const std::vector<uint8_t>& data, uint8_t cmd) {
        if (cmd != 49 || data.empty()) return false;
        // Find first tab-delimited field (the tax group)
        // Daisy sale payload: price\tqty\tname\ttax_group\t...
        // Tax group is 4th field; but a simple check: if any byte 0x41..0x5A (A-Z) appears
        // in a position that looks like a standalone letter field, flag it.
        // Simplified: scan for a single ASCII uppercase letter between tabs or at start
        for (size_t i = 0; i < data.size(); ++i) {
            uint8_t b = data[i];
            if (b >= 0x41 && b <= 0x5A) { // ASCII A-Z
                // Check it's a standalone field (surrounded by tabs or boundaries)
                bool prevTab = (i == 0 || data[i-1] == '\t');
                bool nextTab = (i + 1 == data.size() || data[i+1] == '\t');
                if (prevTab && nextTab) return true;
            }
        }
        return false;
    }

    // CMD 53 (payment slot): non-CASH type → UNSUPPORTED
    static bool isNonCashPayment(const std::vector<uint8_t>& data, uint8_t cmd) {
        if (cmd != 53 || data.empty()) return false;
        // Payment type code: Daisy CASH is type 0/P, CARD is type 1/N etc.
        // Simplified: if data starts with '1' or 'N' it's non-CASH
        return data[0] == '1' || data[0] == 'N' || data[0] == 'C';
    }

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

    // Parse Daisy REQUEST frame: 01 LEN(raw) SEQ CMD DATA 05 BCC(4) 03
    static bool parseRequest(const std::vector<uint8_t>& v,
                              uint8_t& seq, uint8_t& cmd,
                              std::vector<uint8_t>& data) {
        if (v.size() < 10 || v[0] != 0x01 || v.back() != 0x03) return false;
        const uint8_t lenByte = v[1];
        if (lenByte < 0x24) return false;
        const size_t dataLen = lenByte - 0x24;
        if (v.size() != 10 + dataLen) return false;
        if (v[4 + dataLen] != 0x05) return false;
        uint16_t bccStored;
        if (!unNibble4(v, 5 + dataLen, bccStored)) return false;
        uint16_t bccCalc = 0;
        for (size_t i = 1; i <= 4 + dataLen; ++i) bccCalc = uint16_t(bccCalc + v[i]);
        if (bccCalc != bccStored) return false;
        seq = v[2];
        cmd = v[3];
        data.assign(v.begin() + 4, v.begin() + 4 + dataLen);
        return true;
    }

    // Build Daisy RESPONSE frame: 01 LEN(raw=dataLen+0x2B) SEQ CMD DATA 04 STATUS(6,0x80) 05 BCC(4) 03
    static std::vector<uint8_t> buildDefaultResponse(uint8_t seq, uint8_t cmd,
                                                      const std::vector<uint8_t>& data) {
        std::vector<uint8_t> v{0x01, uint8_t(data.size() + 0x2B), seq, cmd};
        v.insert(v.end(), data.begin(), data.end());
        v.push_back(0x04);
        v.insert(v.end(), 6, 0x80);
        v.push_back(0x05);
        uint16_t bcc = 0;
        for (size_t i = 1; i < v.size(); ++i) bcc = uint16_t(bcc + v[i]);
        putNibble4(v, bcc);
        v.push_back(0x03);
        return v;
    }

    // Build Daisy error response with non-zero error status
    static std::vector<uint8_t> buildErrorResponse(uint8_t seq, uint8_t cmd,
                                                     const std::string& /*code*/) {
        // Use status byte 0 set to error bit 0x01
        std::vector<uint8_t> v{0x01, uint8_t(0x2B), seq, cmd}; // dataLen=0
        v.push_back(0x04);
        v.push_back(0x81); // error bit in status byte 0
        v.insert(v.end(), 5, 0x80);
        v.push_back(0x05);
        uint16_t bcc = 0;
        for (size_t i = 1; i < v.size(); ++i) bcc = uint16_t(bcc + v[i]);
        putNibble4(v, bcc);
        v.push_back(0x03);
        return v;
    }
};

} // namespace bee
