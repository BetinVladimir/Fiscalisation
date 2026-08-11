/**
 * TremolProtocol.cpp
 * Core framing, checksum, send/receive for Tremol FP protocol v2507141400.
 *
 * KEY DIFFERENCES from Datecs / Daisy Tech:
 *  - STX = 02h (not 01h)
 *  - ETX = 0Ah/LF (not 03h)
 *  - CS  = XOR (not SUM), nibbles + 30h
 *  - DATA separator = ';' (not tab)
 *  - Sequence field is NBL (not SEQ), range 20h..9Fh
 *  - Two separate response types: ACK (06h) and message (STX)
 *  - RETRY = 0Eh (FD busy) — distinct from NACK
 */

#include "TremolProtocol.h"
#include <stdarg.h>

// ─── Checksum ─────────────────────────────────────────────────────────────────
// XOR of bytes from LEN through last DATA byte (not including CS or ETX)
// Then encode: high nibble + 30h, low nibble + 30h

uint8_t TremolProtocol::xorRange(const uint8_t* buf, uint8_t from, uint8_t to) {
    uint8_t cs = 0;
    for (uint8_t i = from; i <= to; i++) cs ^= buf[i];
    return cs;
}

void TremolProtocol::encodeCS(uint8_t val, uint8_t out[2]) {
    out[0] = ((val >> 4) & 0x0F) + 0x30;
    out[1] = ( val       & 0x0F) + 0x30;
}

uint8_t TremolProtocol::decodeCS(const uint8_t in[2]) {
    return (uint8_t)(((in[0] - 0x30) << 4) | (in[1] - 0x30));
}

// ─── Data string builder ──────────────────────────────────────────────────────
// Joins varargs (const char*) with ';', null-terminated list.
// Returns total chars written (excluding null terminator).

uint8_t TremolProtocol::buildData(char* out, uint8_t outSize, ...) {
    va_list ap;
    va_start(ap, outSize);
    uint8_t pos = 0;
    bool first = true;
    while (true) {
        const char* field = va_arg(ap, const char*);
        if (!field) break;  // nullptr = end sentinel
        if (!first && pos < outSize - 1) out[pos++] = TR_SEP;
        first = false;
        while (*field && pos < outSize - 1) out[pos++] = *field++;
    }
    va_end(ap);
    if (pos < outSize) out[pos] = '\0';
    return pos;
}

// ─── Packet send ──────────────────────────────────────────────────────────────
/*
 * TX layout:
 *  [0]       STX = 02h
 *  [1]       LEN = (1+1+1+dataLen) + 20h = dataLen + 23h  (NBL+CMD+DATA + offset)
 *              Note: LEN counts itself, NBL, CMD, DATA bytes
 *  [2]       NBL = _nbl
 *  [3]       CMD
 *  [4..4+dL-1] DATA
 *  [4+dL]    CS[0]  (high nibble of XOR + 30h)
 *  [5+dL]    CS[1]  (low  nibble of XOR + 30h)
 *  [6+dL]    ETX = 0Ah
 *
 * XOR covers bytes [1..3+dL] = LEN..last DATA byte
 */
size_t TremolProtocol::sendPacket(uint8_t cmd, const char* data) {
    uint8_t dataLen = (data && *data) ? (uint8_t)strlen(data) : 0;
    // safety clamp
    if (dataLen > TR_TX_BUF - 7) dataLen = TR_TX_BUF - 7;

    // LEN = LEN(1) + NBL(1) + CMD(1) + DATA(n) + 20h
    uint8_t lenByte = (uint8_t)(3 + dataLen + 0x20);

    uint8_t pos = 0;
    _txBuf[pos++] = TR_STX;               // [0] STX
    _txBuf[pos++] = lenByte;              // [1] LEN
    _txBuf[pos++] = _nbl;                 // [2] NBL
    _txBuf[pos++] = cmd;                  // [3] CMD
    if (dataLen > 0) {
        memcpy(&_txBuf[pos], data, dataLen);
        pos += dataLen;
    }
    // CS covers [1 .. pos-1]
    uint8_t xv = xorRange(_txBuf, 1, pos - 1);
    uint8_t cs[2];
    encodeCS(xv, cs);
    _txBuf[pos++] = cs[0];
    _txBuf[pos++] = cs[1];
    _txBuf[pos++] = TR_ETX;               // 0Ah LF

    _txLen = pos;
    _serial.write(_txBuf, _txLen);
    return _txLen;
}

