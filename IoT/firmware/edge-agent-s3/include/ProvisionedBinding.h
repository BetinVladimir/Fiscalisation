#pragma once
#include <Arduino.h>
#include <Preferences.h>
#include "EdgeRuntime.h"

namespace beefiscal::edge {
struct SignedBindingEnvelope { String canonicalPayload; String signatureBase64Url; CompositeBinding binding; };

/** The trust anchor is compiled into a signed firmware build. The envelope
 * never supplies its own CA. Generation must increase monotonically. */
class ProvisionedBindingStore final {
public:
    explicit ProvisionedBindingStore(const char* pinnedCaPem) : pinnedCaPem_(pinnedCaPem) {}
    bool begin();
    bool install(const SignedBindingEnvelope&);
    bool load(CompositeBinding&);
    int64_t generation() const { return generation_; }
private:
    bool verify(const SignedBindingEnvelope&) const;
    bool encode(const CompositeBinding&, String&) const;
    bool decode(const String&, CompositeBinding&) const;
    const char* pinnedCaPem_;
    Preferences preferences_;
    bool open_ = false;
    int64_t generation_ = 0;
};
} // namespace beefiscal::edge
