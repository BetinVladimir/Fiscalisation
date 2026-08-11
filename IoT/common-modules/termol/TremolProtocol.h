#pragma once
/**
 * TremolProtocol.h
 * Arduino C++ API — Tremol Fiscal Device Communication Protocol v2507141400
 * Manufacturer: Tremol (Bulgaria)
 *
 * Supported devices: ECR, FPr, Fuel, FUVAS — all BG fiscal device variants
 *
 * ══════════════════════════════════════════════════════════════════════════════
 * PROTOCOL ARCHITECTURE
 * ══════════════════════════════════════════════════════════════════════════════
 *
 * ── REQUEST (SA → FD) ────────────────────────────────────────────────────────
 *   <STX><LEN><NBL><CMD><DATA…><CS><CS><ETX>
 *
 *   STX  = 02h (NOT 01h like Datecs/Daisy!)
 *   LEN  = (count of bytes: LEN+NBL+CMD+DATA) + 20h  → range 20h..FFh
 *   NBL  = message sequence number + 20h             → range 20h..9Fh
 *          (must differ from previous message)
 *   CMD  = command byte (20h..7Fh)
 *   DATA = ASCII fields separated by ';' (semicolons, NOT tabs)
 *          text encoding: cp1251. Up to 3902 bytes.
 *   CS   = 2-byte checksum:
 *          1) XOR of all bytes from LEN through last DATA byte = val (0..FFh)
 *          2) split into two nibbles, each + 30h
 *             e.g. B5h → nibbles Bh and 5h → 3Bh 35h
 *   ETX  = 0Ah (LF — NOT 03h like other protocols!)
 *
 * ── RESPONSES (FD → SA) ──────────────────────────────────────────────────────
 *
 *   A) ACK response  (for commands with no data output):
 *      <ACK><NBL><STE><STE><CS><CS><ETX>
 *      ACK  = 06h
 *      NBL  = echoed sequence number
 *      STE  = two ASCII status digits (see TremolStatus)
 *      CS   = XOR of NBL+STE+STE, encoded same way
 *      ETX  = 0Ah
 *
 *   B) Message response (for commands returning data):
 *      Same structure as request but sent by FD:
 *      <STX><LEN><NBL><CMD><DATA…><CS><CS><ETX>
 *      DATA contains the output fields separated by ';'
 *
 * ── SINGLE-BYTE CONTROL CODES ────────────────────────────────────────────────
 *   NACK  = 15h  — bad packet format; retransmit same NBL
 *   RETRY = 0Eh  — FD busy; wait and retransmit same NBL
 *
 * ── SHORT-FORM STATUS POLL ────────────────────────────────────────────────────
 *   PC sends: 04h
 *   FD replies with single byte:
 *   40h=Ready, 41h=Busy, 42h=NoPaper, 43h=NoPaper+Busy,
 *   44h=Overheat, 45h=Overheat+Busy, 48h=NoDisplay, 49h=NoDisplay+Busy,
 *   50h=WaitPassword, 60h=BusyOtherConn, 70h=WrongPassword
 *
 * ── DATA FORMAT RULES ────────────────────────────────────────────────────────
 *   Price: 1..10 chars, floating point, preceded by +/- or space, e.g. "+12.34"
 *   Qty:   1..10 chars, up to 3 decimal places, e.g. "1.234"
 *   Rate%: 2..7 chars, up to 2 decimal places, preceded by '%', e.g. "%10.00"
 *   VAT class chars (CP1251 Cyrillic): А=C0h Б=C1h В=C2h Г=C3h Д=C4h
 *                                      Е=C5h Ж=C6h З=C7h  *=forbidden
 *   Dept byte encoding: DepNum + 0x80 (e.g. Dept1=0x81, Dept2=0x82…)
 *   URN (Unique Receipt Number): "XXXXXXXX-ZZZZ-NNNNNNN" (24 chars, NRA format)
 */

#include <Arduino.h>

// ─── Protocol constants ───────────────────────────────────────────────────────

#define TR_STX        0x02   // request start (NOT 0x01!)
#define TR_ACK        0x06   // acknowledgement start byte
#define TR_ETX        0x0A   // LF — end of every packet
#define TR_NACK       0x15   // negative ACK: bad format, retransmit same NBL
#define TR_RETRY      0x0E   // FD busy, retransmit same NBL
#define TR_POLL       0x04   // short-form status query

