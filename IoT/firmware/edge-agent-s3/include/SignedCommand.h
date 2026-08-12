#pragma once
#include <Arduino.h>
#include <stdint.h>
#include <functional>
#include "EdgeStorage.h"

namespace beefiscal::edge {

enum class CommandTransport : uint8_t { Mqtt, Ble };
enum class CommandValidationError : uint8_t {
    None, InvalidInput, WrongVersion, WrongDevice, WrongBinding,
    Expired, CapabilityRejected, SignatureRejected, PermissionDenied, Replay
};

struct SignedCommand {
    uint8_t version;
    String deviceId;
    String senderId;
    String commandId;
    uint64_t sequence;
    int64_t bindingVersion;
    String command;
    String canonicalPayload;
    String payloadDigest;
    String capabilityId;
    String signature;
    int64_t issuedAtUnix;
    int64_t expiresAtUnix;
};

struct CommandContext {
    String deviceId;
    int64_t bindingVersion;
    int64_t trustedNowUnix;
    CommandTransport transport;
};

struct CommandValidationResult {
    bool accepted;
    CommandValidationError error;
    explicit operator bool() const { return accepted; }
};

class CapabilityVerifier {
public:
    virtual ~CapabilityVerifier() = default;
    virtual bool permits(const SignedCommand& command, int64_t trustedNowUnix) = 0;
};

class SenderSignatureVerifier {
public:
    virtual ~SenderSignatureVerifier() = default;
    virtual bool verify(const SignedCommand& command) = 0;
};

class SignedCommandValidator final {
public:
    SignedCommandValidator(EdgeStorage& storage, CapabilityVerifier& capabilities,
                           SenderSignatureVerifier& signatures)
        : storage_(storage), capabilities_(capabilities), signatures_(signatures) {}
    CommandValidationResult validateAndReserve(const SignedCommand&, const CommandContext&);
private:
    EdgeStorage& storage_;
    CapabilityVerifier& capabilities_;
    SenderSignatureVerifier& signatures_;
};

class CommandRouter final {
public:
    using Handler = std::function<String(const SignedCommand&)>;
    explicit CommandRouter(SignedCommandValidator& validator) : validator_(validator) {}
    bool registerHandler(const String& command, Handler handler);
    String route(const SignedCommand&, const CommandContext&);
private:
    struct Route { String command; Handler handler; };
    static constexpr size_t kMaxRoutes = 32;
    Route routes_[kMaxRoutes];
    size_t count_ = 0;
    SignedCommandValidator& validator_;
};

} // namespace beefiscal::edge
