#pragma once
/**
 * DaisyProtocol.h
 * Arduino C++ API — Daisy Tech Fiscal Device Communication Protocol v2.0-4 (2026)
 * Manufacturer: Daisy Tech, Sofia, Bulgaria
 *
 * Supported devices: FX21, FX1200B1, Perfect SA, Compact M/S, eXpert, Micro C, etc.
 *
 * ── PROTOCOL SUMMARY ──────────────────────────────────────────────────────────
 * Master/Slave RS232, 8N1. All text fields use CP1251 encoding.
 * Tax groups are Cyrillic letters А..З (CP1251: C0h..C7h).
 *
 * REQUEST (PC → FD):
 *   [STX:01][LEN:1][SEQ:1][CMD:1][DATA:0..200][PST1:05][BCC:4][ETX:03]
 *
 *   LEN  = (LEN + SEQ + CMD + DATA + PST1) byte count + 0x20 offset
 *          If total > 224, use 0xFF.
 *   SEQ  = sequential number + 0x20 offset (0x20..0xFF)
 *   BCC  = sum of bytes [LEN..PST1] inclusive. Each nibble is encoded by
 *          adding 30h, so 0x12AB is 31h 32h 3Ah 3Bh (not ASCII hex).
 *
 * RESPONSE (FD → PC):
 *   [STX:01][LEN:1][SEQ:1][CMD:1][DATA:0..200][PST2:04][STATUS:6][PST1:05][BCC:4][ETX:03]
 *
 *   BCC covers [LEN..PST1] inclusive (i.e. includes STATUS bytes).
 *
 * NON-PACKED:
 *   NAK (15h) — checksum/framing error; retransmit same SEQ+CMD
 *   SYN (16h) — device busy; sent every 100 ms; reset timeout on each
 *   DC1 (11h) — acknowledgement sent BY PC when receiving streamed report lines
 *
 * DELIMITERS used in DATA fields:
 *   TAB = 09h, LF = 0Ah
 */

#include <Arduino.h>

// ─── Protocol constants ───────────────────────────────────────────────────────

#define DAISY_STX      0x01
#define DAISY_PST2     0x04   // response postamble (before STATUS)
#define DAISY_PST1     0x05   // postamble before BCC
#define DAISY_ETX      0x03
#define DAISY_NAK      0x15
#define DAISY_SYN      0x16
#define DAISY_DC1      0x11   // PC acknowledgement for streamed data

#define DAISY_LEN_OFFSET  0x20
#define DAISY_LEN_MAX     0xFF  // used when payload > 224 bytes
#define DAISY_SEQ_MIN     0x20
#define DAISY_SEQ_MAX     0xFF

#define DAISY_MAX_DATA     200
#define DAISY_STATUS_LEN   6

// Total buffer sizes (worst case)
// TX: STX(1)+LEN(1)+SEQ(1)+CMD(1)+DATA(200)+PST1(1)+BCC(4)+ETX(1) = 210
// RX: STX(1)+LEN(1)+SEQ(1)+CMD(1)+DATA(200)+PST2(1)+STAT(6)+PST1(1)+BCC(4)+ETX(1) = 217
#define DAISY_TX_BUF  210
#define DAISY_RX_BUF  220

#define DAISY_TIMEOUT_MS  500
#define DAISY_MAX_RETRIES 3

// CP1251 tax group base (Cyrillic А = 0xC0)
#define DAISY_TAX_BASE  0xC0   // А=C0, Б=C1, В=C2, Г=C3, Д=C4, Е=C5, Ж=C6, З=C7

// ─── Command codes ────────────────────────────────────────────────────────────

