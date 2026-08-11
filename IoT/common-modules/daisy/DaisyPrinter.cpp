/**
 * DaisyPrinter.cpp
 * High-level command implementations for DaisyPrinter.
 *
 * DATA field conventions (from protocol spec):
 *   Delimiters: TAB = 09h, LF = 0Ah
 *   Mandatory params in {}, optional in []
 *   Tax groups: Cyrillic А..З in CP1251 (C0h..C7h)
 *   This API accepts Latin 'A'..'H' and converts internally.
 */

#include "DaisyProtocol.h"
#include <stdarg.h>

// ─── Internal helpers ─────────────────────────────────────────────────────────

bool DaisyPrinter::_exec(uint8_t cmd, const char* data) {
    return _proto.execute(cmd, data, _last);
}

bool DaisyPrinter::_execOk() {
    return _last.ok();
}

// Parse "XXXXXX,YYYYYY" receipt counters from response data
static void parseCounters(const char* data, uint32_t* a, uint32_t* b) {
    if (!data || !*data) return;
    if (a) *a = (uint32_t)atol(data);
    const char* comma = strchr(data, ',');
    if (b && comma) *b = (uint32_t)atol(comma + 1);
}

static bool formatChecked(char* buf, size_t capacity, const char* format, ...) {
    if (buf == nullptr || capacity == 0 || format == nullptr) return false;
    va_list args;
    va_start(args, format);
    int written = vsnprintf(buf, capacity, format, args);
    va_end(args);
    return written >= 0 && (size_t)written < capacity;
}

// ─── Display ─────────────────────────────────────────────────────────────────

bool DaisyPrinter::clearDisplay() {
    return _exec(CMD_CLEAR_DISPLAY) && _execOk();
}

bool DaisyPrinter::displayRow1(const char* text) {
    // CMD 47: Data = Text
    return _exec(CMD_DISPLAY_ROW1, text) && _execOk();
}

bool DaisyPrinter::displayRow2(const char* text) {
    // CMD 35: Data = Text
    return _exec(CMD_DISPLAY_ROW2, text) && _execOk();
}

bool DaisyPrinter::displayRowN(uint8_t row, const char* text) {
    // CMD 46: Data = LineNo,Text
    char buf[DAISY_MAX_DATA];
    if (!formatChecked(buf, sizeof(buf), "%u,%s", (unsigned)row, text ? text : "")) return false;
    return _exec(CMD_DISPLAY_ROW_N, buf) && _execOk();
}

// ─── Non-fiscal receipt ───────────────────────────────────────────────────────

bool DaisyPrinter::startNonfiscal() {
    // CMD 38: no data (basic variant)
    return _exec(CMD_START_NONFISCAL) && _execOk();
}

bool DaisyPrinter::endNonfiscal(uint32_t* allReceipts) {
    // CMD 39: no data; response = AllReceipt (6 bytes)
    if (!_exec(CMD_END_NONFISCAL) || !_execOk()) return false;
    if (allReceipts) *allReceipts = (uint32_t)atol(_last.data);
    return true;
}

bool DaisyPrinter::printNonfiscalText(const char* text) {
    // CMD 42: Data = Text
    return _exec(CMD_PRINT_NONFISCAL_TEXT, text) && _execOk();
}

// ─── Fiscal receipt ───────────────────────────────────────────────────────────

bool DaisyPrinter::startFiscal(uint8_t operNum, const char* password,
                                const char* unp, bool invoice,
                                uint32_t* allRec, uint32_t* fiscRec) {
    // CMD 48: {OperNum},{Password},{UNP}[{TAB}{I}]
    char buf[DAISY_MAX_DATA];
    if (invoice) {
        if (!formatChecked(buf, sizeof(buf), "%u,%s,%s\x09I",
                           (unsigned)operNum, password ? password : "", unp ? unp : "")) return false;
    } else {
        if (!formatChecked(buf, sizeof(buf), "%u,%s,%s",
                           (unsigned)operNum, password ? password : "", unp ? unp : "")) return false;
    }
    if (!_exec(CMD_START_FISCAL, buf) || !_execOk()) return false;
    parseCounters(_last.data, allRec, fiscRec);
    return true;
}

