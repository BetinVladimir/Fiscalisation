#pragma once

#include <Arduino.h>
#include <memory>
#include "DeviceProtocolProvider.h"
#include "EdgeStorage.h"
#include "SignedCommand.h"

namespace beefiscal::edge {

enum class EdgeProfile : uint8_t { Unconfigured, DatecsDp150BluePad50, DaisyCompactS01 };

struct EndpointBinding {
    DeviceVendor vendor;
    TransportChannel channel;
    String model;
    String serial;
    String fiscalMemoryNumber;
};

struct CompositeBinding {
    String tenantId;
    String locationId;
    String registerId;
    String edgeDeviceId;
    int64_t generation = 0;
    EdgeProfile profile = EdgeProfile::Unconfigured;
    EndpointBinding fiscal{};
    bool hasPayment = false;
    EndpointBinding payment{};
    int16_t fiscalCardPaymentCode = -1;
};

enum class RuntimeResultCode : uint8_t {
    Committed, Compensated, RecoveryRequired, Rejected, DeviceUnavailable
};

struct ReceiptLine {
    String name;
    uint8_t taxGroup;
    String unitPrice;
    String quantity;
    String adjustment;
};

struct Tender {
    String operationId;
    PaymentMethod method;
    String amount;
    uint32_t amountMinor;
};

struct ReceiptIntent {
    String operationId;
    String receiptSessionId;
    String uniqueSaleNumber;
    uint8_t operatorNumber;
    String operatorPassword;
    uint32_t tillNumber;
    const ReceiptLine* lines;
    size_t lineCount;
    const Tender* tenders;
    size_t tenderCount;
    String canonicalPayload;
    String signature;
    String senderId;
    uint64_t commandSequence;
    CommandTransport transport;
    int64_t createdAtUnix;
};

struct RuntimeResult {
    RuntimeResultCode code;
    String error;
};

/** One coordinator is used by MQTT and direct BLE. It reserves the operation
 * on SD before the first physical I/O and never blindly retries an ambiguous
 * payment or fiscal close. */
class EdgeRuntime final {
public:
    explicit EdgeRuntime(EdgeStorage& storage) : storage_(storage) {}
    bool configure(const CompositeBinding&, Stream& fiscalTransport,
                   Stream* paymentTransport = nullptr);
    RuntimeResult finalize(const ReceiptIntent&);
    RuntimeResult cancel(const String& operationId, int64_t nowUnix);
    RuntimeResult xReport(const String& operationId, int64_t nowUnix);
    RuntimeResult zReport(const String& operationId, int64_t nowUnix);
    bool physicalReady();
    const CompositeBinding& binding() const { return binding_; }

private:
    RuntimeResult complete(const ReceiptIntent&, RuntimeResultCode, const String&);
    RuntimeResult failAfterCard(const ReceiptIntent&, size_t approvedCards,
                                const String& error);
    EdgeStorage& storage_;
    CompositeBinding binding_{};
    std::unique_ptr<IFiscalDevice> fiscal_;
    std::unique_ptr<IPaymentTerminal> payment_;
};

} // namespace beefiscal::edge