enum DaisyCmd : uint8_t {
    CMD_CLEAR_DISPLAY        = 33,   // 21h
    CMD_DISPLAY_ROW2         = 35,   // 23h
    CMD_START_NONFISCAL      = 38,   // 26h
    CMD_END_NONFISCAL        = 39,   // 27h
    CMD_PRINT_NONFISCAL_TEXT = 42,   // 2Ah
    CMD_CLICHE_OPTIONS       = 43,   // 2Bh
    CMD_PAPER_FEED           = 44,   // 2Ch
    CMD_PAPER_CUT            = 45,   // 2Dh
    CMD_DISPLAY_ROW_N        = 46,   // 2Eh
    CMD_DISPLAY_ROW1         = 47,   // 2Fh
    CMD_START_FISCAL         = 48,   // 30h
    CMD_SALE                 = 49,   // 31h
    CMD_TAX_RATES_INFO       = 50,   // 32h
    CMD_SUBTOTAL             = 51,   // 33h
    CMD_SALE_DISPLAY         = 52,   // 34h
    CMD_TOTAL                = 53,   // 35h
    CMD_PRINT_FISCAL_TEXT    = 54,   // 36h
    CMD_START_FISCAL_NOPRINT = 55,   // 37h
    CMD_END_FISCAL           = 56,   // 38h
    CMD_PRINT_CUSTOMER_INFO  = 57,   // 39h
    CMD_SALE_BY_PLU          = 58,   // 3Ah
    CMD_SET_DATETIME         = 61,   // 3Dh
    CMD_GET_DATETIME         = 62,   // 3Eh
    CMD_DISPLAY_DATETIME     = 63,   // 3Fh
    CMD_LAST_FISCAL_INFO     = 64,   // 40h
    CMD_CURRENT_SUMS         = 65,   // 41h
    CMD_FM_ADDITIONAL        = 66,   // 42h
    CMD_FREE_FM_RECORDS      = 68,   // 44h
    CMD_DAILY_REPORT         = 69,   // 45h
    CMD_SERVICE_CASH         = 70,   // 46h
    CMD_PRINT_DIAGNOSTIC     = 71,   // 47h
    CMD_FM_REPORT_BY_NUM     = 73,   // 49h
    CMD_FD_STATUS            = 74,   // 4Ah
    CMD_RECEIPT_STATUS       = 76,   // 4Ch
    CMD_BRIEF_FM_BY_DATE     = 79,   // 4Fh
    CMD_PRINT_BARCODE        = 84,   // 54h
    CMD_CUSTOMER_QR          = 85,   // 55h
    CMD_DIAGNOSTIC_INFO      = 90,   // 5Ah
    CMD_FM_REPORT_BY_DATE    = 94,   // 5Eh
    CMD_BRIEF_FM_BY_NUM      = 95,   // 5Fh
    CMD_PROGRAM_TAX_RATES    = 96,   // 60h
    CMD_CURRENT_TAX_RATES    = 97,   // 61h
    CMD_GET_BULSTAT          = 99,   // 63h
    CMD_SEND_TO_DISPLAY      = 100,  // 64h
    CMD_OPERATOR_PASSWORD    = 101,  // 65h
    CMD_OPERATOR_NAME        = 102,  // 66h
    CMD_RECEIPT_INFO         = 103,  // 67h
    CMD_RESET_OPER_SALES     = 104,  // 68h
    CMD_OPERATORS_REPORT     = 105,  // 69h
    CMD_OPEN_TILL            = 106,  // 6Ah
    CMD_PLU_MANAGE           = 107,  // 6Bh
    CMD_DETAILED_DAILY       = 108,  // 6Ch
    CMD_PRINT_DUPLICATE      = 109,  // 6Dh
    CMD_CURRENT_DAY_INFO     = 110,  // 6Eh
    CMD_PLU_REPORT           = 111,  // 6Fh
    CMD_OPERATOR_INFO        = 112,  // 70h
    CMD_LAST_DOC_NUM         = 113,  // 71h
    CMD_FM_INFO_BY_NUM       = 114,  // 72h
    CMD_LOAD_LOGO            = 115,  // 73h
    CMD_ISSUED_DOC_QR        = 116,  // 74h
    CMD_FIRST_NOT_SENT       = 117,  // 75h
    CMD_FIRMWARE_INFO        = 118,  // 76h
    CMD_ISSUED_DOC_INFO      = 119,  // 77h
    CMD_CONSTANTS_INFO       = 128,  // 80h
    CMD_CANCEL_RECEIPT       = 130,  // 82h
    CMD_DEPT_MANAGE          = 131,  // 83h
    CMD_LOAD_DISPLAY_TABLE   = 132,  // 84h
    CMD_DISPLAY_CONFIG       = 133,  // 85h
    CMD_PARKING_MANAGE       = 134,  // 86h
    CMD_SALE_BY_DEPT         = 138,  // 8Ah
    CMD_FM_INFO_BY_DATE      = 146,  // 92h
    CMD_TEXT_FIELD           = 149,  // 95h
    CMD_SYS_PARAMS           = 150,  // 96h
    CMD_PAYMENTS_MANAGE      = 151,  // 97h
    CMD_CUSTOMERS_MANAGE     = 152,  // 98h
    CMD_SEND_REPORT_TEXT     = 153,  // 99h
    CMD_ROUTE_MANAGE         = 154,  // 9Ah
    CMD_STATION_MANAGE       = 155,  // 9Bh
    CMD_ROUTE_PRICE          = 156,  // 9Ch
    CMD_TICKET_DISCOUNTS     = 157,  // 9Dh
    CMD_DEPT_REPORT          = 165,  // A5h
    CMD_PRINT_SYS_PARAMS     = 166,  // A6h
    CMD_BATTERY_INFO         = 173,  // ADh
    CMD_ERROR_DESCRIPTION    = 174,  // AEh
    CMD_PRINT_TAX_RATES      = 176,  // B0h
    CMD_TERMINAL_INFO        = 194,  // C2h
    CMD_EJT_REPORTS          = 195,  // C3h
    CMD_CURRENCY_DATES       = 201,  // C9h
};

