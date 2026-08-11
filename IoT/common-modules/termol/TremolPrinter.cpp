/**
 * TremolPrinter.cpp
 * High-level command implementations for Tremol fiscal printers.
 *
 * DATA FORMAT rules (from protocol spec):
 *  - Fields separated by semicolons ';'
 *  - Prices: floating point, +/- prefix, e.g. "+12.50" or "-3.00"
 *  - Qty:    floating point, up to 3 decimals, e.g. "2.000"
 *  - Rate%:  preceded by '%', e.g. "%-10.00"
 *  - VAT:    CP1251 Cyrillic А(C0h)..З(C7h), '*'=forbidden
 *            API accepts Latin 'A'..'H' and converts via vatByte()
 *  - Dept:   raw byte = DepNum + 0x80 (e.g. Dept1=0x81)
 *  - URN:    "XXXXXXXX-ZZZZ-NNNNNNN" (24 chars, NRA BG format)
 */

#include "TremolProtocol.h"

// ─── Internal helpers ─────────────────────────────────────────────────────────

bool TremolPrinter::_exec(uint8_t cmd, const char* data) {
    return _proto.execute(cmd, data, _last);
}

bool TremolPrinter::_ok() {
    return _last.ok();
}

// Parse the STATUS output of CMD_STATUS (0x20):
// Response DATA = 7 ASCII hex chars (each char = one status byte value)
// The 7 bytes are returned as ASCII decimal digits in their raw form.
// Actually per spec the output is <StatusBytes[7]> — 7 raw bytes in DATA.
static void parseStatusBytes(const char* data, uint8_t len, TremolStatus& st) {
    st.clear();
    // Data field contains 7 bytes representing status
    for (uint8_t i = 0; i < 7 && i < len; i++) {
        st.raw[i] = (uint8_t)data[i];
    }
}

// ─── Device info ─────────────────────────────────────────────────────────────

bool TremolPrinter::readStatus(TremolStatus& status) {
    if (!_exec(CMD_STATUS)) return false;
    if (_last.localError != TR_ERR_OK) return false;
    // If it's a message response, DATA has the 7 status bytes
    if (!_last.isAck) {
        parseStatusBytes(_last.data, _last.dataLen, status);
    } else {
        // ACK only: no byte data; STE still gives error info
        status.clear();
    }
    return true;
}

uint8_t TremolPrinter::pollReady() {
    return _proto.pollStatus(300);
}

bool TremolPrinter::readVersion(char* buf, uint8_t bufLen) {
    // CMD 21h: output = DeviceType;CertNum;CertDT;Model;Version
    if (!_exec(CMD_VERSION) || !_last.ok()) return false;
    if (!_last.isAck) {
        strncpy(buf, _last.data, bufLen - 1);
        buf[bufLen - 1] = '\0';
    }
    return true;
}

bool TremolPrinter::printDiagnostics() {
    return _exec(CMD_DIAGNOSTICS) && _ok();
}

// ─── Date / time ─────────────────────────────────────────────────────────────

bool TremolPrinter::setDatetime(const char* dt) {
    // CMD 48h: input = <DateTime "DD-MM-YYYY HH:MM:SS">
    return _exec(CMD_SET_DATETIME, dt) && _ok();
}

bool TremolPrinter::getDatetime(char* buf, uint8_t bufLen) {
    // CMD 68h: output = date;time (two separate fields)
    if (!_exec(CMD_READ_DATETIME) || _last.localError != TR_ERR_OK) return false;
    if (!_last.isAck) {
        strncpy(buf, _last.data, bufLen - 1);
        buf[bufLen - 1] = '\0';
        return true;
    }
    return false;
}

// ─── Display ─────────────────────────────────────────────────────────────────

bool TremolPrinter::clearDisplay() {
    return _exec(CMD_CLEAR_DISPLAY) && _ok();
}

bool TremolPrinter::displayLine1(const char* text20) {
    // CMD 25h: input = <Text[20]>
    return _exec(CMD_DISPLAY_L1, text20) && _ok();
}

bool TremolPrinter::displayLine2(const char* text20) {
    // CMD 26h: input = <Text[20]>
    return _exec(CMD_DISPLAY_L2, text20) && _ok();
}

bool TremolPrinter::displayLines(const char* text40) {
    // CMD 27h: input = <Text[40]> — first 20 → L1, last 20 → L2
    return _exec(CMD_DISPLAY_L12, text40) && _ok();
}

// ─── Paper / drawer ───────────────────────────────────────────────────────────

bool TremolPrinter::paperFeed() {
    return _exec(CMD_PAPER_FEED) && _ok();
}

bool TremolPrinter::cutPaper() {
    return _exec(CMD_CUT_PAPER) && _ok();
}

bool TremolPrinter::openDrawer() {
    return _exec(CMD_CASH_DRAWER) && _ok();
}

// ─── Non-fiscal receipt ───────────────────────────────────────────────────────

