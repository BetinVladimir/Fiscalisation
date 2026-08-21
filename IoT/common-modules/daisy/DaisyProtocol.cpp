/**
 * DaisyProtocol.cpp
 * Core framing, BCC, send/receive for Daisy Tech protocol v2.0-4 (2026).
 */

#include "DaisyProtocol.h"

// ─── BCC encoding ─────────────────────────────────────────────────────────────
// BCC is the arithmetic sum of bytes from LEN to PST1 (inclusive),
// Each nibble is encoded by adding 0x30 (range 0x30..0x3F).

void DaisyProtocol::encodeBCC(uint16_t val, uint8_t out[4]) {
    out[0] = (uint8_t)(0x30 + ((val >> 12) & 0x0F));
    out[1] = (uint8_t)(0x30 + ((val >>  8) & 0x0F));
    out[2] = (uint8_t)(0x30 + ((val >>  4) & 0x0F));
    out[3] = (uint8_t)(0x30 + ( val        & 0x0F));
}

uint16_t DaisyProtocol::decodeBCC(const uint8_t in[4]) {
    uint16_t value = 0;
    return decodeBCC(in, value) ? value : 0;
}

bool DaisyProtocol::decodeBCC(const uint8_t in[4], uint16_t& value) {
    if (in == nullptr) return false;
    uint16_t v = 0;
    for (int i = 0; i < 4; i++) {
        if (in[i] < 0x30 || in[i] > 0x3F) return false;
        v = (uint16_t)((v << 4) | (in[i] - 0x30));
    }
    value = v;
    return true;
}

uint16_t DaisyProtocol::checksum(const uint8_t* buf, uint8_t from, uint8_t to) {
    uint16_t sum = 0;
    for (uint8_t i = from; i <= to; i++) sum += buf[i];
    return sum;
}

// ─── Packet send ──────────────────────────────────────────────────────────────
/*
 * TX layout (byte indices):
 *  [0]     STX = 01h
 *  [1]     LEN = LEN+SEQ+CMD+DATA+PST1 + 0x20 = dataLen + 0x24
 *  [2]     SEQ
 *  [3]     CMD
 *  [4..4+dataLen-1]  DATA
 *  [4+dataLen]       PST1 = 05h
 *  [5+dataLen..8+dataLen]  BCC (4 bytes)
 *  [9+dataLen]       ETX = 03h
 *
 * BCC covers indices [1 .. 5+dataLen] i.e. LEN through PST1 inclusive.
 */
size_t DaisyProtocol::sendPacket(uint8_t cmd, const char* data, uint8_t dataLen) {
    if (cmd < 0x20 || (data == nullptr && dataLen != 0) || dataLen > DAISY_MAX_DATA) return 0;

    // LEN includes its own byte: LEN+SEQ+CMD+DATA+PST1 + offset(0x20)
    uint16_t rawLen = 1 + 1 + 1 + dataLen + 1;
    uint8_t lenByte = (rawLen + DAISY_LEN_OFFSET > 0xFF) ? 0xFF
                    : (uint8_t)(rawLen + DAISY_LEN_OFFSET);

    uint8_t pos = 0;
    _txBuf[pos++] = DAISY_STX;
    _txBuf[pos++] = lenByte;           // [1] LEN
    _txBuf[pos++] = _seq;              // [2] SEQ
    _txBuf[pos++] = cmd;               // [3] CMD
    if (data && dataLen > 0) {
        memcpy(&_txBuf[pos], data, dataLen);
        pos += dataLen;
    }
    _txBuf[pos++] = DAISY_PST1;        // PST1

    // BCC: from index 1 (LEN) to index pos-1 (PST1)
    uint16_t bcc = checksum(_txBuf, 1, pos - 1);
    encodeBCC(bcc, &_txBuf[pos]); pos += 4;
    _txBuf[pos++] = DAISY_ETX;

    _txLen = pos;
    return _writeAll(_txBuf, _txLen) ? _txLen : 0;
}

