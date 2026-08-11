#pragma once

#include <Arduino.h>
#include "DatecsPayBleProfile.h"

// Platform BLE implementations own scanning, bonding, service discovery,
// notification subscription and reconnect. This narrow boundary makes those
// operations testable without coupling the protocol library to one BLE stack.
class DatecsPayBleLink {
public:
    virtual ~DatecsPayBleLink() = default;
    virtual bool connected() const = 0;
    virtual int available() = 0;
    virtual int readByte() = 0;
    // Must synchronously wait for the GATT write completion callback.
    virtual bool writeCharacteristic(const uint8_t* data, uint16_t len) = 0;
};

class DatecsPayBleStream final : public Stream {
public:
    explicit DatecsPayBleStream(DatecsPayBleLink& link) : _link(link) {}

    int available() override { return _link.connected() ? _link.available() : 0; }
    int read() override { return _link.connected() ? _link.readByte() : -1; }

    size_t write(const uint8_t* buffer, size_t size) override {
        if (!_link.connected() || buffer == nullptr) return 0;
        size_t offset = 0;
        while (offset < size) {
            size_t remaining = size - offset;
            uint16_t chunk = (uint16_t)(remaining > DatecsPayBleProfile::VendorWriteChunk
                ? DatecsPayBleProfile::VendorWriteChunk : remaining);
            if (!_link.writeCharacteristic(buffer + offset, chunk)) return offset;
            offset += chunk;
        }
        return offset;
    }

private:
    DatecsPayBleLink& _link;
};
