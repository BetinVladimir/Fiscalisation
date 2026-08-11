#pragma once
/**
 * DatecsProtocol.h
 * Arduino C++ API for Datecs Fiscal Printer Communication Protocol v2.11
 * Supports: DP-25X, DP-150X, WP-500X, WP-50X, FP-700X, FMP-350X, FP-50X, etc.
 *
 * Protocol overview:
 *   - Master/Slave RS232 (USB/LAN/WLAN), 8N1, up to 115200 baud
 *   - Host sends wrapped packet → device responds with wrapped packet
 *   - Non-wrapped single-byte codes: NAK (15h), SYN (16h)
 *   - Response timeout: 500 ms; SYN sent every 60 ms while busy
 *
 * Packet format (Request):
 *   <PRE:01> <LEN:4> <SEQ:1> <CMD:4> <DATA:0..496> <PST:05> <BCC:4> <EOT:03>
 *
 * Packet format (Response):
 *   <PRE:01> <LEN:4> <SEQ:1> <CMD:4> <DATA:0..480> <SEP:04> <STAT:8> <PST:05> <BCC:4> <EOT:03>
 *
 * LEN, CMD, BCC: ASCII-hex encoded, each nibble + 0x30
 * SEQ: raw byte 0x20..0xFF
 */

#include <Arduino.h>

// ─── Constants ────────────────────────────────────────────────────────────────

#define DATECS_PRE       0x01
#define DATECS_PST       0x05
#define DATECS_SEP       0x04
#define DATECS_EOT       0x03
#define DATECS_NAK       0x15
#define DATECS_SYN       0x16

#define DATECS_LEN_OFFSET   0x20   // added to raw length when encoding
#define DATECS_SEQ_MIN      0x20
#define DATECS_SEQ_MAX      0xFF

#define DATECS_MAX_DATA_TX  496
#define DATECS_MAX_DATA_RX  480
#define DATECS_STATUS_LEN   8

// Packet overhead bytes: PRE(1) + LEN(4) + SEQ(1) + CMD(4) + PST(1) + BCC(4) + EOT(1) = 16
// Response adds SEP(1) + STAT(8) = 25 total overhead
#define DATECS_TX_BUF_SIZE  (16 + DATECS_MAX_DATA_TX)
#define DATECS_RX_BUF_SIZE  (25 + DATECS_MAX_DATA_RX)

#define DATECS_TIMEOUT_MS   500
#define DATECS_MAX_RETRIES  3

// ─── Command Codes ────────────────────────────────────────────────────────────