// ─── Local (transport-level) error codes ─────────────────────────────────────

enum DaisyError : int8_t {
    DAISY_OK               =  0,
    DAISY_ERR_TIMEOUT      = -1,   // no response within timeout
    DAISY_ERR_NAK          = -2,   // NAK received; will retry
    DAISY_ERR_BAD_FRAME    = -3,   // malformed packet structure
    DAISY_ERR_BCC          = -4,   // BCC verification failed
    DAISY_ERR_SEQ          = -5,   // response SEQ mismatch
    DAISY_ERR_CMD          = -6,   // response CMD mismatch
    DAISY_ERR_OVERFLOW     = -7,   // DATA too long to fit buffer
    DAISY_ERR_INVALID_ARG  = -8,
    DAISY_ERR_WRITE        = -9,
};

// ─── Status flags (6 bytes) ───────────────────────────────────────────────────

struct DaisyStatus {
    uint8_t raw[DAISY_STATUS_LEN];

    void clear() { memset(raw, 0, DAISY_STATUS_LEN); }

    // ── Byte 0: general ──────────────────────────────────────────────────────
    bool generalError()      const { return raw[0] & 0x20; } // bit5: OR of all *
    bool printMechError()    const { return raw[0] & 0x10; } // bit4 *
    bool noExternalDisplay() const { return raw[0] & 0x08; } // bit3
    bool dateTimeNotSet()    const { return raw[0] & 0x04; } // bit2
    bool invalidCommand()    const { return raw[0] & 0x02; } // bit1 *
    bool syntaxError()       const { return raw[0] & 0x01; } // bit0 *

    // ── Byte 1: general ──────────────────────────────────────────────────────
    bool wrongPassword()     const { return raw[1] & 0x40; } // bit6
    bool cutterError()       const { return raw[1] & 0x20; } // bit5
    bool changedEJT()        const { return raw[1] & 0x08; } // bit3 *
    bool zeroedMemory()      const { return raw[1] & 0x04; } // bit2 *
    bool cmdNotAllowed()     const { return raw[1] & 0x02; } // bit1 *
    bool sumsOverflow()      const { return raw[1] & 0x01; } // bit0

    // ── Byte 2: general ──────────────────────────────────────────────────────
    bool printingEnabled()   const { return raw[2] & 0x40; } // bit6
    bool nonfiscalOpen()     const { return raw[2] & 0x20; } // bit5
    bool paperLowJT()        const { return raw[2] & 0x10; } // bit4
    bool fiscalOpen()        const { return raw[2] & 0x08; } // bit3
    bool outOfPaperJT()      const { return raw[2] & 0x04; } // bit2
    bool paperLow()          const { return raw[2] & 0x02; } // bit1
    bool outOfPaper()        const { return raw[2] & 0x01; } // bit0 *

    // ── Byte 3: FD error number ───────────────────────────────────────────────
    uint8_t errorCode()      const { return raw[3] & 0x7F; } // bits 0-6
    bool    hasDeviceError() const { return errorCode() != 0; }

    // ── Byte 4: FM ────────────────────────────────────────────────────────────
    bool tempDeregistered()  const { return raw[4] & 0x40; } // bit6
    bool fmGeneralError()    const { return raw[4] & 0x20; } // bit5: OR of *
    bool fmFull()            const { return raw[4] & 0x10; } // bit4 *
    bool fmLow()             const { return raw[4] & 0x08; } // bit3 (<50 left)
    bool fmInvalidRecord()   const { return raw[4] & 0x04; } // bit2
    bool taxTerminalError()  const { return raw[4] & 0x02; } // bit1
    bool fmWriteError()      const { return raw[4] & 0x01; } // bit0 *

