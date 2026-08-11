#include "DatecsPay.h"

// ============================================================
//  Error string table
// ============================================================
const char* datecs_error_str(DatecsError e) {
    switch (e) {
        case DatecsError::NoErr:          return "No error";
        case DatecsError::General:        return "General error";
        case DatecsError::InvCmd:         return "Invalid command/subcommand";
        case DatecsError::InvPar:         return "Invalid parameter";
        case DatecsError::InvAdr:         return "Address out of limits";
        case DatecsError::InvVal:         return "Value out of limits";
        case DatecsError::InvLen:         return "Length out of limits";
        case DatecsError::NoPermit:       return "Action not permitted in current state";
        case DatecsError::NoData:         return "No data to return";
        case DatecsError::TimeOut:        return "Timeout";
        case DatecsError::KeyNum:         return "Invalid key number";
        case DatecsError::KeyAttr:        return "Invalid key attributes";
        case DatecsError::InvDevice:      return "Non-existing device";
        case DatecsError::NoSupport:      return "Not supported";
        case DatecsError::PinLimit:       return "PIN entering limit exceeded";
        case DatecsError::Flash:          return "Flash error";
        case DatecsError::Hard:           return "Hardware error";
        case DatecsError::Cancel:         return "CANCEL pressed";
        case DatecsError::InvSign:        return "Invalid signature";
        case DatecsError::InvHead:        return "Invalid header";
        case DatecsError::InvPass:        return "Incorrect password";
        case DatecsError::KeyFormat:      return "Invalid key format";
        case DatecsError::SCR:            return "Smart card reader error";
        case DatecsError::HAL:            return "HAL error";
        case DatecsError::InvKey:         return "Invalid key (may be absent)";
        case DatecsError::NoPinData:      return "PIN length < 4 or > 12";
        case DatecsError::InvRemainder:   return "Invalid ICC key remainder";
        case DatecsError::NoPerm:         return "Action not permitted";
        case DatecsError::NoTMK:          return "TMK not loaded";
        case DatecsError::InvKek:         return "Wrong KEK format";
        case DatecsError::DubKey:         return "Duplicated key";
        case DatecsError::KBD:            return "Keyboard error";
        case DatecsError::KBDNoCal:       return "Keyboard not calibrated";
        case DatecsError::KBDBug:         return "Keyboard bug detected";
        case DatecsError::Busy:           return "Device busy, retry";
        case DatecsError::Tampered:       return "Security tampered";
        case DatecsError::Emsr:           return "Encrypted head error";
        case DatecsError::Accept:         return "OK pressed";
        case DatecsError::InvPAN:         return "Wrong PAN";
        case DatecsError::OutOfMemory:    return "Not enough memory";
        case DatecsError::EMV:            return "EMV error";
        case DatecsError::Crypt:          return "Cryptographic error";
        case DatecsError::ComRcv:         return "Data received";
        case DatecsError::WrongVer:       return "Wrong firmware version";
        case DatecsError::NoPaper:        return "Printer out of paper";
        case DatecsError::TooHot:         return "Printer overheating";
        case DatecsError::NoConnected:    return "Device not connected";
        case DatecsError::UseChip:        return "Use chip reader";
        case DatecsError::EndDay:         return "End of day required";
        case DatecsError::LibTimeout:     return "LIB: stream read timeout";
        case DatecsError::LibBufOverflow: return "LIB: receive buffer overflow";
        case DatecsError::LibBadCsum:     return "LIB: bad checksum";
        case DatecsError::LibBadStart:    return "LIB: bad start byte";
        case DatecsError::LibNullStream:  return "LIB: null stream pointer";
        case DatecsError::LibWriteFailed: return "LIB: stream write failed";
        case DatecsError::LibInvalidArg:  return "LIB: invalid argument";
        default:                          return "Unknown error";
    }
}

// ============================================================
//  TLVParser
// ============================================================
bool TLVParser::next(const uint8_t* buf, uint16_t bufLen,
                     uint16_t& offset, TLVItem& out) {
    if (offset >= bufLen) return false;

    // BER-TLV tags are one to three bytes in the Datecs Pay specifications.
    uint32_t tag;
    if ((buf[offset] & 0x1F) == 0x1F) {
        if (offset + 1 >= bufLen) return false;
        tag = ((uint16_t)buf[offset] << 8) | buf[offset + 1];
        offset += 2;
        if ((tag & 0x80) != 0) {
            if (offset >= bufLen) return false;
            tag = (tag << 8) | buf[offset++];
        }
    } else {
        tag = buf[offset++];
    }

    if (offset >= bufLen) return false;
    uint8_t len = buf[offset++];

    if (offset + len > bufLen) return false;

    out.tag   = tag;
    out.len   = len;
    out.value = &buf[offset];
    offset   += len;
    return true;
}

bool TLVParser::find(const uint8_t* buf, uint16_t bufLen,
                     uint32_t tag, TLVItem& out) {
    uint16_t offset = 0;
    TLVItem item;
    while (next(buf, bufLen, offset, item)) {
        if (item.tag == tag) {
            out = item;
            return true;
        }
    }
    return false;
}

uint32_t TLVParser::toUint32(const TLVItem& item) {
    uint32_t result = 0;
    uint8_t bytes = item.len > 4 ? 4 : item.len;
    for (uint8_t i = 0; i < bytes; i++) {
        result = (result << 8) | item.value[i];
    }
    return result;
}

void TLVParser::toString(const TLVItem& item, char* dst, uint8_t maxLen) {
    if (dst == nullptr || maxLen == 0) return;
    uint8_t copy = (item.len < maxLen - 1) ? item.len : (maxLen - 1);
    memcpy(dst, item.value, copy);
    dst[copy] = '\0';
}