enum DatecsCmd : uint16_t {
    CMD_CLEAR_DISPLAY        = 33,   // 0x21
    CMD_DISPLAY_LINE2        = 35,   // 0x23
    CMD_OPEN_NONFISCAL       = 38,   // 0x26
    CMD_CLOSE_NONFISCAL      = 39,   // 0x27
    CMD_PRINT_NONFISCAL_TEXT = 42,   // 0x2A
    CMD_OPEN_STORNO          = 43,   // 0x2B
    CMD_PAPER_FEED           = 44,   // 0x2C
    CMD_CHECK_PC_MODE        = 45,   // 0x2D
    CMD_PAPER_CUT            = 46,   // 0x2E
    CMD_DISPLAY_LINE1        = 47,   // 0x2F
    CMD_OPEN_FISCAL          = 48,   // 0x30
    CMD_REGISTER_SALE        = 49,   // 0x31
    CMD_GET_VAT_RATES        = 50,   // 0x32
    CMD_SUBTOTAL             = 51,   // 0x33
    CMD_TOTAL                = 53,   // 0x35
    CMD_PRINT_FISCAL_TEXT    = 54,   // 0x36
    CMD_PINPAD               = 55,   // 0x37
    CMD_CLOSE_FISCAL         = 56,   // 0x38
    CMD_PRINT_INVOICE        = 57,   // 0x39
    CMD_REGISTER_ITEM        = 58,   // 0x3A
    CMD_CANCEL_FISCAL        = 60,   // 0x3C
    CMD_SET_DATETIME         = 61,   // 0x3D
    CMD_READ_DATETIME        = 62,   // 0x3E
    CMD_SHOW_DATETIME        = 63,   // 0x3F
    CMD_LAST_FISCAL_INFO     = 64,   // 0x40
    CMD_DAILY_TAXATION       = 65,   // 0x41
    CMD_SET_INVOICE_INTERVAL = 66,   // 0x42
    CMD_FM_REMAINING         = 68,   // 0x44
    CMD_REPORTS              = 69,   // 0x45
    CMD_CASH_INOUT           = 70,   // 0x46
    CMD_GENERAL_INFO         = 71,   // 0x47
    CMD_FISCALIZATION        = 72,   // 0x48
    CMD_READ_STATUS          = 74,   // 0x4A
    CMD_FISCAL_TXN_STATUS    = 76,   // 0x4C
    CMD_PLAY_SOUND           = 80,   // 0x50
    CMD_PROGRAM_VAT          = 83,   // 0x53
    CMD_PRINT_BARCODE        = 84,   // 0x54
    CMD_FM_LAST_DATE         = 86,   // 0x56
    CMD_GET_ITEM_GROUPS      = 87,   // 0x57
    CMD_GET_DEPARTMENT       = 88,   // 0x58
    CMD_TEST_FM              = 89,   // 0x59
    CMD_DIAGNOSTIC           = 90,   // 0x5A
    CMD_PROGRAM_SERIAL       = 91,   // 0x5B
    CMD_PRINT_SEP_LINE       = 92,   // 0x5C
    CMD_FM_REPORT_DATE       = 94,   // 0x5E
    CMD_FM_REPORT_ZNUM       = 95,   // 0x5F
    CMD_SET_SW_PASSWORD      = 96,   // 0x60
    CMD_PROGRAM_TAX_NUM      = 98,   // 0x62
    CMD_READ_TAX_NUM         = 99,   // 0x63
    CMD_READ_ERROR           = 100,  // 0x64
    CMD_SET_OPER_PASSWORD    = 101,  // 0x65
    CMD_RECEIPT_INFO         = 103,  // 0x67
    CMD_OPERATOR_REPORT      = 105,  // 0x69
    CMD_OPEN_DRAWER          = 106,  // 0x6A
    CMD_ITEM_MANAGE          = 107,  // 0x6B
    CMD_PRINT_DUPLICATE      = 109,  // 0x6D
    CMD_ADDITIONAL_DAILY     = 110,  // 0x6E
    CMD_PLU_REPORT           = 111,  // 0x6F
    CMD_OPERATOR_INFO        = 112,  // 0x70
    CMD_CONVERT_AMOUNT       = 115,  // 0x73
    CMD_FM_BINARY            = 116,  // 0x74
    CMD_PRINT_VERT_TEXT      = 122,  // 0x7A
    CMD_DEVICE_INFO          = 123,  // 0x7B
    CMD_SEARCH_RECEIPT       = 124,  // 0x7C
    CMD_EJ_INFO              = 125,  // 0x7D
    CMD_FM_STRUCTURED        = 126,  // 0x7E
    CMD_STAMP_OPS            = 127,  // 0x7F
    CMD_MODEM_INFO           = 135,  // 0x87
    CMD_CLIENTS              = 140,  // 0x8C
    CMD_SERVICE_OPS          = 253,  // 0xFD
    CMD_PROGRAMMING          = 255,  // 0xFF
};

// ─── Error Codes (subset most relevant for Arduino use) ───────────────────────

enum DatecsError : int32_t {
    ERR_OK                    =  0,
    ERR_TIMEOUT               = -1,   // local: no response within 500 ms
    ERR_NAK                   = -2,   // local: received NAK (checksum/form error)
    ERR_INVALID_RESPONSE      = -3,   // local: malformed response packet
    ERR_BCC_MISMATCH          = -4,   // local: BCC verification failed
    ERR_BUFFER_OVERFLOW       = -5,   // local: data too long
    ERR_SEQ_MISMATCH          = -6,   // local: response SEQ != request SEQ
    ERR_CMD_MISMATCH          = -7,   // local: response CMD != request CMD
    ERR_WRITE_FAILED          = -8,   // local: incomplete Stream write