bool DaisyProtocol::_writeAll(const uint8_t* data, uint8_t len) {
    if (data == nullptr || len == 0) return false;
    uint16_t offset = 0;
    while (offset < len) {
        size_t written = _serial.write(data + offset, len - offset);
        if (written == 0 || written > (size_t)(len - offset)) return false;
        offset = (uint16_t)(offset + written);
    }
    return true;
}

// ─── Low-level receive helpers ────────────────────────────────────────────────

bool DaisyProtocol::_waitByte(uint8_t& b, uint16_t ms) {
    unsigned long deadline = millis() + ms;
    while (millis() < deadline) {
        if (_serial.available()) {
            b = (uint8_t)_serial.read();
            return true;
        }
    }
    return false;
}

// Reads bytes into _rxBuf until ETX is found or buffer full or timeout.
// Assumes first byte (STX) is already read and stored in _rxBuf[0].
bool DaisyProtocol::_readUntilETX(uint16_t ms) {
    unsigned long deadline = millis() + ms;
    while (millis() < deadline) {
        if (_serial.available()) {
            uint8_t b = (uint8_t)_serial.read();
            if (_rxLen >= DAISY_RX_BUF) return false;
            _rxBuf[_rxLen++] = b;
            if (b == DAISY_ETX) return true;
        }
    }
    return false;
}

// ─── Packet receive & parse ───────────────────────────────────────────────────
/*
 * RX layout:
 *  [0]     STX = 01h
 *  [1]     LEN
 *  [2]     SEQ
 *  [3]     CMD
 *  [4..4+dataLen-1]  DATA
 *  [4+dataLen]       PST2 = 04h
 *  [5+dataLen..10+dataLen]  STATUS (6 bytes)
 *  [11+dataLen]      PST1 = 05h
 *  [12+dataLen..15+dataLen] BCC (4 bytes)
 *  [16+dataLen]      ETX = 03h
 *
 * LEN = LEN(1)+SEQ(1)+CMD(1)+DATA(n)+PST2(1)+STATUS(6)+PST1(1) + 0x20
 * So dataLen = LEN - 0x20 - 11  (if LEN != 0xFF)
 *
 * BCC covers [1..PST1] i.e. LEN through PST1 inclusive.
 */
bool DaisyProtocol::receivePacket(DaisyResponse& resp) {
    resp.clear();

    uint8_t b;
    unsigned long deadline = millis() + _timeout;

    // Wait for STX, transparently swallow SYN; bail on NAK
    while (true) {
        uint16_t remaining = (uint16_t)max(0L, (long)(deadline - millis()));
        if (!_waitByte(b, remaining)) {
            resp.localError = DAISY_ERR_TIMEOUT;
            return false;
        }
        if (b == DAISY_SYN) {
            // Device busy – extend deadline by one timeout window
            deadline = millis() + _timeout;
            continue;
        }
        if (b == DAISY_NAK) {
            resp.localError = DAISY_ERR_NAK;
            return false;
        }
        if (b == 0x1A || b == 0x1B) {
            resp.localError = DAISY_ERR_BAD_FRAME;
            return false;
        }
        if (b == DAISY_STX) break;
        // Stray byte — keep waiting
    }

    _rxBuf[0] = DAISY_STX;
    _rxLen = 1;

    uint16_t remaining2 = (uint16_t)max(0L, (long)(deadline - millis()));
    if (!_readUntilETX(remaining2)) {
        resp.localError = (_rxLen >= DAISY_RX_BUF) ? DAISY_ERR_OVERFLOW : DAISY_ERR_TIMEOUT;
        return false;
    }

    return _parseBufferedPacket(resp);
}