// ============================================================
//  DatecsPacketBuilder
// ============================================================
DatecsPacketBuilder::DatecsPacketBuilder() : _cmd(0), _dataLen(0), _overflow(false) {}

void DatecsPacketBuilder::begin(uint8_t command) {
    _cmd     = command;
    _dataLen = 0;
    _overflow = false;
    memset(_buf, 0, sizeof(_buf));
}

DatecsPacketBuilder& DatecsPacketBuilder::append(uint8_t b) {
    if (_dataLen < (uint16_t)(sizeof(_buf))) {
        _buf[_dataLen++] = b;
    } else {
        _overflow = true;
    }
    return *this;
}

DatecsPacketBuilder& DatecsPacketBuilder::appendBytes(const uint8_t* data, uint16_t len) {
    if (data == nullptr && len != 0) {
        _overflow = true;
        return *this;
    }
    for (uint16_t i = 0; i < len; i++) append(data[i]);
    return *this;
}

DatecsPacketBuilder& DatecsPacketBuilder::appendUint16BE(uint16_t val) {
    append((uint8_t)(val >> 8));
    append((uint8_t)(val & 0xFF));
    return *this;
}

DatecsPacketBuilder& DatecsPacketBuilder::appendUint32BE(uint32_t val) {
    append((uint8_t)(val >> 24));
    append((uint8_t)(val >> 16));
    append((uint8_t)(val >> 8));
    append((uint8_t)(val & 0xFF));
    return *this;
}

DatecsPacketBuilder& DatecsPacketBuilder::appendString(const char* str, uint8_t len) {
    if (str == nullptr) { _overflow = true; return *this; }
    if (len == 0) {
        size_t rawLen = strlen(str);
        if (rawLen > 255) { _overflow = true; return *this; }
        len = (uint8_t)rawLen;
    }
    appendBytes((const uint8_t*)str, len);
    return *this;
}

DatecsPacketBuilder& DatecsPacketBuilder::appendTLV1(uint8_t tag, uint8_t value) {
    append(tag);
    append(0x01);
    append(value);
    return *this;
}

DatecsPacketBuilder& DatecsPacketBuilder::appendTLV2(uint16_t tag,
                                                      const uint8_t* value, uint8_t vlen) {
    return appendTLV(tag, value, vlen);
}

DatecsPacketBuilder& DatecsPacketBuilder::appendTLV(uint32_t tag,
                                                    const uint8_t* value, uint8_t vlen) {
    if (value == nullptr && vlen != 0) { _overflow = true; return *this; }
    if (tag > 0xFFFF) {
        append((uint8_t)(tag >> 16));
        append((uint8_t)(tag >> 8));
        append((uint8_t)tag);
    } else if (tag > 0xFF) {
        append((uint8_t)(tag >> 8));
        append((uint8_t)(tag & 0xFF));
    } else {
        append((uint8_t)tag);
    }
    append(vlen);
    appendBytes(value, vlen);
    return *this;
}

DatecsPacketBuilder& DatecsPacketBuilder::appendTLV_uint32(uint16_t tag, uint32_t value) {
    uint8_t v[4];
    v[0] = (value >> 24) & 0xFF;
    v[1] = (value >> 16) & 0xFF;
    v[2] = (value >> 8)  & 0xFF;
    v[3] =  value        & 0xFF;
    return appendTLV2(tag, v, 4);
}

DatecsPacketBuilder& DatecsPacketBuilder::appendTLV_string(uint16_t tag, const char* str) {
    if (str == nullptr) { _overflow = true; return *this; }
    size_t rawLen = strlen(str);
    if (rawLen > 255) { _overflow = true; return *this; }
    uint8_t len = (uint8_t)rawLen;
    return appendTLV2(tag, (const uint8_t*)str, len);
}

uint8_t DatecsPacketBuilder::calcCsum(const uint8_t* buf, uint16_t len) {
    uint8_t cs = 0;
    for (uint16_t i = 0; i < len; i++) cs ^= buf[i];
    return cs;
}

uint16_t DatecsPacketBuilder::build(uint8_t* outBuf, uint16_t maxLen) {
    // Format: START(1) CMD(1) 00(1) LH(1) LL(1) DATA(n) CSUM(1)
    uint16_t total = 5 + _dataLen + 1;
    if (_overflow || outBuf == nullptr || total > maxLen) return 0;

    outBuf[0] = PKT_START;
    outBuf[1] = _cmd;
    outBuf[2] = 0x00;
    outBuf[3] = (uint8_t)(_dataLen >> 8);
    outBuf[4] = (uint8_t)(_dataLen & 0xFF);
    memcpy(&outBuf[5], _buf, _dataLen);
    outBuf[5 + _dataLen] = calcCsum(outBuf, 5 + _dataLen);
    return total;
}

// ============================================================
//  DatecsPacketParser
// ============================================================
DatecsError DatecsPacketParser::parse(const uint8_t* raw, uint16_t rawLen,
                                      DatecsPacket& out) {
    // Minimum: START(1) CMD/00(1) ST/00(1) LH(1) LL(1) CSUM(1) = 6 bytes
    if (raw == nullptr || rawLen < 6) return DatecsError::InvLen;
    if (raw[0] != PKT_START) return DatecsError::LibBadStart;

    out.command = raw[1];
    out.status  = raw[2];
    out.dataLen = ((uint16_t)raw[3] << 8) | raw[4];
    out.isEvent = (out.command == EVT_BORICA || out.command == EVT_EXTERNAL ||
                   out.command == EVT_EMV2);

    if ((uint16_t)(5 + out.dataLen + 1) != rawLen) return DatecsError::InvLen;

    uint8_t expectedCsum = DatecsPacketBuilder::calcCsum(raw, 5 + out.dataLen);
    uint8_t actualCsum   = raw[5 + out.dataLen];
    if (expectedCsum != actualCsum) return DatecsError::LibBadCsum;

    if (out.dataLen > sizeof(out.data)) return DatecsError::LibBufOverflow;
    memcpy(out.data, &raw[5], out.dataLen);
    out.csum = actualCsum;

    return DatecsError::NoErr;
}