// NBL (sequence number) range after +20h offset
#define TR_NBL_MIN    0x20
#define TR_NBL_MAX    0x9F

// CS is 2 bytes: each nibble of XOR result + 0x30
#define TR_CS_BYTES   2

// Data field separator
#define TR_SEP        ';'

// VAT class base in CP1251 (А = 0xC0)
#define TR_VAT_BASE   0xC0

// Department byte: DepNum + 0x80
#define TR_DEP_BASE   0x80

// Maximum DATA field length (protocol spec: up to 3902 bytes)
#define TR_MAX_DATA   3902

// Practical Arduino buffer: STX(1)+LEN(1)+NBL(1)+CMD(1)+DATA+CS(2)+ETX(1) = 7+DATA
#define TR_TX_BUF     256    // adjust if large data needed
#define TR_RX_BUF     256

#define TR_TIMEOUT_MS  2000  // response timeout (ms)
#define TR_RETRY_MAX   3     // max retransmissions

// ─── Command codes ────────────────────────────────────────────────────────────

enum TremolCmd : uint8_t {
    // General
    CMD_STATUS          = 0x20,  // ' '  Status (7-byte detailed)
    CMD_VERSION         = 0x21,  // '!'  Version / device info
    CMD_DIAGNOSTICS     = 0x22,  // '"'  Print diagnostics
    CMD_CLEAR_DISPLAY   = 0x24,  // '$'  Clear external display
    CMD_DISPLAY_L1      = 0x25,  // '%'  Display text line 1
    CMD_DISPLAY_L2      = 0x26,  // '&'  Display text line 2
    CMD_DISPLAY_L12     = 0x27,  // '\'' Display text lines 1 and 2
    CMD_DISPLAY_DT      = 0x28,  // '('  Display date and time
    CMD_CUT_PAPER       = 0x29,  // ')'  Cut paper (FP only)
    CMD_CASH_DRAWER     = 0x2A,  // '*'  Open cash drawer
    CMD_PAPER_FEED      = 0x2B,  // '+'  Paper feeding
    CMD_OPEN_NONFISCAL  = 0x2E,  // '.'  Open non-fiscal receipt
    CMD_CLOSE_NONFISCAL = 0x2F,  // '/'  Close non-fiscal receipt
    CMD_OPEN_FISCAL     = 0x30,  // '0'  Open fiscal receipt (all variants)
    CMD_SELL            = 0x31,  // '1'  Sell / correction (VAT class)
    CMD_SELL_PLU_DB     = 0x32,  // '2'  Sell from FD database (PLU#)
    CMD_SUBTOTAL        = 0x33,  // '3'  Subtotal
    CMD_SELL_DEPT       = 0x34,  // '4'  Sell with department definition
    CMD_PAYMENT         = 0x35,  // '5'  Payment
    CMD_AUTO_CLOSE      = 0x36,  // '6'  Automatic cash close
    CMD_PRINT_TEXT      = 0x37,  // '7'  Free text printing
    CMD_CLOSE_FISCAL    = 0x38,  // '8'  Close fiscal receipt
    CMD_CANCEL          = 0x39,  // '9'  Cancel fiscal receipt
    CMD_PRINT_COPY      = 0x3A,  // ':'  Print copy / generate electronic dup
    CMD_RA_PO           = 0x3B,  // ';'  Non-fiscal RA and PO amounts
    CMD_SELL_200DEPT    = 0x3C,  // '<'  Sell with 200-department range
    CMD_DISCOUNT        = 0x3E,  // '>'  Discount / addition
    CMD_SELL_FRAC       = 0x3D,  // '='  Sell with fractional quantity