// ─── Receive helpers ──────────────────────────────────────────────────────────

bool TremolProtocol::_waitByte(uint8_t& b, uint16_t ms) {
    unsigned long deadline = millis() + ms;
    while (millis() < deadline) {
        if (_serial.available()) { b = (uint8_t)_serial.read(); return true; }
    }
    return false;
}

// Read bytes into _rxBuf until ETX (0Ah) found or buffer full or timeout.
// First byte already stored as _rxBuf[0], _rxLen = 1.
bool TremolProtocol::_readUntilETX(uint16_t ms) {
    unsigned long deadline = millis() + ms;
    while (millis() < deadline) {
        if (_serial.available()) {
            uint8_t b = (uint8_t)_serial.read();
            if (_rxLen < TR_RX_BUF) _rxBuf[_rxLen++] = b;
            if (b == TR_ETX) return true;
        }
    }
    return false;
}

// ─── Parse ACK packet ─────────────────────────────────────────────────────────
// ACK layout: [0]=06h [1]=NBL [2]=STE0 [3]=STE1 [4]=CS0 [5]=CS1 [6]=0Ah
// Minimum length = 7 bytes
bool TremolProtocol::_parseACK(TremolResponse& resp) {
    if (_rxLen < 7) return false;
    // Verify CS covers [1..3] = NBL + STE0 + STE1
    uint8_t calcCS = xorRange(_rxBuf, 1, 3);
    uint8_t recvCS = decodeCS(&_rxBuf[4]);
    if (calcCS != recvCS) { resp.localError = TR_ERR_BAD_CS; return false; }

    resp.isAck  = true;
    resp.nbl    = _rxBuf[1];
    resp.ste0   = _rxBuf[2];
    resp.ste1   = _rxBuf[3];
    resp.dataLen = 0;
    resp.data[0] = '\0';
    return true;
}

// ─── Parse message response ───────────────────────────────────────────────────
// Layout: [0]=STX [1]=LEN [2]=NBL [3]=CMD [4..4+dL-1]=DATA [4+dL..4+dL+1]=CS [4+dL+2]=ETX
bool TremolProtocol::_parseMsgResponse(TremolResponse& resp) {
    if (_rxLen < 7) return false;

    uint8_t lenByte = _rxBuf[1];
    // dataLen = LEN - 20h - 3 (for NBL+CMD+LEN itself)
    int16_t dL = (int16_t)(lenByte - 0x20) - 3;
    if (dL < 0) { resp.localError = TR_ERR_BAD_FRAME; return false; }
    uint8_t dataLen = (uint8_t)dL;

    // Expected total: STX(1)+LEN(1)+NBL(1)+CMD(1)+DATA(dL)+CS(2)+ETX(1) = 7+dL
    if (_rxLen < (uint8_t)(7 + dataLen)) {
        resp.localError = TR_ERR_BAD_FRAME;
        return false;
    }

    // CS covers [1 .. 3+dL] = LEN..last DATA
    uint8_t calcCS = xorRange(_rxBuf, 1, 3 + dataLen);
    uint8_t recvCS = decodeCS(&_rxBuf[4 + dataLen]);
    if (calcCS != recvCS) { resp.localError = TR_ERR_BAD_CS; return false; }

    resp.isAck  = false;
    resp.nbl    = _rxBuf[2];
    resp.cmd    = _rxBuf[3];
    resp.dataLen = dataLen;
    if (dataLen > 0) {
        uint8_t copyLen = (dataLen < TR_RX_BUF - 1) ? dataLen : (uint8_t)(TR_RX_BUF - 1);
        memcpy(resp.data, &_rxBuf[4], copyLen);
        resp.data[copyLen] = '\0';
    } else {
        resp.data[0] = '\0';
    }
    // No STE in message response — status only in ACK
    resp.ste0 = '0';
    resp.ste1 = '0';
    return true;
}

// ─── Full receive & parse ─────────────────────────────────────────────────────

