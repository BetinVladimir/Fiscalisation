#include "DeviceProtocolProvider.h"

namespace beefiscal::edge {

CreateResult<IFiscalDevice> DeviceProtocolProvider::fiscal(DeviceVendor vendor,
                                                            TransportChannel channel,
                                                            Stream& transport,
                                                            int16_t paymentCode,
                                                            uint16_t timeoutMs,
                                                            uint8_t retries) {
    return ProtocolFactory::createFiscal({vendor, channel, &transport, timeoutMs, retries, paymentCode});
}

CreateResult<IPaymentTerminal> DeviceProtocolProvider::payment(DeviceVendor vendor,
                                                                TransportChannel channel,
                                                                Stream& transport) {
    return ProtocolFactory::createPayment({vendor, channel, &transport});
}

} // namespace beefiscal::edge

