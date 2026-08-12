#include "ProtocolFacade.h"
#include "FactoryCreators.h"
#include "DatecsProtocol.h"
#include <new>

namespace beefiscal {
namespace {
OperationResult result(bool ok) { return ok ? OperationResult::ok() : OperationResult::fail(FacadeError::VendorFailure); }
class DatecsFiscalAdapter final : public IFiscalDevice {
public:
    explicit DatecsFiscalAdapter(const ConnectionSpec& s)
        : channel_(s.channel), paymentCode_(s.paymentCode), printer_(*s.stream, s.timeoutMs, s.retries) {}
    DeviceVendor vendor() const override { return DeviceVendor::Datecs; }
    TransportChannel channel() const override { return channel_; }
    OperationResult openReceipt(const FiscalReceiptRequest& r) override {
        if (!r.operatorPassword || !r.uniqueSaleNumber) return OperationResult::fail(FacadeError::InvalidArgument);
        return result(printer_.openFiscal(r.operatorNumber, r.operatorPassword, r.tillNumber,
                                          r.uniqueSaleNumber, r.invoice));
    }
    OperationResult addItem(const FiscalSaleItem& i) override {
        if (!i.name || !i.unitPrice || i.taxGroup < 1 || i.taxGroup > 8)
            return OperationResult::fail(FacadeError::InvalidArgument);
        uint8_t type = 0;
        if (i.percentageAdjustment) type = i.percentageAdjustment[0] == '-' ? 2 : 1;
        return result(printer_.registerSale(i.name, i.taxGroup, i.unitPrice,
                                            i.quantity ? i.quantity : "1", type,
                                            i.percentageAdjustment));
    }
    OperationResult addPayment(const FiscalPayment& p) override {
        if (!p.amount) return OperationResult::fail(FacadeError::InvalidArgument);
        if (p.method != PaymentMethod::Cash && paymentCode_ < 0)
            return OperationResult::fail(FacadeError::PaymentCodeRequired);
        const int16_t code = p.method == PaymentMethod::Cash ? 0 : paymentCode_;
        if (code < 0 || code > 6) return OperationResult::fail(FacadeError::InvalidArgument);
        return result(printer_.total(uint8_t(code), p.amount));
    }
    OperationResult closeReceipt() override { return result(printer_.closeFiscal()); }
    OperationResult cancelReceipt() override { return result(printer_.cancelFiscal()); }
    OperationResult printXReport() override { return result(printer_.printReport('X')); }
    OperationResult printZReport() override { return result(printer_.printReport('Z')); }
    OperationResult cashIn(const char* a) override { return a ? result(printer_.cashIn(a)) : OperationResult::fail(FacadeError::InvalidArgument); }
    OperationResult cashOut(const char* a) override { return a ? result(printer_.cashOut(a)) : OperationResult::fail(FacadeError::InvalidArgument); }
private:
    TransportChannel channel_; int16_t paymentCode_; DatecsPrinter printer_;
};
}
IFiscalDevice* createDatecsFiscal(const ConnectionSpec& s) { return new (std::nothrow) DatecsFiscalAdapter(s); }
} // namespace beefiscal