bool DatecsPacketParser::isEvent(const DatecsPacket& pkt) {
    return pkt.isEvent;
}

bool DatecsPacketParser::parseTransactionResult(const DatecsPacket& pkt,
                                                TransactionResult& out) {
    // Subevent byte is data[0]; actual TLV starts at data[1]
    if (pkt.command != EVT_BORICA || pkt.dataLen < 2 ||
        (pkt.data[0] != (uint8_t)BoricaEvent::TransactionComplete &&
         pkt.data[0] != (uint8_t)BoricaEvent::IntermediateTransactionComplete)) return false;
    const uint8_t* tlv    = &pkt.data[1];
    uint16_t       tlvLen = pkt.dataLen - 1;

    TLVItem item;
    if (TLVParser::find(tlv, tlvLen, Tag::TransResult,  item)) out.payResult  = TLVParser::toUint32(item);
    if (TLVParser::find(tlv, tlvLen, Tag::TransError,   item)) out.payError   = TLVParser::toUint32(item);
    if (TLVParser::find(tlv, tlvLen, Tag::Amount,       item)) out.amountAuth = TLVParser::toUint32(item);
    if (TLVParser::find(tlv, tlvLen, Tag::EMV_STAN,     item)) out.emvStan    = TLVParser::toUint32(item);
    if (TLVParser::find(tlv, tlvLen, Tag::HostRRN,      item)) TLVParser::toString(item, out.hostRRN,    sizeof(out.hostRRN));
    if (TLVParser::find(tlv, tlvLen, Tag::HostAuthID,   item)) TLVParser::toString(item, out.hostAuthID, sizeof(out.hostAuthID));
    return true;
}

bool DatecsPacketParser::parseSocketOpenEvent(const DatecsPacket& pkt,
                                               SocketOpenEvent& out) {
    // data[0]=subevent(0x01), [1]=ID, [2]=type, [3..6]=addr, [7..8]=port, [9..10]=timeout, [11..]=TLV
    if (pkt.command != EVT_EXTERNAL || pkt.dataLen < 11 ||
        pkt.data[0] != (uint8_t)ExtEvent::SocketOpen) return false;
    out.socketId    = pkt.data[1];
    out.type        = pkt.data[2];
    memcpy(out.address, &pkt.data[3], 4);
    out.port    = ((uint16_t)pkt.data[7] << 8) | pkt.data[8];
    out.timeout = ((uint16_t)pkt.data[9] << 8) | pkt.data[10];

    // Parse optional TLV tags
    if (pkt.dataLen > 11) {
        const uint8_t* tlv    = &pkt.data[11];
        uint16_t       tlvLen = pkt.dataLen - 11;
        TLVItem item;
        if (TLVParser::find(tlv, tlvLen, 0xDF50, item)) TLVParser::toString(item, out.apn,      sizeof(out.apn));
        if (TLVParser::find(tlv, tlvLen, 0xDF53, item)) TLVParser::toString(item, out.username,  sizeof(out.username));
        if (TLVParser::find(tlv, tlvLen, 0xDF54, item)) TLVParser::toString(item, out.password,  sizeof(out.password));
        if (TLVParser::find(tlv, tlvLen, 0xDF6C, item) && item.len == 4) {
            memcpy(out.hostLanIP, item.value, 4);
        }
        if (TLVParser::find(tlv, tlvLen, 0xDF6D, item)) {
            out.hostLanPort = (uint16_t)TLVParser::toUint32(item);
        }
    }
    return true;
}

bool DatecsPacketParser::parsePinpadInfo(const DatecsPacket& pkt, PinpadInfo& out) {
    if (pkt.dataLen != 43) return false;
    const uint8_t* d = pkt.data;
    memcpy(out.modelName,    d,      20); out.modelName[20]    = '\0';
    memcpy(out.serialNumber, d + 20, 10); out.serialNumber[10] = '\0';
    memcpy(out.softVersion,  d + 30,  4);
    memcpy(out.terminalID,   d + 34,  8); out.terminalID[8]    = '\0';
    out.menuType = d[42];
    return true;
}

bool DatecsPacketParser::parsePinpadStatus(const DatecsPacket& pkt, PinpadStatus& out) {
    if (pkt.dataLen != 2) return false;
    uint8_t rev     = pkt.data[0];
    uint8_t endday  = pkt.data[1];
    if ((rev != 0x00 && rev != 'R' && rev != 'C') || endday > 0x02) return false;
    out.needsReversal  = (rev == 'R');
    out.isHang         = (rev == 'C');
    out.needsEndOfDay  = (endday == 0x01);
    out.needsTmsUpdate = (endday == 0x02);
    return true;
}

bool DatecsPacketParser::parseRTC(const DatecsPacket& pkt, RTCTime& out) {
    if (pkt.dataLen != 6) return false;
    out.year  = pkt.data[0];
    out.month = pkt.data[1];
    out.date  = pkt.data[2];
    out.hour  = pkt.data[3];
    out.min   = pkt.data[4];
    out.sec   = pkt.data[5];
    return true;
}

// ============================================================
//  DatecsPay
// ============================================================
DatecsPay::DatecsPay(Stream& stream) : _stream(stream) {}

// ── Low-level ─────────────────────────────────────────────
DatecsError DatecsPay::writeBytes(const uint8_t* data, uint16_t len) {
    if (data == nullptr || len == 0) return DatecsError::LibInvalidArg;
    uint16_t written = 0;
    while (written < len) {
        size_t n = _stream.write(data + written, len - written);
        if (n == 0 || n > (size_t)(len - written)) return DatecsError::LibWriteFailed;
        written += (uint16_t)n;
    }
    return DatecsError::NoErr;
}