bool DaisyPrinter::startRefund(uint8_t operNum, const char* password, const char* unp,
                               uint8_t reason, uint32_t originalDocument,
                               const char* originalDatetime, const char* originalFmNumber,
                               uint32_t* allRec, uint32_t* fiscRec) {
    if (password == nullptr || unp == nullptr || originalDatetime == nullptr ||
        originalFmNumber == nullptr || reason > 2) return false;
    char buf[DAISY_MAX_DATA + 1];
    if (!formatChecked(buf, sizeof(buf), "%u,%s,%s\x09R%u,%lu,%s\x09%s",
                       (unsigned)operNum, password, unp, (unsigned)reason,
                       (unsigned long)originalDocument, originalDatetime, originalFmNumber)) return false;
    if (!_exec(CMD_START_FISCAL, buf) || !_execOk()) return false;
    parseCounters(_last.data, allRec, fiscRec);
    return true;
}

bool DaisyPrinter::sale(const char* name, char taxGroup, const char* price,
                         const char* qty, const char* percent) {
    return saleEx(name, nullptr, taxGroup, price, qty, percent, nullptr, false);
}

bool DaisyPrinter::saleEx(const char* text1, const char* text2, char taxGroup,
                          const char* price, const char* qty,
                          const char* percent, const char* netto,
                          bool correction) {
    if (price == nullptr || (percent != nullptr && netto != nullptr)) return false;
    char buf[DAISY_MAX_DATA + 1];
    size_t pos = 0;
    auto append = [&](const char* value) -> bool {
        if (value == nullptr) return true;
        size_t len = strlen(value);
        if (len > DAISY_MAX_DATA - pos) return false;
        memcpy(buf + pos, value, len);
        pos += len;
        return true;
    };
    auto one = [&](char value) -> bool {
        if (pos >= DAISY_MAX_DATA) return false;
        buf[pos++] = value;
        return true;
    };
    if (!append(text1)) return false;
    if (text2 != nullptr && (!one('\x0A') || !append(text2))) return false;
    if (!one('\x09') || !one((char)taxGroupByte(taxGroup))) return false;
    if (correction && !one('-')) return false;
    if (!append(price)) return false;
    if (qty != nullptr && (!one('*') || !append(qty))) return false;
    if (percent != nullptr && (!one(',') || !append(percent))) return false;
    if (netto != nullptr && (!one('$') || !append(netto))) return false;
    buf[pos] = '\0';
    return _proto.execute(CMD_SALE, buf, (uint8_t)pos, _last) && _execOk();
}

bool DaisyPrinter::subtotal(bool print, bool display,
                             char* subTotalOut, uint8_t bufLen) {
    // CMD 51: {Print}{Display}  — "10" = print, no display
    char buf[4];
    buf[0] = print   ? '1' : '0';
    buf[1] = display ? '1' : '0';
    buf[2] = '\0';
    if (!_exec(CMD_SUBTOTAL, buf) || !_execOk()) return false;
    if (subTotalOut && bufLen > 0) {
        // Response: SubTotal,Tax1,...Tax8 — copy up to first comma
        uint8_t i = 0;
        while (i < bufLen - 1 && _last.data[i] && _last.data[i] != ',') {
            subTotalOut[i] = _last.data[i]; i++;
        }
        subTotalOut[i] = '\0';
    }
    return true;
}

bool DaisyPrinter::total(char payType, const char* amount,
                          char* changeOut, uint8_t changeLen) {
    // CMD 53: [{Text1}]{TAB}[[{Payment}]{Amount}]
    // Simplest form: TAB + PaymentType + Amount
    if (payType != 'P' && payType != 'N' && payType != 'C' && payType != 'D' &&
        payType != 'U' && payType != 'B' && payType != 'E') return false;
    char buf[DAISY_MAX_DATA];
    if (amount && *amount) {
        if (!formatChecked(buf, sizeof(buf), "\x09%c%s", payType, amount)) return false;
    } else {
        // No amount = exact change with specified payment type
        if (!formatChecked(buf, sizeof(buf), "\x09%c", payType)) return false;
    }
    if (!_exec(CMD_TOTAL, buf) || !_execOk()) return false;
    // Response: PaidCode + Amount (change)
    if (changeOut && changeLen > 0 && _last.dataLen > 1) {
        strncpy(changeOut, _last.data + 1, changeLen - 1);
        changeOut[changeLen - 1] = '\0';
    }
    return _last.resultCode() != 'F';
}