bool DaisyProtocol::_parseBufferedPacket(DaisyResponse& resp) {
    // Minimum valid response without data:
    // STX(1)+LEN(1)+SEQ(1)+CMD(1)+PST2(1)+STAT(6)+PST1(1)+BCC(4)+ETX(1) = 17
    if (_rxLen < 17) {
        resp.localError = DAISY_ERR_BAD_FRAME;
        return false;
    }

    uint8_t lenByte = _rxBuf[1];

    // Derive dataLen from LEN field
    // LEN includes itself and equals DATA + 11 + 0x20.
    // Unless LEN == 0xFF (overflow case)
    uint8_t dataLen;
    if (lenByte == DAISY_LEN_MAX) {
        // Calculate from actual packet length:
        // Total = STX(1)+LEN(1)+SEQ(1)+CMD(1)+DATA+PST2(1)+STAT(6)+PST1(1)+BCC(4)+ETX(1)
        // dataLen = total - 17
        dataLen = (uint8_t)(_rxLen > 17 ? _rxLen - 17 : 0);
    } else {
        int16_t dl = (int16_t)lenByte - DAISY_LEN_OFFSET - 11;
        if (dl < 0) { resp.localError = DAISY_ERR_BAD_FRAME; return false; }
        dataLen = (uint8_t)dl;
    }

    // Verify packet length consistency
    // Expected total = 17 + dataLen
    if (_rxLen != (uint8_t)(17 + dataLen) || _rxBuf[_rxLen - 1] != DAISY_ETX) {
        resp.localError = DAISY_ERR_BAD_FRAME;
        return false;
    }

    uint8_t pst2Pos = 4 + dataLen;
    if (_rxBuf[pst2Pos] != DAISY_PST2) {
        resp.localError = DAISY_ERR_BAD_FRAME;
        return false;
    }

    // PST1 position = 4 + dataLen + 1(PST2) + 6(STATUS) = 11 + dataLen
    uint8_t pst1Pos = 11 + dataLen;
    if (_rxBuf[pst1Pos] != DAISY_PST1) {
        resp.localError = DAISY_ERR_BAD_FRAME;
        return false;
    }

    // Verify BCC: sum of [1..pst1Pos]
    uint16_t calcBCC = checksum(_rxBuf, 1, pst1Pos);
    uint16_t recvBCC = 0;
    if (!decodeBCC(&_rxBuf[pst1Pos + 1], recvBCC) || calcBCC != recvBCC) {
        resp.localError = DAISY_ERR_BCC;
        return false;
    }

    // Extract fields
    resp.seq     = _rxBuf[2];
    resp.cmd     = _rxBuf[3];
    resp.dataLen = dataLen;
    if (dataLen > 0) {
        memcpy(resp.data, &_rxBuf[4], dataLen);
    }
    resp.data[dataLen] = '\0';

    // STATUS: 6 bytes starting at index 5 + dataLen  (after PST2)
    uint8_t statOff = 5 + dataLen;
    for (uint8_t i = 0; i < DAISY_STATUS_LEN; i++) {
        if ((_rxBuf[statOff + i] & 0x80) == 0) {
            resp.localError = DAISY_ERR_BAD_FRAME;
            return false;
        }
        resp.status.raw[i] = _rxBuf[statOff + i];
    }

    resp.localError = DAISY_OK;
    return true;
}

// ─── Execute with retries ─────────────────────────────────────────────────────

bool DaisyProtocol::execute(uint8_t cmd, const char* data,
                             uint8_t dataLen, DaisyResponse& resp) {
    for (uint8_t attempt = 0; attempt < _retries; attempt++) {
        if (sendPacket(cmd, data, dataLen) == 0) {
            resp.clear();
            resp.localError = (dataLen > DAISY_MAX_DATA || (data == nullptr && dataLen != 0))
                ? DAISY_ERR_INVALID_ARG : DAISY_ERR_WRITE;
            return false;
        }

        if (!receivePacket(resp)) {
            if (resp.localError == DAISY_ERR_NAK ||
                resp.localError == DAISY_ERR_TIMEOUT ||
                resp.localError == DAISY_ERR_BCC ||
                resp.localError == DAISY_ERR_BAD_FRAME) {
                // Retransmit with same SEQ (protocol requirement)
                continue;
            }
            return false; // unrecoverable local error
        }

        // Check SEQ and CMD echo
        if (resp.seq != _seq) { resp.localError = DAISY_ERR_SEQ; return false; }
        if (resp.cmd != cmd)  { resp.localError = DAISY_ERR_CMD; return false; }

        _advanceSeq();
        return true;
    }
    return false; // all retries exhausted
}

