#include "SignedCommand.h"

namespace beefiscal::edge {

CommandValidationResult SignedCommandValidator::validateAndReserve(
    const SignedCommand& c, const CommandContext& x) {
    if (c.deviceId.isEmpty() || c.senderId.isEmpty() || c.commandId.isEmpty() ||
        c.command.isEmpty() || c.payloadDigest.isEmpty() || c.capabilityId.isEmpty())
        return {false, CommandValidationError::InvalidInput};
    if (c.version != 2) return {false, CommandValidationError::WrongVersion};
    if (c.deviceId != x.deviceId) return {false, CommandValidationError::WrongDevice};
    if (c.bindingVersion != x.bindingVersion) return {false, CommandValidationError::WrongBinding};
    if (x.trustedNowUnix < c.issuedAtUnix || x.trustedNowUnix > c.expiresAtUnix)
        return {false, CommandValidationError::Expired};
    if (!capabilities_.permits(c, x.trustedNowUnix))
        return {false, CommandValidationError::CapabilityRejected};
    if (!signatures_.verify(c)) return {false, CommandValidationError::SignatureRejected};
    if (!storage_.rememberReplaySequence(c.senderId.c_str(), c.sequence))
        return {false, CommandValidationError::Replay};
    const char* transport = x.transport == CommandTransport::Mqtt ? "MQTT" : "BLE";
    if (!storage_.reserveCommand(c.commandId.c_str(), c.senderId.c_str(), c.sequence,
                                 c.payloadDigest.c_str(), c.capabilityId.c_str(),
                                 transport, x.trustedNowUnix))
        return {false, CommandValidationError::Replay};
    return {true, CommandValidationError::None};
}

bool CommandRouter::registerHandler(const String& command, Handler handler) {
    if (command.isEmpty() || !handler || count_ >= kMaxRoutes) return false;
    for (size_t i = 0; i < count_; ++i) if (routes_[i].command == command) return false;
    routes_[count_++] = {command, handler}; return true;
}

String CommandRouter::route(const SignedCommand& command, const CommandContext& context) {
    CommandValidationResult validation = validator_.validateAndReserve(command, context);
    if (!validation) return "REJECTED:" + String(static_cast<unsigned>(validation.error));
    for (size_t i = 0; i < count_; ++i) {
        if (routes_[i].command == command.command) return routes_[i].handler(command);
    }
    return "UNSUPPORTED_COMMAND";
}

} // namespace beefiscal::edge
