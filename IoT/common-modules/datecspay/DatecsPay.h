#pragma once
#include <Arduino.h>

// ============================================================
//  DatecsPay Protocol Library  v1.0
//  Arduino C++ API for Datecs BluePad/card reader
//  Supports Borica commands (0x3D) and External Internet (0x40)
// ============================================================

// ─── Packet constants ───────────────────────────────────────
static constexpr uint8_t  PKT_START       = 0x3E;  // '>'
static constexpr uint8_t  CMD_BORICA      = 0x3D;
static constexpr uint8_t  CMD_EXTERNAL    = 0x40;
static constexpr uint8_t  EVT_BORICA      = 0x0E;
static constexpr uint8_t  EVT_EXTERNAL    = 0x0F;
static constexpr uint8_t  EVT_EMV2        = 0x0B;
// 1024-byte External Internet payload + subcommand + frame overhead.
static constexpr uint16_t MAX_PACKET_SIZE = 1032;
static constexpr uint32_t RESP_TIMEOUT_MS = 5000;
static constexpr uint32_t BYTE_TIMEOUT_MS = 2000;

// ─── Error codes (from protocol spec) ───────────────────────
enum class DatecsError : uint8_t {
    NoErr          = 0,
    General        = 1,
    InvCmd         = 2,
    InvPar         = 3,
    InvAdr         = 4,
    InvVal         = 5,
    InvLen         = 6,
    NoPermit       = 7,
    NoData         = 8,
    TimeOut        = 9,
    KeyNum         = 10,
    KeyAttr        = 11,
    InvDevice      = 12,
    NoSupport      = 13,
    PinLimit       = 14,
    Flash          = 15,
    Hard           = 16,
    Cancel         = 18,
    InvSign        = 19,
    InvHead        = 20,
    InvPass        = 21,
    KeyFormat      = 22,
    SCR            = 23,
    HAL            = 24,
    InvKey         = 25,
    NoPinData      = 26,
    InvRemainder   = 27,
    NoPerm         = 31,
    NoTMK          = 32,
    InvKek         = 33,
    DubKey         = 34,
    KBD            = 35,
    KBDNoCal       = 36,
    KBDBug         = 37,
    Busy           = 38,  // 0x26 – retry after 100ms
    Tampered       = 39,
    Emsr           = 40,
    Accept         = 41,
    InvPAN         = 42,
    OutOfMemory    = 43,
    EMV            = 44,
    Crypt          = 45,
    ComRcv         = 46,
    WrongVer       = 47,
    NoPaper        = 48,
    TooHot         = 49,
    NoConnected    = 50,
    UseChip        = 51,
    EndDay         = 52,
    // Library-internal errors (> 127)
    LibTimeout     = 0x80,
    LibBufOverflow = 0x81,
    LibBadCsum     = 0x82,
    LibBadStart    = 0x83,
    LibNullStream  = 0x84,
    LibWriteFailed = 0x85,
    LibInvalidArg  = 0x86,
};

const char* datecs_error_str(DatecsError e);

// ─── Subcommands ─────────────────────────────────────────────
enum class BoricaSubCmd : uint8_t {
    Ping            = 0x00,
    TransactionStart= 0x01,
    GetReceiptTags  = 0x02,
    TransactionEnd  = 0x03,
    GetReportTags   = 0x04,
    GetReportInfo   = 0x05,
    GetPinpadInfo   = 0x06,
    GetRTC          = 0x07,
    SetRTC          = 0x08,
    DeleteBatch     = 0x0B,
    ClearReversal   = 0x0C,
    GetPinpadStatus = 0x1A,
    GetMenuInfo     = 0x1D,
    GetPublicKeys   = 0x1E,
    GetSymmetricKeys= 0x1F,
    EditCommParams  = 0x20,
    KeyManagement   = 0x21,
    CheckPassword   = 0x23,
    GetReportByStan = 0x24,
    SelectChipApp   = 0x25,
    GetReaderState  = 0x26,
    GetTerminalTags = 0x27,
};

enum class ExtSubCmd : uint8_t {
    ReceiveData = 0x01,
    EventConfirm= 0x02,
    GetMaxMtu   = 0x03,
};