DatecsError DatecsPay::readPacket(DatecsPacket& out) {
    // Wait for START byte
    uint32_t t0 = millis();
    while (!_stream.available()) {
        if (millis() - t0 > RESP_TIMEOUT_MS) return DatecsError::LibTimeout;
        yield();
    }

    uint8_t  startByte = (uint8_t)_stream.read();
    if (startByte != PKT_START) return DatecsError::LibBadStart;

    _rxBuf[0] = startByte;
    uint16_t pos = 1;

    // Read next 4 bytes (CMD/EVT, ST/00, LH, LL)
    for (uint8_t i = 0; i < 4; i++) {
        t0 = millis();
        while (!_stream.available()) {
            if (millis() - t0 > BYTE_TIMEOUT_MS) return DatecsError::LibTimeout;
            yield();
        }
        if (pos >= MAX_PACKET_SIZE) return DatecsError::LibBufOverflow;
        _rxBuf[pos++] = (uint8_t)_stream.read();
    }

    uint16_t dataLen = ((uint16_t)_rxBuf[3] << 8) | _rxBuf[4];
    // dataLen + 1 byte CSUM
    uint16_t remaining = dataLen + 1;
    if (pos + remaining > MAX_PACKET_SIZE) return DatecsError::LibBufOverflow;

    for (uint16_t i = 0; i < remaining; i++) {
        t0 = millis();
        while (!_stream.available()) {
            if (millis() - t0 > BYTE_TIMEOUT_MS) return DatecsError::LibTimeout;
            yield();
        }
        _rxBuf[pos++] = (uint8_t)_stream.read();
    }

    return DatecsPacketParser::parse(_rxBuf, pos, out);
}

DatecsError DatecsPay::sendPacket(const uint8_t* pkt, uint16_t len,
                                   DatecsPacket& response) {
    if (pkt == nullptr || len < 6 || len > MAX_PACKET_SIZE)
        return DatecsError::LibInvalidArg;
    DatecsError err = writeBytes(pkt, len);
    if (err != DatecsError::NoErr) return err;

    while (true) {
        err = readPacket(response);
        if (err != DatecsError::NoErr) return err;
        if (response.isEvent) {
            err = _dispatchEvent(response);
            if (err != DatecsError::NoErr) return err;
            continue;
        }
        if (response.command != 0x00) return DatecsError::InvCmd;
        if (response.status != 0) return static_cast<DatecsError>(response.status);
        return DatecsError::NoErr;
    }
}

DatecsError DatecsPay::executeBorica(uint8_t subcommand,
                                     const uint8_t* data, uint16_t dataLen,
                                     uint8_t* responseData, uint16_t responseCapacity,
                                     uint16_t& responseLen) {
    responseLen = 0;
    if ((data == nullptr && dataLen != 0) ||
        (responseData == nullptr && responseCapacity != 0))
        return DatecsError::LibInvalidArg;
    DatecsPacketBuilder b;
    b.begin(CMD_BORICA);
    b.append(subcommand).appendBytes(data, dataLen);
    uint16_t len = b.build(_txBuf, sizeof(_txBuf));
    DatecsPacket response;
    DatecsError err = sendPacket(_txBuf, len, response);
    if (err != DatecsError::NoErr) return err;
    if (response.dataLen > responseCapacity) return DatecsError::LibBufOverflow;
    if (response.dataLen != 0) memcpy(responseData, response.data, response.dataLen);
    responseLen = response.dataLen;
    return DatecsError::NoErr;
}

// ── Utility ───────────────────────────────────────────────
DatecsError DatecsPay::ping() {
    DatecsPacketBuilder b;
    b.begin(CMD_BORICA);
    b.append((uint8_t)BoricaSubCmd::Ping);
    uint16_t len = b.build(_txBuf, sizeof(_txBuf));
    DatecsPacket resp;
    return sendPacket(_txBuf, len, resp);
}

DatecsError DatecsPay::getPinpadInfo(PinpadInfo& info) {
    DatecsPacketBuilder b;
    b.begin(CMD_BORICA);
    b.append((uint8_t)BoricaSubCmd::GetPinpadInfo);
    uint16_t len = b.build(_txBuf, sizeof(_txBuf));
    DatecsPacket resp;
    DatecsError err = sendPacket(_txBuf, len, resp);
    if (err != DatecsError::NoErr) return err;
    if (!DatecsPacketParser::parsePinpadInfo(resp, info)) return DatecsError::InvLen;
    return DatecsError::NoErr;
}

DatecsError DatecsPay::getPinpadStatus(PinpadStatus& status) {
    DatecsPacketBuilder b;
    b.begin(CMD_BORICA);
    b.append((uint8_t)BoricaSubCmd::GetPinpadStatus);
    uint16_t len = b.build(_txBuf, sizeof(_txBuf));
    DatecsPacket resp;
    DatecsError err = sendPacket(_txBuf, len, resp);
    if (err != DatecsError::NoErr) return err;
    if (!DatecsPacketParser::parsePinpadStatus(resp, status)) return DatecsError::InvLen;
    return DatecsError::NoErr;
}

DatecsError DatecsPay::getRTC(RTCTime& rtc) {
    DatecsPacketBuilder b;
    b.begin(CMD_BORICA);
    b.append((uint8_t)BoricaSubCmd::GetRTC);
    uint16_t len = b.build(_txBuf, sizeof(_txBuf));
    DatecsPacket resp;
    DatecsError err = sendPacket(_txBuf, len, resp);
    if (err != DatecsError::NoErr) return err;
    if (!DatecsPacketParser::parseRTC(resp, rtc)) return DatecsError::InvLen;
    return DatecsError::NoErr;
}