bool DaisyProtocol::execute(uint8_t cmd, const char* data, DaisyResponse& resp) {
    if (data == nullptr) return execute(cmd, nullptr, 0, resp);
    size_t len = strlen(data);
    if (len > DAISY_MAX_DATA) {
        resp.clear();
        resp.localError = DAISY_ERR_OVERFLOW;
        return false;
    }
    return execute(cmd, data, (uint8_t)len, resp);
}

// ─── Streaming mode ───────────────────────────────────────────────────────────

void DaisyProtocol::sendAck() {
    const uint8_t ack = DAISY_DC1;
    _writeAll(&ack, 1);
}

/*
 * Streamed lines from CMD_SEND_REPORT_TEXT (99h) and CMD_EJT_REPORTS (C3h):
 *
 * Normal line:  1Ah LineNo(6) 09h Font(1) 09h TEXT CR LF
 * End marker:   1Ah 09h 'E' CR LF
 *
 * Structured (C3h 'C'):  1Bh LEN(1) TEXT...
 *
 * Returns: 1 = line in lineBuf, 0 = final packed response in resp, -1 = error
 */
int8_t DaisyProtocol::receiveReportLine(char* lineBuf, uint16_t bufLen,
                                         DaisyResponse& resp, uint16_t timeoutMs) {
    if (lineBuf == nullptr || bufLen == 0) {
        resp.localError = DAISY_ERR_INVALID_ARG;
        return -1;
    }
    uint8_t b;
    while (true) {
        if (!_waitByte(b, timeoutMs)) {
            resp.localError = DAISY_ERR_TIMEOUT;
            return -1;
        }
        if (b == DAISY_SYN) continue;
        break;
    }

    if (b == DAISY_NAK) {
        resp.localError = DAISY_ERR_NAK;
        return -2;
    }

    if (b == DAISY_STX) {
        // Final packed response
        _rxBuf[0] = DAISY_STX;
        _rxLen = 1;
        unsigned long dl = millis() + timeoutMs;
        uint16_t rem = (uint16_t)max(0L, (long)(dl - millis()));
        if (!_readUntilETX(rem)) {
            resp.localError = (_rxLen >= DAISY_RX_BUF) ? DAISY_ERR_OVERFLOW : DAISY_ERR_TIMEOUT;
            return -1;
        }
        resp.clear();
        return _parseBufferedPacket(resp) ? 0 : -1;
    }

    if (b == 0x1B) {
        uint8_t encodedLen = 0;
        if (!_waitByte(encodedLen, timeoutMs)) {
            resp.localError = DAISY_ERR_TIMEOUT;
            return -1;
        }
        uint16_t dataLen = encodedLen;
        bool overflow = dataLen >= bufLen;
        for (uint16_t i = 0; i < dataLen; ++i) {
            uint8_t c = 0;
            if (!_waitByte(c, timeoutMs)) {
                resp.localError = DAISY_ERR_TIMEOUT;
                return -1;
            }
            if (i < bufLen - 1) lineBuf[i] = (char)c;
        }
        lineBuf[overflow ? bufLen - 1 : dataLen] = '\0';
        if (overflow) {
            resp.localError = DAISY_ERR_OVERFLOW;
            return -1;
        }
        resp.dataLen = (uint8_t)dataLen;
        return 2;
    }

    if (b == 0x1A) {
        uint16_t pos = 0;
        bool terminated = false;
        bool overflow = false;
        unsigned long deadline2 = millis() + timeoutMs;
        while (millis() < deadline2) {
            if (_serial.available()) {
                uint8_t c = (uint8_t)_serial.read();
                if (pos < bufLen - 1) lineBuf[pos++] = (char)c;
                else overflow = true;
                if (c == 0x0A) { terminated = true; break; }
            }
        }
        lineBuf[pos] = '\0';
        if (!terminated) { resp.localError = DAISY_ERR_TIMEOUT; return -1; }
        if (overflow) { resp.localError = DAISY_ERR_OVERFLOW; return -1; }
        resp.dataLen = (uint8_t)pos;
        return 1;
    }

    // Unexpected byte
    return -1;
}

