/**
 * DatecsProtocol.cpp
 * Core packet framing, checksum, send/receive implementation.
 */

#include "DatecsProtocol.h"

// ─── Encoding helpers ─────────────────────────────────────────────────────────

// Each nibble of a 16-bit word is sent as ASCII digit: nibble + 0x30
// Word occupies 4 bytes: high-byte-high-nibble first
void DatecsProtocol::encodeWord(uint16_t value, uint8_t out[4]) {
    out[0] = ((value >> 12) & 0x0F) + 0x30;
    out[1] = ((value >>  8) & 0x0F) + 0x30;
    out[2] = ((value >>  4) & 0x0F) + 0x30;
    out[3] = ( value        & 0x0F) + 0x30;
}

uint16_t DatecsProtocol::decodeWord(const uint8_t in[4]) {
    uint16_t v = 0;
    for (int i = 0; i < 4; i++) {
        v = (v << 4) | ((in[i] - 0x30) & 0x0F);
    }
    return v;
}

// BCC = sum of bytes buf[start..end] inclusive, as uint16_t (wraps)
uint16_t DatecsProtocol::computeBCC(const uint8_t* buf, uint16_t start, uint16_t end) {
    uint16_t sum = 0;
    for (uint16_t i = start; i <= end; i++) {
        sum += buf[i];
    }
    return sum;
}

// ─── Packet construction ──────────────────────────────────────────────────────

/*
 * Request layout:
 *  [0]      PRE  = 0x01
 *  [1..4]   LEN  = encode(dataLen + 4(CMD) + 1(SEQ) + 1(PST) + 0x20)
 *  [5]      SEQ
 *  [6..9]   CMD  = encode(cmd)
 *  [10..N]  DATA (dataLen bytes)
 *  [N+1]    PST  = 0x05
 *  [N+2..N+5] BCC = encode(sum of bytes [1..N+1])
 *  [N+6]    EOT  = 0x03
 */
size_t DatecsProtocol::sendPacket(uint16_t cmd, const char* data, uint16_t dataLen) {
    if (data && dataLen == 0) dataLen = (uint16_t)strlen(data);
    if ((!data && dataLen != 0) || dataLen > DATECS_MAX_DATA_TX) return 0;

    // LEN = bytes from after PRE to PST (inclusive) + 0x20 offset
    // LEN itself is included: LEN(4) + SEQ(1) + CMD(4) + DATA(n) + PST(1).
    uint16_t rawLen = 4 + 1 + 4 + dataLen + 1 + DATECS_LEN_OFFSET;

    uint16_t pos = 0;
    _txBuf[pos++] = DATECS_PRE;
    encodeWord(rawLen, &_txBuf[pos]); pos += 4;
    _txBuf[pos++] = _seq;
    encodeWord(cmd, &_txBuf[pos]); pos += 4;
    if (data && dataLen > 0) {
        memcpy(&_txBuf[pos], data, dataLen);
        pos += dataLen;
    }
    _txBuf[pos++] = DATECS_PST;

    // BCC covers bytes [1..pos-1] (after PRE, up to and including PST)
    uint16_t bcc = computeBCC(_txBuf, 1, pos - 1);
    encodeWord(bcc, &_txBuf[pos]); pos += 4;
    _txBuf[pos++] = DATECS_EOT;

    _txLen = pos;
    return _serial.write(_txBuf, _txLen) == _txLen ? _txLen : 0;
}

// ─── Receive helpers ──────────────────────────────────────────────────────────

bool DatecsProtocol::_waitByte(uint8_t& b, uint16_t timeoutMs) {
    unsigned long started = millis();
    while ((unsigned long)(millis() - started) < timeoutMs) {
        if (_serial.available()) {
            b = (uint8_t)_serial.read();
            return true;
        }
    }
    return false;
}

// Read bytes into _rxBuf until EOT is found or buffer overflows or timeout
bool DatecsProtocol::_readUntilEOT(uint16_t timeoutMs) {
    _rxOverflow = false;
    unsigned long started = millis();
    while ((unsigned long)(millis() - started) < timeoutMs) {
        if (_serial.available()) {
            uint8_t b = (uint8_t)_serial.read();
            if (_rxLen >= DATECS_RX_BUF_SIZE) {
                _rxOverflow = true;
                return false;
            }
            _rxBuf[_rxLen++] = b;
            if (b == DATECS_EOT) return true;
        }
    }
    return false;
}