DatecsError DatecsPay::setRTC(const RTCTime& rtc) {
    DatecsPacketBuilder b;
    b.begin(CMD_BORICA);
    b.append((uint8_t)BoricaSubCmd::SetRTC);
    b.append(rtc.year).append(rtc.month).append(rtc.date);
    b.append(rtc.hour).append(rtc.min).append(rtc.sec);
    uint16_t len = b.build(_txBuf, sizeof(_txBuf));
    DatecsPacket resp;
    return sendPacket(_txBuf, len, resp);
}

DatecsError DatecsPay::getReportInfo(uint16_t& recordCount) {
    DatecsPacketBuilder b;
    b.begin(CMD_BORICA);
    b.append((uint8_t)BoricaSubCmd::GetReportInfo);
    uint16_t len = b.build(_txBuf, sizeof(_txBuf));
    DatecsPacket resp;
    DatecsError err = sendPacket(_txBuf, len, resp);
    if (err != DatecsError::NoErr) return err;
    if (resp.dataLen != 2) return DatecsError::InvLen;
    recordCount = ((uint16_t)resp.data[0] << 8) | resp.data[1];
    return DatecsError::NoErr;
}

DatecsError DatecsPay::transactionEnd(bool ok) {
    DatecsPacketBuilder b;
    b.begin(CMD_BORICA);
    b.append((uint8_t)BoricaSubCmd::TransactionEnd);
    b.appendUint16BE(ok ? 0x0001 : 0x0000);
    uint16_t len = b.build(_txBuf, sizeof(_txBuf));
    DatecsPacket resp;
    return sendPacket(_txBuf, len, resp);
}

static DatecsError simpleBoricaCommand(DatecsPay& pay, BoricaSubCmd cmd) {
    uint8_t raw[7] = {PKT_START, CMD_BORICA, 0x00, 0x00, 0x01,
                      (uint8_t)cmd, 0x00};
    raw[6] = DatecsPacketBuilder::calcCsum(raw, 6);
    DatecsPacket response;
    return pay.sendPacket(raw, sizeof(raw), response);
}

DatecsError DatecsPay::deleteBatch() {
    return simpleBoricaCommand(*this, BoricaSubCmd::DeleteBatch);
}

DatecsError DatecsPay::clearReversal() {
    return simpleBoricaCommand(*this, BoricaSubCmd::ClearReversal);
}

DatecsError DatecsPay::selectChipApplication(uint8_t index) {
    DatecsPacketBuilder b;
    b.begin(CMD_BORICA);
    b.append((uint8_t)BoricaSubCmd::SelectChipApp).append(index);
    uint16_t len = b.build(_txBuf, sizeof(_txBuf));
    DatecsPacket response;
    return sendPacket(_txBuf, len, response);
}

// ── Internal transaction helper ───────────────────────────
DatecsError DatecsPay::_startTransaction(TransactionType type,
                                          const uint8_t* paramData,
                                          uint16_t paramLen) {
    if ((paramData == nullptr) != (paramLen == 0)) return DatecsError::LibInvalidArg;
    DatecsPacketBuilder b;
    b.begin(CMD_BORICA);
    b.append((uint8_t)BoricaSubCmd::TransactionStart);
    b.append((uint8_t)type);
    if (paramData && paramLen) b.appendBytes(paramData, paramLen);
    uint16_t len = b.build(_txBuf, sizeof(_txBuf));
    DatecsPacket resp;
    return sendPacket(_txBuf, len, resp);
}

// ── Transactions ──────────────────────────────────────────
DatecsError DatecsPay::startPurchase(uint32_t amountCents) {
    uint8_t data[8];
    data[0] = 0x81; data[1] = 0x04;
    data[2] = (amountCents >> 24) & 0xFF;
    data[3] = (amountCents >> 16) & 0xFF;
    data[4] = (amountCents >>  8) & 0xFF;
    data[5] =  amountCents        & 0xFF;
    return _startTransaction(TransactionType::Purchase, data, 6);
}

// Reusable macro-like helper for amount-only transactions
static uint16_t _buildAmountParam(uint8_t* buf, uint32_t cents) {
    buf[0] = 0x81; buf[1] = 0x04;
    buf[2] = (cents >> 24) & 0xFF;
    buf[3] = (cents >> 16) & 0xFF;
    buf[4] = (cents >>  8) & 0xFF;
    buf[5] =  cents        & 0xFF;
    return 6;
}

static uint16_t _appendTLVuint32(uint8_t* buf, uint16_t tag, uint32_t val) {
    uint8_t pos = 0;
    if (tag > 0xFF) { buf[pos++] = (uint8_t)(tag >> 8); buf[pos++] = (uint8_t)(tag & 0xFF); }
    else             { buf[pos++] = (uint8_t)tag; }
    buf[pos++] = 0x04;
    buf[pos++] = (val >> 24) & 0xFF;
    buf[pos++] = (val >> 16) & 0xFF;
    buf[pos++] = (val >>  8) & 0xFF;
    buf[pos++] =  val        & 0xFF;
    return pos;
}

static bool _appendTLVstring(uint8_t* buf, uint16_t capacity, uint16_t& pos,
                             uint16_t tag, const char* str) {
    if (buf == nullptr || str == nullptr) return false;
    size_t slen = strlen(str);
    uint16_t tagLen = tag > 0xFF ? 2 : 1;
    if (slen > 255 || pos + tagLen + 1 + slen > capacity) return false;
    if (tag > 0xFF) {
        buf[pos++] = (uint8_t)(tag >> 8);
        buf[pos++] = (uint8_t)(tag & 0xFF);
    } else {
        buf[pos++] = (uint8_t)tag;
    }
    buf[pos++] = (uint8_t)slen;
    memcpy(&buf[pos], str, slen);
    pos += (uint16_t)slen;
    return true;
}