bool TremolPrinter::openNonfiscal(uint8_t operNum, const char* operPass, char printType) {
    // CMD 2Eh: <OperNum[1..2]> ; <OperPass[6]> { ; '0' ; <NonFiscalPrintType[1]> }
    char buf[32];
    snprintf(buf, sizeof(buf), "%u;%s;0;%c",
             (unsigned)operNum,
             operPass ? operPass : "000000",
             printType);
    return _exec(CMD_OPEN_NONFISCAL, buf) && _ok();
}

bool TremolPrinter::closeNonfiscal() {
    // CMD 2Fh: no data
    return _exec(CMD_CLOSE_NONFISCAL) && _ok();
}

bool TremolPrinter::printText(const char* text) {
    // CMD 37h: <Text[TextLength-2]>
    // Works in both fiscal and non-fiscal receipts
    return _exec(CMD_PRINT_TEXT, text) && _ok();
}

// ─── Fiscal receipt open ─────────────────────────────────────────────────────

bool TremolPrinter::openFiscal(uint8_t operNum, const char* operPass,
                                char format, char printVAT,
                                char printType, const char* urn) {
    // CMD 30h: <OperNum> ; <OperPass> ; <ReceiptFormat> ; <PrintVAT> ; <FiscalRcpPrintType>
    //          { '$' <UniqueReceiptNumber[24]> }
    char buf[64];
    if (urn && *urn) {
        snprintf(buf, sizeof(buf), "%u;%s;%c;%c;%c$%s",
                 (unsigned)operNum,
                 operPass ? operPass : "000000",
                 format, printVAT, printType, urn);
    } else {
        snprintf(buf, sizeof(buf), "%u;%s;%c;%c;%c",
                 (unsigned)operNum,
                 operPass ? operPass : "000000",
                 format, printVAT, printType);
    }
    return _exec(CMD_OPEN_FISCAL, buf) && _ok();
}

// ─── Sell ─────────────────────────────────────────────────────────────────────

bool TremolPrinter::sell(const char* name, char vatClass, const char* price,
                          const char* qty, const char* discP, const char* discV) {
    // CMD 31h: <NamePLU[36]> ; <OptionVATClass[1]> ; <Price> {* <Qty>} {, <DiscP>} {: <DiscV>}
    char buf[TR_TX_BUF - 8];
    uint8_t vat = TremolPrinter::vatByte(vatClass);
    uint8_t pos = 0;

    // Name (max 36 chars)
    if (name) {
        uint8_t nlen = (uint8_t)strlen(name);
        if (nlen > 36) nlen = 36;
        memcpy(buf + pos, name, nlen); pos += nlen;
    }
    // ;VAT
    buf[pos++] = TR_SEP;
    buf[pos++] = (char)vat;
    // ;Price
    buf[pos++] = TR_SEP;
    if (price) { uint8_t l = (uint8_t)strlen(price); memcpy(buf+pos, price, l); pos += l; }

    // *Qty (optional)
    if (qty && *qty) {
        buf[pos++] = '*';
        uint8_t l = (uint8_t)strlen(qty);
        memcpy(buf+pos, qty, l); pos += l;
    }
    // ,DiscP (optional percent discount)
    if (discP && *discP) {
        buf[pos++] = ',';
        uint8_t l = (uint8_t)strlen(discP);
        memcpy(buf+pos, discP, l); pos += l;
    }
    // :DiscV (optional value discount)
    if (discV && *discV) {
        buf[pos++] = ':';
        uint8_t l = (uint8_t)strlen(discV);
        memcpy(buf+pos, discV, l); pos += l;
    }
    buf[pos] = '\0';
    return _exec(CMD_SELL, buf) && _ok();
}

bool TremolPrinter::sellDept(const char* name, uint8_t depNum, const char* price,
                              const char* qty, const char* discP, const char* discV) {
    // CMD 31h variant: <NamePLU> ; <' '> ; <Price> {* Qty} {, DiscP} {: DiscV} {! DepByte}
    // DepByte = depNum + 0x80
    char buf[TR_TX_BUF - 8];
    uint8_t pos = 0;
    uint8_t depByte = TR_DEP_BASE + depNum;

    if (name) {
        uint8_t nlen = (uint8_t)strlen(name);
        if (nlen > 36) nlen = 36;
        memcpy(buf+pos, name, nlen); pos += nlen;
    }
    buf[pos++] = TR_SEP;
    buf[pos++] = ' ';  // reserved space (department variant)
    buf[pos++] = TR_SEP;
    if (price) { uint8_t l = (uint8_t)strlen(price); memcpy(buf+pos, price, l); pos += l; }
    if (qty && *qty)   { buf[pos++]='*'; uint8_t l=(uint8_t)strlen(qty);   memcpy(buf+pos,qty,l);   pos+=l; }
    if (discP && *discP) { buf[pos++]=','; uint8_t l=(uint8_t)strlen(discP); memcpy(buf+pos,discP,l); pos+=l; }
    if (discV && *discV) { buf[pos++]=':'; uint8_t l=(uint8_t)strlen(discV); memcpy(buf+pos,discV,l); pos+=l; }
    // Department: '!' followed by raw depByte
    buf[pos++] = '!';
    buf[pos++] = (char)depByte;
    buf[pos] = '\0';
    return _exec(CMD_SELL, buf) && _ok();
}

