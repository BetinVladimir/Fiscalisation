#include "DatecsPay.h"
#include "DatecsPayBleTransport.h"

#include <assert.h>
#include <algorithm>
#include <deque>
#include <vector>

class FakeStream final : public Stream {
public:
    std::deque<uint8_t> rx;
    std::vector<uint8_t> tx;
    size_t maxWrite = SIZE_MAX;

    int available() override { return (int)rx.size(); }
    int read() override {
        if (rx.empty()) return -1;
        uint8_t value = rx.front();
        rx.pop_front();
        return value;
    }
    size_t write(const uint8_t* buffer, size_t size) override {
        size_t count = std::min(size, maxWrite);
        tx.insert(tx.end(), buffer, buffer + count);
        return count;
    }
    void queue(const std::vector<uint8_t>& packet) {
        rx.insert(rx.end(), packet.begin(), packet.end());
    }
};

static std::vector<uint8_t> packet(uint8_t cmd, uint8_t status,
                                   const std::vector<uint8_t>& data = {}) {
    std::vector<uint8_t> result = {PKT_START, cmd, status,
        (uint8_t)(data.size() >> 8), (uint8_t)data.size()};
    result.insert(result.end(), data.begin(), data.end());
    result.push_back(DatecsPacketBuilder::calcCsum(result.data(), result.size()));
    return result;
}

static int eventCount = 0;
static void onRaw(uint8_t event, uint8_t subevent, const uint8_t*, uint16_t) {
    assert(event == EVT_EMV2);
    assert(subevent == 0x82);
    ++eventCount;
}

static void testGoldenPurchaseAndPartialWrite() {
    FakeStream io;
    io.maxWrite = 2;
    io.queue(packet(0x00, 0x00));
    DatecsPay pay(io);
    assert(pay.startPurchase(50) == DatecsError::NoErr);
    const std::vector<uint8_t> expected = {
        0x3E, 0x3D, 0x00, 0x00, 0x08, 0x01, 0x01,
        0x81, 0x04, 0x00, 0x00, 0x00, 0x32, 0xBC};
    assert(io.tx == expected);
}

static void testBuilderAndParserBounds() {
    DatecsPacketBuilder builder;
    builder.begin(CMD_BORICA);
    std::vector<uint8_t> huge(MAX_PACKET_SIZE, 0xAA);
    builder.appendBytes(huge.data(), huge.size());
    uint8_t output[MAX_PACKET_SIZE];
    assert(!builder.valid());
    assert(builder.build(output, sizeof(output)) == 0);

    auto valid = packet(0x00, 0x00);
    valid.push_back(0x00);
    DatecsPacket parsed;
    assert(DatecsPacketParser::parse(valid.data(), valid.size(), parsed) == DatecsError::InvLen);
}

static void testStatusAndThreeByteTlv() {
    DatecsPacket status;
    status.dataLen = 2;
    status.data[0] = 'R';
    status.data[1] = 0x02;
    PinpadStatus parsedStatus;
    assert(DatecsPacketParser::parsePinpadStatus(status, parsedStatus));
    assert(parsedStatus.needsReversal);
    assert(parsedStatus.needsTmsUpdate);
    assert(!parsedStatus.needsEndOfDay);

    const uint8_t tlv[] = {0xDF, 0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x64};
    TLVItem item;
    assert(TLVParser::find(tlv, sizeof(tlv), Tag::MaxCashbackAmount, item));
    assert(TLVParser::toUint32(item) == 100);
}

static void testEventDemultiplexing() {
    FakeStream io;
    io.queue(packet(EVT_EMV2, 0x00, {0x82, 0x15}));
    io.queue(packet(0x00, 0x00));
    DatecsPay pay(io);
    pay.onRawEvent(onRaw);
    eventCount = 0;
    assert(pay.ping() == DatecsError::NoErr);
    assert(eventCount == 1);
}

static void testMtuChunking() {
    FakeStream io;
    io.queue(packet(0x00, 0x00, {0x00, 0x04})); // GET MAX MTU
    io.queue(packet(0x00, 0x00));
    io.queue(packet(0x00, 0x00));
    io.queue(packet(0x00, 0x00));
    DatecsPay pay(io);
    const uint8_t data[10] = {0,1,2,3,4,5,6,7,8,9};
    assert(pay.receiveData(data, sizeof(data)) == DatecsError::NoErr);
    // GET MTU (7 bytes) + RECEIVE frames (11, 11, 9 bytes).
    assert(io.tx.size() == 38);
    assert(io.tx[7 + 4] == 0x05);  // first frame DATA length: subcommand + 4
    assert(io.tx[7 + 11 + 4] == 0x05);
    assert(io.tx[7 + 22 + 4] == 0x03);
}

static void testInvalidStringsAndWriteFailure() {
    FakeStream io;
    DatecsPay pay(io);
    assert(pay.startVoidPurchase(100, nullptr, "A") == DatecsError::LibInvalidArg);
    io.maxWrite = 0;
    io.queue(packet(0x00, 0x00));
    assert(pay.ping() == DatecsError::LibWriteFailed);
}

class FakeBleLink final : public DatecsPayBleLink {
public:
    bool isConnected = true;
    std::vector<uint16_t> chunks;
    bool connected() const override { return isConnected; }
    int available() override { return 0; }
    int readByte() override { return -1; }
    bool writeCharacteristic(const uint8_t*, uint16_t len) override {
        chunks.push_back(len);
        return true;
    }
};

static void testBleTransportChunksAndDisconnect() {
    FakeBleLink link;
    DatecsPayBleStream stream(link);
    uint8_t data[40] = {};
    assert(stream.write(data, sizeof(data)) == sizeof(data));
    assert((link.chunks == std::vector<uint16_t>{19, 19, 2}));
    link.isConnected = false;
    assert(stream.write(data, sizeof(data)) == 0);
    assert(stream.read() == -1);
}

int main() {
    testGoldenPurchaseAndPartialWrite();
    testBuilderAndParserBounds();
    testStatusAndThreeByteTlv();
    testEventDemultiplexing();
    testMtuChunking();
    testInvalidStringsAndWriteFailure();
    testBleTransportChunksAndDisconnect();
    return 0;
}
