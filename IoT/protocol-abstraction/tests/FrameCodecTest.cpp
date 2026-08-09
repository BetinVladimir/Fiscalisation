#include "FrameCodec.hpp"
#include "FiscalDriver.hpp"
#include "CommandPayload.hpp"
#include <cassert>
#include <vector>
using namespace bee;

static void testExtendedDatecsReadCommands() {
    std::vector<uint8_t> payload;
    std::string error;

    assert(buildDatecsErrorLookup(-111016, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "-111016\t");
    assert(!buildDatecsErrorLookup(0, payload, error));
    assert(buildDatecsDeviceIdentity(payload));
    assert(std::string(payload.begin(), payload.end()) == "1\t");

    std::string taxNumber;
    const std::string taxNumberRaw = "0\t000713391\t";
    assert(parseDatecsTaxNumber(
        std::vector<uint8_t>(taxNumberRaw.begin(), taxNumberRaw.end()),
        taxNumber,
        error));
    assert(taxNumber == "000713391");

    ErrorDescriptionResult description;
    const std::string descriptionRaw = "0\t-111016\tReceipt closed\t";
    assert(parseDatecsErrorDescription(
        std::vector<uint8_t>(descriptionRaw.begin(), descriptionRaw.end()),
        description,
        error));
    assert(description.code == -111016);
    assert(description.message == "Receipt closed");

    CurrentReceiptResult receipt;
    const std::string receiptRaw =
        "0\t0.00\t20.00\t0.00\t0.00\t0.00\t0.00\t0.00\t0.00\t1\t5\t0\t";
    assert(parseDatecsCurrentReceipt(
        std::vector<uint8_t>(receiptRaw.begin(), receiptRaw.end()),
        receipt,
        error));
    assert(receipt.taxTotals[1] == "20.00");
    assert(receipt.invoice);
    assert(receipt.nextInvoiceNumber == "5");
    assert(!receipt.reversal);

    const std::string invalidReceiptRaw =
        "0\t0.00\t0.00\t0.00\t0.00\t0.00\t0.00\t0.00\t0.00\t2\t5\t0\t";
    assert(!parseDatecsCurrentReceipt(
        std::vector<uint8_t>(invalidReceiptRaw.begin(), invalidReceiptRaw.end()),
        receipt,
        error));

    DeviceIdentityResult identity;
    const std::string identityRaw =
        "0\tDT636555\t02636555\tDATECS LTD\tSofia 4\t000713391\tShop\tSofia 4\t";
    assert(parseDatecsDeviceIdentity(
        std::vector<uint8_t>(identityRaw.begin(), identityRaw.end()),
        identity,
        error));
    assert(identity.serialNumber == "DT636555");
    assert(identity.fiscalMemoryNumber == "02636555");
    assert(identity.taxNumber == "000713391");
    assert(identity.premisesName == "Shop");

    DailyTaxationResult daily;
    assert(buildDatecsDailyTaxation(0, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "0\t");
    assert(!buildDatecsDailyTaxation(4, payload, error));
    const std::string dailyRaw =
        "0\t7\t22.40\t127.22\t0.00\t0.00\t0.00\t0.00\t0.00\t0.00\t";
    assert(parseDatecsDailyTaxation(
        std::vector<uint8_t>(dailyRaw.begin(), dailyRaw.end()), daily, error));
    assert(daily.reportNumber == 7);
    assert(daily.taxValues[0] == "22.40");
    assert(daily.taxValues[1] == "127.22");

    uint32_t reportsLeft = 0;
    const std::string reportsLeftRaw = "0\t3644\t";
    assert(parseDatecsReportsLeft(
        std::vector<uint8_t>(reportsLeftRaw.begin(), reportsLeftRaw.end()),
        reportsLeft,
        error));
    assert(reportsLeft == 3644);
    const std::string invalidReportsLeftRaw = "0\t3651\t";
    assert(!parseDatecsReportsLeft(
        std::vector<uint8_t>(invalidReportsLeftRaw.begin(), invalidReportsLeftRaw.end()),
        reportsLeft,
        error));

    std::string lastFiscalDateTime;
    const std::string lastFiscalDateTimeRaw = "0\t07-03-2020 16:10:52\t";
    assert(parseDatecsLastFiscalRecordDateTime(
        std::vector<uint8_t>(lastFiscalDateTimeRaw.begin(), lastFiscalDateTimeRaw.end()),
        lastFiscalDateTime,
        error));
    assert(lastFiscalDateTime == "07-03-2020 16:10:52");

    assert(buildDatecsDiagnosticInfo(false, payload));
    assert(payload.empty());
    assert(buildDatecsDiagnosticInfo(true, payload));
    assert(std::string(payload.begin(), payload.end()) == "1\t");
    DiagnosticInfoResult diagnostic;
    const std::string diagnosticRaw =
        "0\tWP-50X\t261216\t12Mar19\t1631\t1426\t00000000\tDT636555\t02636555\t";
    assert(parseDatecsDiagnosticInfo(
        std::vector<uint8_t>(diagnosticRaw.begin(), diagnosticRaw.end()),
        diagnostic,
        error));
    assert(diagnostic.deviceName == "WP-50X");
    assert(diagnostic.checksum == "1426");
    assert(diagnostic.serialNumber == "DT636555");
    assert(diagnostic.fiscalMemoryNumber == "02636555");
    const std::string invalidDiagnosticRaw =
        "0\tWP-50X\t261216\t12Mar19\t1631\tBAD\t00000000\tDT636555\t02636555\t";
    assert(!parseDatecsDiagnosticInfo(
        std::vector<uint8_t>(invalidDiagnosticRaw.begin(), invalidDiagnosticRaw.end()),
        diagnostic,
        error));

    assert(buildDatecsItemGroupInfo(1, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "1\t");
    assert(buildDatecsItemGroupInfo(0, payload, error));
    assert(payload.empty());
    assert(!buildDatecsItemGroupInfo(100, payload, error));
    ItemGroupInfoResult itemGroup;
    const std::string itemGroupRaw = "0\t0.000\t0.00\tGROUP 1\t";
    assert(parseDatecsItemGroupInfo(
        std::vector<uint8_t>(itemGroupRaw.begin(), itemGroupRaw.end()),
        itemGroup,
        error));
    assert(itemGroup.totalSales == "0.000");
    assert(itemGroup.totalSum == "0.00");
    assert(itemGroup.name == "GROUP 1");
    const std::string invalidItemGroupRaw = "0\t0.0000\t0.00\tGROUP 1\t";
    assert(!parseDatecsItemGroupInfo(
        std::vector<uint8_t>(invalidItemGroupRaw.begin(), invalidItemGroupRaw.end()),
        itemGroup,
        error));

    assert(buildDatecsDepartmentInfo(1, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "1\t");
    assert(buildDatecsDepartmentInfo(0, payload, error));
    assert(payload.empty());
    assert(!buildDatecsDepartmentInfo(100, payload, error));
    DepartmentInfoResult department;
    const std::string departmentRaw =
        "0\t2\t1.00\t0.000\t0.00\t0.000\t0.00\tDEPARTMENT 1\t";
    assert(parseDatecsDepartmentInfo(
        std::vector<uint8_t>(departmentRaw.begin(), departmentRaw.end()),
        department,
        error));
    assert(department.taxGroup == 2);
    assert(department.price == "1.00");
    assert(department.totalSales == "0.000");
    assert(department.reversalSum == "0.00");
    assert(department.name == "DEPARTMENT 1");
    const std::string invalidDepartmentRaw =
        "0\tX\t1.00\t0.000\t0.00\t0.000\t0.00\tDEPARTMENT 1\t";
    assert(!parseDatecsDepartmentInfo(
        std::vector<uint8_t>(invalidDepartmentRaw.begin(), invalidDepartmentRaw.end()),
        department,
        error));

    assert(buildDatecsFiscalMemoryTest(false, payload));
    assert(std::string(payload.begin(), payload.end()) == "0\t");
    assert(buildDatecsFiscalMemoryTest(true, payload));
    assert(std::string(payload.begin(), payload.end()) == "1\t");
    uint32_t fiscalMemoryRecords = 0;
    const std::string fiscalMemoryTestRaw = "0\t0015\t";
    assert(parseDatecsFiscalMemoryTest(
        std::vector<uint8_t>(fiscalMemoryTestRaw.begin(), fiscalMemoryTestRaw.end()),
        fiscalMemoryRecords,
        error));
    assert(fiscalMemoryRecords == 15);
    const std::string invalidFiscalMemoryTestRaw = "0\t17\t";
    assert(!parseDatecsFiscalMemoryTest(
        std::vector<uint8_t>(invalidFiscalMemoryTestRaw.begin(), invalidFiscalMemoryTestRaw.end()),
        fiscalMemoryRecords,
        error));

    assert(buildDatecsAdditionalDailyInfo(0, 0, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "0\t");
    assert(buildDatecsAdditionalDailyInfo(5, 30, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "5\t30\t");
    assert(!buildDatecsAdditionalDailyInfo(6, 0, payload, error));
    assert(!buildDatecsAdditionalDailyInfo(0, 31, payload, error));

    DailyPaymentsResult dailyPayments;
    const std::string dailyPaymentsRaw =
        "0\t19.04\t1.00\t9.00\t1.00\t1.00\t1.00\t1.00\t";
    assert(parseDatecsDailyPayments(
        std::vector<uint8_t>(dailyPaymentsRaw.begin(), dailyPaymentsRaw.end()),
        dailyPayments,
        error));
    assert(dailyPayments.amounts[0] == "19.04");
    assert(dailyPayments.amounts[6] == "1.00");

    DailyCountSumResult dailySales;
    const std::string dailySalesRaw = "0\t2\t34.00\t";
    assert(parseDatecsDailySales(
        std::vector<uint8_t>(dailySalesRaw.begin(), dailySalesRaw.end()),
        dailySales,
        error));
    assert(dailySales.count == 2);
    assert(dailySales.sum == "34.00");

    DailyDualCountSumResult dailyDual;
    const std::string dailyDualRaw = "0\t3\t1.01\t3\t-2.47\t";
    assert(parseDatecsDailyDualCountSum(
        std::vector<uint8_t>(dailyDualRaw.begin(), dailyDualRaw.end()),
        dailyDual,
        error));
    assert(dailyDual.counts[1] == 3);
    assert(dailyDual.sums[1] == "-2.47");
    const std::string invalidDailyDualRaw = "0\t1000000\t1.01\t3\t2.47\t";
    assert(!parseDatecsDailyDualCountSum(
        std::vector<uint8_t>(invalidDailyDualRaw.begin(), invalidDailyDualRaw.end()),
        dailyDual,
        error));

    DailyCashMovementResult dailyCash;
    const std::string dailyCashRaw =
        "0\t1\t1000.00\t1\t-50.00\t0\t0.00\t0\t0.00\t";
    assert(parseDatecsDailyCashMovements(
        std::vector<uint8_t>(dailyCashRaw.begin(), dailyCashRaw.end()),
        dailyCash,
        error));
    assert(dailyCash.counts[0] == 1);
    assert(dailyCash.sums[1] == "-50.00");

    assert(buildDatecsOperatorInfo(30, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "30\t");
    assert(!buildDatecsOperatorInfo(0, payload, error));
    assert(!buildDatecsOperatorInfo(31, payload, error));
    OperatorInfoResult operatorInfo;
    const std::string operatorInfoRaw =
        "0\t2\t34.00\t1\t-5.00\t3\t-2.47\t4\t1.01\t5\t-0.50\t";
    assert(parseDatecsOperatorInfo(
        std::vector<uint8_t>(operatorInfoRaw.begin(), operatorInfoRaw.end()),
        operatorInfo,
        error));
    assert(operatorInfo.counts[0] == 2);
    assert(operatorInfo.sums[2] == "-2.47");
    const std::string invalidOperatorInfoRaw =
        "0\t65536\t34.00\t1\t5.00\t3\t2.47\t4\t1.01\t5\t0.50\t";
    assert(!parseDatecsOperatorInfo(
        std::vector<uint8_t>(invalidOperatorInfoRaw.begin(), invalidOperatorInfoRaw.end()),
        operatorInfo,
        error));

    CurrencyConversionPayload conversion{1, "100"};
    assert(buildDatecsCurrencyConversion(conversion, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "1\t100\t");
    conversion.direction = 2;
    assert(!buildDatecsCurrencyConversion(conversion, payload, error));
    conversion = {0, "0"};
    assert(!buildDatecsCurrencyConversion(conversion, payload, error));
    std::string convertedAmount;
    const std::string convertedAmountRaw = "0\t195.58\t";
    assert(parseDatecsCurrencyConversion(
        std::vector<uint8_t>(convertedAmountRaw.begin(), convertedAmountRaw.end()),
        convertedAmount,
        error));
    assert(convertedAmount == "195.58");
    const std::string invalidConvertedAmountRaw = "0\t+195.58\t";
    assert(!parseDatecsCurrencyConversion(
        std::vector<uint8_t>(invalidConvertedAmountRaw.begin(), invalidConvertedAmountRaw.end()),
        convertedAmount,
        error));

    FiscalMemoryReadPayload memoryRead{"010101", 1};
    assert(buildDatecsFiscalMemoryRead(memoryRead, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "0\t010101\t1\t");
    memoryRead.address = "01010G";
    assert(!buildDatecsFiscalMemoryRead(memoryRead, payload, error));
    memoryRead = {"FFFFFF", 104};
    assert(buildDatecsFiscalMemoryRead(memoryRead, payload, error));
    memoryRead.byteCount = 105;
    assert(!buildDatecsFiscalMemoryRead(memoryRead, payload, error));
    std::vector<uint8_t> memoryBytes;
    const std::string memoryReadRaw = "0\tFF\t";
    assert(parseDatecsFiscalMemoryRead(
        std::vector<uint8_t>(memoryReadRaw.begin(), memoryReadRaw.end()),
        1,
        memoryBytes,
        error));
    assert(memoryBytes.size() == 1 && memoryBytes[0] == 0xFF);
    const std::string invalidMemoryReadRaw = "0\tFG\t";
    assert(!parseDatecsFiscalMemoryRead(
        std::vector<uint8_t>(invalidMemoryReadRaw.begin(), invalidMemoryReadRaw.end()),
        1,
        memoryBytes,
        error));
    assert(!parseDatecsFiscalMemoryRead(
        std::vector<uint8_t>(memoryReadRaw.begin(), memoryReadRaw.end()),
        2,
        memoryBytes,
        error));

    assert(buildDatecsDeviceInfo(1, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "1\t");
    assert(buildDatecsDeviceInfo(5, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "5\t");
    assert(!buildDatecsDeviceInfo(0, payload, error));
    assert(!buildDatecsDeviceInfo(6, payload, error));

    DevicePowerNetworkResult powerNetwork;
    const std::string powerNetworkRaw = "0\t8666\t4157\t96\t1\t";
    assert(parseDatecsDevicePowerNetwork(
        std::vector<uint8_t>(powerNetworkRaw.begin(), powerNetworkRaw.end()),
        powerNetwork,
        error));
    assert(powerNetwork.mainBatteryMv == 8666);
    assert(powerNetwork.signalAvailable && powerNetwork.signalPercent == 96);
    assert(powerNetwork.networkAvailable && powerNetwork.networkRegistered);
    const std::string bc50PowerRaw = "0\t8666\t4157\t\t\t";
    assert(parseDatecsDevicePowerNetwork(
        std::vector<uint8_t>(bc50PowerRaw.begin(), bc50PowerRaw.end()),
        powerNetwork,
        error));
    assert(!powerNetwork.signalAvailable && !powerNetwork.networkAvailable);

    LastFiscalReceiptInfoResult lastReceiptInfo;
    const std::string lastReceiptInfoRaw =
        "0\t270\t04-04-2020 17:38:24\t103\t04-04-2020 17:38:25\t";
    assert(parseDatecsLastFiscalReceiptInfo(
        std::vector<uint8_t>(lastReceiptInfoRaw.begin(), lastReceiptInfoRaw.end()),
        lastReceiptInfo,
        error));
    assert(lastReceiptInfo.receiptNumber == 270);
    assert(lastReceiptInfo.zReportNumber == 103);
    const std::string invalidLastReceiptInfoRaw =
        "0\t10000\t04-04-2020 17:38:24\t103\t04-04-2020 17:38:25\t";
    assert(!parseDatecsLastFiscalReceiptInfo(
        std::vector<uint8_t>(invalidLastReceiptInfoRaw.begin(), invalidLastReceiptInfoRaw.end()),
        lastReceiptInfo,
        error));

    assert(parseDatecsDeviceInfoVerification({'0', '\t'}, error));
    assert(!parseDatecsDeviceInfoVerification({'0', '\t', 'X', '\t'}, error));
    DeviceBatteryResult battery;
    const std::string batteryRaw = "0\t8666\t99\t";
    assert(parseDatecsDeviceBattery(
        std::vector<uint8_t>(batteryRaw.begin(), batteryRaw.end()), battery, error));
    assert(battery.mainBatteryMv == 8666 && battery.chargePercent == 99);

    ReceiptPeriodSearchPayload periodSearch{
        "01-05-19 00:00:00", "03-05-19 19:00:00", 0};
    assert(buildDatecsReceiptPeriodSearch(periodSearch, payload, error));
    assert(std::string(payload.begin(), payload.end()) ==
        "01-05-19 00:00:00\t03-05-19 19:00:00\t0\t");
    periodSearch.documentType = 11;
    assert(!buildDatecsReceiptPeriodSearch(periodSearch, payload, error));
    ReceiptPeriodSearchResult periodResult;
    const std::string periodResultRaw =
        "0\t01-05-19 00:00:00 DST\t03-05-19 19:00:00 DST\t471\t479\t";
    assert(parseDatecsReceiptPeriodSearch(
        std::vector<uint8_t>(periodResultRaw.begin(), periodResultRaw.end()),
        0,
        periodResult,
        error));
    assert(periodResult.firstDocument == 471 && periodResult.lastDocument == 479);
    const std::string invalidPeriodResultRaw =
        "0\t01-05-19 00:00:00 DST\t03-05-19 19:00:00 DST\t479\t471\t";
    assert(!parseDatecsReceiptPeriodSearch(
        std::vector<uint8_t>(invalidPeriodResultRaw.begin(), invalidPeriodResultRaw.end()),
        0,
        periodResult,
        error));

    EjDocumentSelector ejSelector{0, 1, 0};
    assert(buildDatecsEjDocumentSelector(ejSelector, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "0\t1\t0\t");
    ejSelector = {2, 9999999999ULL, 3};
    assert(buildDatecsEjDocumentSelector(ejSelector, payload, error));
    ejSelector.documentNumber = 10000000000ULL;
    assert(!buildDatecsEjDocumentSelector(ejSelector, payload, error));
    ejSelector = {1, 0, 1};
    assert(!buildDatecsEjDocumentSelector(ejSelector, payload, error));
    ejSelector = {4, 1, 0};
    assert(!buildDatecsEjDocumentSelector(ejSelector, payload, error));

    EjDocumentInfoResult ejInfo;
    const std::string ejInfoRaw =
        "0\t1\t1\t01-01-00\t01:36:09\t4\t1\t";
    assert(parseDatecsEjDocumentInfo(
        std::vector<uint8_t>(ejInfoRaw.begin(), ejInfoRaw.end()), ejInfo, error));
    assert(ejInfo.globalDocumentNumber == 1);
    assert(ejInfo.recordNumber == 1);
    assert(ejInfo.dateTime == "01-01-00 01:36:09");
    assert(ejInfo.documentType == 4 && ejInfo.zReportNumber == 1);
    const std::string combinedEjInfoRaw =
        "0\t471\t471\t01-05-19 00:00:00 DST\t1\t100\t";
    assert(parseDatecsEjDocumentInfo(
        std::vector<uint8_t>(combinedEjInfoRaw.begin(), combinedEjInfoRaw.end()),
        ejInfo,
        error));
    const std::string invalidEjInfoRaw =
        "0\t1\t3660\t01-01-00 01:36:09\t2\t1\t";
    assert(!parseDatecsEjDocumentInfo(
        std::vector<uint8_t>(invalidEjInfoRaw.begin(), invalidEjInfoRaw.end()),
        ejInfo,
        error));

    std::string ejText;
    const std::string ejTextRaw = "0\tFISCAL RECEIPT\t";
    assert(parseDatecsEjTextLine(
        std::vector<uint8_t>(ejTextRaw.begin(), ejTextRaw.end()), ejText, error));
    assert(ejText == "FISCAL RECEIPT");
    assert(parseDatecsEjTextLine({'0', '\t'}, ejText, error) && ejText.empty());

    std::string ejBase64;
    const std::string ejBase64Raw = "0\tAQIDBA==\t";
    assert(parseDatecsEjBase64Data(
        std::vector<uint8_t>(ejBase64Raw.begin(), ejBase64Raw.end()),
        ejBase64,
        error));
    const std::string invalidEjBase64Raw = "0\tAQID*===\t";
    assert(!parseDatecsEjBase64Data(
        std::vector<uint8_t>(invalidEjBase64Raw.begin(), invalidEjBase64Raw.end()),
        ejBase64,
        error));
    assert(parseDatecsEjAcknowledge({'0', '\t'}, error));

    EjCsvRange csvRange{471, 479};
    assert(buildDatecsEjCsvRange(csvRange, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "9\t471\t479\t");
    csvRange = {479, 471};
    assert(!buildDatecsEjCsvRange(csvRange, payload, error));
    assert(buildDatecsEjCsvRead(payload));
    assert(std::string(payload.begin(), payload.end()) == "8\t");
    EjCsvRow csvRow;
    const std::string csvRowRaw = "0\t02636555\tFISCAL\t471\tDT000001-ABCD-0000001\tITEM\t1.00\t1.000\t1.00\t1.00\t";
    assert(parseDatecsEjCsvRow(
        std::vector<uint8_t>(csvRowRaw.begin(), csvRowRaw.end()), csvRow, error));
    assert(csvRow.columns.size() == 9 && csvRow.columns[2] == "471");

    assert(buildDatecsFiscalMemoryStructured(0, 0, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "0\t\t");
    assert(buildDatecsFiscalMemoryStructured(9, 9999, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "9\t9999\t");
    assert(!buildDatecsFiscalMemoryStructured(10, 1, payload, error));
    FiscalMemoryCapacityResult fmCapacity;
    const std::string fmCapacityRaw = "0\t124\t3650\t";
    assert(parseDatecsFiscalMemoryCapacity(
        std::vector<uint8_t>(fmCapacityRaw.begin(), fmCapacityRaw.end()),
        fmCapacity,
        error));
    assert(fmCapacity.nonEmpty == 124 && fmCapacity.maximum == 3650);
    const std::string invalidFmCapacityRaw = "0\t3651\t3650\t";
    assert(!parseDatecsFiscalMemoryCapacity(
        std::vector<uint8_t>(invalidFmCapacityRaw.begin(), invalidFmCapacityRaw.end()),
        fmCapacity,
        error));

    FiscalMemoryValueEventResult fmValue;
    const std::string fmIdRaw = "0\tDT636518\t07-01-2019\t14:28\t";
    assert(parseDatecsFiscalMemoryValueEvent(
        std::vector<uint8_t>(fmIdRaw.begin(), fmIdRaw.end()), 8, fmValue, error));
    assert(fmValue.value == "DT636518");
    assert(fmValue.dateTime == "07-01-2019 14:28");
    std::string fiscalizedAt;
    const std::string fiscalizedAtRaw = "0\t07-01-2019\t14:28\t";
    assert(parseDatecsFiscalMemoryDateEvent(
        std::vector<uint8_t>(fiscalizedAtRaw.begin(), fiscalizedAtRaw.end()),
        fiscalizedAt,
        error));

    FiscalMemoryVatEventResult fmVat;
    const std::string fmVatRaw =
        "0\t0.00\t0.00\t20.00\t9.00\t100.00\t100.00\t100.00\t100.00\t1\t2\t07-01-2019 14:28\t";
    assert(parseDatecsFiscalMemoryVatEvent(
        std::vector<uint8_t>(fmVatRaw.begin(), fmVatRaw.end()), fmVat, error));
    assert(fmVat.rates[2] == "20.00" && fmVat.decimalPoint == 2);
    FiscalMemoryCounterEventResult fmReset;
    const std::string fmResetRaw = "0\t11\t25-01-2019 14:00\t";
    assert(parseDatecsFiscalMemoryCounterEvent(
        std::vector<uint8_t>(fmResetRaw.begin(), fmResetRaw.end()), fmReset, error));
    assert(fmReset.zReportNumber == 11);

    FiscalMemoryKlenEventResult fmKlen;
    const std::string fmKlenRaw =
        "0\t16158\t39\t09-07-2019 09:55\t16873\t99\t30-09-2019 16:06\t";
    assert(parseDatecsFiscalMemoryKlenEvent(
        std::vector<uint8_t>(fmKlenRaw.begin(), fmKlenRaw.end()), fmKlen, error));
    assert(fmKlen.closed && fmKlen.closeLastReceipt == 16873);
    const std::string fmKlenOpenRaw = "0\t16158\t39\t09-07-2019 09:55\t\t\t\t";
    assert(parseDatecsFiscalMemoryKlenEvent(
        std::vector<uint8_t>(fmKlenOpenRaw.begin(), fmKlenOpenRaw.end()), fmKlen, error));
    assert(!fmKlen.closed);

    FiscalMemoryZRecordResult fmZ;
    const std::string fmZRaw =
        "0\t0124\t13-11-2019 09:25\t236.00\t150.00\t0.00\t0.00\t0.00\t0.00\t0.00\t0.00\t386.00\t38.00\t80.00\t0.00\t0.00\t0.00\t0.00\t0.00\t0.00\t118.00\tE9329E8C5B0458E38E457B2F43B43D7CEE3FAD45\t17215\t3\t";
    assert(parseDatecsFiscalMemoryZRecord(
        std::vector<uint8_t>(fmZRaw.begin(), fmZRaw.end()), fmZ, error));
    assert(fmZ.number == 124 && fmZ.salesTotal == "386.00");
    assert(fmZ.hash == "E9329E8C5B0458E38E457B2F43B43D7CEE3FAD45");

    assert(buildDatecsModemInfo('s', payload, error));
    assert(std::string(payload.begin(), payload.end()) == "s\t");
    assert(buildDatecsModemInfo('i', payload, error));
    assert(buildDatecsModemInfo('M', payload, error));
    assert(std::string(payload.begin(), payload.end()) == "M\t");
    assert(!buildDatecsModemInfo('m', payload, error));
    std::string modemIdentifier;
    const std::string imeiRaw = "0\t868997036275004\t";
    assert(parseDatecsModemIdentifier(
        std::vector<uint8_t>(imeiRaw.begin(), imeiRaw.end()),
        modemIdentifier,
        error));
    assert(modemIdentifier == "868997036275004");
    const std::string invalidImeiRaw = "0\t86899703627500X\t";
    assert(!parseDatecsModemIdentifier(
        std::vector<uint8_t>(invalidImeiRaw.begin(), invalidImeiRaw.end()),
        modemIdentifier,
        error));
    ModemStatusResult modem;
    const std::string modemRaw =
        "0\t64\t868997036275004\t284013911523671\tMobiltel EAD\t";
    assert(parseDatecsModemStatus(
        std::vector<uint8_t>(modemRaw.begin(), modemRaw.end()), modem, error));
    assert(modem.signalPercent == 64);
    assert(modem.imei == "868997036275004");
    assert(modem.imsi == "284013911523671");
    const std::string invalidModemRaw =
        "0\t101\t868997036275004\t284013911523671\tMobiltel EAD\t";
    assert(!parseDatecsModemStatus(
        std::vector<uint8_t>(invalidModemRaw.begin(), invalidModemRaw.end()),
        modem,
        error));

    ReversalOpenPayload reversal{21, "12345678", 9876, 0, 428,
        "24-04-19 08:36:27", "02636571", false, "", "",
        "DT636497-0021-0010001"};
    assert(buildDatecsOpenReversal(reversal, payload, error));
    assert(std::string(payload.begin(), payload.end()) ==
        "21\t12345678\t9876\t0\t428\t24-04-19 08:36:27\t02636571\t\t\t\tDT636497-0021-0010001\t");
    reversal.invoice = true;
    reversal.originalInvoiceNumber = "1234567890";
    reversal.invoiceReason = "Returned goods";
    assert(buildDatecsOpenReversal(reversal, payload, error));
    reversal.invoiceReason.clear();
    assert(!buildDatecsOpenReversal(reversal, payload, error));
    uint32_t slipNumber = 0;
    const std::string reversalRaw = "0\t470\t";
    assert(parseDatecsSlipNumber(
        std::vector<uint8_t>(reversalRaw.begin(), reversalRaw.end()),
        slipNumber,
        error));
    assert(slipNumber == 470);

    assert(buildDatecsPcConnectionMode(-1, payload, error));
    assert(payload.empty());
    assert(buildDatecsPcConnectionMode(0, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "0\t");
    assert(!buildDatecsPcConnectionMode(2, payload, error));
    const std::string acknowledgementRaw = "0\t";
    assert(parseDatecsAcknowledge(
        std::vector<uint8_t>(acknowledgementRaw.begin(), acknowledgementRaw.end()),
        error));

    ProgrammedItemPayload programmed{4, "5", "", 2, "10"};
    assert(buildDatecsProgrammedItem(programmed, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "4\t5\t\t2\t10\t");
    programmed.pluCode = 100001;
    assert(!buildDatecsProgrammedItem(programmed, payload, error));
    const std::string programmedRaw = "0\t501\t";
    assert(parseDatecsSlipNumber(
        std::vector<uint8_t>(programmedRaw.begin(), programmedRaw.end()),
        slipNumber,
        error));
    assert(slipNumber == 501);

    assert(buildDatecsGeneralInfo(2, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "2\t");
    assert(!buildDatecsGeneralInfo(5, payload, error));
    NapConnectionResult nap;
    const std::string napRaw =
        "0\t04-03-2020 20:36:40\t21-03-2020 15:56:31\t232\t0\t0\t0\t0\t0\t0\t4574\t29-11-2019 14:20:24\t0\t5\t";
    assert(parseDatecsNapConnection(
        std::vector<uint8_t>(napRaw.begin(), napRaw.end()), nap, error));
    assert(nap.lastZReport == 232);
    assert(nap.lastReceivedSaleDocument == 4574);
    assert(nap.remainingMinutes == 5);
    const std::string invalidNapRaw =
        "0\t04-03-2020 20:36:40\t21-03-2020 15:56:31\t232\t0\t0\t-100\t0\t0\t0\t4574\t29-11-2019 14:20:24\t0\t5\t";
    assert(!parseDatecsNapConnection(
        std::vector<uint8_t>(invalidNapRaw.begin(), invalidNapRaw.end()), nap, error));
    CommandSpec napSpec;
    assert(datecsCommandSpec(71, napSpec));
    assert(std::string(napSpec.canonical) == "GetNapTransmissionStatus/GetDiagnosticInfo");

    FiscalMemoryDateReportPayload fmDate{0, "17-05-19", "17-05-19"};
    assert(buildDatecsFiscalMemoryDateReport(fmDate, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "0\t17-05-19\t17-05-19\t");
    fmDate.startDate = "18-05-19";
    assert(!buildDatecsFiscalMemoryDateReport(fmDate, payload, error));
    fmDate = FiscalMemoryDateReportPayload{1, "", ""};
    assert(buildDatecsFiscalMemoryDateReport(fmDate, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "1\t\t\t");

    FiscalMemoryNumberReportPayload fmNumber{0, 1, 2};
    assert(buildDatecsFiscalMemoryNumberReport(fmNumber, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "0\t1\t2\t");
    fmNumber = FiscalMemoryNumberReportPayload{1, 2, 1};
    assert(!buildDatecsFiscalMemoryNumberReport(fmNumber, payload, error));
    fmNumber = FiscalMemoryNumberReportPayload{1, 1, 0};
    assert(buildDatecsFiscalMemoryNumberReport(fmNumber, payload, error));

    OperatorReportPayload operatorReport{1, 2, false};
    assert(buildDatecsOperatorReport(operatorReport, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "1\t2\t0\t");
    operatorReport = OperatorReportPayload{2, 1, false};
    assert(!buildDatecsOperatorReport(operatorReport, payload, error));

    PluReportPayload pluReport{0, 1, 2};
    assert(buildDatecsPluReport(pluReport, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "0\t1\t2\t");
    pluReport = PluReportPayload{4, 1, 2};
    assert(!buildDatecsPluReport(pluReport, payload, error));
    pluReport = PluReportPayload{3, 100001, 0};
    assert(!buildDatecsPluReport(pluReport, payload, error));
    CommandSpec fmDateSpec;
    assert(datecsCommandSpec(94, fmDateSpec));
    assert(std::string(fmDateSpec.canonical) == "FiscalMemoryReport");
    CommandSpec fmNumberSpec;
    assert(datecsCommandSpec(95, fmNumberSpec));
    assert(std::string(fmNumberSpec.canonical) == "FiscalMemoryReport");
    CommandSpec modemSpec;
    assert(datecsCommandSpec(135, modemSpec));
    assert(std::string(modemSpec.canonical) == "GetDiagnosticInfo");
}

struct ExtendedDatecsReadCommandTests {
    ExtendedDatecsReadCommandTests() { testExtendedDatecsReadCommands(); }
};

static ExtendedDatecsReadCommandTests extendedDatecsReadCommandTests;
static std::vector<uint8_t>daisyResponse(){std::vector<uint8_t>v{1,0x2a,0x20,74,4,0x80,0x80,0x80,0x80,0x80,0x80,5};uint16_t s=0;for(size_t i=1;i<v.size();i++)s+=v[i];const char*h="0123456789ABCDEF";v.push_back(h[(s>>12)&15]);v.push_back(h[(s>>8)&15]);v.push_back(h[(s>>4)&15]);v.push_back(h[s&15]);v.push_back(3);return v;}
static void addWord(std::vector<uint8_t>&v,uint16_t x){for(int n:{12,8,4,0})v.push_back(uint8_t(((x>>n)&15)+0x30));}
static std::vector<uint8_t>datecsResponse(){std::vector<uint8_t>v{1};addWord(v,0x2f);v.push_back(0x20);addWord(v,74);v.push_back(4);for(int i=0;i<8;i++)v.push_back(0x80);v.push_back(5);uint16_t s=0;for(size_t i=1;i<v.size();i++)s+=v[i];addWord(v,s);v.push_back(3);return v;}
int main(){auto d=DaisyCodec::encode(0x20,48,{'1',',','0'});assert(d.size()==13&&d.front()==1&&d.back()==3);ParsedFrame p;std::string e;auto dr=daisyResponse();assert(DaisyCodec::decode(dr,p,e)&&p.command==74&&p.status.size()==6);dr[12]^=1;assert(!DaisyCodec::decode(dr,p,e)&&e=="DAISY_BCC");auto x=DatecsCodec::encode(0x20,48,{'1','\t','1'});assert(x.size()==19&&x.front()==1&&x.back()==3);auto xr=datecsResponse();assert(DatecsCodec::decode(xr,p,e)&&p.command==74&&p.status.size()==8);xr[20]^=1;assert(!DatecsCodec::decode(xr,p,e));assert(isDatecsDocumented(255)&&isDatecsDocumented(43)&&!isDatecsDocumented(34));assert(isDaisyDocumented(201)&&isDaisyDocumented(130)&&!isDaisyDocumented(34));for(size_t i=0;i<73;i++){if(i)assert(DatecsAllCommands[i]>DatecsAllCommands[i-1]);CommandSpec s{};assert(datecsCommandSpec(DatecsAllCommands[i],s)&&s.code==DatecsAllCommands[i]);}for(size_t i=0;i<88;i++){if(i)assert(DaisyAllCommands[i]>DaisyAllCommands[i-1]);CommandSpec s{};assert(daisyCommandSpec(DaisyAllCommands[i],s)&&s.code==DaisyAllCommands[i]);}CommandSpec s{};assert(datecsCommandSpec(55,s)&&s.disposition==Disposition::Excluded);assert(daisyCommandSpec(194,s)&&s.disposition==Disposition::Excluded);assert(!datecsCommandSpec(34,s)&&!daisyCommandSpec(34,s));std::vector<uint8_t> payload;OpenReceiptPayload open{1,"1","DY000600-OP01-0000001",24,false};assert(buildDaisyOpenReceipt(open,payload,e)&&std::string(payload.begin(),payload.end())=="1,1,DY000600-OP01-0000001");assert(buildDatecsOpenReceipt(open,payload,e)&&std::string(payload.begin(),payload.end())=="1\t1\tDY000600-OP01-0000001\t24\t\t");open.unp="bad";assert(!buildDaisyOpenReceipt(open,payload,e)&&e=="DAISY_OPEN_FIELDS");SaleItemPayload item{"Coffee",'B',"2.65","3.000",2,"5.00",2,"pcs"};assert(buildDaisySaleItem(item,payload,e)&&payload[7]==0xC1&&std::string(payload.begin()+8,payload.end())=="2.65*3.000,-5.00");assert(buildDatecsSaleItem(item,payload,e)&&std::string(payload.begin(),payload.end())=="Coffee\t2\t2.65\t3.000\t2\t5.00\t2\tpcs\t");PaymentPayload pay{"10.00",'P',0};assert(buildDaisyPayment(pay,payload,e)&&std::string(payload.begin(),payload.end())=="\tP10.00");assert(buildDatecsPayment(pay,payload,e)&&std::string(payload.begin(),payload.end())=="0\t10.00\t");assert(buildDaisyDailyReport(true,payload)&&std::string(payload.begin(),payload.end())=="0");assert(buildDatecsDailyReport(false,payload)&&std::string(payload.begin(),payload.end())=="X\t");CashMovementPayload cash{"50.00",true};assert(buildDaisyCashMovement(cash,payload,e)&&std::string(payload.begin(),payload.end())=="-50.00,\t$EUR");assert(buildDatecsCashMovement(cash,payload,e)&&std::string(payload.begin(),payload.end())=="1\t50.00\t");DatecsResult result;assert(parseDatecsResult({'0','\t','R','\t','5','.','0','9','\t'},result,e)&&result.errorCode==0&&result.fields.size()==2&&result.fields[0]=="R");assert(!parseDatecsResult({'1','\t'},result,e));uint32_t all=0,fiscal=0;assert(parseDaisyOpenReceiptResult({'0','0','0','0','0','2',',','0','0','0','0','0','1'},all,fiscal,e)&&all==2&&fiscal==1);ReceiptStatusResult status;assert(parseDaisyReceiptStatus({'1',',','2',',','5','.','0','0',',','2','.','0','0',',','3','.','0','0'},status,e)&&status.openState==1&&status.items==2);assert(parseDatecsReceiptStatus({'0','\t','1','\t','5','1','7','\t','2','\t','5','.','0','0','\t','2','.','0','0','\t'},status,e)&&status.openState==1&&status.receiptNumber==517&&status.items==2);SubtotalPayload subtotal{true,false,2,"10.00"};assert(buildDatecsSubtotal(subtotal,payload,e)&&std::string(payload.begin(),payload.end())=="1\t0\t2\t10.00\t");subtotal.discountType=5;assert(!buildDatecsSubtotal(subtotal,payload,e)&&e=="DATECS_SUBTOTAL_FIELDS");assert(buildDatecsLastFiscalEntry(3,payload,e)&&std::string(payload.begin(),payload.end())=="3\t");assert(!buildDatecsLastFiscalEntry(4,payload,e));TaxRatesResult taxes;std::string taxRaw="0\t1\t00.00\t20.00\t20.00\t09.00\t100.00\t100.00\t100.00\t100.00\t01-01-26\t";assert(parseDatecsTaxRates(std::vector<uint8_t>(taxRaw.begin(),taxRaw.end()),taxes,e)&&taxes.firstZReport==1&&taxes.rates[1]=="20.00");SubtotalResult subtotalResult;std::string subtotalRaw="0\t473\t35.77\t21.46\t14.31\t0.00\t0.00\t0.00\t0.00\t0.00\t0.00\t";assert(parseDatecsSubtotal(std::vector<uint8_t>(subtotalRaw.begin(),subtotalRaw.end()),subtotalResult,e)&&subtotalResult.slipNumber==473&&subtotalResult.subtotal=="35.77");std::string clock;std::string clockRaw="0\t14-05-26 11:32:13 DST\t";assert(parseDatecsDateTime(std::vector<uint8_t>(clockRaw.begin(),clockRaw.end()),clock,e)&&clock=="14-05-26 11:32:13 DST");assert(!parseDatecsDateTime({'0','\t','2','0','2','6','/','0','5','/','1','4','\t'},clock,e));LastFiscalEntryResult last;std::string lastRaw="0\t6\t0.00\t20.00\t0.00\t0.00\t0.00\t0.00\t0.00\t0.00\t08-05-26\t";assert(parseDatecsLastFiscalEntry(std::vector<uint8_t>(lastRaw.begin(),lastRaw.end()),last,e)&&last.reportNumber==6&&last.taxValues[1]=="20.00");}
