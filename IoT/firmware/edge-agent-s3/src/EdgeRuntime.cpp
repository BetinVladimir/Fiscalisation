#include "EdgeRuntime.h"

namespace beefiscal::edge {

bool EdgeRuntime::configure(const CompositeBinding& value, Stream& fiscalTransport,
                            Stream* paymentTransport) {
    if (value.tenantId.isEmpty() || value.locationId.isEmpty() ||
        value.registerId.isEmpty() || value.edgeDeviceId.isEmpty() ||
        value.generation < 1 || value.profile == EdgeProfile::Unconfigured)
        return false;
    if (value.profile == EdgeProfile::DatecsDp150BluePad50 &&
        (value.fiscal.vendor != DeviceVendor::Datecs ||
         value.fiscal.channel != TransportChannel::Rs232 || !value.hasPayment ||
         value.payment.vendor != DeviceVendor::DatecsPay ||
         value.payment.channel != TransportChannel::BleGatt || paymentTransport == nullptr))
        return false;
    if (value.profile == EdgeProfile::DaisyCompactS01 &&
        (value.fiscal.vendor != DeviceVendor::Daisy ||
         value.fiscal.channel != TransportChannel::UsbSerial || value.hasPayment))
        return false;
    auto fiscal = DeviceProtocolProvider::fiscal(value.fiscal.vendor,
        value.fiscal.channel, fiscalTransport, value.fiscalCardPaymentCode);
    if (!fiscal) return false;
    std::unique_ptr<IPaymentTerminal> payment;
    if (value.hasPayment) {
        auto created = DeviceProtocolProvider::payment(value.payment.vendor,
            value.payment.channel, *paymentTransport);
        if (!created) return false;
        payment = std::move(created.instance);
    }
    binding_ = value; fiscal_ = std::move(fiscal.instance); payment_ = std::move(payment);
    return true;
}

bool EdgeRuntime::physicalReady() {
    if (!fiscal_) return false;
    if (payment_ && !payment_->ping()) return false;
    return true;
}

RuntimeResult EdgeRuntime::complete(const ReceiptIntent& intent,
                                    RuntimeResultCode code, const String& error) {
    storage_.completeCommand(intent.operationId.c_str(),
        code == RuntimeResultCode::Committed ? "COMMITTED" :
        code == RuntimeResultCode::Compensated ? "COMPENSATED" :
        code == RuntimeResultCode::RecoveryRequired ? "RECOVERY_REQUIRED" : "REJECTED",
        intent.signature.c_str(), intent.createdAtUnix);
    storage_.enqueue(intent.operationId.c_str(), "RECEIPT_RESULT",
                     intent.canonicalPayload.c_str(), intent.signature.c_str(),
                     intent.createdAtUnix);
    return {code, error};
}

RuntimeResult EdgeRuntime::failAfterCard(const ReceiptIntent& intent,
                                         size_t approvedCards, const String& error) {
    bool compensated = true;
    for (size_t i = approvedCards; i > 0; --i) {
        const Tender& tender = intent.tenders[i - 1];
        // RRN/auth are retained by the payment protocol implementation. A failed
        // reversal is ambiguous and must never be converted to an ordinary error.
        OperationResult reversed = payment_->finishTransaction(false);
        compensated = compensated && static_cast<bool>(reversed);
    }
    return complete(intent, compensated ? RuntimeResultCode::Compensated :
                    RuntimeResultCode::RecoveryRequired, error);
}

RuntimeResult EdgeRuntime::finalize(const ReceiptIntent& intent) {
    if (!fiscal_ || intent.operationId.isEmpty() || intent.receiptSessionId.isEmpty() ||
        intent.uniqueSaleNumber.isEmpty() || intent.senderId.isEmpty() ||
        intent.commandSequence == 0 || intent.lineCount == 0 ||
        intent.tenderCount == 0 || !storage_.isOpen())
        return {RuntimeResultCode::Rejected, "INVALID_OR_UNCONFIGURED"};
    if (!physicalReady()) return {RuntimeResultCode::DeviceUnavailable, "PHYSICAL_DEVICE_UNAVAILABLE"};
    StorageResult reserved = storage_.reserveCommand(intent.operationId.c_str(),
        intent.senderId.c_str(), intent.commandSequence, intent.receiptSessionId.c_str(),
        "sale.finalize", intent.transport == CommandTransport::Mqtt ? "MQTT" : "BLE",
        intent.createdAtUnix);
    if (!reserved) return {RuntimeResultCode::Rejected, "DUPLICATE_OR_STORAGE_FAILURE"};

    size_t approvedCards = 0;
    for (size_t i = 0; i < intent.tenderCount; ++i) {
        if (intent.tenders[i].method != PaymentMethod::Card) continue;
        if (!payment_) return complete(intent, RuntimeResultCode::Rejected,
                                       "PAYMENT_TERMINAL_NOT_CONFIGURED");
        OperationResult paid = payment_->purchase(intent.tenders[i].amountMinor);
        if (!paid) return failAfterCard(intent, approvedCards, "CARD_PAYMENT_FAILED");
        ++approvedCards;
    }
    FiscalReceiptRequest receipt{intent.operatorNumber, intent.operatorPassword.c_str(),
                                 intent.uniqueSaleNumber.c_str(), intent.tillNumber, false};
    if (!fiscal_->openReceipt(receipt))
        return failAfterCard(intent, approvedCards, "FISCAL_OPEN_FAILED");
    for (size_t i = 0; i < intent.lineCount; ++i) {
        const ReceiptLine& line = intent.lines[i];
        FiscalSaleItem item{line.name.c_str(), line.taxGroup, line.unitPrice.c_str(),
                            line.quantity.c_str(), line.adjustment.isEmpty() ? nullptr : line.adjustment.c_str()};
        if (!fiscal_->addItem(item)) {
            fiscal_->cancelReceipt();
            return failAfterCard(intent, approvedCards, "FISCAL_ITEM_FAILED");
        }
    }
    for (size_t i = 0; i < intent.tenderCount; ++i) {
        FiscalPayment payment{intent.tenders[i].method, intent.tenders[i].amount.c_str()};
        if (!fiscal_->addPayment(payment)) {
            fiscal_->cancelReceipt();
            return failAfterCard(intent, approvedCards, "FISCAL_PAYMENT_FAILED");
        }
    }
    if (!fiscal_->closeReceipt())
        return complete(intent, RuntimeResultCode::RecoveryRequired, "FISCAL_CLOSE_UNKNOWN");
    for (size_t i = 0; i < approvedCards; ++i) payment_->finishTransaction(true);
    return complete(intent, RuntimeResultCode::Committed, "");
}

RuntimeResult EdgeRuntime::cancel(const String& id, int64_t now) {
    if (!fiscal_ || id.isEmpty()) return {RuntimeResultCode::Rejected, "INVALID_OR_UNCONFIGURED"};
    ReceiptIntent synthetic{}; synthetic.operationId=id; synthetic.createdAtUnix=now;
    return complete(synthetic, fiscal_->cancelReceipt() ? RuntimeResultCode::Committed :
                    RuntimeResultCode::RecoveryRequired, "");
}
RuntimeResult EdgeRuntime::xReport(const String&, int64_t) {
    return {fiscal_ && fiscal_->printXReport() ? RuntimeResultCode::Committed : RuntimeResultCode::DeviceUnavailable, ""};
}
RuntimeResult EdgeRuntime::zReport(const String&, int64_t) {
    return {fiscal_ && fiscal_->printZReport() ? RuntimeResultCode::Committed : RuntimeResultCode::DeviceUnavailable, ""};
}

} // namespace beefiscal::edge
