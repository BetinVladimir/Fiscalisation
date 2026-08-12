#include "ProtocolFacade.h"
#include "FactoryCreators.h"
#include "DatecsPay.h"
#include <new>

namespace beefiscal {
namespace {
OperationResult result(DatecsError e) {
    return e == DatecsError::NoErr ? OperationResult::ok()
                                    : OperationResult::fail(FacadeError::VendorFailure, int32_t(e));
}
class DatecsPayTerminalAdapter final : public IPaymentTerminal {
public:
    explicit DatecsPayTerminalAdapter(const ConnectionSpec& s)
        : channel_(s.channel), terminal_(*s.stream) {}
    DeviceVendor vendor() const override { return DeviceVendor::DatecsPay; }
    TransportChannel channel() const override { return channel_; }
    OperationResult ping() override { return result(terminal_.ping()); }
    OperationResult purchase(uint32_t amount) override {
        if (!amount) return OperationResult::fail(FacadeError::InvalidArgument);
        return result(terminal_.startPurchase(amount));
    }
    OperationResult voidPurchase(uint32_t amount, const char* rrn, const char* auth) override {
        if (!amount || !rrn || !auth) return OperationResult::fail(FacadeError::InvalidArgument);
        return result(terminal_.startVoidPurchase(amount, rrn, auth));
    }
    OperationResult finishTransaction(bool approved) override { return result(terminal_.transactionEnd(approved)); }
    OperationResult endOfDay() override { return result(terminal_.startEndOfDay()); }
    bool processEvents() override { return terminal_.processEvents(); }
private:
    TransportChannel channel_; DatecsPay terminal_;
};
}
IPaymentTerminal* createDatecsPayTerminal(const ConnectionSpec& s) {
    return new (std::nothrow) DatecsPayTerminalAdapter(s);
}
} // namespace beefiscal
