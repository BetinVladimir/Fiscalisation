#pragma once
#include <Arduino.h>
#include <stdint.h>

namespace beefiscal::edge {

enum class BleSessionState : uint8_t { Disconnected, ChallengeIssued, Authorized, Closed };

struct BleChallenge {
    uint8_t sessionId[16];
    uint8_t deviceNonce[32];
    uint8_t ephemeralPublicKey[65];
};

struct EncryptedGattFrame {
    uint8_t version;
    uint8_t sessionId[16];
    uint64_t sequence;
    uint16_t fragmentIndex;
    uint16_t fragmentCount;
    const uint8_t* ciphertext;
    size_t ciphertextLength;
    uint8_t tag[16];
};

class SecureBleSession final {
public:
    SecureBleSession();
    bool beginChallenge(BleChallenge& output);
    bool authorize(const uint8_t* appNonce, size_t appNonceLength,
                   const uint8_t* peerEphemeralPublicKey, size_t peerKeyLength,
                   const uint8_t* capability, size_t capabilityLength,
                   const uint8_t* proof, size_t proofLength);
    bool openFrame(const EncryptedGattFrame&, uint8_t* plaintext,
                   size_t plaintextCapacity, size_t& plaintextLength);
    void close();
    BleSessionState state() const { return state_; }
private:
    BleSessionState state_ = BleSessionState::Disconnected;
    uint8_t sessionId_[16]{};
    uint8_t sessionKey_[32]{};
    uint64_t nextSequence_ = 1;
};

} // namespace beefiscal::edge