    // Device errors (from ErrorCode field in response DATA)
    ERR_FP_INVALID_COMMAND    = -112000,
    ERR_FP_INVALID_SYNTAX     = -112001,
    ERR_FP_NOT_PERMITTED      = -112002,
    ERR_FP_OVERFLOW           = -112003,
    ERR_FP_WRONG_DATETIME     = -112004,
    ERR_FP_NEED_PC_MODE       = -112005,
    ERR_FP_NO_PAPER           = -112006,
    ERR_FP_COVER_OPEN         = -112007,
    ERR_FP_PRINTER_FAILURE    = -112008,
};

// ─── Status Bits ─────────────────────────────────────────────────────────────

struct DatecsStatus {
    uint8_t raw[DATECS_STATUS_LEN];

    // Byte 0
    bool generalError()       const { return raw[0] & 0x20; }  // bit 5, OR of all # errors
    bool coverOpen()          const { return raw[0] & 0x40; }  // bit 6
    bool printMechFailure()   const { return raw[0] & 0x10; }  // bit 4
    bool rtcNotSynced()       const { return raw[0] & 0x04; }  // bit 2
    bool invalidCommand()     const { return raw[0] & 0x02; }  // bit 1 (#)
    bool syntaxError()        const { return raw[0] & 0x01; }  // bit 0 (#)

    // Byte 1
    bool notPermitted()       const { return raw[1] & 0x02; }  // bit 1 (#)
    bool overflow()           const { return raw[1] & 0x01; }  // bit 0 (#)

    // Byte 2
    bool nonfiscalOpen()      const { return raw[2] & 0x20; }  // bit 5
    bool ejNearlyFull()       const { return raw[2] & 0x10; }  // bit 4
    bool fiscalOpen()         const { return raw[2] & 0x08; }  // bit 3
    bool ejFull()             const { return raw[2] & 0x04; }  // bit 2
    bool paperNearEnd()       const { return raw[2] & 0x02; }  // bit 1
    bool paperEnd()           const { return raw[2] & 0x01; }  // bit 0 (#)

    // Byte 4 – Fiscal memory
    bool fmNotFound()         const { return raw[4] & 0x40; }  // bit 6
    bool fmErrorsPresent()    const { return raw[4] & 0x20; }  // bit 5 (OR of *)
    bool fmLow()              const { return raw[4] & 0x08; }  // bit 3 (<60 reports left)
    bool fmSerialSet()        const { return raw[4] & 0x04; }  // bit 2
    bool fmTaxNumSet()        const { return raw[4] & 0x02; }  // bit 1
    bool fmFull()             const { return raw[4] & 0x10; }  // bit 4 (*)
    bool fmAccessError()      const { return raw[4] & 0x01; }  // bit 0 (*)

    // Byte 5
    bool vatSet()             const { return raw[5] & 0x10; }  // bit 4
    bool fiscalized()         const { return raw[5] & 0x08; }  // bit 3
    bool fmFormatted()        const { return raw[5] & 0x02; }  // bit 1

    bool hasError() const {
        return generalError() || coverOpen() || printMechFailure() ||
               notPermitted() || overflow() || paperEnd() || ejFull() ||
               fmFull() || fmAccessError() || fmNotFound();
    }

    void clear() { memset(raw, 0, DATECS_STATUS_LEN); }
};

// ─── Response Packet ─────────────────────────────────────────────────────────

struct DatecsResponse {
    uint8_t      seq;
    uint16_t     cmd;
    char         data[DATECS_MAX_DATA_RX + 1]; // null-terminated
    uint16_t     dataLen;
    DatecsStatus status;
    DatecsError  localError;   // ERR_OK if comms OK; negative if local failure
    int32_t      deviceError;  // parsed from data field (0 = no error)

    bool ok() const { return localError == ERR_OK && deviceError == 0 && !status.hasError(); }
    void clear() {
        seq = 0; cmd = 0; dataLen = 0; data[0] = '\0';
        status.clear(); localError = ERR_OK; deviceError = 0;
    }
};

// ─── Core Protocol Class ──────────────────────────────────────────────────────