// ─── Transaction types ───────────────────────────────────────
enum class TransactionType : uint8_t {
    Purchase            = 0x01,
    PurchaseCashback    = 0x02,
    PurchaseReference   = 0x03,
    CashAdvance         = 0x04,
    Authorization       = 0x05,
    PurchaseCode        = 0x06,
    VoidPurchase        = 0x07,
    VoidCashAdvance     = 0x08,
    VoidAuthorization   = 0x09,
    EndOfDay            = 0x0A,
    LoyaltyBalance      = 0x0B,
    LoyaltySpend        = 0x0C,
    VoidLoyaltySpend    = 0x0D,
    TestConnection      = 0x0E,
    TmsUpdate           = 0x0F,
    Refund              = 0x10,
};

// ─── Borica events ───────────────────────────────────────────
enum class BoricaEvent : uint8_t {
    TransactionComplete             = 0x01,
    IntermediateTransactionComplete = 0x02,
    HangTransaction                 = 0x03,
    SelectChipApplication           = 0x3F,
    SendLogData                     = 0x10,
};

// ─── External internet events ────────────────────────────────
enum class ExtEvent : uint8_t {
    SocketOpen  = 0x01,
    SocketClose = 0x02,
    SendData    = 0x03,
};

// ─── TLV tags ────────────────────────────────────────────────
namespace Tag {
    static constexpr uint16_t Amount            = 0x0081;
    static constexpr uint16_t Cashback          = 0x9F04;
    static constexpr uint16_t Tip               = 0xDF63;
    static constexpr uint16_t RRN              = 0xDF01;
    static constexpr uint16_t AuthID           = 0xDF02;
    static constexpr uint16_t Reference        = 0xDF03;
    static constexpr uint16_t TransResult      = 0xDF05;
    static constexpr uint16_t TransError       = 0xDF06;
    static constexpr uint16_t HostRRN          = 0xDF07;
    static constexpr uint16_t HostAuthID       = 0xDF08;
    static constexpr uint16_t HostCode         = 0xDF09;
    static constexpr uint16_t MaskedPAN        = 0xDF0A;
    static constexpr uint16_t LoyaltyPts       = 0xDF0B;
    static constexpr uint16_t TransType        = 0xDF10;
    static constexpr uint16_t ToType           = 0xDF12;
    static constexpr uint16_t CvmSignature     = 0xDF23;
    static constexpr uint16_t PayInterface     = 0xDF25;
    static constexpr uint16_t TermCurrName     = 0xDF27;
    static constexpr uint16_t MerchPhone       = 0xDF28;
    static constexpr uint16_t MerchPostCode    = 0xDF29;
    static constexpr uint16_t MerchTitleBG     = 0xDF2E;
    static constexpr uint16_t MerchAddrBG      = 0xDF2F;
    static constexpr uint16_t MerchCityBG      = 0xDF30;
    static constexpr uint16_t MerchNameBG      = 0xDF31;
    static constexpr uint16_t AmountEUR        = 0xDF04;
    static constexpr uint16_t CardScheme       = 0xDF00;
    static constexpr uint16_t CLCardScheme     = 0xDF60;
    static constexpr uint16_t TransBatchNum    = 0xDF61;
    static constexpr uint16_t InterfaceID      = 0xDF62;
    static constexpr uint16_t EMV_STAN         = 0x9F41;
    static constexpr uint16_t TransDate        = 0x009A;
    static constexpr uint16_t TransTime        = 0x9F21;
    static constexpr uint16_t CardholderName   = 0x5F20;
    static constexpr uint16_t TerminalID       = 0x9F1C;
    static constexpr uint16_t MerchantID       = 0x9F16;
    static constexpr uint16_t AppCryptogram    = 0x9F26;
    static constexpr uint16_t TermAID          = 0x9F06;
    static constexpr uint16_t IssuerID         = 0xDF79;
    static constexpr uint16_t PinEntered       = 0xDF7F;
    static constexpr uint16_t TerminalSerial   = 0x9F1E;
    static constexpr uint16_t BatchNumber      = 0xDF32;
    static constexpr uint32_t MaxCashbackAmount = 0xDF8004;
    static constexpr uint32_t CashbackCurrency  = 0xDF8005;
    static constexpr uint32_t BcardScaDeclined  = 0xDF8006;
}

