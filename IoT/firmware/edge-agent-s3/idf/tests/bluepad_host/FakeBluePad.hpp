#pragma once
// FakeBluePad — emulated BluePay BLE card reader for host-side tests.
// purchase(ref) is idempotent: repeated calls with the same ref return the cached
// result and realCharges() increments only on the first execution per ref.
// Modes: APPROVE / DECLINE / TIMEOUT_AFTER_SEND / BLE_DISCONNECT.
#include <map>
#include <string>
#include <utility>

namespace bee {

enum class BluePadMode {
    APPROVE,
    DECLINE,
    TIMEOUT_AFTER_SEND,
    BLE_DISCONNECT,
};

enum class BluePadResult {
    APPROVED,
    DECLINED,
    TIMEOUT,
    DISCONNECTED,
};

class FakeBluePad {
public:
    explicit FakeBluePad(BluePadMode mode) : mode_(mode) {}

    // purchase(ref): idempotent — first call executes, increments realCharges;
    // subsequent calls with the same ref return the cached result unchanged.
    BluePadResult purchase(const std::string& ref) {
        auto it = cache_.find(ref);
        if (it != cache_.end()) return it->second;

        ++real_charges_;
        BluePadResult result{};
        switch (mode_) {
            case BluePadMode::APPROVE:
                result = BluePadResult::APPROVED;    break;
            case BluePadMode::DECLINE:
                result = BluePadResult::DECLINED;    break;
            case BluePadMode::TIMEOUT_AFTER_SEND:
                result = BluePadResult::TIMEOUT;     break;
            case BluePadMode::BLE_DISCONNECT:
                result = BluePadResult::DISCONNECTED; break;
        }
        cache_[ref] = result;
        return result;
    }

    // Number of unique references that actually triggered a charge.
    int real_charges() const { return real_charges_; }

    void reset() {
        cache_.clear();
        real_charges_ = 0;
    }

private:
    BluePadMode mode_;
    int real_charges_ = 0;
    std::map<std::string, BluePadResult> cache_;
};

} // namespace bee
