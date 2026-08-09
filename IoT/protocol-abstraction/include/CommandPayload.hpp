#pragma once
#include <array>
#include <cstdint>
#include <string>
#include <vector>

namespace bee {
struct OpenReceiptPayload {
  uint8_t operatorNumber{};
  std::string password;
  std::string unp;
  uint32_t tillNumber{1};
  bool invoice{};
};

struct SaleItemPayload {
  std::string name;
  char taxGroup{}; // canonical A..H
  std::string unitPrice;
  std::string quantity{"1.000"};
  uint8_t discountType{}; // 0 none, 1/+%, 2/-%, 3/+sum, 4/-sum
  std::string discountValue;
  uint8_t department{};
  std::string unit;
};

struct PaymentPayload {
  std::string amount;
  char daisyPaymentCode{}; // configured device code: P,N,C,D,U,B,E
  uint8_t datecsPaidMode{}; // configured device mode: 0..5
};

struct CashMovementPayload { std::string amount; bool cashOut{}; };
struct ReceiptStatusResult { uint8_t openState{}; uint32_t receiptNumber{}; uint32_t items{}; std::string amount; std::string paid; };

struct DatecsResult {
  int errorCode{};
  std::vector<std::string> fields;
};
struct TaxRatesResult { uint32_t firstZReport{}; std::array<std::string,8> rates{}; std::string effectiveDate; };
struct SubtotalPayload { bool print{}; bool display{}; uint8_t discountType{}; std::string discountValue; };
struct SubtotalResult { uint32_t slipNumber{}; std::string subtotal; std::array<std::string,8> taxTotals{}; };
struct LastFiscalEntryResult { uint32_t reportNumber{}; std::array<std::string,8> taxValues{}; std::string date; };
struct ErrorDescriptionResult { int code{}; std::string message; };
struct CurrentReceiptResult { std::array<std::string,8> taxTotals{}; bool invoice{}; std::string nextInvoiceNumber; bool reversal{}; };
struct DeviceIdentityResult { std::string serialNumber; std::string fiscalMemoryNumber; std::string companyName; std::string companyAddress; std::string taxNumber; std::string premisesName; std::string premisesAddress; };
struct DailyTaxationResult { uint32_t reportNumber{}; std::array<std::string,8> taxValues{}; };
struct DiagnosticInfoResult { std::string deviceName; std::string firmwareRevision; std::string firmwareDate; std::string firmwareTime; std::string checksum; std::string switches; std::string serialNumber; std::string fiscalMemoryNumber; };
struct ItemGroupInfoResult { std::string totalSales; std::string totalSum; std::string name; };
struct DepartmentInfoResult { uint32_t taxGroup{}; std::string price; std::string totalSales; std::string totalSum; std::string reversalSales; std::string reversalSum; std::string name; };
struct DailyPaymentsResult { std::array<std::string,7> amounts{}; };
struct DailyCountSumResult { uint32_t count{}; std::string sum; };
struct DailyDualCountSumResult { std::array<uint32_t,2> counts{}; std::array<std::string,2> sums{}; };
struct DailyCashMovementResult { std::array<uint32_t,4> counts{}; std::array<std::string,4> sums{}; };
struct OperatorInfoResult { std::array<uint32_t,5> counts{}; std::array<std::string,5> sums{}; };
struct CurrencyConversionPayload { uint8_t direction{}; std::string amount; };
struct FiscalMemoryReadPayload { std::string address; uint8_t byteCount{}; };
struct DevicePowerNetworkResult { uint32_t mainBatteryMv{}; uint32_t ramBatteryMv{}; bool signalAvailable{}; uint32_t signalPercent{}; bool networkAvailable{}; bool networkRegistered{}; };
struct LastFiscalReceiptInfoResult { uint32_t receiptNumber{}; std::string receiptDateTime; uint32_t zReportNumber{}; std::string zReportDateTime; };
struct DeviceBatteryResult { uint32_t mainBatteryMv{}; uint32_t chargePercent{}; };
struct ReceiptPeriodSearchPayload { std::string startDateTime; std::string endDateTime; uint8_t documentType{}; };
struct ReceiptPeriodSearchResult { std::string startDateTime; std::string endDateTime; uint64_t firstDocument{}; uint64_t lastDocument{}; };
struct EjDocumentSelector { uint8_t option{}; uint64_t documentNumber{}; uint8_t recordType{}; };
struct EjDocumentInfoResult { uint32_t globalDocumentNumber{}; uint64_t recordNumber{}; std::string dateTime; uint8_t documentType{}; uint32_t zReportNumber{}; };
struct EjCsvRange { uint32_t firstDocument{}; uint32_t lastDocument{}; };
struct EjCsvRow { std::vector<std::string> columns; };
struct FiscalMemoryCapacityResult { uint32_t nonEmpty{}; uint32_t maximum{}; };
struct FiscalMemoryZRecordResult { uint32_t number{}; std::string dateTime; std::array<std::string,8> sales{}; std::string salesTotal; std::array<std::string,8> reversals{}; std::string reversalTotal; std::string hash; uint32_t lastDocument{}; uint32_t klenNumber{}; };
struct FiscalMemoryValueEventResult { std::string value; std::string dateTime; };
struct FiscalMemoryVatEventResult { std::array<std::string,8> rates{}; uint32_t zReportNumber{}; uint8_t decimalPoint{}; std::string dateTime; };
struct FiscalMemoryCounterEventResult { uint32_t zReportNumber{}; std::string dateTime; };
struct FiscalMemoryKlenEventResult { uint32_t openLastReceipt{}; uint32_t openLastZReport{}; std::string openedAt; bool closed{}; uint32_t closeLastReceipt{}; uint32_t closeLastZReport{}; std::string closedAt; };
struct ModemStatusResult { uint32_t signalPercent{}; std::string imei; std::string imsi; std::string mobileOperator; };
struct ReversalOpenPayload {
  uint8_t operatorNumber{};
  std::string password;
  uint32_t tillNumber{1};
  uint8_t reason{};
  uint32_t originalDocument{};
  std::string originalDateTime;
  std::string fiscalMemoryNumber;
  bool invoice{};
  std::string originalInvoiceNumber;
  std::string invoiceReason;
  std::string unp;
};
struct ProgrammedItemPayload {
  uint32_t pluCode{};
  std::string quantity{"1.000"};
  std::string price;
  uint8_t discountType{};
  std::string discountValue;
};
struct NapConnectionResult {
  std::string lastConnection;
  std::string nextConnection;
  uint32_t lastZReport{};
  uint32_t zReportWithError{};
  uint32_t zErrorCount{};
  int zErrorStatus{};
  uint32_t saleDocumentWithError{};
  uint32_t saleErrorCount{};
  int saleErrorStatus{};
  uint32_t lastReceivedSaleDocument{};
  std::string lastReceivedSaleDateTime;
  uint32_t lastError{};
  uint32_t remainingMinutes{};
};
struct FiscalMemoryDateReportPayload { uint8_t type{}; std::string startDate; std::string endDate; };
struct FiscalMemoryNumberReportPayload { uint8_t type{}; uint16_t firstZReport{}; uint16_t lastZReport{}; };
struct OperatorReportPayload { uint8_t firstOperator{}; uint8_t lastOperator{}; bool clear{}; };
struct PluReportPayload { uint8_t type{}; uint32_t firstPlu{}; uint32_t lastPlu{}; };

bool buildDaisyOpenReceipt(const OpenReceiptPayload&, std::vector<uint8_t>&, std::string&);
bool buildDatecsOpenReceipt(const OpenReceiptPayload&, std::vector<uint8_t>&, std::string&);
bool buildDaisySaleItem(const SaleItemPayload&, std::vector<uint8_t>&, std::string&);
bool buildDatecsSaleItem(const SaleItemPayload&, std::vector<uint8_t>&, std::string&);
bool buildDaisyPayment(const PaymentPayload&, std::vector<uint8_t>&, std::string&);
bool buildDatecsPayment(const PaymentPayload&, std::vector<uint8_t>&, std::string&);
bool buildDaisyDailyReport(bool zReport, std::vector<uint8_t>&);
bool buildDatecsDailyReport(bool zReport, std::vector<uint8_t>&);
bool buildDaisyCashMovement(const CashMovementPayload&, std::vector<uint8_t>&, std::string&);
bool buildDatecsCashMovement(const CashMovementPayload&, std::vector<uint8_t>&, std::string&);
bool buildDatecsSubtotal(const SubtotalPayload&, std::vector<uint8_t>&, std::string&);
bool buildDatecsLastFiscalEntry(uint8_t, std::vector<uint8_t>&, std::string&);
bool buildDatecsErrorLookup(int, std::vector<uint8_t>&, std::string&);
bool buildDatecsDeviceIdentity(std::vector<uint8_t>&);
bool buildDatecsDailyTaxation(uint8_t, std::vector<uint8_t>&, std::string&);
bool buildDatecsDiagnosticInfo(bool, std::vector<uint8_t>&);
bool buildDatecsItemGroupInfo(uint8_t, std::vector<uint8_t>&, std::string&);
bool buildDatecsDepartmentInfo(uint8_t, std::vector<uint8_t>&, std::string&);
bool buildDatecsFiscalMemoryTest(bool, std::vector<uint8_t>&);
bool buildDatecsAdditionalDailyInfo(uint8_t, uint8_t, std::vector<uint8_t>&, std::string&);
bool buildDatecsOperatorInfo(uint8_t, std::vector<uint8_t>&, std::string&);
bool buildDatecsCurrencyConversion(const CurrencyConversionPayload&, std::vector<uint8_t>&, std::string&);
bool buildDatecsFiscalMemoryRead(const FiscalMemoryReadPayload&, std::vector<uint8_t>&, std::string&);
bool buildDatecsDeviceInfo(uint8_t, std::vector<uint8_t>&, std::string&);
bool buildDatecsReceiptPeriodSearch(const ReceiptPeriodSearchPayload&, std::vector<uint8_t>&, std::string&);
bool buildDatecsEjDocumentSelector(const EjDocumentSelector&, std::vector<uint8_t>&, std::string&);
bool buildDatecsEjCsvRange(const EjCsvRange&, std::vector<uint8_t>&, std::string&);
bool buildDatecsEjCsvRead(std::vector<uint8_t>&);
bool buildDatecsFiscalMemoryStructured(uint8_t, uint16_t, std::vector<uint8_t>&, std::string&);
bool buildDatecsModemInfo(char, std::vector<uint8_t>&, std::string&);
bool buildDatecsOpenReversal(const ReversalOpenPayload&, std::vector<uint8_t>&, std::string&);
bool buildDatecsPcConnectionMode(int, std::vector<uint8_t>&, std::string&);
bool buildDatecsProgrammedItem(const ProgrammedItemPayload&, std::vector<uint8_t>&, std::string&);
bool buildDatecsGeneralInfo(uint8_t, std::vector<uint8_t>&, std::string&);
bool buildDatecsFiscalMemoryDateReport(const FiscalMemoryDateReportPayload&, std::vector<uint8_t>&, std::string&);
bool buildDatecsFiscalMemoryNumberReport(const FiscalMemoryNumberReportPayload&, std::vector<uint8_t>&, std::string&);
bool buildDatecsOperatorReport(const OperatorReportPayload&, std::vector<uint8_t>&, std::string&);
bool buildDatecsPluReport(const PluReportPayload&, std::vector<uint8_t>&, std::string&);
bool parseDatecsResult(const std::vector<uint8_t>&, DatecsResult&, std::string&);
bool parseDatecsTaxRates(const std::vector<uint8_t>&, TaxRatesResult&, std::string&);
bool parseDatecsSubtotal(const std::vector<uint8_t>&, SubtotalResult&, std::string&);
bool parseDatecsDateTime(const std::vector<uint8_t>&, std::string&, std::string&);
bool parseDatecsLastFiscalEntry(const std::vector<uint8_t>&, LastFiscalEntryResult&, std::string&);
bool parseDatecsTaxNumber(const std::vector<uint8_t>&, std::string&, std::string&);
bool parseDatecsErrorDescription(const std::vector<uint8_t>&, ErrorDescriptionResult&, std::string&);
bool parseDatecsCurrentReceipt(const std::vector<uint8_t>&, CurrentReceiptResult&, std::string&);
bool parseDatecsDeviceIdentity(const std::vector<uint8_t>&, DeviceIdentityResult&, std::string&);
bool parseDatecsDailyTaxation(const std::vector<uint8_t>&, DailyTaxationResult&, std::string&);
bool parseDatecsReportsLeft(const std::vector<uint8_t>&, uint32_t&, std::string&);
bool parseDatecsLastFiscalRecordDateTime(const std::vector<uint8_t>&, std::string&, std::string&);
bool parseDatecsDiagnosticInfo(const std::vector<uint8_t>&, DiagnosticInfoResult&, std::string&);
bool parseDatecsItemGroupInfo(const std::vector<uint8_t>&, ItemGroupInfoResult&, std::string&);
bool parseDatecsDepartmentInfo(const std::vector<uint8_t>&, DepartmentInfoResult&, std::string&);
bool parseDatecsFiscalMemoryTest(const std::vector<uint8_t>&, uint32_t&, std::string&);
bool parseDatecsDailyPayments(const std::vector<uint8_t>&, DailyPaymentsResult&, std::string&);
bool parseDatecsDailySales(const std::vector<uint8_t>&, DailyCountSumResult&, std::string&);
bool parseDatecsDailyDualCountSum(const std::vector<uint8_t>&, DailyDualCountSumResult&, std::string&);
bool parseDatecsDailyCashMovements(const std::vector<uint8_t>&, DailyCashMovementResult&, std::string&);
bool parseDatecsOperatorInfo(const std::vector<uint8_t>&, OperatorInfoResult&, std::string&);
bool parseDatecsCurrencyConversion(const std::vector<uint8_t>&, std::string&, std::string&);
bool parseDatecsFiscalMemoryRead(const std::vector<uint8_t>&, uint8_t, std::vector<uint8_t>&, std::string&);
bool parseDatecsDevicePowerNetwork(const std::vector<uint8_t>&, DevicePowerNetworkResult&, std::string&);
bool parseDatecsLastFiscalReceiptInfo(const std::vector<uint8_t>&, LastFiscalReceiptInfoResult&, std::string&);
bool parseDatecsDeviceInfoVerification(const std::vector<uint8_t>&, std::string&);
bool parseDatecsDeviceBattery(const std::vector<uint8_t>&, DeviceBatteryResult&, std::string&);
bool parseDatecsReceiptPeriodSearch(const std::vector<uint8_t>&, uint8_t, ReceiptPeriodSearchResult&, std::string&);
bool parseDatecsEjDocumentInfo(const std::vector<uint8_t>&, EjDocumentInfoResult&, std::string&);
bool parseDatecsEjTextLine(const std::vector<uint8_t>&, std::string&, std::string&);
bool parseDatecsEjBase64Data(const std::vector<uint8_t>&, std::string&, std::string&);
bool parseDatecsEjAcknowledge(const std::vector<uint8_t>&, std::string&);
bool parseDatecsEjCsvRow(const std::vector<uint8_t>&, EjCsvRow&, std::string&);
bool parseDatecsFiscalMemoryCapacity(const std::vector<uint8_t>&, FiscalMemoryCapacityResult&, std::string&);
bool parseDatecsFiscalMemoryZRecord(const std::vector<uint8_t>&, FiscalMemoryZRecordResult&, std::string&);
bool parseDatecsFiscalMemoryValueEvent(const std::vector<uint8_t>&, size_t, FiscalMemoryValueEventResult&, std::string&);
bool parseDatecsFiscalMemoryDateEvent(const std::vector<uint8_t>&, std::string&, std::string&);
bool parseDatecsFiscalMemoryVatEvent(const std::vector<uint8_t>&, FiscalMemoryVatEventResult&, std::string&);
bool parseDatecsFiscalMemoryCounterEvent(const std::vector<uint8_t>&, FiscalMemoryCounterEventResult&, std::string&);
bool parseDatecsFiscalMemoryKlenEvent(const std::vector<uint8_t>&, FiscalMemoryKlenEventResult&, std::string&);
bool parseDatecsModemIdentifier(const std::vector<uint8_t>&, std::string&, std::string&);
bool parseDatecsModemStatus(const std::vector<uint8_t>&, ModemStatusResult&, std::string&);
bool parseDatecsSlipNumber(const std::vector<uint8_t>&, uint32_t&, std::string&);
bool parseDatecsAcknowledge(const std::vector<uint8_t>&, std::string&);
bool parseDatecsNapConnection(const std::vector<uint8_t>&, NapConnectionResult&, std::string&);
bool parseDaisyOpenReceiptResult(const std::vector<uint8_t>&, uint32_t&, uint32_t&, std::string&);
bool parseDaisyReceiptStatus(const std::vector<uint8_t>&, ReceiptStatusResult&, std::string&);
bool parseDatecsReceiptStatus(const std::vector<uint8_t>&, ReceiptStatusResult&, std::string&);
}
