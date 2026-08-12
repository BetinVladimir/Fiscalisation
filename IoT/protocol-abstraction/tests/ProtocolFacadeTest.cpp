#include "ProtocolFacade.h"
#include <assert.h>
#include <vector>

using namespace beefiscal;

class FakeStream final : public Stream {
public:
    int available() override { return 0; }
    int read() override { return -1; }
    size_t write(const uint8_t* b, size_t n) override {
        written.insert(written.end(), b, b + n); return n;
    }
    std::vector<uint8_t> written;
};

int main() {
    FakeStream stream;

    assert(ProtocolFactory::supportsFiscal(DeviceVendor::Daisy, TransportChannel::UsbSerial));
    assert(ProtocolFactory::supportsFiscal(DeviceVendor::Datecs, TransportChannel::Rs232));
    assert(ProtocolFactory::supportsFiscal(DeviceVendor::Tremol, TransportChannel::UartTtl));
    assert(!ProtocolFactory::supportsFiscal(DeviceVendor::DatecsPay, TransportChannel::BleGatt));
    assert(ProtocolFactory::supportsPayment(DeviceVendor::DatecsPay, TransportChannel::BleGatt));
    assert(!ProtocolFactory::supportsPayment(DeviceVendor::Datecs, TransportChannel::BleGatt));

    ConnectionSpec daisy{DeviceVendor::Daisy, TransportChannel::UsbSerial, &stream};
    auto fiscal = ProtocolFactory::createFiscal(daisy);
    assert(fiscal && fiscal.instance->vendor() == DeviceVendor::Daisy);
    assert(fiscal.instance->channel() == TransportChannel::UsbSerial);
    assert(fiscal.instance->addItem({nullptr, 1, "1.00"}).error == FacadeError::InvalidArgument);
    assert(fiscal.instance->addPayment({PaymentMethod::Card, "1.00"}).error == FacadeError::PaymentCodeRequired);
    assert(stream.written.empty()); // fail-closed validation never reaches hardware

    ConnectionSpec datecs{DeviceVendor::Datecs, TransportChannel::Rs232, &stream};
    auto datecsFiscal = ProtocolFactory::createFiscal(datecs);
    assert(datecsFiscal && datecsFiscal.instance->vendor() == DeviceVendor::Datecs);

    ConnectionSpec tremol{DeviceVendor::Tremol, TransportChannel::UartTtl, &stream};
    auto tremolFiscal = ProtocolFactory::createFiscal(tremol);
    assert(tremolFiscal && tremolFiscal.instance->vendor() == DeviceVendor::Tremol);

    ConnectionSpec terminal{DeviceVendor::DatecsPay, TransportChannel::BleGatt, &stream};
    auto payment = ProtocolFactory::createPayment(terminal);
    assert(payment && payment.instance->vendor() == DeviceVendor::DatecsPay);
    assert(payment.instance->channel() == TransportChannel::BleGatt);
    assert(payment.instance->purchase(0).error == FacadeError::InvalidArgument);

    assert(ProtocolFactory::createFiscal(terminal).error == FacadeError::UnsupportedVendor);
    assert(ProtocolFactory::createPayment(datecs).error == FacadeError::UnsupportedVendor);
    ConnectionSpec badChannel{DeviceVendor::Daisy, TransportChannel::BleGatt, &stream};
    assert(ProtocolFactory::createFiscal(badChannel).error == FacadeError::UnsupportedChannel);
    ConnectionSpec noStream{DeviceVendor::Daisy, TransportChannel::UsbSerial, nullptr};
    assert(ProtocolFactory::createFiscal(noStream).error == FacadeError::InvalidArgument);
    return 0;
}