    // ── Byte 5: FM ────────────────────────────────────────────────────────────
    bool dataTransferOpen()  const { return raw[5] & 0x40; } // bit6
    bool fdIdProgrammed()    const { return raw[5] & 0x20; } // bit5
    bool taxRatesSet()       const { return raw[5] & 0x10; } // bit4
    bool fdActivated()       const { return raw[5] & 0x08; } // bit3 (fiscalized)
    bool fmOverflowed()      const { return raw[5] & 0x01; } // bit0 *

    // ── Aggregate checks ─────────────────────────────────────────────────────
    bool anyError() const {
        return generalError() || fmGeneralError() || dateTimeNotSet() ||
               wrongPassword() || cutterError() || sumsOverflow() ||
               outOfPaper() || outOfPaperJT() || hasDeviceError() ||
               fmFull() || fmInvalidRecord() || taxTerminalError() ||
               fmWriteError() || fmOverflowed() || changedEJT() || zeroedMemory();
    }
    bool receiptOpen() const { return fiscalOpen() || nonfiscalOpen(); }
};

// ─── Parsed response ─────────────────────────────────────────────────────────

struct DaisyResponse {
    uint8_t     seq;
    uint8_t     cmd;
    char        data[DAISY_MAX_DATA + 1];   // null-terminated
    uint8_t     dataLen;
    DaisyStatus status;
    DaisyError  localError;

    bool ok() const {
        return localError == DAISY_OK
            && !status.generalError()
            && !status.fmGeneralError()
            && !status.syntaxError()
            && !status.invalidCommand()
            && !status.cmdNotAllowed()
            && !status.wrongPassword()
            && !status.cutterError()
            && !status.dateTimeNotSet()
            && !status.sumsOverflow()
            && !status.outOfPaper()
            && !status.outOfPaperJT()
            && !status.hasDeviceError()
            && !status.fmFull()
            && !status.fmInvalidRecord()
            && !status.taxTerminalError()
            && !status.fmWriteError()
            && !status.fmOverflowed();
    }

    // First char of DATA — commonly 'P' (pass) or 'F' (fail)
    char resultCode() const { return dataLen > 0 ? data[0] : '\0'; }
    bool passed()     const { return resultCode() == 'P'; }

    void clear() {
        seq = cmd = dataLen = 0;
        data[0] = '\0';
        status.clear();
        localError = DAISY_OK;
    }
};

// ─── Core protocol class ──────────────────────────────────────────────────────

class DaisyProtocol {
public:
    explicit DaisyProtocol(Stream& serial,
                           uint16_t timeoutMs  = DAISY_TIMEOUT_MS,
                           uint8_t  maxRetries = DAISY_MAX_RETRIES)
        : _serial(serial), _timeout(timeoutMs), _retries(maxRetries),
          _seq(DAISY_SEQ_MIN), _txLen(0), _rxLen(0) {}

    // ── Primary API ───────────────────────────────────────────────────────────

    /**
     * Send command + data, receive and parse response. Retries on NAK/timeout.
     * @return true on successful communication (check resp.ok() for device-level success)
     */
    bool execute(uint8_t cmd, const char* data, uint8_t dataLen, DaisyResponse& resp);

    // Convenience overloads
    bool execute(uint8_t cmd, DaisyResponse& resp) {
        return execute(cmd, nullptr, 0, resp);
    }
    bool execute(uint8_t cmd, const char* data, DaisyResponse& resp);

    // ── Streaming / report mode ───────────────────────────────────────────────

    /**
     * Send DC1 (11h) acknowledgement — required when receiving streamed report lines
     * from commands 153 (99h) and 195 (C3h).
     */
    void sendAck();

    /**
     * Wait for one streamed report line (1Ah framed) or the final packed response.
     * Fills lineBuf (null-terminated) if a line is received.
     * @return  1 = line received, 0 = final packed response received (in resp), -1 = timeout
     */
    int8_t receiveReportLine(char* lineBuf, uint16_t bufLen,
                             DaisyResponse& resp, uint16_t timeoutMs = 1000);

