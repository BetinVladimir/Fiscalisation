#pragma once
#include "ProtocolFacade.h"

namespace beefiscal {
IFiscalDevice* createDaisyFiscal(const ConnectionSpec&);
IFiscalDevice* createDatecsFiscal(const ConnectionSpec&);
IFiscalDevice* createTremolFiscal(const ConnectionSpec&);
IPaymentTerminal* createDatecsPayTerminal(const ConnectionSpec&);
}