bool TremolProtocol::receivePacket(TremolResponse& resp) {
    resp.clear();
    uint8_t b;
    unsigned long deadline = millis() + _timeout;

    // Wait for first byte: ACK(06), STX(02), NACK(15), RETRY(0E)
    while (true) {
        uint16_t rem = (uint16_t)max(0L, (long)(deadline - millis()));
        if (!_waitByte(b, rem)) { resp.localError = TR_ERR_TIMEOUT; return false; }
        if (b == TR_NACK)  { resp.localError = TR_ERR_NACK;  return false; }
        if (b == TR_RETRY) {
            // FD busy: wait a bit, let caller retry with same NBL
            resp.localError = TR_ERR_NACK;  // same retry semantics
            return false;
        }
        if (b == TR_ACK || b == TR_STX) break;
        // Stray byte — keep waiting
    }

    _rxBuf[0] = b;
    _rxLen = 1;
    uint16_t rem2 = (uint16_t)max(0L, (long)(deadline - millis()));
    if (!_readUntilETX(rem2)) { resp.localError = TR_ERR_TIMEOUT; return false; }

    if (_rxBuf[0] == TR_ACK) return _parseACK(resp);
    return _parseMsgResponse(resp);
}

// ─── Execute with retries ─────────────────────────────────────────────────────

bool TremolProtocol::execute(uint8_t cmd, const char* data, TremolResponse& resp) {
    for (uint8_t attempt = 0; attempt < _maxRetry; attempt++) {
        sendPacket(cmd, data);
        if (receivePacket(resp)) {
            // Verify echoed NBL
            if (resp.nbl != _nbl) { resp.localError = TR_ERR_NBL; return false; }
            _advanceNBL();
            return true;
        }
        if (resp.localError == TR_ERR_NACK || resp.localError == TR_ERR_TIMEOUT) {
            // Retransmit same NBL
            delay(50);
            continue;
        }
        break; // unrecoverable
    }
    return false;
}

// ─── Short-form poll ──────────────────────────────────────────────────────────

uint8_t TremolProtocol::pollStatus(uint16_t timeoutMs) {
    _serial.write(TR_POLL);
    uint8_t b = 0xFF;
    _waitByte(b, timeoutMs);
    return b;
}

// ─── Utilities ────────────────────────────────────────────────────────────────

void TremolProtocol::_advanceNBL() {
    _nbl++;
    if (_nbl > TR_NBL_MAX) _nbl = TR_NBL_MIN;
}

const char* TremolProtocol::localErrorStr(int8_t err) {
    switch (err) {
        case TR_ERR_OK:         return "OK";
        case TR_ERR_TIMEOUT:    return "Timeout";
        case TR_ERR_NACK:       return "NACK/Retry";
        case TR_ERR_BAD_CS:     return "Bad checksum";
        case TR_ERR_BAD_FRAME:  return "Bad frame";
        case TR_ERR_NBL:        return "NBL mismatch";
        case TR_ERR_OVERFLOW:   return "Data overflow";
        default:                return "Unknown";
    }
}

// ─── TremolResponse helpers ───────────────────────────────────────────────────

const char* TremolResponse::fdErrorStr() const {
    switch (ste0) {
        case '0': return "OK";
        case '1': return "No paper / printer failure";
        case '2': return "Registers overflow";
        case '3': return "Clock failure / wrong date-time";
        case '4': return "Opened fiscal receipt";
        case '5': return "Payment residue";
        case '6': return "Opened non-fiscal receipt";
        case '7': return "Payment registered, receipt not closed";
        case '8': return "Fiscal memory failure";
        case '9': return "Wrong password";
        case 'a': return "Missing external display";
        case 'b': return "24h block - unprinted Z";
        case 'c': return "Printer overheated";
        case 'd': return "Power interrupt in fiscal receipt";
        case 'e': return "EJ overflow";
        case 'f': return "Insufficient conditions";
        default:  return "Unknown FD error";
    }
}

const char* TremolResponse::cmdErrorStr() const {
    switch (ste1) {
        case '0': return "OK";
        case '1': return "Invalid command";
        case '2': return "Illegal command (mode)";
        case '3': return "Z daily report not zeroed";
        case '4': return "Syntax error";
        case '5': return "Input registers overflow";
        case '6': return "Zero input registers";
        case '7': return "Unavailable for correction";
        case '8': return "Insufficient amount on hand";
        default:  return "Unknown command error";
    }
}