    // Fiscal / setup
    CMD_SET_UIC         = 0x41,  // 'A'  Set customer UIC / confirm fiscalization
    CMD_CHANGE_VAT      = 0x42,  // 'B'  Change VAT rates
    CMD_DECIMAL_POINT   = 0x43,  // 'C'  Change decimal point position
    CMD_PROG_PAYMENTS   = 0x44,  // 'D'  Program payment types
    CMD_PROG_PARAMS     = 0x45,  // 'E'  Program parameters
    CMD_PROG_DISPLAY    = 0x46,  // 'F'  Program external display
    CMD_PROG_DEPT       = 0x47,  // 'G'  Program department
    CMD_SET_DATETIME    = 0x48,  // 'H'  Program date and time
    CMD_PROG_HEADER     = 0x49,  // 'I'  Program header/footer/greeting
    CMD_PROG_OPERATOR   = 0x4A,  // 'J'  Program operator name/password
    CMD_PROG_PLU        = 0x4B,  // 'K'  Program article (PLU)
    CMD_PRINT_LOGO      = 0x4C,  // 'L'  Print logo
    CMD_PROG_LOGO       = 0x4D,  // 'M'  Program logo (numbered)
    CMD_NETWORK_CFG     = 0x4E,  // 'N'  LAN/WiFi/BT/GPRS settings
    CMD_PROG_OPTIONS    = 0x4F,  // 'O'  Program options (various)
    CMD_PROG_INVOICE_RNG= 0x50,  // 'P'  Program invoice number range
    CMD_PRINT_QR        = 0x51,  // 'Q'  Print barcode QP
    CMD_CUSTOMERS_DB    = 0x52,  // 'R'  Customer database
    CMD_LOGO_NUM        = 0x23,  // '#'  Set active logo number
    CMD_SET_FD_TYPE     = 0x56,  // 'V'  Set type of fiscal device
    CMD_SCALE_QTY       = 0x5A,  // 'Z'  Read scale quantity

    // Read
    CMD_READ_FD_NUMS    = 0x60,  // '`'  Read FD numbers
    CMD_READ_REG_INFO   = 0x61,  // 'a'  Read registration information
    CMD_READ_VAT        = 0x62,  // 'b'  Read VAT rates
    CMD_READ_DECIMAL    = 0x63,  // 'c'  Read decimal point position
    CMD_READ_PAYMENTS   = 0x64,  // 'd'  Read payment types
    CMD_READ_PARAMS     = 0x65,  // 'e'  Read parameters
    CMD_READ_PRN_STATUS = 0x66,  // 'f'  Read detailed printer status
    CMD_READ_DEPT_REGS  = 0x67,  // 'g'  Read department registers
    CMD_READ_DATETIME   = 0x68,  // 'h'  Read date and time
    CMD_READ_HEADER     = 0x69,  // 'i'  Read header / footer / greeting
    CMD_READ_OPERATOR   = 0x6A,  // 'j'  Read operator name/password
    CMD_READ_PLU        = 0x6B,  // 'k'  Read article
    CMD_READ_CUSTOMER   = 0x52,  // 'R'  Read customer database (same cmd as write, diff option)
    CMD_READ_DAILY_VAT  = 0x6D,  // 'm'  Read daily sale amounts by VAT
    CMD_READ_REGISTERS  = 0x6E,  // 'n'  Read registers
    CMD_READ_OPER_RPT   = 0x6F,  // 'o'  Read operator's report
    CMD_READ_INV_RANGE  = 0x70,  // 'p'  Read invoice number range
    CMD_READ_RCPT_NUM   = 0x71,  // 'q'  Read total receipt number
    CMD_READ_RCPT_INFO  = 0x72,  // 'r'  Read current receipt info
    CMD_READ_DAILY_INFO = 0x73,  // 's'  Read last daily report info
    CMD_READ_FREE_FM    = 0x74,  // 't'  Read free FM recording records
    CMD_READ_CURRENCY   = 0x5E,  // '^'  Read currency parameter

    // Reports print
    CMD_PRINT_DEPT_RPT  = 0x76,  // 'v'  Print department report
    CMD_PRINT_FM_SPEC   = 0x77,  // 'w'  Print FM special events / brief
    CMD_PRINT_FM_DETAIL = 0x78,  // 'x'  Print/read detailed FM report by Z-blocks
    CMD_PRINT_FM_BRIEF  = 0x79,  // 'y'  Print/read brief FM report by Z-blocks
    CMD_PRINT_FM_DATE   = 0x7A,  // 'z'  Print/read detailed FM report by date
    CMD_PRINT_FM_BDATE  = 0x7B,  // '{'  Print/read brief FM report by date
    CMD_PRINT_XZ        = 0x7C,  // '|'  Print daily X/Z report
    CMD_PRINT_OPER_RPT  = 0x7D,  // '}'  Print operator's report
    CMD_PRINT_PLU_RPT   = 0x7E,  // '~'  Print article report
    CMD_PRINT_DETAIL    = 0x7F,  // DEL  Print detailed daily report
};