class DatecsProtocol {
public:
    /**
     * @param serial   Hardware or Software Serial port (Stream&)
     * @param timeoutMs  Response timeout in ms (default 500)
     * @param maxRetries  Retransmissions on NAK/timeout (default 3)
     */
    explicit DatecsProtocol(Stream& serial,
                            uint16_t timeoutMs  = DATECS_TIMEOUT_MS,
                            uint8_t  maxRetries = DATECS_MAX_RETRIES)
        : _serial(serial), _timeout(timeoutMs), _retries(maxRetries ? maxRetries : 1),
          _seq(DATECS_SEQ_MIN), _txLen(0), _rxLen(0), _rxOverflow(false) {}

    // ── Low-level send/receive ────────────────────────────────────────────────

    /**
     * Build and send a wrapped request packet.
     * @param cmd     Command code (DatecsCmd)
     * @param data    Optional data payload (ASCII, tab-separated params)
     * @param dataLen Length of data. If 0, strlen(data) is used.
     * @return number of bytes written to serial
     */
    size_t sendPacket(uint16_t cmd, const char* data = nullptr, uint16_t dataLen = 0);

    /**
     * Receive and parse the next response packet from the device.
     * Blocks until a complete packet is received or timeout expires.
     * Handles SYN (busy) transparently.
     * @param resp  Output structure filled with parsed response
     * @return true if a valid wrapped response was received
     */
    bool receivePacket(DatecsResponse& resp);

    /**
     * Send a command and wait for response (with retries on NAK/timeout).
     * This is the primary method for all command execution.
     * @param cmd      Command code
     * @param data     Optional request data
     * @param dataLen  Length; 0 = auto (strlen)
     * @param resp     Parsed response output
     * @return true on success
     */
    bool execute(uint16_t cmd, const char* data, uint16_t dataLen, DatecsResponse& resp);

    // Convenience overload – null-terminated string data
    bool execute(uint16_t cmd, DatecsResponse& resp) {
        return execute(cmd, nullptr, 0, resp);
    }
    bool execute(uint16_t cmd, const char* data, DatecsResponse& resp) {
        return execute(cmd, data, data ? (uint16_t)strlen(data) : 0, resp);
    }

    // ── Sequence number management ────────────────────────────────────────────
    void resetSeq()        { _seq = DATECS_SEQ_MIN; }
    uint8_t currentSeq()   const { return _seq; }

    // ── Utility ───────────────────────────────────────────────────────────────

    /** Encode a 4-nibble ASCII-hex word as used in LEN/CMD/BCC fields */
    static void encodeWord(uint16_t value, uint8_t out[4]);

    /** Decode a 4-byte ASCII-hex word back to uint16_t */
    static uint16_t decodeWord(const uint8_t in[4]);

    /** Compute BCC: sum of bytes from after PRE up to and including PST */
    static uint16_t computeBCC(const uint8_t* buf, uint16_t start, uint16_t end);

    /** Parse the first integer (ErrorCode) from a response data field */
    static int32_t parseErrorCode(const char* data);

    /** Last error string for debug output */
    static const char* errorString(DatecsError err);

    // Expose last raw TX/RX buffers for debugging
    const uint8_t* lastTxBuf() const { return _txBuf; }
    uint16_t       lastTxLen() const { return _txLen; }
    const uint8_t* lastRxBuf() const { return _rxBuf; }
    uint16_t       lastRxLen() const { return _rxLen; }

private:
    Stream&  _serial;
    uint16_t _timeout;
    uint8_t  _retries;
    uint8_t  _seq;

    uint8_t  _txBuf[DATECS_TX_BUF_SIZE];
    uint16_t _txLen;
    uint8_t  _rxBuf[DATECS_RX_BUF_SIZE];
    uint16_t _rxLen;
    bool     _rxOverflow;

    void     _advanceSeq();
    bool     _waitByte(uint8_t& b, uint16_t timeoutMs);
    bool     _readUntilEOT(uint16_t timeoutMs);
};

// ─── High-Level Command API ───────────────────────────────────────────────────

class DatecsPrinter {
public:
    explicit DatecsPrinter(Stream& serial,
                           uint16_t timeoutMs  = DATECS_TIMEOUT_MS,
                           uint8_t  maxRetries = DATECS_MAX_RETRIES)
        : _proto(serial, timeoutMs, maxRetries) {}