// ─── Parsed packet ───────────────────────────────────────────
struct DatecsPacket {
    uint8_t  command   = 0;      // command, response (0x00), or event
    uint8_t  status    = 0;      // ST field (response only)
    uint16_t dataLen   = 0;
    uint8_t  data[MAX_PACKET_SIZE - 8] = {};
    uint8_t  csum      = 0;
    bool     isEvent   = false;  // true if packet is an unsolicited event
};

// ─── Socket open event data ──────────────────────────────────
struct SocketOpenEvent {
    uint8_t  socketId   = 0;
    uint8_t  type       = 0;    // 1=TCP, 2=UDP, 3=TCP_TLS, 4=UDP_TLS
    uint8_t  address[4] = {};
    uint16_t port       = 0;
    uint16_t timeout    = 0;
    char     apn[32]    = {};
    char     username[32] = {};
    char     password[32] = {};
    uint8_t  hostLanIP[4] = {};
    uint16_t hostLanPort = 0;
};

// ─── Pinpad info ─────────────────────────────────────────────
struct PinpadInfo {
    char    modelName[21]    = {};
    char    serialNumber[11] = {};
    uint8_t softVersion[4]   = {};
    char    terminalID[9]    = {};
    uint8_t menuType         = 0;
};

// ─── Pinpad status ───────────────────────────────────────────
struct PinpadStatus {
    bool    needsReversal  = false;
    bool    isHang         = false;
    bool    needsEndOfDay  = false;
    bool    needsTmsUpdate = false;
};

// ─── Transaction result ──────────────────────────────────────
struct TransactionResult {
    uint32_t payResult    = 0;
    uint32_t payError     = 0;
    uint32_t amountAuth   = 0;
    uint32_t emvStan      = 0;
    char     hostRRN[16]  = {};
    char     hostAuthID[16] = {};
};

// ─── RTC ─────────────────────────────────────────────────────
struct RTCTime {
    uint8_t year = 0;   // HEX, e.g. 0x19 = year 2019 (add 2000)
    uint8_t month = 0;
    uint8_t date = 0;
    uint8_t hour = 0;
    uint8_t min = 0;
    uint8_t sec = 0;
};

// ─── TLV item ────────────────────────────────────────────────
struct TLVItem {
    uint32_t tag    = 0;
    uint8_t  len    = 0;
    const uint8_t* value = nullptr;  // pointer into source buffer
};

// ============================================================
//  TLV Parser utility
// ============================================================
class TLVParser {
public:
    // Parse one TLV item from buf at *offset, advance offset.
    // Returns false when buf is exhausted or malformed.
    static bool next(const uint8_t* buf, uint16_t bufLen,
                     uint16_t& offset, TLVItem& out);

    // Find first occurrence of tag; returns false if not found.
    static bool find(const uint8_t* buf, uint16_t bufLen,
                     uint32_t tag, TLVItem& out);

    // Helper: read big-endian uint32 from TLV value (len 1..4)
    static uint32_t toUint32(const TLVItem& item);

    // Helper: copy string from TLV value (null-terminated)
    static void toString(const TLVItem& item, char* dst, uint8_t maxLen);
};

// ============================================================
//  Packet Builder
// ============================================================
class DatecsPacketBuilder {
public:
    DatecsPacketBuilder();

    // Reset builder, choose command type
    void begin(uint8_t command);

    // Append raw bytes
    DatecsPacketBuilder& append(uint8_t b);
    DatecsPacketBuilder& appendBytes(const uint8_t* data, uint16_t len);
    DatecsPacketBuilder& appendUint16BE(uint16_t val);
    DatecsPacketBuilder& appendUint32BE(uint32_t val);
    DatecsPacketBuilder& appendString(const char* str, uint8_t len = 0);

    // Append TLV entry
    DatecsPacketBuilder& appendTLV1(uint8_t tag, uint8_t value);
    DatecsPacketBuilder& appendTLV2(uint16_t tag, const uint8_t* value, uint8_t vlen);
    DatecsPacketBuilder& appendTLV(uint32_t tag, const uint8_t* value, uint8_t vlen);
    DatecsPacketBuilder& appendTLV_uint32(uint16_t tag, uint32_t value);  // 4-byte BIG-ENDIAN
    DatecsPacketBuilder& appendTLV_string(uint16_t tag, const char* str);

    // Finalise: fills LH LL and CSUM, writes to outBuf.
    // Returns total packet length (including start byte and checksum).
    uint16_t build(uint8_t* outBuf, uint16_t maxLen);
    bool valid() const { return !_overflow; }