// ─── Subtotal ─────────────────────────────────────────────────────────────────

bool TremolPrinter::subtotal(char print, char display,
                              const char* discP, const char* discV,
                              char* outBuf, uint8_t outLen) {
    // CMD 33h: <OptionPrinting> ; <OptionDisplay> {: DiscV} {, DiscP}
    char buf[32];
    uint8_t pos = 0;
    buf[pos++] = print;
    buf[pos++] = TR_SEP;
    buf[pos++] = display;
    if (discV && *discV) { buf[pos++]=':'; uint8_t l=(uint8_t)strlen(discV); memcpy(buf+pos,discV,l); pos+=l; }
    if (discP && *discP) { buf[pos++]=','; uint8_t l=(uint8_t)strlen(discP); memcpy(buf+pos,discP,l); pos+=l; }
    buf[pos] = '\0';

    if (!_exec(CMD_SUBTOTAL) || _last.localError != TR_ERR_OK) return false;
    // STE errors in ACK still signal failure
    if (!_ok()) return false;
    // Message response contains SubtotalValue
    if (!_last.isAck && outBuf && outLen > 0) {
        strncpy(outBuf, _last.data, outLen - 1);
        outBuf[outLen - 1] = '\0';
    }
    return true;
}

// ─── Payment ──────────────────────────────────────────────────────────────────

bool TremolPrinter::payment(char payType, char withChange,
                             const char* amount, char changeType) {
    // CMD 35h: <PaymentType[1..2]> ; <OptionChange[1]> ; <Amount[1..10]> { * <OptionChangeType> }
    char buf[32];
    uint8_t pos = 0;
    buf[pos++] = payType;
    buf[pos++] = TR_SEP;
    buf[pos++] = withChange;
    buf[pos++] = TR_SEP;
    if (amount && *amount) {
        uint8_t l = (uint8_t)strlen(amount);
        memcpy(buf+pos, amount, l); pos += l;
    }
    if (changeType != '0') {
        buf[pos++] = '*';
        buf[pos++] = changeType;
    }
    buf[pos] = '\0';
    return _exec(CMD_PAYMENT, buf) && _ok();
}

bool TremolPrinter::cashPayClose() {
    // CMD 36h: no data — pays exact cash and closes
    return _exec(CMD_AUTO_CLOSE) && _ok();
}

bool TremolPrinter::closeFiscal() {
    // CMD 38h: no data
    return _exec(CMD_CLOSE_FISCAL) && _ok();
}

bool TremolPrinter::cancelFiscal() {
    // CMD 39h: no data
    return _exec(CMD_CANCEL) && _ok();
}

// ─── RA / PO (cash in / out) ─────────────────────────────────────────────────

bool TremolPrinter::cashIn(const char* amount, const char* text) {
    // CMD 3Bh: <OptionRApayment['R']> ; <Amount> { ; <Text> }
    // Positive = RA (cash in)
    char buf[64];
    if (text && *text) {
        snprintf(buf, sizeof(buf), "R;%s;%s", amount ? amount : "0", text);
    } else {
        snprintf(buf, sizeof(buf), "R;%s", amount ? amount : "0");
    }
    return _exec(CMD_RA_PO, buf) && _ok();
}

bool TremolPrinter::cashOut(const char* amount, const char* text) {
    // CMD 3Bh: <OptionPOpayment['P']> ; <Amount> { ; <Text> }
    // Negative = PO (cash out)
    char buf[64];
    if (text && *text) {
        snprintf(buf, sizeof(buf), "P;%s;%s", amount ? amount : "0", text);
    } else {
        snprintf(buf, sizeof(buf), "P;%s", amount ? amount : "0");
    }
    return _exec(CMD_RA_PO, buf) && _ok();
}

// ─── Reports ──────────────────────────────────────────────────────────────────

bool TremolPrinter::printXReport() {
    // CMD 7Ch: <Option['0']>  — X report (no Z clearing)
    return _exec(CMD_PRINT_XZ, "0") && _ok();
}

bool TremolPrinter::printZReport() {
    // CMD 7Ch: <Option['1']>  — Z report (with FM write)
    return _exec(CMD_PRINT_XZ, "1") && _ok();
}

bool TremolPrinter::printDeptReport() {
    return _exec(CMD_PRINT_DEPT_RPT) && _ok();
}

bool TremolPrinter::printOperatorReport() {
    return _exec(CMD_PRINT_OPER_RPT) && _ok();
}