// ─── Status bytes (STE field in ACK response) ─────────────────────────────────
// STE is a 2-char ASCII string: first char = FD error, second = command error.
// Both are ASCII digits: '0'=OK, '1'..'f' = error code.

enum TremolFDError : uint8_t {
    FD_OK                   = '0',  // 30h
    FD_NO_PAPER             = '1',  // 31h  Out of paper / printer failure
    FD_REGS_OVERFLOW        = '2',  // 32h
    FD_CLOCK_FAIL           = '3',  // 33h  Clock failure or bad date/time
    FD_OPEN_FISCAL          = '4',  // 34h  Opened fiscal receipt
    FD_PAYMENT_RESIDUE      = '5',  // 35h  Payment residue account
    FD_OPEN_NONFISCAL_ERR   = '6',  // 36h  Opened non-fiscal receipt
    FD_PAYMENT_NOT_CLOSED   = '7',  // 37h  Registered payment but not closed
    FD_FM_FAILURE           = '8',  // 38h  Fiscal memory failure
    FD_WRONG_PASSWORD       = '9',  // 39h
    FD_NO_DISPLAY           = 'a',  // 3Ah
    FD_24H_BLOCK            = 'b',  // 3Bh  24-hour block (unprinted Z)
    FD_OVERHEAT             = 'c',  // 3Ch
    FD_POWER_INTERRUPT      = 'd',  // 3Dh  Power cut in fiscal receipt
    FD_EJ_OVERFLOW          = 'e',  // 3Eh  EJ overflow
    FD_INSUFFICIENT         = 'f',  // 3Fh
};

enum TremolCmdError : uint8_t {
    CMD_ERR_OK              = '0',  // 30h
    CMD_ERR_INVALID         = '1',  // 31h  Invalid command
    CMD_ERR_ILLEGAL         = '2',  // 32h  Illegal (mode not allowed)
    CMD_ERR_Z_NOT_ZERO      = '3',  // 33h  Z daily report not zeroed
    CMD_ERR_SYNTAX          = '4',  // 34h  Syntax error
    CMD_ERR_INPUT_OVERFLOW  = '5',  // 35h  Input registers overflow
    CMD_ERR_ZERO_INPUT      = '6',  // 36h  Zero input registers
    CMD_ERR_NO_CORRECTION   = '7',  // 37h  Unavailable for correction
    CMD_ERR_INSUF_AMOUNT    = '8',  // 38h  Insufficient amount on hand
};

// ─── Detailed status (7 bytes from CMD_STATUS / 0x20) ────────────────────────

struct TremolStatus {
    uint8_t raw[7];

    void clear() { memset(raw, 0, 7); }

    // ── ST0 ──────────────────────────────────────────────────────────────────
    bool fmReadOnly()           const { return raw[0] & (1<<0); }
    bool powerDownInReceipt()   const { return raw[0] & (1<<1); }
    bool printerOverheat()      const { return raw[0] & (1<<2); }
    bool dateTimeNotSet()       const { return raw[0] & (1<<3); }
    bool dateTimeWrong()        const { return raw[0] & (1<<4); }
    bool ramReset()             const { return raw[0] & (1<<5); }
    bool hwClockError()         const { return raw[0] & (1<<6); }

    // ── ST1 ──────────────────────────────────────────────────────────────────
    bool noPaper()              const { return raw[1] & (1<<0); }
    bool regsOverflow()         const { return raw[1] & (1<<1); }
    bool customerRptNotZero()   const { return raw[1] & (1<<2); }
    bool dailyRptNotZero()      const { return raw[1] & (1<<3); }
    bool articleRptNotZero()    const { return raw[1] & (1<<4); }
    bool operatorRptNotZero()   const { return raw[1] & (1<<5); }
    bool nonPrintedCopy()       const { return raw[1] & (1<<6); }