bool DaisyPrinter::endFiscal(uint32_t* allRec, uint32_t* fiscRec) {
    // CMD 56: no data; response = AllReceipt,FiscReceipt
    if (!_exec(CMD_END_FISCAL) || !_execOk()) return false;
    parseCounters(_last.data, allRec, fiscRec);
    return true;
}

bool DaisyPrinter::cancelReceipt() {
    // CMD 130: no data
    return _exec(CMD_CANCEL_RECEIPT) && _execOk();
}

bool DaisyPrinter::printFiscalText(const char* text) {
    // CMD 54: Data = Text
    return _exec(CMD_PRINT_FISCAL_TEXT, text) && _execOk();
}

bool DaisyPrinter::printCustomerInfo(const char* identNo, const char* regNo,
                                      const char* seller, const char* receiver,
                                      const char* client, const char* address) {
    // CMD 57: IdentNo[TAB RegNo [TAB Seller [TAB Receiver [TAB Client [TAB Address]]]]]
    char buf[DAISY_MAX_DATA];
    uint8_t pos = 0;

    auto append = [&](const char* s) {
        if (!s) return;
        uint8_t l = (uint8_t)strlen(s);
        if (pos + l < DAISY_MAX_DATA - 2) { memcpy(&buf[pos], s, l); pos += l; }
    };
    auto appendTab = [&]() { if (pos < DAISY_MAX_DATA - 1) buf[pos++] = 0x09; };

    append(identNo);
    if (regNo || seller || receiver || client || address) {
        appendTab(); append(regNo);
        if (seller || receiver || client || address) {
            appendTab(); append(seller);
            if (receiver || client || address) {
                appendTab(); append(receiver);
                if (client || address) {
                    appendTab(); append(client);
                    if (address) { appendTab(); append(address); }
                }
            }
        }
    }
    buf[pos] = '\0';
    return _exec(CMD_PRINT_CUSTOMER_INFO, buf) && _execOk();
}

// ─── Paper ────────────────────────────────────────────────────────────────────

bool DaisyPrinter::paperFeed(uint8_t lines) {
    // CMD 44: [Lines]
    char buf[8];
    if (!formatChecked(buf, sizeof(buf), "%u", (unsigned)lines)) return false;
    return _exec(CMD_PAPER_FEED, buf) && _execOk();
}

bool DaisyPrinter::paperCut(uint8_t mode) {
    // CMD 45: [Mode] — 1=full, 2=partial
    // Response: "P" or "F"
    char buf[4];
    if (!formatChecked(buf, sizeof(buf), "%u", (unsigned)mode)) return false;
    if (!_exec(CMD_PAPER_CUT, buf)) return false;
    return _last.ok() && _last.passed();
}

// ─── Date / time ─────────────────────────────────────────────────────────────

bool DaisyPrinter::setDatetime(const char* dt) {
    // CMD 61: {DD-MM-YY HH:mm[:SS]}
    return _exec(CMD_SET_DATETIME, dt) && _execOk();
}

bool DaisyPrinter::getDatetime(char* buf, uint8_t bufLen) {
    if (buf == nullptr || bufLen == 0) return false;
    // CMD 62: no data; response = "DD.MM.YY HH:mm:SS"
    if (!_exec(CMD_GET_DATETIME) || !_execOk()) return false;
    strncpy(buf, _last.data, bufLen - 1);
    buf[bufLen - 1] = '\0';
    return true;
}

// ─── Status ───────────────────────────────────────────────────────────────────

bool DaisyPrinter::getStatus(DaisyStatus& status) {
    // CMD 74: no data; response = {S0}{S1}{S2}{S3}{S4}{S5}
    // Status is always in every response, but CMD 74 returns it in DATA too
    if (!_exec(CMD_FD_STATUS)) return false;
    status = _last.status;
    return _last.localError == DAISY_OK;
}

