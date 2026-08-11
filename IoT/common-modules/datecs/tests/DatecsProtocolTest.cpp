#include "Arduino.h"
#include "../DatecsProtocol.h"

#include <assert.h>
#include <deque>
#include <string>
#include <vector>

class FakeStream final : public Stream {
public:
    bool autoReply = true;
    std::deque<uint8_t> input;
    std::vector<std::vector<uint8_t>> writes;

    int available() override { return input.empty() ? 0 : (int)input.size(); }
    int read() override {
        if (input.empty()) return -1;
        uint8_t b = input.front();
        input.pop_front();
        return b;
    }
    size_t write(const uint8_t* buffer, size_t size) override {
        writes.emplace_back(buffer, buffer + size);
        if (autoReply && size >= 10) enqueueResponse(buffer[5], DatecsProtocol::decodeWord(buffer + 6), "0\t");
        return size;
    }
    void enqueueResponse(uint8_t seq, uint16_t cmd, const std::string& data, bool corruptBCC = false) {
        const uint16_t rawLen = (uint16_t)(4 + 1 + 4 + data.size() + 1 + 8 + 1);
        std::vector<uint8_t> frame;
        frame.push_back(DATECS_PRE);
        uint8_t word[4];
        DatecsProtocol::encodeWord((uint16_t)(rawLen + DATECS_LEN_OFFSET), word);
        frame.insert(frame.end(), word, word + 4);
        frame.push_back(seq);
        DatecsProtocol::encodeWord(cmd, word);
        frame.insert(frame.end(), word, word + 4);
        frame.insert(frame.end(), data.begin(), data.end());
        frame.push_back(DATECS_SEP);
        for (int i = 0; i < 8; ++i) frame.push_back(0x80);
        frame.push_back(DATECS_PST);
        uint16_t bcc = DatecsProtocol::computeBCC(frame.data(), 1, (uint16_t)(frame.size() - 1));
        if (corruptBCC) ++bcc;
        DatecsProtocol::encodeWord(bcc, word);
        frame.insert(frame.end(), word, word + 4);
        frame.push_back(DATECS_EOT);
        input.insert(input.end(), frame.begin(), frame.end());
    }
};

static std::string requestData(const std::vector<uint8_t>& frame) {
    uint16_t encoded = DatecsProtocol::decodeWord(frame.data() + 1);
    size_t dataLen = encoded - DATECS_LEN_OFFSET - 10;
    return std::string((const char*)frame.data() + 10, dataLen);
}

static void testGoldenRequestAndResponse() {
    FakeStream serial;
    DatecsProtocol protocol(serial);
    DatecsResponse response;
    assert(protocol.execute(CMD_CLEAR_DISPLAY, response));
    const auto& frame = serial.writes.back();
    assert(frame.size() == 16);
    assert(frame[0] == 0x01 && frame[5] == 0x20 && frame[10] == 0x05 && frame[15] == 0x03);
    assert(DatecsProtocol::decodeWord(frame.data() + 1) == 0x2A);
    assert(DatecsProtocol::decodeWord(frame.data() + 6) == 33);
    const uint8_t expected[] = {0x01,0x30,0x30,0x32,0x3A,0x20,0x30,0x30,0x32,0x31,0x05,0x30,0x31,0x3B,0x34,0x03};
    assert(frame == std::vector<uint8_t>(expected, expected + sizeof(expected)));
    assert(response.ok() && response.dataLen == 2 && std::string(response.data) == "0\t");
}

static void testVendorManualResponseVector() {
    FakeStream serial;
    serial.autoReply = false;
    const uint8_t manual[] = {
        0x01,0x30,0x30,0x33,0x35,0x24,0x30,0x30,0x32,0x31,0x30,0x09,0x04,
        0x80,0x80,0x80,0x80,0x86,0x9A,0x80,0x80,0x05,0x30,0x36,0x31,0x31,0x03
    };
    serial.input.insert(serial.input.end(), manual, manual + sizeof(manual));
    DatecsProtocol protocol(serial);
    DatecsResponse response;
    assert(protocol.receivePacket(response));
    assert(response.seq == 0x24 && response.cmd == CMD_CLEAR_DISPLAY);
    assert(response.ok() && response.status.fmSerialSet() && response.status.fiscalized());
}

static void testSequenceWrap() {
    FakeStream serial;
    DatecsProtocol protocol(serial);
    DatecsResponse response;
    for (int i = 0; i < 224; ++i) assert(protocol.execute(CMD_CLEAR_DISPLAY, response));
    assert(protocol.currentSeq() == DATECS_SEQ_MIN);
}

static void testMalformedResponseRejected() {
    FakeStream serial;
    serial.autoReply = false;
    serial.enqueueResponse(0x20, CMD_CLEAR_DISPLAY, "0\t", true);
    DatecsProtocol protocol(serial, 20, 1);
    DatecsResponse response;
    assert(!protocol.execute(CMD_CLEAR_DISPLAY, response));
    assert(response.localError == ERR_BCC_MISMATCH);
}

static void testFiscalCommandPayloads() {
    FakeStream serial;
    DatecsPrinter printer(serial);
    assert(printer.openFiscal(1, "12345678", 24, "DT636533-0020-0010110", false));
    assert(requestData(serial.writes.back()) == "1\t12345678\tDT636533-0020-0010110\t24\t\t");

    assert(printer.registerSale("Item", 2, "2.65", "3", 2, "5", 2, "pcs"));
    assert(requestData(serial.writes.back()) == "Item\t2\t2.65\t3\t2\t5\t2\tpcs\t");

    size_t before = serial.writes.size();
    assert(printer.payAndClose(0, "10.00"));
    assert(serial.writes.size() == before + 2);
    assert(DatecsProtocol::decodeWord(serial.writes[before].data() + 6) == CMD_TOTAL);
    assert(requestData(serial.writes[before]) == "0\t10.00\t");
    assert(DatecsProtocol::decodeWord(serial.writes[before + 1].data() + 6) == CMD_CLOSE_FISCAL);
    assert(requestData(serial.writes[before + 1]).empty());

    assert(printer.totalWithPinpad("12.50", 1));
    assert(requestData(serial.writes.back()) == "2\t12.50\t1\t");

    assert(printer.openStorno(1, "12345678", 24, 1, 428,
                              "24-04-19 08:36:27", "02636571",
                              "DT636497-0021-0010001"));
    assert(requestData(serial.writes.back()) ==
           "1\t12345678\t24\t1\t428\t24-04-19 08:36:27\t02636571\t\t\t\tDT636497-0021-0010001\t");
}

static void testDatetimeAndDeviceInfoParsing() {
    FakeStream serial;
    serial.autoReply = false;
    serial.enqueueResponse(0x20, CMD_READ_DATETIME, "0\t12-08-26 10:20:30\t");
    DatecsPrinter printer(serial);
    char datetime[32];
    assert(printer.readDatetime(datetime, sizeof(datetime)));
    assert(std::string(datetime) == "12-08-26 10:20:30");

    serial.autoReply = true;
    char info[32];
    assert(printer.deviceInfo(info, sizeof(info)));
    assert(requestData(serial.writes.back()) == "1\t");
}

int main() {
    testGoldenRequestAndResponse();
    testVendorManualResponseVector();
    testSequenceWrap();
    testMalformedResponseRejected();
    testFiscalCommandPayloads();
    testDatetimeAndDeviceInfoParsing();
    return 0;
}
