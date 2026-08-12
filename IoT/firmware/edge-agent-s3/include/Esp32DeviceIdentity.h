#pragma once

#include <Arduino.h>
#include <Preferences.h>
#include <mbedtls/ecp.h>

namespace beefiscal::edge {

// Owns the device identity for the complete firmware lifetime. The class never
// exposes the private scalar. Production builds must enable NVS and flash
// encryption so that the persisted representation is protected at rest.
class Esp32DeviceIdentity final {
public:
    Esp32DeviceIdentity();
    ~Esp32DeviceIdentity();

    Esp32DeviceIdentity(const Esp32DeviceIdentity&) = delete;
    Esp32DeviceIdentity& operator=(const Esp32DeviceIdentity&) = delete;

    bool begin();
    bool ready() const { return ready_; }
    bool generatedOnThisBoot() const { return generatedOnThisBoot_; }

    String publicJwk() const;
    String publicKeyThumbprint() const;

    // Signs SHA-256(message) and returns the canonical IEEE P1363 r||s form.
    bool sign(const uint8_t* message, size_t messageLength,
              uint8_t signature[64]);

private:
    bool load();
    bool generate();
    bool persist();
    bool selfTest();

    Preferences preferences_;
    mbedtls_ecp_keypair key_;
    bool preferencesOpen_ = false;
    bool ready_ = false;
    bool generatedOnThisBoot_ = false;
};

} // namespace beefiscal::edge
