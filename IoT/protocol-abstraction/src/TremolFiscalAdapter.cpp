#include "ProtocolFacade.h"
#include "FactoryCreators.h"
#include "TremolProtocol.h"
#include <new>
#include <stdio.h>

namespace beefiscal {
namespace {
OperationResult result(bool ok) { return ok ? OperationResult::ok() : OperationResult::fail(FacadeError::VendorFailure); }
class TremolFiscalAdapter final : public IFiscalDevice {
public:
    explicit TremolFiscalAdapter(const ConnectionSpec& s)
        : channel_(s.channel), paymentCode_(s.paymentCode), printer_(*s.stream, s.timeoutMs, s.retries) {}
    DeviceVendor vendor() const override { return DeviceVendor::Tremol; }
    TransportChannel channel() const override { return channel_; }
    OperationResult openReceipt(const FiscalReceiptRequest& r) override {
        if (!r.operatorPassword || !r.uniqueSaleNumber) return OperationResult::fail(FacadeError::InvalidArgument);
        if (r.invoice) return OperationResult::fail(FacadeError::UnsupportedOperation);
        return result(printer_.openFiscal(r.operatorNumber, r.operatorPassword, '1', '1', '0', r.uniqueSaleNumber));
    }
    OperationResult addItem(const FiscalSaleItem& i) override {
        if (!i.name || !i.unitPrice || i.taxGroup < 1 || i.taxGroup > 8)
            return OperationResult::fail(FacadeError::InvalidArgument);
        char percent[32]; const char* p = nullptr;
        if (i.percentageAdjustment) {
            if (snprintf(percent, sizeof(percent), "%%%s", i.percentageAdjustment) >= int(sizeof(percent)))
                return OperationResult::fail(FacadeError::InvalidArgument);
            p = percent;
        }
        return result(printer_.sell(i.name, char('A' + i.taxGroup - 1), i.unitPrice,
                                    i.quantity, p));
    }
    OperationResult addPayment(const FiscalPayment& p) override {
        if (!p.amount) return OperationResult::fail(FacadeError::InvalidArgument);
        if (p.method != PaymentMethod::Cash && paymentCode_ < 0)
            return OperationResult::fail(FacadeError::PaymentCodeRequired);
        const char code = p.method == PaymentMethod::Cash ? '0' : char(paymentCode_);
        return result(printer_.payment(code, '0', p.amount));
    }
    OperationResult closeReceipt() override { return result(printer_.closeFiscal()); }
    OperationResult cancelReceipt() override { return result(printer_.cancelFiscal()); }
    OperationResult printXReport() override { return result(printer_.printXReport()); }
    OperationResult printZReport() override { return result(printer_.printZReport()); }
    OperationResult cashIn(const char* a) override { return a ? result(printer_.cashIn(a)) : OperationResult::fail(FacadeError::InvalidArgument); }
    OperationResult cashOut(const char* a) override { return a ? result(printer_.cashOut(a)) : OperationResult::fail(FacadeError::InvalidArgument); }
private:
    TransportChannel channel_; int16_t paymentCode_; TremolPrinter printer_;
};
}
IFiscalDevice* createTremolFiscal(const ConnectionSpec& s) { return new (std::nothrow) TremolFiscalAdapter(s); }
} // namespace beefiscal