    // Convenience: return checksum over built packet (for verification)
    static uint8_t calcCsum(const uint8_t* buf, uint16_t len);

private:
    uint8_t  _cmd;
    uint8_t  _buf[MAX_PACKET_SIZE - 6];
    uint16_t _dataLen;
    bool _overflow;
};

// ============================================================
//  Packet Parser
// ============================================================
class DatecsPacketParser {
public:
    // Parse raw bytes into DatecsPacket.
    // Returns DatecsError::NoErr on success.
    static DatecsError parse(const uint8_t* raw, uint16_t rawLen,
                             DatecsPacket& out);

    // Parse unsolicited event from CARD READER (EVT_BORICA / EVT_EXTERNAL)
    static bool isEvent(const DatecsPacket& pkt);

    // Extract transaction result from TRANSACTION COMPLETE event data
    static bool parseTransactionResult(const DatecsPacket& pkt,
                                       TransactionResult& out);

    // Extract SocketOpen event details
    static bool parseSocketOpenEvent(const DatecsPacket& pkt,
                                     SocketOpenEvent& out);

    // Extract PinpadInfo from GET PINPAD INFO response
    static bool parsePinpadInfo(const DatecsPacket& pkt, PinpadInfo& out);

    // Extract PinpadStatus from GET PINPAD STATUS response
    static bool parsePinpadStatus(const DatecsPacket& pkt, PinpadStatus& out);

    // Extract RTC from GET RTC response
    static bool parseRTC(const DatecsPacket& pkt, RTCTime& out);
};

// ============================================================
//  High-level DatecsPay API
//  Wraps builder + send/receive over a Stream (HardwareSerial etc.)
// ============================================================
class DatecsPay {
public:
    // Pass the serial stream connected to the pinpad
    explicit DatecsPay(Stream& stream);

    // ── Low-level ──────────────────────────────────────────
    // Send a pre-built packet and wait for response.
    DatecsError sendPacket(const uint8_t* pkt, uint16_t len,
                           DatecsPacket& response);

    // Execute any documented Borica subcommand not covered by a typed helper.
    // `data` excludes the subcommand byte; response data is copied verbatim.
    DatecsError executeBorica(uint8_t subcommand, const uint8_t* data, uint16_t dataLen,
                              uint8_t* responseData, uint16_t responseCapacity,
                              uint16_t& responseLen);

    // Read a packet from stream (blocking, with timeout)
    DatecsError readPacket(DatecsPacket& out);

    // Write raw bytes to stream
    DatecsError writeBytes(const uint8_t* data, uint16_t len);

    // ── Utility ────────────────────────────────────────────
    DatecsError ping();
    DatecsError getPinpadInfo(PinpadInfo& info);
    DatecsError getPinpadStatus(PinpadStatus& status);
    DatecsError getRTC(RTCTime& rtc);
    DatecsError setRTC(const RTCTime& rtc);
    DatecsError getReportInfo(uint16_t& recordCount);
    DatecsError transactionEnd(bool ok = true);
    DatecsError deleteBatch();
    DatecsError clearReversal();
    DatecsError selectChipApplication(uint8_t index);

    // ── Transactions ───────────────────────────────────────
    DatecsError startPurchase(uint32_t amountCents);
    DatecsError startPurchaseWithTip(uint32_t totalCents, uint32_t tipCents);
    DatecsError startPurchaseWithCashback(uint32_t amountCents, uint32_t cashbackCents);
    DatecsError startPurchaseWithReference(uint32_t amountCents, const char* ref);
    DatecsError startCashAdvance(uint32_t amountCents);
    DatecsError startAuthorization(uint32_t amountCents);
    DatecsError startPurchaseWithCode(uint32_t amountCents,
                                      const char* rrn, const char* authID);
    DatecsError startVoidPurchase(uint32_t amountCents,
                                  const char* rrn, const char* authID);
    DatecsError startVoidPurchaseWithTip(uint32_t amountCents, uint32_t tipCents,
                                         const char* rrn, const char* authID);
    DatecsError startVoidPurchaseWithCashback(uint32_t amountCents, uint32_t cashbackCents,
                                              const char* rrn, const char* authID);
    DatecsError startVoidCashAdvance(uint32_t amountCents,
                                     const char* rrn, const char* authID);
    DatecsError startVoidAuthorization(uint32_t amountCents,
                                       const char* rrn, const char* authID);
    DatecsError startEndOfDay();
    DatecsError startTestConnection();
    DatecsError startTmsUpdate();
    DatecsError startRefund(uint32_t amountCents,
                            const char* rrn, const char* authID);