    using ReportChunkCallback = bool(*)(uint8_t type, const uint8_t* data,
                                        uint16_t len, void* context);
    bool executeStreaming(uint8_t cmd, const char* data, uint8_t dataLen,
                          ReportChunkCallback callback, void* context,
                          DaisyResponse& finalResponse,
                          uint16_t lineTimeoutMs = 1000);

    // ── Low-level / debug ─────────────────────────────────────────────────────

    size_t   sendPacket(uint8_t cmd, const char* data, uint8_t dataLen);
    bool     receivePacket(DaisyResponse& resp);
    void     resetSeq()     { _seq = DAISY_SEQ_MIN; }
    uint8_t  currentSeq()   const { return _seq; }

    const uint8_t* lastTxBuf() const { return _txBuf; }
    uint8_t        lastTxLen() const { return _txLen; }
    const uint8_t* lastRxBuf() const { return _rxBuf; }
    uint8_t        lastRxLen() const { return _rxLen; }

    // ── Static utilities ──────────────────────────────────────────────────────

    /** Encode 16-bit BCC as four nibble+0x30 bytes (0x12AB → 31 32 3A 3B). */
    static void   encodeBCC(uint16_t val, uint8_t out[4]);

    /** Decode four protocol BCC bytes (each nibble + 30h). */
    static uint16_t decodeBCC(const uint8_t in[4]);
    static bool     decodeBCC(const uint8_t in[4], uint16_t& value);

    /** Compute checksum: sum of bytes buf[from..to] inclusive */
    static uint16_t checksum(const uint8_t* buf, uint8_t from, uint8_t to);

    static const char* errorString(DaisyError e);

private:
    Stream&  _serial;
    uint16_t _timeout;
    uint8_t  _retries;
    uint8_t  _seq;

    uint8_t _txBuf[DAISY_TX_BUF];
    uint8_t _txLen;
    uint8_t _rxBuf[DAISY_RX_BUF];
    uint8_t _rxLen;

    bool    _waitByte(uint8_t& b, uint16_t ms);
    bool    _readUntilETX(uint16_t ms);
    bool    _parseBufferedPacket(DaisyResponse& resp);
    bool    _writeAll(const uint8_t* data, uint8_t len);
    void    _advanceSeq();
};

// ─── High-level printer API ───────────────────────────────────────────────────

class DaisyPrinter {
public:
    explicit DaisyPrinter(Stream& serial,
                          uint16_t timeoutMs  = DAISY_TIMEOUT_MS,
                          uint8_t  maxRetries = DAISY_MAX_RETRIES)
        : _proto(serial, timeoutMs, maxRetries) {}

    // ── Display ───────────────────────────────────────────────────────────────
    bool clearDisplay();
    bool displayRow1(const char* text);
    bool displayRow2(const char* text);
    bool displayRowN(uint8_t row, const char* text);

    // ── Non-fiscal receipt ────────────────────────────────────────────────────
    bool startNonfiscal();
    bool endNonfiscal(uint32_t* allReceipts = nullptr);
    bool printNonfiscalText(const char* text);

    // ── Fiscal receipt ────────────────────────────────────────────────────────
    /**
     * Open a fiscal receipt (standard variant).
     * @param operNum   Operator number (1..max)
     * @param password  Operator password (up to 6 digits as string)
     * @param unp       Unique sale number, e.g. "DY000600-OP01-0000001"
     * @param invoice   true = invoice receipt
     * @param allRec    Output: total receipts today
     * @param fiscRec   Output: fiscal receipts today
     */
    bool startFiscal(uint8_t operNum, const char* password, const char* unp,
                     bool invoice = false,
                     uint32_t* allRec = nullptr, uint32_t* fiscRec = nullptr);
    bool startRefund(uint8_t operNum, const char* password, const char* unp,
                     uint8_t reason, uint32_t originalDocument,
                     const char* originalDatetime, const char* originalFmNumber,
                     uint32_t* allRec = nullptr, uint32_t* fiscRec = nullptr);

