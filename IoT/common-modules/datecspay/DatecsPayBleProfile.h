#pragma once

#include <stdint.h>

// Datecs BLE byte-stream profile copied from the vendor Android SDK example.
// The service UUID is product-specific; discovery must match this prefix and
// then verify that all four characteristics exist before declaring READY.
namespace DatecsPayBleProfile {
    static constexpr const char* ServiceUuidPrefix = "d839fc3c-84dd-4c36-9126-";
    static constexpr const char* PowerCharacteristic = "22ffc547-1bef-48e2-aa87-b87e23ac0bbd";
    static constexpr const char* WakeCharacteristic  = "f953144b-e33a-4079-b202-e3d7c1f3dbb0";
    static constexpr const char* ReadCharacteristic  = "1f6b14c9-97fa-4f1e-aaa6-7e152fdd04f4";
    static constexpr const char* WriteCharacteristic = "b378db85-4ec3-4daa-828e-1b99607bd6a0";
    static constexpr uint16_t VendorWriteChunk = 19;
    static constexpr uint8_t PowerOff = 0x30;
    static constexpr uint8_t PowerOn  = 0x31;
}
