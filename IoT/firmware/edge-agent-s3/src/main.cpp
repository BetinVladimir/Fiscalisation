#include <Arduino.h>
#include "DeviceProtocolProvider.h"
#include "EdgeStorage.h"
#include "Esp32DeviceIdentity.h"

#ifndef EDGE_FISCAL_UART_RX
#define EDGE_FISCAL_UART_RX 18
#endif
#ifndef EDGE_FISCAL_UART_TX
#define EDGE_FISCAL_UART_TX 17
#endif
#ifndef EDGE_FISCAL_UART_BAUD
#define EDGE_FISCAL_UART_BAUD 115200
#endif

using namespace beefiscal;
using namespace beefiscal::edge;

namespace {
HardwareSerial fiscalSerial(1);
EdgeStorage storage;
Esp32DeviceIdentity deviceIdentity;
std::unique_ptr<IFiscalDevice> fiscalDevice;

void logFailure(const char* component, const StorageResult& result) {
    Serial.printf("[%s] error=%u sqlite=%d message=%s\n", component,
                  static_cast<unsigned>(result.error), result.sqliteCode,
                  result.message.c_str());
}
}

void setup() {
    Serial.begin(115200);
    delay(250);
    Serial.println("BeeFiscal edge-agent-s3 starting");

    if (!deviceIdentity.begin()) {
        Serial.println("Device identity initialization failed; networking and commands remain disabled");
        return;
    }
    const String thumbprint = deviceIdentity.publicKeyThumbprint();
    Serial.printf("Device identity ready (%s), thumbprint suffix=%s\n",
                  deviceIdentity.generatedOnThisBoot() ? "generated" : "loaded",
                  thumbprint.substring(thumbprint.length() > 8 ? thumbprint.length() - 8 : 0).c_str());

    StorageResult opened = storage.begin();
    if (!opened) {
        logFailure("storage", opened);
    } else {
        Serial.printf("SD/SQLite ready, card size=%llu bytes\n", storage.cardSizeBytes());
    }

    fiscalSerial.begin(EDGE_FISCAL_UART_BAUD, SERIAL_8N1,
                       EDGE_FISCAL_UART_RX, EDGE_FISCAL_UART_TX);

    // Default development profile. Production provisioning must select vendor,
    // channel and programmed payment code from signed device configuration.
    auto created = DeviceProtocolProvider::fiscal(
        DeviceVendor::Daisy, TransportChannel::UartTtl, fiscalSerial);
    if (!created) {
        Serial.printf("Fiscal adapter creation failed: %u\n",
                      static_cast<unsigned>(created.error));
    } else {
        fiscalDevice = std::move(created.instance);
        Serial.println("Fiscal adapter ready");
    }
}

void loop() {
    // Network/MQTT and command orchestration will consume fiscalDevice and the
    // durable transaction journal. No demo fiscal command is sent on boot.
    delay(1000);
}
