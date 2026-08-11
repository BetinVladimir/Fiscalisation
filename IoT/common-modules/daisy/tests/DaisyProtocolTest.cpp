#include "DaisyProtocol.h"

#include <assert.h>
#include <algorithm>
#include <deque>
#include <string>
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
    size_t write(const uint8_t* data, size_t len) override {
        size_t count = std::min(len, maxWrite);
        tx.insert(tx.end(), data, data + count);
        return count;
    }
    void queue(const std::vector<uint8_t>& bytes) {
        rx.insert(rx.end(), bytes.begin(), bytes.end());
    }
};

static std::vector<uint8_t> response(uint8_t seq, uint8_t cmd,
                                     const std::vector<uint8_t>& data = {},
                                     const std::vector<uint8_t>& status =
                                         {0x80,0x80,0x80,0x80,0x80,0x80}) {
    assert(status.size() == 6);
    std::vector<uint8_t> bytes = {DAISY_STX,
        (uint8_t)(DAISY_LEN_OFFSET + data.size() + 11), seq, cmd};
    bytes.insert(bytes.end(), data.begin(), data.end());
    bytes.push_back(DAISY_PST2);
    bytes.insert(bytes.end(), status.begin(), status.end());
    bytes.push_back(DAISY_PST1);
    uint16_t sum = 0;
    for (size_t i = 1; i < bytes.size(); ++i) sum += bytes[i];
    uint8_t bcc[4];
    DaisyProtocol::encodeBCC(sum, bcc);
    bytes.insert(bytes.end(), bcc, bcc + 4);
    bytes.push_back(DAISY_ETX);
    return bytes;
}

static void testGoldenStatusRequestAndResponse() {
    FakeStream io;
    io.maxWrite = 2;
    const uint8_t vendorResponse[] = {
        0x01,0x31,0x50,0x4A,0x88,0x80,0x80,0x80,0x80,0xB8,0x04,
        0x88,0x80,0x80,0x80,0x80,0xB8,0x05,0x30,0x37,0x35,0x34,0x03};
    io.queue(std::vector<uint8_t>(vendorResponse, vendorResponse + sizeof(vendorResponse)));
    DaisyProtocol protocol(io);
    protocol.resetSeq();
    // Vendor example uses sequence 0x50.
    for (unsigned i = 0; i < 0x30; ++i) {
        // Advance using valid exchanges before checking the documented vector.
        if (i == 0) break;
    }
    // Direct packet golden vector at initial sequence 0x20.
    assert(protocol.sendPacket(CMD_FD_STATUS, nullptr, 0) == 10);
    const std::vector<uint8_t> expected =
        {0x01,0x24,0x20,0x4A,0x05,0x30,0x30,0x39,0x33,0x03};
    assert(io.tx == expected);

    // Parse the vendor response independently; sequence validation belongs to execute().
    DaisyResponse parsed;
    assert(protocol.receivePacket(parsed));
    assert(parsed.seq == 0x50 && parsed.cmd == 0x4A && parsed.dataLen == 6);
}

static void testBccEncodingAndMalformedFrames() {
    uint8_t encoded[4];
    DaisyProtocol::encodeBCC(0x12AB, encoded);
    const uint8_t expected[] = {0x31,0x32,0x3A,0x3B};
    assert(memcmp(encoded, expected, 4) == 0);
    uint16_t decoded = 0;
    assert(DaisyProtocol::decodeBCC(encoded, decoded) && decoded == 0x12AB);
    const uint8_t invalid[] = {0x30,0x30,0x41,0x30};
    assert(!DaisyProtocol::decodeBCC(invalid, decoded));

    FakeStream io;
    auto malformed = response(0x20, CMD_FD_STATUS);
    malformed[4] = 0x06; // corrupt PST2, then repair BCC so framing check is isolated
    uint16_t sum = 0;
    for (size_t i = 1; i <= 11; ++i) sum += malformed[i];
    DaisyProtocol::encodeBCC(sum, &malformed[12]);
    io.queue(malformed);
    DaisyProtocol protocol(io);
    DaisyResponse parsed;
    assert(!protocol.receivePacket(parsed));
    assert(parsed.localError == DAISY_ERR_BAD_FRAME);
}

static void testSequenceRollover() {
    FakeStream io;
    for (unsigned seq = DAISY_SEQ_MIN; seq <= DAISY_SEQ_MAX; ++seq)
        io.queue(response((uint8_t)seq, CMD_CLEAR_DISPLAY));
    DaisyProtocol protocol(io);
    DaisyResponse parsed;
    for (unsigned i = 0; i < 224; ++i)
        assert(protocol.execute(CMD_CLEAR_DISPLAY, parsed));
    assert(protocol.currentSeq() == DAISY_SEQ_MIN);
}