    // ── ST2 ──────────────────────────────────────────────────────────────────
    bool openNonfiscal()        const { return raw[2] & (1<<0); }
    bool openFiscal()           const { return raw[2] & (1<<1); }
    bool openFiscalDetailed()   const { return raw[2] & (1<<2); }
    bool openFiscalWithVAT()    const { return raw[2] & (1<<3); }
    bool openInvoice()          const { return raw[2] & (1<<4); }
    bool sdNearFull()           const { return raw[2] & (1<<5); }
    bool sdFull()               const { return raw[2] & (1<<6); }
    bool anyReceiptOpen()       const { return raw[2] & 0x1F; }

    // ── ST3 (FM) ──────────────────────────────────────────────────────────────
    bool noFMmodule()           const { return raw[3] & (1<<0); }
    bool fmError()              const { return raw[3] & (1<<1); }
    bool fmFull()               const { return raw[3] & (1<<2); }
    bool fmNearFull()           const { return raw[3] & (1<<3); }
    bool decimalFraction()      const { return raw[3] & (1<<4); }
    bool fiscalized()           const { return raw[3] & (1<<5); }
    bool fmProduced()           const { return raw[3] & (1<<6); }

    // ── ST4 ──────────────────────────────────────────────────────────────────
    bool autoCutter()           const { return raw[4] & (1<<0); }
    bool transparentDisplay()   const { return raw[4] & (1<<1); }
    bool speed9600()            const { return raw[4] & (1<<2); }
    bool autoDrawer()           const { return raw[4] & (1<<4); }
    bool customerLogoInRcpt()   const { return raw[4] & (1<<5); }

    // ── ST5 ──────────────────────────────────────────────────────────────────
    bool wrongSIM()             const { return raw[5] & (1<<0); }
    bool blocked3daysNoOper()   const { return raw[5] & (1<<1); }
    bool noNRATask()            const { return raw[5] & (1<<2); }
    bool wrongSDcard()          const { return raw[5] & (1<<5); }
    bool deregistered()         const { return raw[5] & (1<<6); }

    // ── ST6 ──────────────────────────────────────────────────────────────────
    bool noSIMcard()            const { return raw[6] & (1<<0); }
    bool noGPRSmodem()          const { return raw[6] & (1<<1); }
    bool noMobileOper()         const { return raw[6] & (1<<2); }
    bool noGPRSservice()        const { return raw[6] & (1<<3); }
    bool paperNearEnd()         const { return raw[6] & (1<<4); }
    bool blockedUnsentRcpts()   const { return raw[6] & (1<<5); }

    bool hasError() const {
        return noPaper() || fmFull() || fmError() || powerDownInReceipt() ||
               regsOverflow() || blockedUnsentRcpts();
    }
};

// ─── Response ─────────────────────────────────────────────────────────────────

struct TremolResponse {
    bool     isAck;       // true = ACK packet, false = message response
    uint8_t  nbl;         // echoed sequence number
    uint8_t  cmd;         // echoed command (message response only)
    char     data[TR_RX_BUF]; // null-terminated response data fields
    uint8_t  dataLen;
    uint8_t  ste0;        // FD status byte (ASCII '0'..'f')
    uint8_t  ste1;        // command status byte (ASCII '0'..'f')
    int8_t   localError;  // 0=OK, negative=transport error

    bool ok()       const { return localError == 0 && ste0 == '0' && ste1 == '0'; }
    bool fdOk()     const { return ste0 == '0'; }
    bool cmdOk()    const { return ste1 == '0'; }
    const char* fdErrorStr()  const;
    const char* cmdErrorStr() const;

    void clear() {
        isAck = true; nbl = cmd = dataLen = 0; data[0] = '\0';
        ste0 = '0'; ste1 = '0'; localError = 0;
    }
};

// ─── Transport error codes ────────────────────────────────────────────────────

#define TR_ERR_OK         0
#define TR_ERR_TIMEOUT   -1
#define TR_ERR_NACK      -2
#define TR_ERR_BAD_CS    -3
#define TR_ERR_BAD_FRAME -4
#define TR_ERR_NBL       -5
#define TR_ERR_OVERFLOW  -6

// ─── Core protocol class ──────────────────────────────────────────────────────

