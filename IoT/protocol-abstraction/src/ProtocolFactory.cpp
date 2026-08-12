#include "ProtocolFacade.h"
#include "FactoryCreators.h"

namespace beefiscal {
namespace {
bool serialFiscalChannel(TransportChannel c) {
    return c == TransportChannel::Rs232 || c == TransportChannel::UartTtl ||
           c == TransportChannel::UsbSerial || c == TransportChannel::Embedded;
}
template <typename T> CreateResult<T> failure(FacadeError e) {
    CreateResult<T> r; r.error = e; return r;
}
}

bool ProtocolFactory::supportsFiscal(DeviceVendor v, TransportChannel c) {
    return serialFiscalChannel(c) &&
           (v == DeviceVendor::Daisy || v == DeviceVendor::Datecs || v == DeviceVendor::Tremol);
}

bool ProtocolFactory::supportsPayment(DeviceVendor v, TransportChannel c) {
    return v == DeviceVendor::DatecsPay &&
           (c == TransportChannel::BleGatt || c == TransportChannel::Embedded);
}

CreateResult<IFiscalDevice> ProtocolFactory::createFiscal(const ConnectionSpec& s) {
    if (!s.stream) return failure<IFiscalDevice>(FacadeError::InvalidArgument);
    if (s.vendor == DeviceVendor::DatecsPay) return failure<IFiscalDevice>(FacadeError::UnsupportedVendor);
    if (!supportsFiscal(s.vendor, s.channel)) return failure<IFiscalDevice>(FacadeError::UnsupportedChannel);
    IFiscalDevice* p = nullptr;
    switch (s.vendor) {
        case DeviceVendor::Daisy: p = createDaisyFiscal(s); break;
        case DeviceVendor::Datecs: p = createDatecsFiscal(s); break;
        case DeviceVendor::Tremol: p = createTremolFiscal(s); break;
        default: break;
    }
    if (!p) return failure<IFiscalDevice>(FacadeError::AllocationFailed);
    CreateResult<IFiscalDevice> r; r.instance.reset(p); return r;
}

CreateResult<IPaymentTerminal> ProtocolFactory::createPayment(const ConnectionSpec& s) {
    if (!s.stream) return failure<IPaymentTerminal>(FacadeError::InvalidArgument);
    if (s.vendor != DeviceVendor::DatecsPay) return failure<IPaymentTerminal>(FacadeError::UnsupportedVendor);
    if (!supportsPayment(s.vendor, s.channel)) return failure<IPaymentTerminal>(FacadeError::UnsupportedChannel);
    IPaymentTerminal* p = createDatecsPayTerminal(s);
    if (!p) return failure<IPaymentTerminal>(FacadeError::AllocationFailed);
    CreateResult<IPaymentTerminal> r; r.instance.reset(p); return r;
}
} // namespace beefiscal