DatecsError DatecsPay::startPurchaseWithTip(uint32_t totalCents, uint32_t tipCents) {
    uint8_t  data[32]; uint16_t pos = 0;
    pos += _buildAmountParam(&data[pos], totalCents);
    pos += _appendTLVuint32(&data[pos], Tag::Tip, tipCents);
    return _startTransaction(TransactionType::Purchase, data, pos);
}

DatecsError DatecsPay::startPurchaseWithCashback(uint32_t amountCents, uint32_t cashbackCents) {
    uint8_t  data[32]; uint16_t pos = 0;
    pos += _buildAmountParam(&data[pos], amountCents);
    pos += _appendTLVuint32(&data[pos], Tag::Cashback, cashbackCents);
    return _startTransaction(TransactionType::PurchaseCashback, data, pos);
}

DatecsError DatecsPay::startPurchaseWithReference(uint32_t amountCents, const char* ref) {
    uint8_t  data[64]; uint16_t pos = 0;
    pos += _buildAmountParam(&data[pos], amountCents);
    if (!_appendTLVstring(data, sizeof(data), pos, Tag::Reference, ref))
        return DatecsError::LibInvalidArg;
    return _startTransaction(TransactionType::PurchaseReference, data, pos);
}

DatecsError DatecsPay::startCashAdvance(uint32_t amountCents) {
    uint8_t data[8]; _buildAmountParam(data, amountCents);
    return _startTransaction(TransactionType::CashAdvance, data, 6);
}

DatecsError DatecsPay::startAuthorization(uint32_t amountCents) {
    uint8_t data[8]; _buildAmountParam(data, amountCents);
    return _startTransaction(TransactionType::Authorization, data, 6);
}

DatecsError DatecsPay::startPurchaseWithCode(uint32_t amountCents,
                                              const char* rrn, const char* authID) {
    uint8_t  data[64]; uint16_t pos = 0;
    pos += _buildAmountParam(&data[pos], amountCents);
    if (!_appendTLVstring(data, sizeof(data), pos, Tag::RRN, rrn) ||
        !_appendTLVstring(data, sizeof(data), pos, Tag::AuthID, authID))
        return DatecsError::LibInvalidArg;
    return _startTransaction(TransactionType::PurchaseCode, data, pos);
}

DatecsError DatecsPay::startVoidPurchase(uint32_t amountCents,
                                          const char* rrn, const char* authID) {
    uint8_t  data[64]; uint16_t pos = 0;
    pos += _buildAmountParam(&data[pos], amountCents);
    if (!_appendTLVstring(data, sizeof(data), pos, Tag::RRN, rrn) ||
        !_appendTLVstring(data, sizeof(data), pos, Tag::AuthID, authID))
        return DatecsError::LibInvalidArg;
    return _startTransaction(TransactionType::VoidPurchase, data, pos);
}

DatecsError DatecsPay::startVoidPurchaseWithTip(uint32_t amountCents, uint32_t tipCents,
                                                 const char* rrn, const char* authID) {
    uint8_t  data[80]; uint16_t pos = 0;
    pos += _buildAmountParam(&data[pos], amountCents);
    pos += _appendTLVuint32(&data[pos], Tag::Tip, tipCents);
    if (!_appendTLVstring(data, sizeof(data), pos, Tag::RRN, rrn) ||
        !_appendTLVstring(data, sizeof(data), pos, Tag::AuthID, authID))
        return DatecsError::LibInvalidArg;
    return _startTransaction(TransactionType::VoidPurchase, data, pos);
}

DatecsError DatecsPay::startVoidPurchaseWithCashback(uint32_t amountCents,
                                                      uint32_t cashbackCents,
                                                      const char* rrn, const char* authID) {
    uint8_t  data[80]; uint16_t pos = 0;
    pos += _buildAmountParam(&data[pos], amountCents);
    pos += _appendTLVuint32(&data[pos], Tag::Cashback, cashbackCents);
    if (!_appendTLVstring(data, sizeof(data), pos, Tag::RRN, rrn) ||
        !_appendTLVstring(data, sizeof(data), pos, Tag::AuthID, authID))
        return DatecsError::LibInvalidArg;
    return _startTransaction(TransactionType::VoidPurchase, data, pos);
}

DatecsError DatecsPay::startVoidCashAdvance(uint32_t amountCents,
                                             const char* rrn, const char* authID) {
    uint8_t  data[64]; uint16_t pos = 0;
    pos += _buildAmountParam(&data[pos], amountCents);
    if (!_appendTLVstring(data, sizeof(data), pos, Tag::RRN, rrn) ||
        !_appendTLVstring(data, sizeof(data), pos, Tag::AuthID, authID))
        return DatecsError::LibInvalidArg;
    return _startTransaction(TransactionType::VoidCashAdvance, data, pos);
}

DatecsError DatecsPay::startVoidAuthorization(uint32_t amountCents,
                                               const char* rrn, const char* authID) {
    uint8_t  data[64]; uint16_t pos = 0;
    pos += _buildAmountParam(&data[pos], amountCents);
    if (!_appendTLVstring(data, sizeof(data), pos, Tag::RRN, rrn) ||
        !_appendTLVstring(data, sizeof(data), pos, Tag::AuthID, authID))
        return DatecsError::LibInvalidArg;
    return _startTransaction(TransactionType::VoidAuthorization, data, pos);
}

DatecsError DatecsPay::startEndOfDay() {
    return _startTransaction(TransactionType::EndOfDay, nullptr, 0);
}

DatecsError DatecsPay::startTestConnection() {
    return _startTransaction(TransactionType::TestConnection, nullptr, 0);
}