class TremolProtocol {
public:
    explicit TremolProtocol(Stream& serial,
                            uint16_t timeoutMs = TR_TIMEOUT_MS,
                            uint8_t  maxRetry  = TR_RETRY_MAX)
        : _serial(serial), _timeout(timeoutMs), _maxRetry(maxRetry),
          _nbl(TR_NBL_MIN), _txLen(0), _rxLen(0) {}

    // ── Primary API ───────────────────────────────────────────────────────────

    /** Send command with data fields (';'-joined), receive and parse response. */
    bool execute(uint8_t cmd, const char* data, TremolResponse& resp);

    bool execute(uint8_t cmd, TremolResponse& resp) {
        return execute(cmd, nullptr, resp);
    }

    // ── Low-level ─────────────────────────────────────────────────────────────

    size_t sendPacket(uint8_t cmd, const char* data);
    bool   receivePacket(TremolResponse& resp);

    /** Send short-form poll (04h) and return single-byte FD state. */
    uint8_t pollStatus(uint16_t timeoutMs = 200);

    void    resetNBL()       { _nbl = TR_NBL_MIN; }
    uint8_t currentNBL()     const { return _nbl; }

    // ── Static utilities ──────────────────────────────────────────────────────

    /** Compute CS: XOR of bytes buf[from..to], then split into 2 nibble+30h bytes */
    static uint8_t  xorRange(const uint8_t* buf, uint8_t from, uint8_t to);
    static void     encodeCS(uint8_t xorVal, uint8_t out[2]);
    static uint8_t  decodeCS(const uint8_t in[2]);

    /** Build a DATA string by joining fields with ';'. Returns chars written. */
    static uint8_t buildData(char* out, uint8_t outSize, ...);
    // va_list terminated by nullptr sentinel

    static const char* localErrorStr(int8_t err);

    const uint8_t* lastTxBuf() const { return _txBuf; }
    uint8_t        lastTxLen() const { return _txLen; }
    const uint8_t* lastRxBuf() const { return _rxBuf; }
    uint8_t        lastRxLen() const { return _rxLen; }

private:
    Stream&  _serial;
    uint16_t _timeout;
    uint8_t  _maxRetry;
    uint8_t  _nbl;
    uint8_t  _txBuf[TR_TX_BUF];
    uint8_t  _txLen;
    uint8_t  _rxBuf[TR_RX_BUF];
    uint8_t  _rxLen;

    bool    _waitByte(uint8_t& b, uint16_t ms);
    bool    _readUntilETX(uint16_t ms);
    bool    _parseACK(TremolResponse& resp);
    bool    _parseMsgResponse(TremolResponse& resp);
    void    _advanceNBL();
};

// ─── High-level printer API ───────────────────────────────────────────────────

class TremolPrinter {
public:
    explicit TremolPrinter(Stream& serial,
                           uint16_t timeoutMs = TR_TIMEOUT_MS,
                           uint8_t  maxRetry  = TR_RETRY_MAX)
        : _proto(serial, timeoutMs, maxRetry) {}

    // ── Device info ───────────────────────────────────────────────────────────

    /** Returns 7-byte detailed status in `status`. */
    bool readStatus(TremolStatus& status);

    /** Quick single-byte poll. Returns raw FD state byte (40h=Ready, etc.) */
    uint8_t pollReady();

    /** Reads version / device type into buf */
    bool readVersion(char* buf, uint8_t bufLen);

    /** Prints diagnostic receipt. */
    bool printDiagnostics();

    // ── Date / time ───────────────────────────────────────────────────────────
    /** @param dt Format: "DD-MM-YYYY HH:MM:SS" */
    bool setDatetime(const char* dt);
    bool getDatetime(char* buf, uint8_t bufLen);

    // ── Display ───────────────────────────────────────────────────────────────
    bool clearDisplay();
    bool displayLine1(const char* text20);
    bool displayLine2(const char* text20);
    bool displayLines(const char* text40);  // first 20 on L1, last 20 on L2

    // ── Paper / drawer ────────────────────────────────────────────────────────
    bool paperFeed();
    bool cutPaper();
    bool openDrawer();

    // ── Non-fiscal receipt ────────────────────────────────────────────────────
    /**
     * @param operNum   1..20
     * @param operPass  6-char password
     * @param printType '0'=step-by-step (default), '1'=postponed
     */
    bool openNonfiscal(uint8_t operNum, const char* operPass, char printType = '0');
    bool closeNonfiscal();
    bool printText(const char* text);  // also works in fiscal receipt

