#include "SecureBleSession.h"
#include <esp_system.h>
#include <string.h>

namespace beefiscal::edge {

SecureBleSession::SecureBleSession() = default;

bool SecureBleSession::beginChallenge(BleChallenge& out) {
    close();
    esp_fill_random(sessionId_, sizeof(sessionId_));
    memcpy(out.sessionId, sessionId_, sizeof(sessionId_));
    esp_fill_random(out.deviceNonce, sizeof(out.deviceNonce));
    // Ephemeral ECDH is implemented directly with the ESP32 crypto stack in the
    // security milestone. Never authorize until capability + proof validation
    // and key derivation are implemented.
    memset(out.ephemeralPublicKey, 0, sizeof(out.ephemeralPublicKey));
    state_ = BleSessionState::ChallengeIssued;
    return true;
}

bool SecureBleSession::authorize(const uint8_t*, size_t, const uint8_t*, size_t,
                                 const uint8_t*, size_t, const uint8_t*, size_t) {
    // Fail closed: placeholder cannot accidentally authorize plaintext BLE.
    return false;
}

bool SecureBleSession::openFrame(const EncryptedGattFrame&, uint8_t*, size_t, size_t& length) {
    length = 0;
    return false;
}

void SecureBleSession::close() {
    memset(sessionId_, 0, sizeof(sessionId_));
    memset(sessionKey_, 0, sizeof(sessionKey_));
    nextSequence_ = 1;
    state_ = BleSessionState::Disconnected;
}

} // namespace beefiscal::edge