// ─── Packet parsing ───────────────────────────────────────────────────────────

/*
 * Response layout:
 *  [0]       PRE  = 0x01
 *  [1..4]    LEN
 *  [5]       SEQ
 *  [6..9]    CMD
 *  [10..M]   DATA
 *  [M+1]     SEP  = 0x04
 *  [M+2..M+9] STAT (8 raw bytes, each nibble + 0x80)
 *  [M+10]    PST  = 0x05
 *  [M+11..M+14] BCC
 *  [M+15]    EOT  = 0x03
 *
 * Status bytes arrive as raw bytes with bit7=1 always set.
 * LEN field tells us the byte count from [1] through PST (inclusive) + 0x20.
 */
bool DatecsProtocol::receivePacket(DatecsResponse& resp) {
    resp.clear();

    uint8_t firstByte;
    unsigned long windowStarted = millis();

    // Drain SYN bytes (device still processing); wait for PRE
    while (true) {
        unsigned long elapsed = (unsigned long)(millis() - windowStarted);
        if (elapsed >= _timeout || !_waitByte(firstByte, (uint16_t)(_timeout - elapsed))) {
            resp.localError = ERR_TIMEOUT;
            return false;
        }
        if (firstByte == DATECS_NAK) {
            resp.localError = ERR_NAK;
            return false;
        }
        if (firstByte == DATECS_SYN) {
            // Device busy; reset timeout window on each SYN
            windowStarted = millis();
            continue;
        }
        if (firstByte == DATECS_PRE) break;
        // Unexpected byte – keep waiting
    }

    // PRE received; read remainder into _rxBuf starting from byte index 1
    _rxBuf[0] = DATECS_PRE;
    _rxLen = 1;

    // Read until EOT; remaining timeout
    unsigned long elapsed = (unsigned long)(millis() - windowStarted);
    uint16_t remaining = elapsed >= _timeout ? 0 : (uint16_t)(_timeout - elapsed);
    if (!_readUntilEOT(remaining)) {
        resp.localError = _rxOverflow ? ERR_BUFFER_OVERFLOW : ERR_TIMEOUT;
        return false;
    }

    // Minimum valid response: PRE(1)+LEN(4)+SEQ(1)+CMD(4)+SEP(1)+STAT(8)+PST(1)+BCC(4)+EOT(1) = 25 bytes
    if (_rxLen < 25) {
        resp.localError = ERR_INVALID_RESPONSE;
        return false;
    }

    // Decode LEN
    for (uint8_t i = 1; i <= 4; ++i) {
        if (_rxBuf[i] < 0x30 || _rxBuf[i] > 0x3F) {
            resp.localError = ERR_INVALID_RESPONSE;
            return false;
        }
    }
    uint16_t encodedLen = decodeWord(&_rxBuf[1]);
    if (encodedLen < DATECS_LEN_OFFSET) {
        resp.localError = ERR_INVALID_RESPONSE;
        return false;
    }
    uint16_t rawLen     = encodedLen - DATECS_LEN_OFFSET;
    // rawLen includes LEN(4)+SEQ(1)+CMD(4)+DATA+SEP(1)+STAT(8)+PST(1).
    if (rawLen < 19) {
        resp.localError = ERR_INVALID_RESPONSE;
        return false;
    }
    uint16_t dataLen = rawLen - 19;
    if (dataLen > DATECS_MAX_DATA_RX || _rxLen != (uint16_t)(rawLen + 6)) {
        resp.localError = ERR_INVALID_RESPONSE;
        return false;
    }

    // Verify BCC
    // PST position = 1(PRE) + 4(LEN) + rawLen - 1  = 4 + rawLen
    uint16_t pstPos = rawLen;  // PRE is index 0; rawLen ends at PST.
    if (pstPos + 5 >= _rxLen || _rxBuf[pstPos] != DATECS_PST ||
        _rxBuf[_rxLen - 1] != DATECS_EOT) {
        resp.localError = ERR_INVALID_RESPONSE;
        return false;
    }
    uint16_t bccCalc = computeBCC(_rxBuf, 1, pstPos);
    for (uint8_t i = 1; i <= 4; ++i) {
        if (_rxBuf[pstPos + i] < 0x30 || _rxBuf[pstPos + i] > 0x3F) {
            resp.localError = ERR_INVALID_RESPONSE;
            return false;
        }
    }
    uint16_t bccRecv = decodeWord(&_rxBuf[pstPos + 1]);
    if (bccCalc != bccRecv) {
        resp.localError = ERR_BCC_MISMATCH;
        return false;
    }

    // Parse SEQ and CMD
    resp.seq = _rxBuf[5];
    if (resp.seq < DATECS_SEQ_MIN) {
        resp.localError = ERR_INVALID_RESPONSE;
        return false;
    }
    for (uint8_t i = 6; i <= 9; ++i) {
        if (_rxBuf[i] < 0x30 || _rxBuf[i] > 0x3F) {
            resp.localError = ERR_INVALID_RESPONSE;
            return false;
        }
    }
    resp.cmd = decodeWord(&_rxBuf[6]);

    // Copy DATA
    resp.dataLen = dataLen;
    if (dataLen > 0) {
        memcpy(resp.data, &_rxBuf[10], dataLen);
    }
    resp.data[dataLen] = '\0';

    // Parse STAT – 8 bytes immediately after SEP
    // SEP is at index 10 + dataLen
    uint16_t sepPos = 10 + dataLen;
    if (_rxBuf[sepPos] != DATECS_SEP) {
        resp.localError = ERR_INVALID_RESPONSE;
        return false;
    }
    for (uint8_t i = 0; i < DATECS_STATUS_LEN; i++) {
        if ((_rxBuf[sepPos + 1 + i] & 0x80) == 0) {
            resp.localError = ERR_INVALID_RESPONSE;
            return false;
        }
        resp.status.raw[i] = _rxBuf[sepPos + 1 + i];
    }

    // Parse device ErrorCode from data (first integer token)
    resp.deviceError = parseErrorCode(resp.data);

    resp.localError = ERR_OK;
    return true;
}

