#pragma once

#include <ProtocolFacade.h>

namespace beefiscal::edge {

class DeviceProtocolProvider final {
public:
    static CreateResult<IFiscalDevice> fiscal(DeviceVendor vendor,
                                               TransportChannel channel,
                                               Stream& transport,
                                               int16_t paymentCode = -1,
                                               uint16_t timeoutMs = 500,
                                               uint8_t retries = 3);

    static CreateResult<IPaymentTerminal> payment(DeviceVendor vendor,
                                                   TransportChannel channel,
                                                   Stream& transport);
};

} // namespace beefiscal::edge