DatecsError DatecsPay::startTmsUpdate() {
    return _startTransaction(TransactionType::TmsUpdate, nullptr, 0);
}

DatecsError DatecsPay::startRefund(uint32_t amountCents,
                                    const char* rrn, const char* authID) {
    (void)amountCents;
    (void)rrn;
    (void)authID;
    // Transaction type 0x10 is explicitly Romania-only in Datecs Pay v3.
    return DatecsError::NoSupport;
}

// ── Receipt / report ──────────────────────────────────────
DatecsError DatecsPay::getReceiptTags(const uint16_t* tags, uint8_t tagCount,
                                       uint8_t* buf, uint16_t bufSize, uint16_t& outLen) {
    if ((tags == nullptr && tagCount != 0) || buf == nullptr) return DatecsError::LibInvalidArg;
    DatecsPacketBuilder b;
    b.begin(CMD_BORICA);
    b.append((uint8_t)BoricaSubCmd::GetReceiptTags);
    for (uint8_t i = 0; i < tagCount; i++) {
        if (tags[i] > 0xFF) {
            b.append((uint8_t)(tags[i] >> 8));
            b.append((uint8_t)(tags[i] & 0xFF));
        } else {
            b.append((uint8_t)tags[i]);
        }
    }
    uint16_t len = b.build(_txBuf, sizeof(_txBuf));
    DatecsPacket resp;
    DatecsError err = sendPacket(_txBuf, len, resp);
    if (err != DatecsError::NoErr) return err;
    if (resp.dataLen > bufSize) return DatecsError::LibBufOverflow;
    memcpy(buf, resp.data, resp.dataLen);
    outLen = resp.dataLen;
    return DatecsError::NoErr;
}

DatecsError DatecsPay::getReceiptTagsRaw(const uint8_t* encodedTags,
                                          uint16_t encodedTagsLen,
                                          uint8_t* buf, uint16_t bufSize,
                                          uint16_t& outLen) {
    if ((encodedTags == nullptr && encodedTagsLen != 0) || buf == nullptr)
        return DatecsError::LibInvalidArg;
    DatecsPacketBuilder b;
    b.begin(CMD_BORICA);
    b.append((uint8_t)BoricaSubCmd::GetReceiptTags);
    b.appendBytes(encodedTags, encodedTagsLen);
    uint16_t len = b.build(_txBuf, sizeof(_txBuf));
    DatecsPacket response;
    DatecsError err = sendPacket(_txBuf, len, response);
    if (err != DatecsError::NoErr) return err;
    if (response.dataLen > bufSize) return DatecsError::LibBufOverflow;
    memcpy(buf, response.data, response.dataLen);
    outLen = response.dataLen;
    return DatecsError::NoErr;
}

DatecsError DatecsPay::getReportTags(const uint16_t* tags, uint8_t tagCount,
                                      uint8_t* buf, uint16_t bufSize,
                                      uint16_t& outLen) {
    if ((tags == nullptr && tagCount != 0) || buf == nullptr) return DatecsError::LibInvalidArg;
    DatecsPacketBuilder b;
    b.begin(CMD_BORICA);
    b.append((uint8_t)BoricaSubCmd::GetReportTags);
    for (uint8_t i = 0; i < tagCount; ++i) {
        if (tags[i] > 0xFF) b.append((uint8_t)(tags[i] >> 8));
        b.append((uint8_t)tags[i]);
    }
    uint16_t len = b.build(_txBuf, sizeof(_txBuf));
    DatecsPacket response;
    DatecsError err = sendPacket(_txBuf, len, response);
    if (err != DatecsError::NoErr) return err;
    if (response.dataLen > bufSize) return DatecsError::LibBufOverflow;
    memcpy(buf, response.data, response.dataLen);
    outLen = response.dataLen;
    return DatecsError::NoErr;
}

DatecsError DatecsPay::getReportTagsRaw(const uint8_t* encodedTags,
                                         uint16_t encodedTagsLen,
                                         uint8_t* buf, uint16_t bufSize,
                                         uint16_t& outLen) {
    if ((encodedTags == nullptr && encodedTagsLen != 0) || buf == nullptr)
        return DatecsError::LibInvalidArg;
    DatecsPacketBuilder b;
    b.begin(CMD_BORICA);
    b.append((uint8_t)BoricaSubCmd::GetReportTags);
    b.appendBytes(encodedTags, encodedTagsLen);
    uint16_t len = b.build(_txBuf, sizeof(_txBuf));
    DatecsPacket response;
    DatecsError err = sendPacket(_txBuf, len, response);
    if (err != DatecsError::NoErr) return err;
    if (response.dataLen > bufSize) return DatecsError::LibBufOverflow;
    memcpy(buf, response.data, response.dataLen);
    outLen = response.dataLen;
    return DatecsError::NoErr;
}

DatecsError DatecsPay::getReportTagsByStan(uint32_t stan,
                                            const uint16_t* tags, uint8_t tagCount,
                                            uint8_t* buf, uint16_t bufSize,
                                            uint16_t& outLen) {
    if ((tags == nullptr && tagCount != 0) || buf == nullptr) return DatecsError::LibInvalidArg;
    DatecsPacketBuilder b;
    b.begin(CMD_BORICA);
    b.append((uint8_t)BoricaSubCmd::GetReportByStan);
    b.appendUint32BE(stan);
    for (uint8_t i = 0; i < tagCount; i++) {
        if (tags[i] > 0xFF) {
            b.append((uint8_t)(tags[i] >> 8));
            b.append((uint8_t)(tags[i] & 0xFF));
        } else {
            b.append((uint8_t)tags[i]);
        }
    }
    uint16_t len = b.build(_txBuf, sizeof(_txBuf));
    DatecsPacket resp;
    DatecsError err = sendPacket(_txBuf, len, resp);
    if (err != DatecsError::NoErr) return err;
    if (resp.dataLen > bufSize) return DatecsError::LibBufOverflow;
    memcpy(buf, resp.data, resp.dataLen);
    outLen = resp.dataLen;
    return DatecsError::NoErr;
}