bool DaisyPrinter::getReceiptStatus(bool& open, uint8_t& items,
                                     char* totalBuf, uint8_t bufLen) {
    // CMD 76: no data; response = Open,Items,Amount[,Tender,Remainder]
    if (!_exec(CMD_RECEIPT_STATUS) || !_execOk()) return false;
    open  = (_last.data[0] == '1');
    const char* p = strchr(_last.data, ',');
    items = p ? (uint8_t)atoi(p + 1) : 0;
    if (totalBuf && bufLen > 0) {
        // Skip to third field
        const char* p2 = p ? strchr(p + 1, ',') : nullptr;
        if (p2) {
            strncpy(totalBuf, p2 + 1, bufLen - 1);
            // Trim at next comma
            char* stop = strchr(totalBuf, ',');
            if (stop) *stop = '\0';
            totalBuf[bufLen - 1] = '\0';
        }
    }
    return true;
}

// ─── Reports ─────────────────────────────────────────────────────────────────

bool DaisyPrinter::dailyReportZ() {
    // CMD 69: Operation "0" = Z report with clearing
    return _exec(CMD_DAILY_REPORT, "0") && _execOk();
}

bool DaisyPrinter::dailyReportX() {
    // CMD 69: Operation "2" = X report without clearing
    return _exec(CMD_DAILY_REPORT, "2") && _execOk();
}

bool DaisyPrinter::openTill(uint16_t pulseMs) {
    // CMD 106: [{ms}]
    char buf[8];
    if (!formatChecked(buf, sizeof(buf), "%u", (unsigned)pulseMs)) return false;
    return _exec(CMD_OPEN_TILL, buf) && _execOk();
}

// ─── Cash in / out ────────────────────────────────────────────────────────────

bool DaisyPrinter::cashIn(const char* amount, const char* currency) {
    // CMD 70: {Amount}  (positive = service entered / R/A)
    if (amount == nullptr) return false;
    if (currency == nullptr)
        return _exec(CMD_SERVICE_CASH, amount) && _execOk() && _last.passed();
    if (strcmp(currency, "EUR") != 0 && strcmp(currency, "BGN") != 0) return false;
    char buf[32];
    if (!formatChecked(buf, sizeof(buf), "%s,$%s", amount, currency)) return false;
    return _exec(CMD_SERVICE_CASH, buf) && _execOk() && _last.passed();
}

bool DaisyPrinter::cashOut(const char* amount, const char* currency) {
    // CMD 70: {-Amount} (negative = service derived / P/O)
    char buf[20];
    if (amount == nullptr) return false;
    if (currency != nullptr) {
        if (strcmp(currency, "EUR") != 0 && strcmp(currency, "BGN") != 0) return false;
        if (!formatChecked(buf, sizeof(buf), "-%s,$%s", amount, currency)) return false;
    } else if (!formatChecked(buf, sizeof(buf), "-%s", amount)) return false;
    return _exec(CMD_SERVICE_CASH, buf) && _execOk() && _last.passed();
}

// ─── Device info ─────────────────────────────────────────────────────────────

bool DaisyPrinter::getDiagnosticInfo(char* buf, uint8_t bufLen) {
    if (buf == nullptr || bufLen == 0) return false;
    // CMD 90: {Calculate}; use "1" to recalculate checksum
    // Response: FirmwareRev FirmwareDate FirmwareTime,CheckSum,Sw,Country,SerNum,FMNo
    if (!_exec(CMD_DIAGNOSTIC_INFO, "1") || !_execOk()) return false;
    strncpy(buf, _last.data, bufLen - 1);
    buf[bufLen - 1] = '\0';
    return true;
}

bool DaisyPrinter::getBulstat(char* buf, uint8_t bufLen) {
    if (buf == nullptr || bufLen == 0) return false;
    // CMD 99: no data; response = IdentNo
    if (!_exec(CMD_GET_BULSTAT) || !_execOk()) return false;
    strncpy(buf, _last.data, bufLen - 1);
    buf[bufLen - 1] = '\0';
    return true;
}

bool DaisyPrinter::getCurrentTaxRates(char* buf, uint8_t bufLen) {
    if (buf == nullptr || bufLen == 0) return false;
    // CMD 97: no data; response = Tax1,Tax2,...Tax8
    if (!_exec(CMD_CURRENT_TAX_RATES) || !_execOk()) return false;
    strncpy(buf, _last.data, bufLen - 1);
    buf[bufLen - 1] = '\0';
    return true;
}