    // ── Fiscal receipt ────────────────────────────────────────────────────────
    /**
     * Open standard fiscal receipt.
     * @param operNum   1..20
     * @param operPass  6-char password string
     * @param format    '1'=detailed, '0'=brief
     * @param printVAT  '1'=yes, '0'=no
     * @param printType '0'=step-by-step, '2'=postponed, '4'=buffered
     * @param urn       Optional 24-char unique receipt number (NRA format), nullptr=omit
     */
    bool openFiscal(uint8_t operNum, const char* operPass,
                    char format = '1', char printVAT = '1',
                    char printType = '0', const char* urn = nullptr);

    /**
     * Sell item with VAT class.
     * @param name      Up to 36 chars (34 printed); use '|' for line break
     * @param vatClass  'A'..'H' (Latin, auto-converted to CP1251 Cyrillic)
     *                  OR pass raw CP1251 byte (0xC0..0xC7)
     *                  '*' = forbidden
     * @param price     e.g. "+12.50" or "12.50" (use "-" prefix for correction)
     * @param qty       Optional quantity, e.g. "2.000" (nullptr = 1.000)
     * @param discP     Optional discount/addition percent, e.g. "%-10.00" (nullptr=none)
     * @param discV     Optional discount/addition value, e.g. "-1.50" (nullptr=none)
     */
    bool sell(const char* name, char vatClass, const char* price,
              const char* qty = nullptr,
              const char* discP = nullptr, const char* discV = nullptr);

    /**
     * Sell item attached to a department (VAT from department).
     * @param depNum    Department number 1..127
     */
    bool sellDept(const char* name, uint8_t depNum, const char* price,
                  const char* qty = nullptr,
                  const char* discP = nullptr, const char* discV = nullptr);

    /**
     * Subtotal.
     * @param print   '1'=print, '0'=no print
     * @param display '1'=show on display, '0'=no
     * @param discP   Optional % discount on subtotal (nullptr=none)
     * @param discV   Optional value discount on subtotal (nullptr=none)
     * @param outBuf  Optional buffer to receive SubtotalValue string
     */
    bool subtotal(char print = '0', char display = '1',
                  const char* discP = nullptr, const char* discV = nullptr,
                  char* outBuf = nullptr, uint8_t outLen = 0);

    /**
     * Payment.
     * @param payType     '0'=cash (default), '1'..'11'=other types
     * @param withChange  '0'=with change, '1'=without change
     * @param amount      Amount tendered string (e.g. "20.00")
     * @param changeType  Optional: '0'=change in cash, '1'=same type, '2'=in currency
     */
    bool payment(char payType = '0', char withChange = '0',
                 const char* amount = nullptr, char changeType = '0');

    /** Pay exact amount in cash and auto-close receipt. */
    bool cashPayClose();

    /** Close fiscal receipt (after payment is complete). */
    bool closeFiscal();

    /** Cancel fiscal receipt (voids all sales). */
    bool cancelFiscal();

    // ── RA / PO (cash in / out) ───────────────────────────────────────────────
    bool cashIn(const char* amount,  const char* text = nullptr);
    bool cashOut(const char* amount, const char* text = nullptr);

    // ── Reports ───────────────────────────────────────────────────────────────
    bool printXReport();
    bool printZReport();
    bool printDeptReport();
    bool printOperatorReport();

    // ── Access underlying protocol ────────────────────────────────────────────
    TremolProtocol& proto() { return _proto; }
    const TremolResponse& lastResponse() const { return _last; }

    /** Map Latin 'A'..'H' to CP1251 Cyrillic VAT class byte */
    static uint8_t vatByte(char latin) {
        if (latin >= 'A' && latin <= 'H') return TR_VAT_BASE + (uint8_t)(latin - 'A');
        if (latin >= 'a' && latin <= 'h') return TR_VAT_BASE + (uint8_t)(latin - 'a');
        if (latin == '*') return '*';
        return TR_VAT_BASE; // default А
    }

private:
    TremolProtocol _proto;
    TremolResponse _last;

    bool _exec(uint8_t cmd, const char* data = nullptr);
    bool _ok();
};
