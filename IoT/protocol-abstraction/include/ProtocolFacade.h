#pragma once

#include <Arduino.h>
#include <stdint.h>
#include <memory>

namespace beefiscal {

enum class DeviceVendor : uint8_t { Daisy, Datecs, Tremol, DatecsPay };
enum class TransportChannel : uint8_t { Rs232, UartTtl, UsbSerial, BleGatt, Embedded };
enum class FacadeError : uint8_t {
    None, InvalidArgument, UnsupportedVendor, UnsupportedChannel,
    PaymentCodeRequired, UnsupportedOperation, AllocationFailed, VendorFailure
};
enum class PaymentMethod : uint8_t { Cash, Card, Other };

struct ConnectionSpec {
    DeviceVendor vendor;
    TransportChannel channel;
    Stream* stream;
    uint16_t timeoutMs = 500;
    uint8_t retries = 3;
    // Vendor-programmed payment code. Required for Card/Other; never guessed.
    int16_t paymentCode = -1;
};

struct OperationResult {
    bool success;
    FacadeError error;
    int32_t vendorCode;

    static OperationResult ok() { return {true, FacadeError::None, 0}; }
    static OperationResult fail(FacadeError error, int32_t code = 0) {
        return {false, error, code};
    }
    explicit operator bool() const { return success; }
};

struct FiscalReceiptRequest {
    uint8_t operatorNumber;
    const char* operatorPassword;
    const char* uniqueSaleNumber;
    uint32_t tillNumber = 1;
    bool invoice = false;
};

struct FiscalSaleItem {
    const char* name;
    uint8_t taxGroup;                 // 1..8 => A..H
    const char* unitPrice;
    const char* quantity = "1";
    const char* percentageAdjustment = nullptr; // e.g. "-10.00"
};

struct FiscalPayment {
    PaymentMethod method;
    const char* amount;
};

class IFiscalDevice {
public:
    virtual ~IFiscalDevice() = default;
    virtual DeviceVendor vendor() const = 0;
    virtual TransportChannel channel() const = 0;
    virtual OperationResult openReceipt(const FiscalReceiptRequest&) = 0;
    virtual OperationResult addItem(const FiscalSaleItem&) = 0;
    virtual OperationResult addPayment(const FiscalPayment&) = 0;
    virtual OperationResult closeReceipt() = 0;
    virtual OperationResult cancelReceipt() = 0;
    virtual OperationResult printXReport() = 0;
    virtual OperationResult printZReport() = 0;
    virtual OperationResult cashIn(const char* amount) = 0;
    virtual OperationResult cashOut(const char* amount) = 0;
};

class IPaymentTerminal {
public:
    virtual ~IPaymentTerminal() = default;
    virtual DeviceVendor vendor() const = 0;
    virtual TransportChannel channel() const = 0;
    virtual OperationResult ping() = 0;
    virtual OperationResult purchase(uint32_t amountMinor) = 0;
    virtual OperationResult voidPurchase(uint32_t amountMinor,
                                         const char* rrn,
                                         const char* authorizationId) = 0;
    virtual OperationResult finishTransaction(bool approved) = 0;
    virtual OperationResult endOfDay() = 0;
    virtual bool processEvents() = 0;
};

template <typename T> struct CreateResult {
    std::unique_ptr<T> instance;
    FacadeError error = FacadeError::None;
    explicit operator bool() const { return instance.get() != nullptr; }
};

class ProtocolFactory {
public:
    static CreateResult<IFiscalDevice> createFiscal(const ConnectionSpec&);
    static CreateResult<IPaymentTerminal> createPayment(const ConnectionSpec&);
    static bool supportsFiscal(DeviceVendor, TransportChannel);
    static bool supportsPayment(DeviceVendor, TransportChannel);
};

} // namespace beefiscal
