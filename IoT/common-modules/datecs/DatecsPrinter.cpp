/**
 * DatecsPrinter.cpp
 * High-level command methods for DatecsPrinter.
 * Each method builds the correct DATA field (tab-separated params as per protocol),
 * executes the command, and returns true only if both comms and device report success.
 */

#include "DatecsProtocol.h"

static bool datecsDigits(const char* value, size_t exactLength) {
    if (!value || strlen(value) != exactLength) return false;
    for (size_t i = 0; i < exactLength; ++i)
        if (value[i] < '0' || value[i] > '9') return false;
    return true;
}

static bool datecsUNP(const char* value) {
    if (!value || strlen(value) != 21 || value[8] != '-' || value[13] != '-') return false;
    if (value[0] < 'A' || value[0] > 'Z' || value[1] < 'A' || value[1] > 'Z') return false;
    for (size_t i = 2; i < 8; ++i) if (value[i] < '0' || value[i] > '9') return false;
    for (size_t i = 9; i < 13; ++i) {
        char c = value[i];
        if (!((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z'))) return false;
    }
    for (size_t i = 14; i < 21; ++i) if (value[i] < '0' || value[i] > '9') return false;
    return true;
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

bool DatecsPrinter::_exec(uint16_t cmd, const char* data) {
    return _proto.execute(cmd, data, _lastResp);
}

bool DatecsPrinter::_checkResp() {
    if (_lastResp.localError != DATECS_ERR_OK) return false;
    if (_lastResp.deviceError < 0) return false;
    // Status error bits that block operation
    if (_lastResp.status.syntaxError())   return false;
    if (_lastResp.status.invalidCommand()) return false;
    if (_lastResp.status.notPermitted())  return false;
    return true;
}

// ─── Display ─────────────────────────────────────────────────────────────────

bool DatecsPrinter::clearDisplay() {
    return _exec(CMD_CLEAR_DISPLAY) && _checkResp();
}

bool DatecsPrinter::displayLine1(const char* text) {
    // CMD 47: data = <text>\t
    char buf[24];
    int n = snprintf(buf, sizeof(buf), "%s\t", text ? text : "");
    if (n < 0 || n >= (int)sizeof(buf)) return false;
    return _exec(CMD_DISPLAY_LINE1, buf) && _checkResp();
}

bool DatecsPrinter::displayLine2(const char* text) {
    // CMD 35: data = <text>\t
    char buf[24];
    int n = snprintf(buf, sizeof(buf), "%s\t", text ? text : "");
    if (n < 0 || n >= (int)sizeof(buf)) return false;
    return _exec(CMD_DISPLAY_LINE2, buf) && _checkResp();
}

// ─── Non-fiscal receipt ───────────────────────────────────────────────────────

bool DatecsPrinter::openNonfiscal() {
    return _exec(CMD_OPEN_NONFISCAL) && _checkResp();
}

bool DatecsPrinter::closeNonfiscal() {
    return _exec(CMD_CLOSE_NONFISCAL) && _checkResp();
}

bool DatecsPrinter::printNonfiscalText(const char* text) {
    // CMD 42: Text, Bold, Italic, Height, Underline, Alignment.
    char buf[DATECS_MAX_DATA_TX];
    int n = snprintf(buf, sizeof(buf), "%s\t\t\t\t\t\t", text ? text : "");
    if (n < 0 || n >= (int)sizeof(buf)) return false;
    return _exec(CMD_PRINT_NONFISCAL_TEXT, buf) && _checkResp();
}

// ─── Fiscal receipt ───────────────────────────────────────────────────────────

bool DatecsPrinter::openFiscal(uint8_t operatorNum, const char* operatorPass,
                               uint32_t tillNum, const char* uniqueSaleNumber,
                               bool invoice) {
    if (operatorNum < 1 || operatorNum > 30 || !datecsDigits(operatorPass, 8) ||
        tillNum < 1 || tillNum > 99999 || (uniqueSaleNumber && uniqueSaleNumber[0] && !datecsUNP(uniqueSaleNumber)))
        return false;
    char buf[96];
    int n;
    if (uniqueSaleNumber && uniqueSaleNumber[0]) {
        // Syntax #2: OpCode, OpPwd, NSale (UNP), TillNmb, Invoice.
        n = snprintf(buf, sizeof(buf), "%u\t%s\t%s\t%lu\t%s\t",
                 (unsigned)operatorNum, operatorPass ? operatorPass : "",
                 uniqueSaleNumber, (unsigned long)tillNum, invoice ? "I" : "");
    } else {
        // Syntax #1. The final separator is mandatory even for blank Invoice.
        n = snprintf(buf, sizeof(buf), "%u\t%s\t%lu\t%s\t",
                 (unsigned)operatorNum, operatorPass ? operatorPass : "",
                 (unsigned long)tillNum, invoice ? "I" : "");
    }
    if (n < 0 || n >= (int)sizeof(buf)) return false;
    return _exec(CMD_OPEN_FISCAL, buf) && _checkResp();
}

bool DatecsPrinter::openStorno(uint8_t operatorNum, const char* operatorPass,
                                uint32_t tillNum, uint8_t stornoReason,
                                uint32_t originalDocumentNumber,
                                const char* originalDateTime,
                                const char* fiscalMemoryNumber,
                                const char* originalUniqueSaleNumber,
                                bool invoice, const char* invoiceNumber,
                                const char* invoiceReason) {
    if (operatorNum < 1 || operatorNum > 30 || !datecsDigits(operatorPass, 8) ||
        tillNum < 1 || tillNum > 99999 || stornoReason > 2 ||
        originalDocumentNumber < 1 || originalDocumentNumber > 9999999 ||
        !originalDateTime || !datecsDigits(fiscalMemoryNumber, 8) ||
        (originalUniqueSaleNumber && originalUniqueSaleNumber[0] && !datecsUNP(originalUniqueSaleNumber))) return false;
    if (invoice && (!invoiceNumber || !invoiceNumber[0])) return false;
    char buf[DATECS_MAX_DATA_TX];
    int n = snprintf(buf, sizeof(buf), "%u\t%s\t%lu\t%u\t%lu\t%s\t%s\t%s\t%s\t%s\t%s\t",
                     (unsigned)operatorNum, operatorPass, (unsigned long)tillNum,
                     (unsigned)stornoReason, (unsigned long)originalDocumentNumber,
                     originalDateTime, fiscalMemoryNumber, invoice ? "I" : "",
                     invoice ? invoiceNumber : "", invoice ? (invoiceReason ? invoiceReason : "") : "",
                     originalUniqueSaleNumber ? originalUniqueSaleNumber : "");
    if (n < 0 || n >= (int)sizeof(buf)) return false;
    return _exec(CMD_OPEN_STORNO, buf) && _checkResp();
}

bool DatecsPrinter::registerSale(const char* name, uint8_t vatGroup,
                                  const char* price, const char* qty,
                                  uint8_t discountType, const char* discountValue,
                                  uint8_t department, const char* unit) {
    if (!name || !price || vatGroup < 1 || vatGroup > 8 || discountType > 4 || department > 99)
        return false;
    if (discountType != 0 && (!discountValue || !discountValue[0])) return false;
    char buf[DATECS_MAX_DATA_TX];
    int n;
    if (unit && unit[0]) {
        n = snprintf(buf, sizeof(buf), "%s\t%u\t%s\t%s\t%u\t%s\t%u\t%s\t",
                     name, (unsigned)vatGroup, price, qty ? qty : "1",
                     (unsigned)discountType, discountValue ? discountValue : "",
                     (unsigned)department, unit);
    } else {
        n = snprintf(buf, sizeof(buf), "%s\t%u\t%s\t%s\t%u\t%s\t%u\t",
                     name, (unsigned)vatGroup, price, qty ? qty : "1",
                     (unsigned)discountType, discountValue ? discountValue : "",
                     (unsigned)department);
    }
    if (n < 0 || n >= (int)sizeof(buf)) return false;
    return _exec(CMD_REGISTER_SALE, buf) && _checkResp();
}

bool DatecsPrinter::subtotal(bool display, bool print) {
    // CMD 51: {Print}\t{Display}\t{DiscountType}\t{DiscountValue}\t
    char buf[8];
    snprintf(buf, sizeof(buf), "%c\t%c\t",
             print   ? '1' : '0',
             display ? '1' : '0');
    strncat(buf, "\t\t", sizeof(buf) - strlen(buf) - 1);
    return _exec(CMD_SUBTOTAL, buf) && _checkResp();
}

bool DatecsPrinter::total(uint8_t payType, const char* amount) {
    if (payType > 6 || !amount || !amount[0]) return false;
    char buf[32];
    snprintf(buf, sizeof(buf), "%u\t%s\t", (unsigned)payType, amount);
    return _exec(CMD_TOTAL, buf) && _checkResp();
}

bool DatecsPrinter::totalWithPinpad(const char* amount, uint8_t transactionType) {
    if (!amount || !amount[0] || (transactionType != 1 && transactionType != 12)) return false;
    char buf[40];
    int n = snprintf(buf, sizeof(buf), "2\t%s\t%u\t", amount, (unsigned)transactionType);
    if (n < 0 || n >= (int)sizeof(buf)) return false;
    return _exec(CMD_TOTAL, buf) && _checkResp();
}

bool DatecsPrinter::closeFiscal() {
    return _exec(CMD_CLOSE_FISCAL) && _checkResp();
}

bool DatecsPrinter::payAndClose(uint8_t payType, const char* amount) {
    return total(payType, amount) && closeFiscal();
}

bool DatecsPrinter::cancelFiscal() {
    return _exec(CMD_CANCEL_FISCAL) && _checkResp();
}

bool DatecsPrinter::printFiscalText(const char* text) {
    // CMD 54: Text, Bold, Italic, DoubleH, Underline, Alignment.
    char buf[DATECS_MAX_DATA_TX];
    int n = snprintf(buf, sizeof(buf), "%s\t\t\t\t\t\t", text ? text : "");
    if (n < 0 || n >= (int)sizeof(buf)) return false;
    return _exec(CMD_PRINT_FISCAL_TEXT, buf) && _checkResp();
}

// ─── Paper ───────────────────────────────────────────────────────────────────

bool DatecsPrinter::paperFeed(uint8_t lines) {
    // CMD 44: {Lines}\t
    char buf[8];
    snprintf(buf, sizeof(buf), "%u\t", (unsigned)lines);
    return _exec(CMD_PAPER_FEED, buf) && _checkResp();
}

bool DatecsPrinter::paperCut() {
    return _exec(CMD_PAPER_CUT) && _checkResp();
}

// ─── Date / Time ─────────────────────────────────────────────────────────────

bool DatecsPrinter::setDatetime(const char* dt) {
    // CMD 61: {DateTime}\t  (format: "DD-MM-YY hh:mm:ss")
    char buf[32];
    snprintf(buf, sizeof(buf), "%s\t", dt ? dt : "");
    return _exec(CMD_SET_DATETIME, buf) && _checkResp();
}

bool DatecsPrinter::readDatetime(char* buf, uint8_t bufLen) {
    // CMD 62: no params; response data = <datetime>\t
    if (!buf || bufLen == 0 || !_exec(CMD_READ_DATETIME) || !_checkResp()) return false;
    // Answer is ErrorCode<TAB>DateTime<TAB>; return the second token.
    const char* src = strchr(_lastResp.data, '\t');
    if (!src) return false;
    ++src;
    uint8_t i = 0;
    while (i < bufLen - 1 && src[i] && src[i] != '\t') {
        buf[i] = src[i];
        i++;
    }
    buf[i] = '\0';
    return true;
}

// ─── Status & Info ────────────────────────────────────────────────────────────

bool DatecsPrinter::readStatus(DatecsStatus& status) {
    // CMD 74: Reading the Status
    if (!_exec(CMD_READ_STATUS)) return false;
    status = _lastResp.status;
    return _lastResp.localError == DATECS_ERR_OK;
}

bool DatecsPrinter::checkPcMode() {
    // CMD 45: returns 0 in data if in PC mode
    if (!_exec(CMD_CHECK_PC_MODE)) return false;
    return _lastResp.deviceError == 0;
}

bool DatecsPrinter::openDrawer() {
    // CMD 106: no params
    return _exec(CMD_OPEN_DRAWER) && _checkResp();
}

// ─── Reports ─────────────────────────────────────────────────────────────────

bool DatecsPrinter::printReport(char type) {
    // CMD 69: {Option}\t   ('X' or 'Z' or 'D'/'G'/'P')
    char buf[4];
    snprintf(buf, sizeof(buf), "%c\t", type);
    return _exec(CMD_REPORTS, buf) && _checkResp();
}

// ─── Cash in / out ───────────────────────────────────────────────────────────

bool DatecsPrinter::cashIn(const char* amount) {
    // CMD 70: '0'\t{Amount}\t
    char buf[32];
    snprintf(buf, sizeof(buf), "0\t%s\t", amount ? amount : "0");
    return _exec(CMD_CASH_INOUT, buf) && _checkResp();
}

bool DatecsPrinter::cashOut(const char* amount) {
    // CMD 70: '1'\t{Amount}\t
    char buf[32];
    snprintf(buf, sizeof(buf), "1\t%s\t", amount ? amount : "0");
    return _exec(CMD_CASH_INOUT, buf) && _checkResp();
}

// ─── Device info ─────────────────────────────────────────────────────────────

bool DatecsPrinter::deviceInfo(char* buf, uint16_t bufLen) {
    // CMD 123 option 1: serial, FM number, headers and tax number.
    if (!buf || bufLen == 0 || !_exec(CMD_DEVICE_INFO, "1\t") || !_checkResp()) return false;
    strncpy(buf, _lastResp.data, bufLen - 1);
    buf[bufLen - 1] = '\0';
    return _lastResp.localError == DATECS_ERR_OK;
}