    // ── Display ───────────────────────────────────────────────────────────────
    bool clearDisplay();
    bool displayLine1(const char* text);   // CMD 47
    bool displayLine2(const char* text);   // CMD 35

    // ── Non-fiscal receipts ───────────────────────────────────────────────────
    bool openNonfiscal();
    bool closeNonfiscal();
    bool printNonfiscalText(const char* text);

    // ── Fiscal receipts ───────────────────────────────────────────────────────
    /**
     * Open a fiscal receipt.
     * @param operatorNum  Operator number (1-based)
     * @param operatorPass Operator password string
     * @param tillNum      Till/Cashier number
     */
    bool openFiscal(uint8_t operatorNum, const char* operatorPass,
                    uint32_t tillNum = 1, const char* uniqueSaleNumber = nullptr,
                    bool invoice = false);
    bool openStorno(uint8_t operatorNum, const char* operatorPass,
                    uint32_t tillNum, uint8_t stornoReason,
                    uint32_t originalDocumentNumber,
                    const char* originalDateTime,
                    const char* fiscalMemoryNumber,
                    const char* originalUniqueSaleNumber,
                    bool invoice = false,
                    const char* invoiceNumber = nullptr,
                    const char* invoiceReason = nullptr);

    /**
     * Register a sale line.
     * @param name     Item name (up to 36 chars)
     * @param vatGroup VAT group number (1..8 = A..H)
     * @param price    Unit price (e.g. 12.50 → "12.50")
     * @param qty      Quantity string (e.g. "1", "2.500")
     * @param discountType 0 none, 1/2 percentage surcharge/discount, 3/4 sum surcharge/discount
     */
    bool registerSale(const char* name, uint8_t vatGroup, const char* price,
                      const char* qty = "1", uint8_t discountType = 0,
                      const char* discountValue = nullptr,
                      uint8_t department = 0, const char* unit = nullptr);

    /** Print subtotal line. @param display show on display; @param print print on paper */
    bool subtotal(bool display = true, bool print = false);

    /**
     * Command 53 TOTAL followed separately by command 56 close.
     * @param payType 0=cash, 1=credit card, 2=debit card, 3..5=other, 6=foreign currency
     */
    bool total(uint8_t payType, const char* amount);
    bool totalWithPinpad(const char* amount, uint8_t transactionType = 1);
    bool closeFiscal();
    bool payAndClose(uint8_t payType = 0, const char* amount = nullptr);

    bool cancelFiscal();
    bool printFiscalText(const char* text);

    // ── Paper ─────────────────────────────────────────────────────────────────
    bool paperFeed(uint8_t lines = 1);
    bool paperCut();

    // ── Date / Time ───────────────────────────────────────────────────────────
    /** @param dt Format: "DD-MM-YY hh:mm:ss" */
    bool setDatetime(const char* dt);
    /** Reads datetime into buf (must be ≥ 20 bytes). Returns false on error. */
    bool readDatetime(char* buf, uint8_t bufLen);

    // ── Status & Info ─────────────────────────────────────────────────────────
    bool readStatus(DatecsStatus& status);
    bool checkPcMode();
    bool openDrawer();

    // ── Reports ───────────────────────────────────────────────────────────────
    /** 'X' = X-report (no reset), 'Z' = Z-report (daily close) */
    bool printReport(char type);

    // ── Cash in / out ─────────────────────────────────────────────────────────
    bool cashIn(const char* amount);
    bool cashOut(const char* amount);

    // ── Device info ───────────────────────────────────────────────────────────
    /** Fills buf with raw device info response data. */
    bool deviceInfo(char* buf, uint8_t bufLen);

    // ── Access underlying protocol for custom commands ────────────────────────
    DatecsProtocol& proto() { return _proto; }

    // Last response is always stored here for inspection
    const DatecsResponse& lastResponse() const { return _lastResp; }

private:
    DatecsProtocol _proto;
    DatecsResponse _lastResp;

    bool _exec(uint16_t cmd, const char* data = nullptr);
    bool _checkResp();
};