    // ── Receipt / report ───────────────────────────────────
    // Get receipt TLV data after transaction; fills buf, returns used length.
    DatecsError getReceiptTags(const uint16_t* tags, uint8_t tagCount,
                               uint8_t* buf, uint16_t bufSize, uint16_t& outLen);
    DatecsError getReceiptTagsRaw(const uint8_t* encodedTags, uint16_t encodedTagsLen,
                                  uint8_t* buf, uint16_t bufSize, uint16_t& outLen);
    DatecsError getReportTags(const uint16_t* tags, uint8_t tagCount,
                              uint8_t* buf, uint16_t bufSize, uint16_t& outLen);
    DatecsError getReportTagsRaw(const uint8_t* encodedTags, uint16_t encodedTagsLen,
                                 uint8_t* buf, uint16_t bufSize, uint16_t& outLen);

    // Get report tags for a specific STAN
    DatecsError getReportTagsByStan(uint32_t stan,
                                    const uint16_t* tags, uint8_t tagCount,
                                    uint8_t* buf, uint16_t bufSize, uint16_t& outLen);

    // ── External internet helpers ──────────────────────────
    // Confirm a socket event (OPEN/CLOSE/SEND)
    DatecsError confirmSocketEvent(uint8_t evtType, bool success,
                                   uint16_t mtu = 0x0400);

    // Forward host response data to pinpad (handles errBusy retry)
    DatecsError receiveData(const uint8_t* data, uint16_t len);

    // Get pinpad's max MTU
    DatecsError getMaxMtu(uint16_t& mtu);

    // ── Event handling callback types ──────────────────────
    using TransactionCompleteCallback = void(*)(const TransactionResult&);
    using SocketOpenCallback          = void(*)(const SocketOpenEvent&);
    using SocketCloseCallback         = void(*)(uint8_t socketId);
    using SendDataCallback            = void(*)(uint8_t socketId,
                                                const uint8_t* data, uint16_t len);
    using SelectChipAppCallback       = void(*)(const uint8_t* appNamesData, uint16_t len);
    using HangTransactionCallback     = void(*)(const uint8_t* tlvData, uint16_t len);
    using RawEventCallback            = void(*)(uint8_t event, uint8_t subevent,
                                                const uint8_t* data, uint16_t len);

    void onTransactionComplete(TransactionCompleteCallback cb)  { _cbTxComplete = cb; }
    void onSocketOpen(SocketOpenCallback cb)                    { _cbSockOpen   = cb; }
    void onSocketClose(SocketCloseCallback cb)                  { _cbSockClose  = cb; }
    void onSendData(SendDataCallback cb)                        { _cbSendData   = cb; }
    void onSelectChipApp(SelectChipAppCallback cb)              { _cbChipApp    = cb; }
    void onHangTransaction(HangTransactionCallback cb)          { _cbHang       = cb; }
    void onRawEvent(RawEventCallback cb)                        { _cbRawEvent   = cb; }

    // ── Event loop ────────────────────────────────────────
    // Call from loop() to process any incoming unsolicited events.
    // Returns true if an event was dispatched.
    bool processEvents();

private:
    Stream& _stream;
    uint8_t _txBuf[MAX_PACKET_SIZE];
    uint8_t _rxBuf[MAX_PACKET_SIZE];

    TransactionCompleteCallback _cbTxComplete = nullptr;
    SocketOpenCallback          _cbSockOpen   = nullptr;
    SocketCloseCallback         _cbSockClose  = nullptr;
    SendDataCallback            _cbSendData   = nullptr;
    SelectChipAppCallback       _cbChipApp    = nullptr;
    HangTransactionCallback     _cbHang       = nullptr;
    RawEventCallback            _cbRawEvent   = nullptr;

    DatecsError _startTransaction(TransactionType type,
                                  const uint8_t* paramData, uint16_t paramLen);
    DatecsError _dispatchEvent(const DatecsPacket& pkt);
};