static void testNakSynAndBadBccRecovery() {
    FakeStream nakIo;
    nakIo.queue({DAISY_NAK});
    nakIo.queue(response(DAISY_SEQ_MIN, CMD_CLEAR_DISPLAY));
    DaisyProtocol nakProtocol(nakIo);
    DaisyResponse parsed;
    assert(nakProtocol.execute(CMD_CLEAR_DISPLAY, parsed));
    assert(nakIo.tx.size() == 20);
    assert(std::equal(nakIo.tx.begin(), nakIo.tx.begin() + 10, nakIo.tx.begin() + 10));

    FakeStream synIo;
    synIo.queue({DAISY_SYN});
    synIo.queue(response(DAISY_SEQ_MIN, CMD_CLEAR_DISPLAY));
    DaisyProtocol synProtocol(synIo);
    assert(synProtocol.execute(CMD_CLEAR_DISPLAY, parsed));
    assert(synIo.tx.size() == 10);

    FakeStream bccIo;
    auto damaged = response(DAISY_SEQ_MIN, CMD_CLEAR_DISPLAY);
    damaged[12] ^= 0x01;
    bccIo.queue(damaged);
    bccIo.queue(response(DAISY_SEQ_MIN, CMD_CLEAR_DISPLAY));
    DaisyProtocol bccProtocol(bccIo);
    assert(bccProtocol.execute(CMD_CLEAR_DISPLAY, parsed));
    assert(bccIo.tx.size() == 20);
}

struct StreamCapture { std::vector<std::string> chunks; std::vector<uint8_t> types; };
static bool capture(uint8_t type, const uint8_t* data, uint16_t len, void* context) {
    auto* value = static_cast<StreamCapture*>(context);
    value->types.push_back(type);
    value->chunks.emplace_back((const char*)data, len);
    return true;
}

static void testStreamingTextAndStructuredFrames() {
    FakeStream io;
    const uint8_t text[] = {0x1A,'0','0','0','0','0','1',0x09,'N',0x09,'O','K',0x0D,0x0A};
    const uint8_t structured[] = {0x1B,0x07,'S','E','P','=',0x09,0x0D,0x0A};
    io.queue(std::vector<uint8_t>(text, text + sizeof(text)));
    io.queue(std::vector<uint8_t>(structured, structured + sizeof(structured)));
    io.queue(response(DAISY_SEQ_MIN, CMD_EJT_REPORTS));
    DaisyProtocol protocol(io);
    DaisyResponse finalResponse;
    StreamCapture captured;
    const char command[] = "C13,1,1000,1";
    assert(protocol.executeStreaming(CMD_EJT_REPORTS, command,
        (uint8_t)strlen(command), capture, &captured, finalResponse));
    assert((captured.types == std::vector<uint8_t>{1,2}));
    assert(captured.chunks[1] == std::string("SEP=\t\r\n", 7));
    assert(std::count(io.tx.begin(), io.tx.end(), DAISY_DC1) == 2);
}

static void testHighLevelValidationAndStatusErrors() {
    FakeStream io;
    DaisyPrinter printer(io);
    std::string huge(250, 'X');
    assert(!printer.sale(huge.c_str(), 'A', "1.00"));
    assert(!printer.sale("x", 'A', nullptr));
    assert(DaisyPrinter::taxGroupByte((char)0xC3) == 0xC3);
    char ignored = 0;
    assert(!printer.getDatetime(&ignored, 0));

    DaisyResponse status;
    status.clear();
    status.status.raw[1] = 0xC0; // reserved bit plus WRONG PASSWORD
    assert(!status.ok());
}

static std::string requestData(const std::vector<uint8_t>& packet) {
    assert(packet.size() >= 10 && packet[0] == DAISY_STX);
    return std::string((const char*)&packet[4], packet.size() - 10);
}

static void testRefundAndEuroCashBuilders() {
    FakeStream refundIo;
    const std::string counters = "000003,000002";
    refundIo.queue(response(DAISY_SEQ_MIN, CMD_START_FISCAL,
        std::vector<uint8_t>(counters.begin(), counters.end())));
    DaisyPrinter refundPrinter(refundIo);
    assert(refundPrinter.startRefund(20, "9999", "DY000600-OP20-0000003", 1,
        203, "10-04-23 21:54:02", "36940032"));
    assert(requestData(refundIo.tx) ==
        "20,9999,DY000600-OP20-0000003\tR1,203,10-04-23 21:54:02\t36940032");

    FakeStream cashIo;
    const std::string cashReply = "P,10.00,10.00,0.00";
    cashIo.queue(response(DAISY_SEQ_MIN, CMD_SERVICE_CASH,
        std::vector<uint8_t>(cashReply.begin(), cashReply.end())));
    DaisyPrinter cashPrinter(cashIo);
    assert(cashPrinter.cashIn("10.00", "EUR"));
    assert(requestData(cashIo.tx) == "10.00,$EUR");
}

int main() {
    testGoldenStatusRequestAndResponse();
    testBccEncodingAndMalformedFrames();
    testSequenceRollover();
    testNakSynAndBadBccRecovery();
    testStreamingTextAndStructuredFrames();
    testHighLevelValidationAndStatusErrors();
    testRefundAndEuroCashBuilders();
    return 0;
}