bool DaisyProtocol::executeStreaming(uint8_t cmd, const char* data, uint8_t dataLen,
                                      ReportChunkCallback callback, void* context,
                                      DaisyResponse& finalResponse,
                                      uint16_t lineTimeoutMs) {
    if (callback == nullptr || (data == nullptr && dataLen != 0) || dataLen > DAISY_MAX_DATA) {
        finalResponse.clear();
        finalResponse.localError = DAISY_ERR_INVALID_ARG;
        return false;
    }
    for (uint8_t attempt = 0; attempt < _retries; ++attempt) {
        if (sendPacket(cmd, data, dataLen) == 0) {
            finalResponse.clear();
            finalResponse.localError = DAISY_ERR_WRITE;
            return false;
        }
        bool acceptedChunk = false;
        while (true) {
            char chunk[256];
            int8_t kind = receiveReportLine(chunk, sizeof(chunk), finalResponse, lineTimeoutMs);
            if (kind == 0) {
                if (finalResponse.seq != _seq) { finalResponse.localError = DAISY_ERR_SEQ; return false; }
                if (finalResponse.cmd != cmd) { finalResponse.localError = DAISY_ERR_CMD; return false; }
                _advanceSeq();
                return true;
            }
            if (kind == 1 || kind == 2) {
                uint16_t length = finalResponse.dataLen;
                if (!callback((uint8_t)kind, (const uint8_t*)chunk, length, context)) {
                    finalResponse.localError = DAISY_ERR_INVALID_ARG;
                    return false;
                }
                const uint8_t ack = DAISY_DC1;
                if (!_writeAll(&ack, 1)) {
                    finalResponse.localError = DAISY_ERR_WRITE;
                    return false;
                }
                acceptedChunk = true;
                continue;
            }
            if (!acceptedChunk && (finalResponse.localError == DAISY_ERR_NAK ||
                                   finalResponse.localError == DAISY_ERR_TIMEOUT)) break;
            return false;
        }
    }
    return false;
}

// ─── Utilities ────────────────────────────────────────────────────────────────

void DaisyProtocol::_advanceSeq() {
    _seq = (_seq == DAISY_SEQ_MAX) ? DAISY_SEQ_MIN : (uint8_t)(_seq + 1);
}

const char* DaisyProtocol::errorString(DaisyError e) {
    switch (e) {
        case DAISY_OK:            return "OK";
        case DAISY_ERR_TIMEOUT:   return "Timeout";
        case DAISY_ERR_NAK:       return "NAK";
        case DAISY_ERR_BAD_FRAME: return "Bad frame";
        case DAISY_ERR_BCC:       return "BCC error";
        case DAISY_ERR_SEQ:       return "SEQ mismatch";
        case DAISY_ERR_CMD:       return "CMD mismatch";
        case DAISY_ERR_OVERFLOW:  return "Data overflow";
        case DAISY_ERR_INVALID_ARG:return "Invalid argument";
        case DAISY_ERR_WRITE:     return "Stream write failed";
        default:                  return "Unknown";
    }
}