    /**
     * Register a sale line.
     * @param name     Item name (CP1251, up to COMMENT_LEN chars)
     * @param taxGroup Tax group: 'A'..'H' (Latin) mapped to Cyrillic internally,
     *                 OR pass the raw CP1251 byte (0xC0..0xC7) as a char.
     *                 For simplicity this API accepts 'A'=А, 'B'=Б, ... 'H'=З.
     * @param price    Price string, e.g. "12.50"
     * @param qty      Quantity string, e.g. "1" or "2.500" (optional)
     * @param percent  Discount/surcharge percent, e.g. "-10.00" (optional, nullptr = none)
     */
    bool sale(const char* name, char taxGroup, const char* price,
              const char* qty = nullptr, const char* percent = nullptr);
    bool saleEx(const char* text1, const char* text2, char taxGroup,
                const char* price, const char* qty = nullptr,
                const char* percent = nullptr, const char* netto = nullptr,
                bool correction = false);

    /** Subtotal. @param print print on paper; @param display show on display */
    bool subtotal(bool print, bool display,
                  char* subTotalOut = nullptr, uint8_t bufLen = 0);

    /**
     * Total / payment.
     * @param payType  'P'=cash, 'N'=pay1, 'C'=pay2, 'D'=pay3, 'B'=pay4
     * @param amount   Amount tendered as string (nullptr = exact amount)
     * @param changeOut Output buffer for change amount (optional)
     */
    bool total(char payType = 'P', const char* amount = nullptr,
               char* changeOut = nullptr, uint8_t changeLen = 0);

    bool endFiscal(uint32_t* allRec = nullptr, uint32_t* fiscRec = nullptr);
    bool cancelReceipt();
    bool printFiscalText(const char* text);

    /**
     * Print customer info for invoice receipt.
     * All parameters except identNo are optional (pass nullptr to omit).
     */
    bool printCustomerInfo(const char* identNo, const char* regNo = nullptr,
                           const char* seller = nullptr, const char* receiver = nullptr,
                           const char* client = nullptr, const char* address = nullptr);

    // ── Paper ─────────────────────────────────────────────────────────────────
    bool paperFeed(uint8_t lines = 1);
    bool paperCut(uint8_t mode = 1);  // 1=full, 2=partial

    // ── Date / time ───────────────────────────────────────────────────────────
    /** Format: "DD-MM-YY HH:mm" or "DD-MM-YY HH:mm:SS" */
    bool setDatetime(const char* dt);
    /** Reads datetime into buf (min 20 bytes). Returns false on error. */
    bool getDatetime(char* buf, uint8_t bufLen);

    // ── Status ────────────────────────────────────────────────────────────────
    bool getStatus(DaisyStatus& status);

    // ── Receipt status ────────────────────────────────────────────────────────
    /** Quick check if a receipt is open and how many items are in it. */
    bool getReceiptStatus(bool& open, uint8_t& items, char* totalBuf = nullptr, uint8_t bufLen = 0);

    // ── Reports ───────────────────────────────────────────────────────────────
    bool dailyReportZ();    // Z report (with clearing)
    bool dailyReportX();    // X report (without clearing)
    bool openTill(uint16_t pulseMs = 100);

    // ── Cash in / out ─────────────────────────────────────────────────────────
    bool cashIn(const char* amount, const char* currency = nullptr);
    bool cashOut(const char* amount, const char* currency = nullptr);

    // ── Device info ───────────────────────────────────────────────────────────
    /** Returns firmware version + date + CS + serial in buf */
    bool getDiagnosticInfo(char* buf, uint8_t bufLen);
    bool getBulstat(char* buf, uint8_t bufLen);

    // ── Tax groups ────────────────────────────────────────────────────────────
    bool getCurrentTaxRates(char* buf, uint8_t bufLen);

    // ── Access underlying protocol ────────────────────────────────────────────
    DaisyProtocol& proto() { return _proto; }
    const DaisyResponse& lastResponse() const { return _last; }

    // ── Tax group helper ──────────────────────────────────────────────────────
    /** Convert Latin 'A'..'H' to CP1251 Cyrillic byte for tax group field */
    static uint8_t taxGroupByte(char latin) {
        uint8_t raw = (uint8_t)latin;
        if (raw >= 0xC0 && raw <= 0xC7) return raw;
        if (latin >= 'A' && latin <= 'H') return DAISY_TAX_BASE + (latin - 'A');
        if (latin >= 'a' && latin <= 'h') return DAISY_TAX_BASE + (latin - 'a');
        return DAISY_TAX_BASE; // default А
    }

private:
    DaisyProtocol _proto;
    DaisyResponse _last;

    bool _exec(uint8_t cmd, const char* data = nullptr);
    bool _execOk();  // checks _last.ok()
};