// ── External internet helpers ─────────────────────────────
DatecsError DatecsPay::confirmSocketEvent(uint8_t evtType, bool success,
                                           uint16_t mtu) {
    if (evtType < (uint8_t)ExtEvent::SocketOpen || evtType > (uint8_t)ExtEvent::SendData)
        return DatecsError::LibInvalidArg;
    DatecsPacketBuilder b;
    b.begin(CMD_EXTERNAL);
    b.append((uint8_t)ExtSubCmd::EventConfirm);
    b.append(evtType);
    b.append(success ? 0x00 : 0x01);
    if (mtu != 0) b.appendUint16BE(mtu);
    uint16_t len = b.build(_txBuf, sizeof(_txBuf));
    DatecsPacket resp;
    return sendPacket(_txBuf, len, resp);
}

DatecsError DatecsPay::receiveData(const uint8_t* data, uint16_t len) {
    if (data == nullptr && len != 0) return DatecsError::LibInvalidArg;
    if (len == 0) return DatecsError::NoErr;
    uint16_t pinpadMtu = 0;
    DatecsError err = getMaxMtu(pinpadMtu);
    if (err != DatecsError::NoErr) return err;
    uint16_t chunkLimit = pinpadMtu;
    const uint16_t builderLimit = 1024; // protocol default and documented maximum chunk
    if (chunkLimit == 0 || chunkLimit > builderLimit) chunkLimit = builderLimit;

    uint16_t offset = 0;
    do {
        uint16_t chunk = len - offset;
        if (chunk > chunkLimit) chunk = chunkLimit;
        while (true) {
            DatecsPacketBuilder b;
            b.begin(CMD_EXTERNAL);
            b.append((uint8_t)ExtSubCmd::ReceiveData);
            b.appendBytes(data == nullptr ? nullptr : data + offset, chunk);
            uint16_t pktLen = b.build(_txBuf, sizeof(_txBuf));
            DatecsPacket resp;
            err = sendPacket(_txBuf, pktLen, resp);
            if (err == DatecsError::Busy) {
                delay(100);
                continue;
            }
            if (err != DatecsError::NoErr) return err;
            break;
        }
        offset += chunk;
    } while (offset < len);
    return DatecsError::NoErr;
}

DatecsError DatecsPay::getMaxMtu(uint16_t& mtu) {
    DatecsPacketBuilder b;
    b.begin(CMD_EXTERNAL);
    b.append((uint8_t)ExtSubCmd::GetMaxMtu);
    uint16_t len = b.build(_txBuf, sizeof(_txBuf));
    DatecsPacket resp;
    DatecsError err = sendPacket(_txBuf, len, resp);
    if (err != DatecsError::NoErr) return err;
    if (resp.dataLen != 2) return DatecsError::InvLen;
    mtu = ((uint16_t)resp.data[0] << 8) | resp.data[1];
    return DatecsError::NoErr;
}

// ── Event dispatch ────────────────────────────────────────
DatecsError DatecsPay::_dispatchEvent(const DatecsPacket& pkt) {
    if (pkt.dataLen == 0) return DatecsError::InvLen;
    if (_cbRawEvent) {
        _cbRawEvent(pkt.command, pkt.data[0], pkt.dataLen > 1 ? &pkt.data[1] : nullptr,
                    pkt.dataLen > 1 ? pkt.dataLen - 1 : 0);
    }
    if (pkt.command == EVT_BORICA) {
        uint8_t subevent = pkt.data[0];
        switch ((BoricaEvent)subevent) {
            case BoricaEvent::TransactionComplete:
            case BoricaEvent::IntermediateTransactionComplete: {
                if (_cbTxComplete) {
                    TransactionResult res;
                    if (!DatecsPacketParser::parseTransactionResult(pkt, res))
                        return DatecsError::InvLen;
                    _cbTxComplete(res);
                }
                break;
            }
            case BoricaEvent::HangTransaction: {
                if (_cbHang && pkt.dataLen > 1)
                    _cbHang(&pkt.data[1], pkt.dataLen - 1);
                break;
            }
            case BoricaEvent::SelectChipApplication: {
                if (_cbChipApp && pkt.dataLen > 1)
                    _cbChipApp(&pkt.data[1], pkt.dataLen - 1);
                break;
            }
            default: break;
        }
    } else if (pkt.command == EVT_EXTERNAL) {
        uint8_t subevent = pkt.data[0];
        switch ((ExtEvent)subevent) {
            case ExtEvent::SocketOpen: {
                if (_cbSockOpen) {
                    SocketOpenEvent evt;
                    if (!DatecsPacketParser::parseSocketOpenEvent(pkt, evt))
                        return DatecsError::InvLen;
                    _cbSockOpen(evt);
                }
                break;
            }
            case ExtEvent::SocketClose: {
                if (_cbSockClose && pkt.dataLen >= 2)
                    _cbSockClose(pkt.data[1]);
                break;
            }
            case ExtEvent::SendData: {
                if (_cbSendData && pkt.dataLen >= 2)
                    _cbSendData(pkt.data[1], &pkt.data[2], pkt.dataLen - 2);
                break;
            }
        }
    }
    return DatecsError::NoErr;
}

bool DatecsPay::processEvents() {
    if (!_stream.available()) return false;
    DatecsPacket pkt;
    DatecsError err = readPacket(pkt);
    if (err == DatecsError::NoErr && pkt.isEvent) {
        _dispatchEvent(pkt);
        return true;
    }
    return false;
}