// ─── Execute (with retries) ───────────────────────────────────────────────────

bool DatecsProtocol::execute(uint16_t cmd, const char* data,
                              uint16_t dataLen, DatecsResponse& resp) {
    uint8_t savedSeq = _seq;
    for (uint8_t attempt = 0; attempt < _retries; attempt++) {
        if ((!data && dataLen != 0) || dataLen > DATECS_MAX_DATA_TX) {
            resp.clear();
            resp.localError = ERR_BUFFER_OVERFLOW;
            return false;
        }
        if (sendPacket(cmd, data, dataLen) == 0) {
            resp.clear();
            resp.localError = ERR_WRITE_FAILED;
            return false;
        }
        if (receivePacket(resp)) {
            // Validate SEQ and CMD echo
            if (resp.seq != _seq) {
                resp.localError = ERR_SEQ_MISMATCH;
                // Don't retry on mismatch – protocol confused
                break;
            }
            if (resp.cmd != cmd) {
                resp.localError = ERR_CMD_MISMATCH;
                break;
            }
            _advanceSeq();
            return true;
        }
        // On NAK or timeout: retransmit with same SEQ (do NOT advance)
        if (resp.localError == ERR_NAK || resp.localError == ERR_TIMEOUT) {
            continue;
        }
        // Other local errors: abort
        break;
    }
    // All retries exhausted or unrecoverable error
    _seq = savedSeq; // keep seq stable so caller can retry at higher level
    return false;
}

// ─── Utilities ────────────────────────────────────────────────────────────────

void DatecsProtocol::_advanceSeq() {
    _seq = (_seq == DATECS_SEQ_MAX) ? DATECS_SEQ_MIN : (uint8_t)(_seq + 1);
}

int32_t DatecsProtocol::parseErrorCode(const char* data) {
    if (!data || data[0] == '\0') return 0;
    // ErrorCode is the first token before '\t' or end of string
    return (int32_t)atol(data);
}

const char* DatecsProtocol::errorString(DatecsError err) {
    switch (err) {
        case ERR_OK:               return "OK";
        case ERR_TIMEOUT:          return "Timeout";
        case ERR_NAK:              return "NAK received";
        case ERR_INVALID_RESPONSE: return "Invalid response";
        case ERR_BCC_MISMATCH:     return "BCC mismatch";
        case ERR_BUFFER_OVERFLOW:  return "Buffer overflow";
        case ERR_SEQ_MISMATCH:     return "SEQ mismatch";
        case ERR_CMD_MISMATCH:     return "CMD mismatch";
        case ERR_WRITE_FAILED:     return "Serial write failed";
        default:                   return "Unknown error";
    }
}
