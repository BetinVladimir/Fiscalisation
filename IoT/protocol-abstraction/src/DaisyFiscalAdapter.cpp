#include "ProtocolFacade.h"
#include "FactoryCreators.h"
#include "DaisyProtocol.h"
#include <new>

namespace beefiscal {
namespace {
OperationResult result(bool ok) { return ok ? OperationResult::ok() : OperationResult::fail(FacadeError::VendorFailure); }
bool validItem(const FiscalSaleItem& i) {
    return i.name && i.unitPrice && i.taxGroup >= 1 && i.taxGroup <= 8;
}
class DaisyFiscalAdapter final : public IFiscalDevice {
public:
    explicit DaisyFiscalAdapter(const ConnectionSpec& s)
        : channel_(s.channel), paymentCode_(s.paymentCode), printer_(*s.stream, s.timeoutMs, s.retries) {}
    DeviceVendor vendor() const override { return DeviceVendor::Daisy; }
    TransportChannel channel() const override { return channel_; }
    OperationResult openReceipt(const FiscalReceiptRequest& r) override {
        if (!r.operatorPassword || !r.uniqueSaleNumber) return OperationResult::fail(FacadeError::InvalidArgument);
        return result(printer_.startFiscal(r.operatorNumber, r.operatorPassword, r.uniqueSaleNumber, r.invoice));
    }
    OperationResult addItem(const FiscalSaleItem& i) override {
        if (!validItem(i)) return OperationResult::fail(FacadeError::InvalidArgument);
        return result(printer_.sale(i.name, char('A' + i.taxGroup - 1), i.unitPrice,
                                    i.quantity, i.percentageAdjustment));
    }
    OperationResult addPayment(const FiscalPayment& p) override {
        if (!p.amount) return OperationResult::fail(FacadeError::InvalidArgument);
        if (p.method != PaymentMethod::Cash && paymentCode_ < 0)
            return OperationResult::fail(FacadeError::PaymentCodeRequired);
        const char code = p.method == PaymentMethod::Cash ? 'P' : char(paymentCode_);
        return result(printer_.total(code, p.amount));
    }
    OperationResult closeReceipt() override { return result(printer_.endFiscal()); }
    OperationResult cancelReceipt() override { return result(printer_.cancelReceipt()); }
    OperationResult printXReport() override { return result(printer_.dailyReportX()); }
    OperationResult printZReport() override { return result(printer_.dailyReportZ()); }
    OperationResult cashIn(const char* a) override { return a ? result(printer_.cashIn(a)) : OperationResult::fail(FacadeError::InvalidArgument); }
    OperationResult cashOut(const char* a) override { return a ? result(printer_.cashOut(a)) : OperationResult::fail(FacadeError::InvalidArgument); }
private:
    TransportChannel channel_; int16_t paymentCode_; DaisyPrinter printer_;
};
}
IFiscalDevice* createDaisyFiscal(const ConnectionSpec& s) { return new (std::nothrow) DaisyFiscalAdapter(s); }
} // namespace beefiscal
