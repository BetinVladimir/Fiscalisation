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

static void testDatecsOptionalPeripheralCommands() {
    std::vector<uint8_t> payload;
    std::string error;
    for (uint16_t command : {33, 46, 63}) {
        assert(buildDatecsOptionalNoParameters(command, payload, error));
        assert(payload.empty());
    }
    assert(!buildDatecsOptionalNoParameters(48, payload, error));

    assert(buildDatecsDisplayText(35, "Test text display", payload, error));
    assert(std::string(payload.begin(), payload.end()) == "Test text display\t");
    assert(buildDatecsDisplayText(47, "Upper line", payload, error));
    assert(!buildDatecsDisplayText(47, "This display text is too long", payload, error));
    assert(!buildDatecsDisplayText(33, "Wrong command", payload, error));

    assert(buildDatecsPaperFeed(4, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "4\t");
    assert(buildDatecsPaperFeed(0, payload, error));
    assert(payload.empty());
    assert(!buildDatecsPaperFeed(100, payload, error));

    SoundPayload sound{250, 1150};
    assert(buildDatecsSound(sound, payload));
    assert(std::string(payload.begin(), payload.end()) == "250\t1150\t");

    BarcodePayload barcode{1, "12345678", 0};
    assert(buildDatecsBarcode(barcode, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "1\t12345678\t\t");
    barcode = BarcodePayload{2, "123456789012", 0};
    assert(!buildDatecsBarcode(barcode, payload, error));
    barcode = BarcodePayload{4, "https://beeloy.example", 4};
    assert(buildDatecsBarcode(barcode, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "4\thttps://beeloy.example\t4\t");
    barcode = BarcodePayload{3, "ABC", 4};
    assert(!buildDatecsBarcode(barcode, payload, error));

    assert(buildDatecsSeparator(1, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "1\t");
    assert(!buildDatecsSeparator(0, payload, error));
    assert(!buildDatecsSeparator(5, payload, error));

    assert(buildDatecsDrawer(0, payload));
    assert(std::string(payload.begin(), payload.end()) == "0\t");
    assert(buildDatecsDrawer(65535, payload));
    assert(std::string(payload.begin(), payload.end()) == "65535\t");

    const std::string ackRaw = "0\t";
    assert(parseDatecsAcknowledge(
        std::vector<uint8_t>(ackRaw.begin(), ackRaw.end()), error));
    for (uint16_t command : {33, 35, 44, 46, 47, 63, 80, 84, 92, 106}) {
        CommandSpec spec;
        assert(datecsCommandSpec(command, spec));
        assert(spec.disposition == Disposition::Optional);
    }
}

static void testDatecsOptionalInvoiceAndClients() {
    std::vector<uint8_t> payload;
    std::string error;
    InvoiceDataPayload invoice{"Seller", "Receiver", "Buyer", "Address 1", "", 0,
                               "000713391", "BG000713391"};
    assert(buildDatecsInvoiceData(invoice, payload, error));
    assert(std::string(payload.begin(), payload.end()) ==
           "Seller\tReceiver\tBuyer\tAddress 1\t\t0\t000713391\tBG000713391\t");
    invoice.taxNumber = "1234567";
    assert(!buildDatecsInvoiceData(invoice, payload, error));
    invoice.taxNumber = "000713391";
    invoice.seller = std::string(37, 'S');
    assert(!buildDatecsInvoiceData(invoice, payload, error));

    assert(buildDatecsClientInfo(payload));
    assert(std::string(payload.begin(), payload.end()) == "I\t");
    ClientRecordPayload client{1, "NAME", 1, "9503216616", "RECEIVER NAME", "",
                               "ADDR1", "ADDR2"};
    assert(buildDatecsClientProgram(client, payload, error));
    assert(std::string(payload.begin(), payload.end()) ==
           "P\t1\tNAME\t1\t9503216616\tRECEIVER NAME\t\tADDR1\tADDR2\t");
    client.index = 1001;
    assert(!buildDatecsClientProgram(client, payload, error));

    assert(buildDatecsClientDelete(1, 20, false, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "D\t1\t20\t");
    assert(buildDatecsClientDelete(0, 0, true, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "D\tA\t\t");
    assert(!buildDatecsClientDelete(20, 10, false, payload, error));
    assert(!buildDatecsClientDelete(1, 0, true, payload, error));

    assert(buildDatecsClientRead(1000, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "R\t1000\t");
    assert(!buildDatecsClientRead(0, payload, error));
    assert(buildDatecsClientSeek('F', 0, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "F\t\t");
    assert(buildDatecsClientSeek('L', 1000, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "L\t1000\t");
    assert(!buildDatecsClientSeek('N', 1, payload, error));
    assert(buildDatecsClientNext(payload));
    assert(std::string(payload.begin(), payload.end()) == "N\t");
    assert(buildDatecsClientFindByTaxNumber("9503216616", payload, error));
    assert(std::string(payload.begin(), payload.end()) == "T\t9503216616\t");
    assert(!buildDatecsClientFindByTaxNumber("ABC", payload, error));
    assert(buildDatecsClientFindUnprogrammed(false, 0, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "X\t\t");
    assert(buildDatecsClientFindUnprogrammed(true, 1000, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "x\t1000\t");
    assert(!buildDatecsClientFindUnprogrammed(false, 1001, payload, error));

    ClientDirectoryInfoResult info;
    const std::string infoRaw = "0\t1000\t17\t36\t";
    assert(parseDatecsClientInfo(std::vector<uint8_t>(infoRaw.begin(), infoRaw.end()),
                                 info, error));
    assert(info.total == 1000 && info.programmed == 17 && info.nameLength == 36);
    const std::string badInfoRaw = "0\t999\t17\t36\t";
    assert(!parseDatecsClientInfo(std::vector<uint8_t>(badInfoRaw.begin(), badInfoRaw.end()),
                                  info, error));

    ClientRecordResult record;
    const std::string recordRaw =
        "0\t1\t9503216616\t1\t\tNAME\tRECEIVER NAME\tADDR1\tADDR2\t";
    assert(parseDatecsClientRecord(
        std::vector<uint8_t>(recordRaw.begin(), recordRaw.end()), record, error));
    assert(record.index == 1 && record.taxNumberType == 1 && record.name == "NAME" &&
           record.vatNumber.empty() && record.address2 == "ADDR2");
    const std::string shortRecordRaw = "0\t2\t9202252212\t1\t\tNAME 2\t";
    assert(parseDatecsClientRecord(
        std::vector<uint8_t>(shortRecordRaw.begin(), shortRecordRaw.end()), record, error));
    assert(record.index == 2 && record.receiverName.empty() && record.address2.empty());
    const std::string badRecordRaw = "0\t1001\t9503216616\t1\t\tNAME\t";
    assert(!parseDatecsClientRecord(
        std::vector<uint8_t>(badRecordRaw.begin(), badRecordRaw.end()), record, error));

    uint16_t index = 0;
    const std::string indexRaw = "0\t1000\t";
    assert(parseDatecsClientIndex(std::vector<uint8_t>(indexRaw.begin(), indexRaw.end()),
                                  index, error) && index == 1000);
    assert(!parseDatecsClientIndex({'0', '\t', '0', '\t'}, index, error));
    assert(parseDatecsAcknowledge({'0', '\t'}, error));

    for (uint16_t command : {57, 140}) {
        CommandSpec spec;
        assert(datecsCommandSpec(command, spec));
        assert(spec.disposition == Disposition::Optional);
        assert(spec.retry == RetryClass::LookupThenDecide);
    }
    CommandSpec invoiceSpec;
    assert(datecsCommandSpec(57, invoiceSpec));
    assert(std::string(invoiceSpec.canonical) == "SetInvoiceData");
    CommandSpec clientsSpec;
    assert(datecsCommandSpec(140, clientsSpec));
    assert(std::string(clientsSpec.canonical) == "ClientDirectory");
}

struct ExtendedDatecsReadCommandTests {
    ExtendedDatecsReadCommandTests() { testExtendedDatecsReadCommands(); }
};

static ExtendedDatecsReadCommandTests extendedDatecsReadCommandTests;
struct DatecsOptionalPeripheralCommandTests {
    DatecsOptionalPeripheralCommandTests() { testDatecsOptionalPeripheralCommands(); }
};
static DatecsOptionalPeripheralCommandTests datecsOptionalPeripheralCommandTests;
struct DatecsOptionalInvoiceAndClientTests {
    DatecsOptionalInvoiceAndClientTests() { testDatecsOptionalInvoiceAndClients(); }
};
static DatecsOptionalInvoiceAndClientTests datecsOptionalInvoiceAndClientTests;

static void testDaisyFiscalMemoryReadCommands() {
    std::vector<uint8_t> payload;
    std::string error;

    assert(buildDaisyTaxRatesPeriod("010126", "310126", payload, error));
    assert(std::string(payload.begin(), payload.end()) == "010126,310126");
    assert(buildDaisyTaxRatesPeriod("", "", payload, error) && payload.empty());
    assert(!buildDaisyTaxRatesPeriod("310126", "010126", payload, error));
    assert(!buildDaisyTaxRatesPeriod("010126", "", payload, error));

    SubtotalPayload subtotalRequest{true, false, 2, "10.00"};
    assert(buildDaisySubtotal(subtotalRequest, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "10,-10.00");
    subtotalRequest.discountType = 0;
    subtotalRequest.discountValue = "10.00";
    assert(!buildDaisySubtotal(subtotalRequest, payload, error));

    assert(buildDaisyFiscalTotals('T', payload, error));
    assert(std::string(payload.begin(), payload.end()) == "T");
    assert(buildDaisyFiscalTotals('\0', payload, error) && payload.empty());
    assert(!buildDaisyFiscalTotals('X', payload, error));

    DaisyTaxRatesResult taxRates;
    const std::string taxRaw = "P,0.00,20.00,20.00,9.00,0.00,0.00,0.00,0.00,010126";
    assert(parseDaisyTaxRates(std::vector<uint8_t>(taxRaw.begin(), taxRaw.end()),
                              taxRates, error));
    assert(taxRates.found && taxRates.rates[1] == "20.00" &&
           taxRates.effectiveDate == "010126");
    assert(parseDaisyTaxRates({'F'}, taxRates, error) && !taxRates.found);
    assert(!parseDaisyTaxRates({'P', ',', '2', '0', '.', '0', '0'}, taxRates, error));

    DaisyFiscalTotalsResult subtotal;
    std::string subtotalAmount;
    const std::string subtotalRaw =
        "35.77,0.00,35.77,0.00,0.00,0.00,0.00,0.00,0.00";
    assert(parseDaisySubtotal(
        std::vector<uint8_t>(subtotalRaw.begin(), subtotalRaw.end()), subtotal,
        subtotalAmount, error));
    assert(subtotalAmount == "35.77" && subtotal.sales[1] == "35.77");

    std::string clock;
    const std::string clockRaw = "14.05.26 11:32:13";
    assert(parseDaisyDateTime(std::vector<uint8_t>(clockRaw.begin(), clockRaw.end()),
                              clock, error));
    assert(!parseDaisyDateTime({'1', '4', '-', '0', '5', '-', '2', '6'}, clock, error));

    DaisyLastFiscalRecordResult last;
    const std::string lastRaw =
        "6,0.00,20.00,0.00,0.00,0.00,0.00,0.00,0.00,"
        "0.00,1.00,0.00,0.00,0.00,0.00,0.00,0.00,080526";
    assert(parseDaisyLastFiscalRecord(
        std::vector<uint8_t>(lastRaw.begin(), lastRaw.end()), last, error));
    assert(last.reportNumber == 6 && last.sales[1] == "20.00" &&
           last.reversals[1] == "1.00" && last.date == "080526");

    DaisyFiscalTotalsResult current;
    const std::string currentRaw =
        "0.00,20.00,0.00,0.00,0.00,0.00,0.00,0.00,"
        "0.00,1.00,0.00,0.00,0.00,0.00,0.00,0.00";
    assert(parseDaisyCurrentFiscalTotals(
        std::vector<uint8_t>(currentRaw.begin(), currentRaw.end()), current, error));
    assert(current.sales[1] == "20.00" && current.reversals[1] == "1.00");

    DaisyFiscalizationResult fiscalization;
    const std::string fiscalizationRaw = "010126,4,9";
    assert(parseDaisyFiscalization(
        std::vector<uint8_t>(fiscalizationRaw.begin(), fiscalizationRaw.end()),
        fiscalization, error));
    assert(fiscalization.date == "010126" && fiscalization.lastBgnReport == 4 &&
           fiscalization.lastReport == 9);
    const std::string badFiscalizationRaw = "010126,10,9";
    assert(!parseDaisyFiscalization(
        std::vector<uint8_t>(badFiscalizationRaw.begin(), badFiscalizationRaw.end()),
        fiscalization, error));

    DaisyFreeFiscalRecordsResult freeRecords;
    assert(parseDaisyFreeFiscalRecords({'3', '6', '5', '0', ',', '3', '6', '5', '0'},
                                        freeRecords, error));
    assert(freeRecords.logical == 3650 && freeRecords.physical == 3650);
    assert(!parseDaisyFreeFiscalRecords({'3', ',', '2'}, freeRecords, error));
}

static void testDaisyDeviceAndCurrencyInfoCommands() {
    std::vector<uint8_t> payload;
    std::string error;
    assert(buildDaisyDiagnostic(true, payload));
    assert(std::string(payload.begin(), payload.end()) == "1");
    assert(buildDaisyDiagnostic(false, payload) && payload.empty());
    assert(buildDaisyErrorDescription(127, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "127");
    assert(!buildDaisyErrorDescription(0, payload, error));
    assert(buildDaisyCurrencyTransition('I', 0, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "I");
    assert(buildDaisyCurrencyTransition('R', 2, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "R2");
    assert(!buildDaisyCurrencyTransition('I', 1, payload, error));
    assert(!buildDaisyCurrencyTransition('R', 5, payload, error));

    DaisyDiagnosticResult diagnostic;
    const std::string diagnosticRaw =
        "01.02.03 14-05-2026 11:32,12AF,0101,6,DY123456,12345678";
    assert(parseDaisyDiagnostic(
        std::vector<uint8_t>(diagnosticRaw.begin(), diagnosticRaw.end()), diagnostic,
        error));
    assert(diagnostic.firmwareRevision == "01.02.03" && diagnostic.country == 6 &&
           diagnostic.fiscalMemoryNumber == "12345678");
    const std::string badDiagnostic =
        "short 14/05/2026 11:32,12AF,0101,6,DY123456,12345678";
    assert(!parseDaisyDiagnostic(
        std::vector<uint8_t>(badDiagnostic.begin(), badDiagnostic.end()), diagnostic,
        error));

    std::array<std::string, 8> rates;
    const std::string ratesRaw = "0.00,20.00,20.00,9.00,0.00,0.00,0.00,0.00";
    assert(parseDaisyCurrentTaxRates(
        std::vector<uint8_t>(ratesRaw.begin(), ratesRaw.end()), rates, error));
    assert(rates[1] == "20.00");
    assert(!parseDaisyCurrentTaxRates({'2', '0', '.', '0', '0'}, rates, error));

    std::string taxNumber;
    bool fiscalized = false;
    assert(parseDaisyTaxNumber({'1','2','3','4','5','6','7','8','9'}, taxNumber,
                               fiscalized, error));
    assert(fiscalized && taxNumber == "123456789");
    assert(parseDaisyTaxNumber({'-','-','-','-','-','-','-','-','-'}, taxNumber,
                               fiscalized, error) && !fiscalized);
    assert(!parseDaisyTaxNumber({'1','2','-','4','5','6','7','8','9'}, taxNumber,
                                fiscalized, error));

    DaisyReceiptInfoResult receipt;
    const std::string receiptRaw =
        "1,0.00,20.00,0.00,0.00,0.00,0.00,0.00,0.00,1,0000000042";
    assert(parseDaisyReceiptInfo(
        std::vector<uint8_t>(receiptRaw.begin(), receiptRaw.end()), receipt, error));
    assert(receipt.canVoid && receipt.invoice && receipt.nextInvoiceNumber == "0000000042");
    const std::string noInvoiceRaw =
        "0,0.00,20.00,0.00,0.00,0.00,0.00,0.00,0.00,0";
    assert(parseDaisyReceiptInfo(
        std::vector<uint8_t>(noInvoiceRaw.begin(), noInvoiceRaw.end()), receipt, error));
    assert(!receipt.invoice && receipt.nextInvoiceNumber.empty());

    uint32_t document = 0;
    assert(parseDaisyLastDocumentNumber({'4','2'}, document, error) && document == 42);
    assert(!parseDaisyLastDocumentNumber({'X'}, document, error));

    bool allSent = false;
    std::string deviceError;
    assert(parseDaisyFirstUnsentReceipt({'4','3'}, document, allSent, deviceError,
                                        error));
    assert(document == 43 && !allSent && deviceError.empty());
    assert(parseDaisyFirstUnsentReceipt({'-','-','-'}, document, allSent, deviceError,
                                        error) && allSent);
    const std::string unsentError = "error17";
    assert(parseDaisyFirstUnsentReceipt(
        std::vector<uint8_t>(unsentError.begin(), unsentError.end()), document, allSent,
        deviceError, error));
    assert(deviceError == "error17");

    DaisyFirmwareResult firmware;
    const std::string firmwareRaw = "10,FD-CERT-2026,NRA-CERT-2026";
    assert(parseDaisyFirmware(
        std::vector<uint8_t>(firmwareRaw.begin(), firmwareRaw.end()), firmware, error));
    assert(firmware.eShopSupported && !firmware.eShopActive &&
           firmware.confirmedCertificate == "NRA-CERT-2026");
    const std::string badFirmwareRaw = "12,FD-CERT,NRA-CERT";
    assert(!parseDaisyFirmware(
        std::vector<uint8_t>(badFirmwareRaw.begin(), badFirmwareRaw.end()), firmware,
        error));

    DaisyErrorDescriptionResult description;
    const std::string descriptionRaw = "17,Invalid fiscal memory state";
    assert(parseDaisyErrorDescription(
        std::vector<uint8_t>(descriptionRaw.begin(), descriptionRaw.end()), 17,
        description, error));
    assert(description.code == 17 && description.description == "Invalid fiscal memory state");
    assert(!parseDaisyErrorDescription(
        std::vector<uint8_t>(descriptionRaw.begin(), descriptionRaw.end()), 18,
        description, error));

    DaisyCurrencyTransitionResult transition;
    const std::string transitionRaw =
        "4,0,010126,XXXXXX,XXXXXX,XXXXXX,1.95583,080826";
    assert(parseDaisyCurrencyTransitionInfo(
        std::vector<uint8_t>(transitionRaw.begin(), transitionRaw.end()), transition,
        error));
    assert(transition.zone == 4 && !transition.dailyReportRequired &&
           transition.currencyRate == "1.95583");
    const std::string badTransitionRaw =
        "5,0,010126,XXXXXX,XXXXXX,XXXXXX,1.95583,080826";
    assert(!parseDaisyCurrencyTransitionInfo(
        std::vector<uint8_t>(badTransitionRaw.begin(), badTransitionRaw.end()), transition,
        error));

    DaisyCurrencyTransitionDateResult transitionDate;
    const std::string dateTwoRaw = "010126,1.95583";
    assert(parseDaisyCurrencyTransitionDate(
        std::vector<uint8_t>(dateTwoRaw.begin(), dateTwoRaw.end()), 2,
        transitionDate, error));
    assert(transitionDate.date == "010126" && transitionDate.currencyRate == "1.95583");
    assert(parseDaisyCurrencyTransitionDate({'X','X','X','X','X','X'}, 4,
                                             transitionDate, error));
    assert(!parseDaisyCurrencyTransitionDate(
        std::vector<uint8_t>(dateTwoRaw.begin(), dateTwoRaw.end()), 4,
        transitionDate, error));
}

static void testDaisySaleAndReportCommands() {
    std::vector<uint8_t> payload;
    std::string error;
    SaleItemPayload displayed{"Coffee", 'B', "2.65", "2.000", 0, "", 0, ""};
    assert(buildDaisySaleAndDisplay(displayed, payload, error));
    assert(payload[7] == 0xC1 &&
           std::string(payload.begin() + 8, payload.end()) == "2.65*2.000");

    DaisyProgrammedItemPayload plu{false, "1234567890123", "2.000", "3.50", 2,
                                   "5.00"};
    assert(buildDaisyProgrammedItem(plu, payload, error));
    assert(std::string(payload.begin(), payload.end()) ==
           "1234567890123*2.000@3.50,-5.00");
    plu.correction = true;
    assert(!buildDaisyProgrammedItem(plu, payload, error));
    plu.discountType = 0;
    plu.discountValue.clear();
    assert(buildDaisyProgrammedItem(plu, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "-1234567890123*2.000@3.50");
    plu.pluNumber = "bad/value";
    assert(!buildDaisyProgrammedItem(plu, payload, error));

    assert(buildDaisyFiscalMemoryNumberReport(1, 3650, true, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "1,3650,PAY");
    assert(!buildDaisyFiscalMemoryNumberReport(20, 10, false, payload, error));
    assert(buildDaisyFiscalMemoryDateReport("010126", "310126", false, payload,
                                            error));
    assert(std::string(payload.begin(), payload.end()) == "010126,310126");
    assert(buildDaisyFiscalMemoryDateReport("010126", "310126", true, payload,
                                            error));
    assert(std::string(payload.begin(), payload.end()) == "010126,310126,PAY");
    assert(!buildDaisyFiscalMemoryDateReport("310126", "010126", false, payload,
                                             error));

    assert(buildDaisyDailyReportOption(0, true, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "0N");
    assert(buildDaisyDailyReportOption(9, false, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "9");
    assert(!buildDaisyDailyReportOption(4, false, payload, error));

    assert(buildDaisyPluReport('\0', payload, error) && payload.empty());
    assert(buildDaisyPluReport('Z', payload, error));
    assert(std::string(payload.begin(), payload.end()) == "Z");
    assert(!buildDaisyPluReport('X', payload, error));
    assert(buildDaisyDepartmentReport('X', payload, error));
    assert(std::string(payload.begin(), payload.end()) == "X");
    assert(!buildDaisyDepartmentReport('Q', payload, error));

    for (uint16_t command : {105, 130, 166}) {
        assert(buildDaisySupportedNoData(command, payload, error));
        assert(payload.empty());
    }
    assert(!buildDaisySupportedNoData(104, payload, error));

    bool passed = false;
    assert(parseDaisyPassFail({'P'}, passed, error) && passed);
    assert(parseDaisyPassFail({'F'}, passed, error) && !passed);
    assert(!parseDaisyPassFail({'O', 'K'}, passed, error));

    DaisyDailyReportResult report;
    const std::string reportRaw =
        "42,0.00,20.00,0.00,0.00,0.00,0.00,0.00,0.00,"
        "0.00,1.00,0.00,0.00,0.00,0.00,0.00,0.00";
    assert(parseDaisyDailyReport(
        std::vector<uint8_t>(reportRaw.begin(), reportRaw.end()), report, error));
    assert(report.closure == 42 && report.totals.sales[1] == "20.00" &&
           report.totals.reversals[1] == "1.00");
    assert(!parseDaisyDailyReport({'4', '2', ',', '0'}, report, error));
}

static void testDaisyRemainingSupportedCommands() {
    std::vector<uint8_t> payload;
    std::string error;

    assert(buildDaisyCurrentDay(true, payload));
    assert(std::string(payload.begin(), payload.end()) == "A");
    assert(buildDaisyCurrentDay(false, payload) && payload.empty());
    DaisyCurrentDayResult day;
    const std::string dayRaw =
        "10.00,1.00,2.00,3.00,4.00,5.00,42,1234,0000000123";
    assert(parseDaisyCurrentDay(
        std::vector<uint8_t>(dayRaw.begin(), dayRaw.end()), day, error));
    assert(day.payments.size() == 6 && day.payments[5] == "5.00" &&
           day.lastZReport == 42 && day.nextDocument == 1234 &&
           day.nextInvoice == "0000000123");
    assert(!parseDaisyCurrentDay({'1', ',', '2'}, day, error));

    assert(buildDaisyOperatorInfo(7, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "7");
    assert(!buildDaisyOperatorInfo(0, payload, error));
    DaisyOperatorResult op;
    const std::string opRaw =
        "12,10;25.00,2;1.00,1;0.50,3;4.00,Ada\t"
        "1,0,1,2;3.00,1;1.00,4;5.00";
    assert(parseDaisyOperatorInfo(
        std::vector<uint8_t>(opRaw.begin(), opRaw.end()), op, error));
    assert(op.receipts == 12 && op.sales.count == 10 &&
           op.refundEnabled[0] && !op.refundEnabled[1] &&
           op.refunds[2].amount == "5.00");
    assert(!parseDaisyOperatorInfo({'1', ',', 'x'}, op, error));

    assert(buildDaisyFmInformationNumber(10, 4, 20, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "10,4,20");
    assert(buildDaisyFmInformationNumber(10, 3, 0, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "10,3");
    assert(!buildDaisyFmInformationNumber(10, 2, 20, payload, error));
    assert(buildDaisyFmInformationDate("010126", 6, "310126", payload, error));
    assert(std::string(payload.begin(), payload.end()) == "010126,6,310126");
    assert(!buildDaisyFmInformationDate("310126", 4, "010126", payload, error));
    DaisyFmQueryResult fm;
    const std::string fmRaw =
        "P,0.00,20.00,0.00,0.00,0.00,0.00,0.00,0.00,"
        "0.00,1.00,0.00,0.00,0.00,0.00,0.00,0.00";
    assert(parseDaisyFmInformation(
        std::vector<uint8_t>(fmRaw.begin(), fmRaw.end()), 4, fm, error));
    assert(fm.state == DaisyFmQueryState::Valid && fm.hasReversals &&
           fm.values[1] == "20.00" && fm.reversals[1] == "1.00");
    assert(parseDaisyFmInformation({'F'}, 4, fm, error) &&
           fm.state == DaisyFmQueryState::InvalidChecksum);
    assert(parseDaisyFmInformation({'E'}, 4, fm, error) &&
           fm.state == DaisyFmQueryState::NoData);
    assert(!parseDaisyFmInformation({'P', ',', '0'}, 4, fm, error));

    assert(buildDaisyQrDocument(10, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "9");
    assert(buildDaisyQrDocument(0, payload, error) && payload.empty());
    DaisyQrDocumentResult qr;
    const std::string qrRaw =
        "PS,14,36940099*000123*2026-08-09*09:19:02*2.50";
    assert(parseDaisyQrDocument(
        std::vector<uint8_t>(qrRaw.begin(), qrRaw.end()), qr, error));
    assert(qr.found && qr.overallType == 'S' && qr.hasQr &&
           qr.documentType == 14);
    assert(parseDaisyQrDocument({'P','F',',','X','X','X','X','X'}, qr, error) &&
           qr.found && !qr.hasQr);
    assert(parseDaisyQrDocument({'F'}, qr, error) && !qr.found);

    assert(buildDaisyIssuedDocument(246, true, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "246,S");
    assert(buildDaisyIssuedDocument(0, true, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "S");
    DaisyIssuedDocumentResult document;
    const std::string documentRaw =
        "P000246\t04.05.2023 08:49:12\t65\t0\t10\t1\t"
        "DY999636-OP01-1234567\t000000,SHA1:"
        "70BCE-5EAC9-4CEFE-64231\n73FF1-A8D54-B193C-885E8";
    assert(parseDaisyIssuedDocument(
        std::vector<uint8_t>(documentRaw.begin(), documentRaw.end()), true,
        document, error));
    assert(document.found && document.number == 246 && document.description == 65 &&
           document.multiplier100 && document.sha1.size() == 47);
    assert(!parseDaisyIssuedDocument(
        std::vector<uint8_t>(documentRaw.begin(), documentRaw.end()), false,
        document, error));
    assert(parseDaisyIssuedDocument({'F'}, false, document, error) && !document.found);

    for (uint16_t command : {128, 153}) {
        assert(buildDaisySupportedNoData(command, payload, error));
        assert(payload.empty());
    }
    DaisyConstantsResult constants;
    const std::string constantsRaw =
        "384,120,12,8,,,A,8,48,48,32,8,8,0,13,20,100000,1,1,0,20,16,0,0,0,0";
    assert(parseDaisyConstants(
        std::vector<uint8_t>(constantsRaw.begin(), constantsRaw.end()), constants,
        error));
    assert(constants.logoWidth == 384 && constants.paymentCount == 12 &&
           constants.taxGroupCount == 8 && constants.pluCount == 100000 &&
           constants.operatorCount == 20);
    assert(!parseDaisyConstants({'1', ',', '2'}, constants, error));

    DaisyDepartmentSalePayload department{false, 12, "5.50", "2.000", 2, "10.00"};
    assert(buildDaisyDepartmentSale(department, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "12@5.50*2.000,-10.00");
    department.correction = true;
    assert(!buildDaisyDepartmentSale(department, payload, error));
    department.discountType = 0;
    department.discountValue.clear();
    assert(buildDaisyDepartmentSale(department, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "-12@5.50*2.000");

    DaisyTextReportLine line;
    const std::vector<uint8_t> reportLine = {
        0x1a,'0','0','0','0','4','2','\t','N','\t','T','o','t','a','l','\r','\n'};
    assert(parseDaisyTextReportLine(reportLine, line, error));
    assert(!line.end && line.lineNumber == 42 && line.font == 'N' &&
           line.text == "Total");
    assert(parseDaisyTextReportLine({0x1a,'\t','E','\r','\n'}, line, error) &&
           line.end && line.font == 'E');
    assert(!parseDaisyTextReportLine({0x1a,'0','0','0','0','4','2','\t','X','\r','\n'},
                                     line, error));
}

static void testDaisyOptionalCommands() {
    std::vector<uint8_t> payload;
    std::string error;
    for (uint16_t command : {33, 63, 71, 176}) {
        assert(buildDaisyOptionalNoData(command, payload, error));
        assert(payload.empty());
    }
    assert(!buildDaisyOptionalNoData(34, payload, error));
    assert(parseDaisyNoData({}, error));
    assert(!parseDaisyNoData({'P'}, error));

    assert(buildDaisyDisplayText(35, "Customer total", payload, error));
    assert(std::string(payload.begin(), payload.end()) == "Customer total");
    assert(buildDaisyDisplayText(47, "Welcome", payload, error));
    assert(!buildDaisyDisplayText(46, "wrong overload", payload, error));
    assert(!buildDaisyDisplayText(35, "bad\ntext", payload, error));
    assert(buildDaisyDisplayLine(2, "EUR 2.50", payload, error));
    assert(std::string(payload.begin(), payload.end()) == "2,EUR 2.50");
    assert(!buildDaisyDisplayLine(0, "EUR", payload, error));

    assert(buildDaisyPaperFeed(0, payload, error) && payload.empty());
    assert(buildDaisyPaperFeed(12, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "12");
    assert(!buildDaisyPaperFeed(1000, payload, error));
    assert(buildDaisyPaperCut(0, payload, error) && payload.empty());
    assert(buildDaisyPaperCut(2, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "2");
    assert(!buildDaisyPaperCut(3, payload, error));
    bool passed = false;
    assert(parseDaisyPassFail({'P'}, passed, error) && passed);

    DaisyInvoiceCustomerPayload invoice{
        "123456789", "BG123456789", "Seller", "Ada", "Analytical Engines",
        {"1 Main Street", "Sofia"}};
    assert(buildDaisyInvoiceCustomer(invoice, payload, error));
    assert(std::string(payload.begin(), payload.end()) ==
           "123456789\tBG123456789\tSeller\tAda\tAnalytical Engines\t"
           "1 Main Street\tSofia");
    invoice.identificationNumber = "bad\tvalue";
    assert(!buildDaisyInvoiceCustomer(invoice, payload, error));

    DaisyBarcodePayload barcode{2, "123456789012", 'R', 2, 15, false, true};
    assert(buildDaisyBarcode(barcode, payload, error));
    assert(std::string(payload.begin(), payload.end()) ==
           "2,123456789012\tR,2,15,0");
    barcode.data = "123";
    assert(!buildDaisyBarcode(barcode, payload, error));
    barcode = DaisyBarcodePayload{12, "A12D", 'C', 0, 0, true, false};
    assert(buildDaisyBarcode(barcode, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "12,A12D");
    barcode.data = "E123";
    assert(!buildDaisyBarcode(barcode, payload, error));

    assert(buildDaisyCustomerQr('Q', {"customer-reference"}, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "Q,customer-reference");
    assert(buildDaisyCustomerQr('P', {"DNO", "DT", "TAM"}, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "P,DNO,DT,TAM");
    assert(buildDaisyCustomerQr('R', {}, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "R");
    assert(!buildDaisyCustomerQr('P', {"UNKNOWN"}, payload, error));
    std::vector<std::string> qrFields;
    assert(parseDaisyCustomerQr({}, 'Q', passed, qrFields, error) && passed);
    assert(parseDaisyCustomerQr({'P'}, 'P', passed, qrFields, error) && passed);
    assert(parseDaisyCustomerQr({'F','2'}, 'P', passed, qrFields, error) &&
           !passed && qrFields[0] == "2");
    const std::string qrTemplate = "DNO,DT,TAM";
    assert(parseDaisyCustomerQr(
        std::vector<uint8_t>(qrTemplate.begin(), qrTemplate.end()), 'R', passed,
        qrFields, error));
    assert(qrFields.size() == 3 && qrFields[2] == "TAM");

    assert(buildDaisyDrawer(0, payload, error) && payload.empty());
    assert(buildDaisyDrawer(50, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "50");
    assert(!buildDaisyDrawer(49, payload, error));

    assert(buildDaisyDisplayConfiguration('R', "2", payload, error));
    assert(std::string(payload.begin(), payload.end()) == "R,2");
    assert(buildDaisyDisplayConfiguration('0', "I1B2C3D4E5F60718293A4,100",
                                          payload, error));
    assert(std::string(payload.begin(), payload.end()) ==
           "0,I1B2C3D4E5F60718293A4,100");
    assert(buildDaisyDisplayConfiguration('9', "D", payload, error));
    assert(!buildDaisyDisplayConfiguration('0', "Ixyz", payload, error));

    DaisyCustomerRecord customer{"123456789", "BG123456789", "Ada", "Engine Ltd",
                                 "Sofia 1", "Floor 2"};
    assert(buildDaisyCustomerProgram(customer, payload, error));
    assert(std::string(payload.begin(), payload.end()) ==
           "P123456789\tBG123456789\tAda\tEngine Ltd\tSofia 1\tFloor 2");
    assert(buildDaisyCustomerDelete("", true, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "DA");
    assert(buildDaisyCustomerDelete("123456789", false, payload, error));
    assert(std::string(payload.begin(), payload.end()) == "D123456789");
    assert(buildDaisyCustomerRead("123456789", payload, error));
    assert(std::string(payload.begin(), payload.end()) == "R123456789");
    assert(buildDaisyCustomerIteration('F', payload, error));
    assert(!buildDaisyCustomerIteration('R', payload, error));
    const std::string customerRaw =
        "P,P123456789\tBG123456789\tAda\tEngine Ltd\tSofia 1\tFloor 2";
    bool found = false;
    DaisyCustomerRecord readCustomer;
    assert(parseDaisyCustomerRecord(
        std::vector<uint8_t>(customerRaw.begin(), customerRaw.end()), readCustomer,
        found, error));
    assert(found && readCustomer.eik == "123456789" &&
           readCustomer.address2 == "Floor 2");
    assert(parseDaisyCustomerRecord({'F'}, readCustomer, found, error) && !found);

    assert(buildDaisyBattery(payload));
    assert(std::string(payload.begin(), payload.end()) == "15");
    DaisyBatteryResult battery;
    assert(parseDaisyBattery({'3','8','5','0',' ','9','2'}, battery, error));
    assert(battery.millivolts == 3850 && battery.capacityPercent == 92);
    assert(!parseDaisyBattery({'4','0','0','1',' ','5','0'}, battery, error));
}

struct DaisyFiscalMemoryReadCommandTests {
    DaisyFiscalMemoryReadCommandTests() {
        testDaisyFiscalMemoryReadCommands();
        testDaisyDeviceAndCurrencyInfoCommands();
        testDaisySaleAndReportCommands();
        testDaisyRemainingSupportedCommands();
        testDaisyOptionalCommands();
    }
};
static DaisyFiscalMemoryReadCommandTests daisyFiscalMemoryReadCommandTests;
static std::vector<uint8_t>daisyResponse(){std::vector<uint8_t>v{1,0x2a,0x20,74,4,0x80,0x80,0x80,0x80,0x80,0x80,5};uint16_t s=0;for(size_t i=1;i<v.size();i++)s+=v[i];const char*h="0123456789ABCDEF";v.push_back(h[(s>>12)&15]);v.push_back(h[(s>>8)&15]);v.push_back(h[(s>>4)&15]);v.push_back(h[s&15]);v.push_back(3);return v;}
static void addWord(std::vector<uint8_t>&v,uint16_t x){for(int n:{12,8,4,0})v.push_back(uint8_t(((x>>n)&15)+0x30));}
static std::vector<uint8_t>datecsResponse(){std::vector<uint8_t>v{1};addWord(v,0x2f);v.push_back(0x20);addWord(v,74);v.push_back(4);for(int i=0;i<8;i++)v.push_back(0x80);v.push_back(5);uint16_t s=0;for(size_t i=1;i<v.size();i++)s+=v[i];addWord(v,s);v.push_back(3);return v;}
int main(){auto d=DaisyCodec::encode(0x20,48,{'1',',','0'});assert(d.size()==13&&d.front()==1&&d.back()==3);ParsedFrame p;std::string e;auto dr=daisyResponse();assert(DaisyCodec::decode(dr,p,e)&&p.command==74&&p.status.size()==6);dr[12]^=1;assert(!DaisyCodec::decode(dr,p,e)&&e=="DAISY_BCC");auto x=DatecsCodec::encode(0x20,48,{'1','\t','1'});assert(x.size()==19&&x.front()==1&&x.back()==3);auto xr=datecsResponse();assert(DatecsCodec::decode(xr,p,e)&&p.command==74&&p.status.size()==8);xr[20]^=1;assert(!DatecsCodec::decode(xr,p,e));assert(isDatecsDocumented(255)&&isDatecsDocumented(43)&&!isDatecsDocumented(34));assert(isDaisyDocumented(201)&&isDaisyDocumented(130)&&!isDaisyDocumented(34));for(size_t i=0;i<73;i++){if(i)assert(DatecsAllCommands[i]>DatecsAllCommands[i-1]);CommandSpec s{};assert(datecsCommandSpec(DatecsAllCommands[i],s)&&s.code==DatecsAllCommands[i]);}for(size_t i=0;i<88;i++){if(i)assert(DaisyAllCommands[i]>DaisyAllCommands[i-1]);CommandSpec s{};assert(daisyCommandSpec(DaisyAllCommands[i],s)&&s.code==DaisyAllCommands[i]);}CommandSpec s{};assert(datecsCommandSpec(55,s)&&s.disposition==Disposition::Excluded);assert(daisyCommandSpec(194,s)&&s.disposition==Disposition::Excluded);assert(!datecsCommandSpec(34,s)&&!daisyCommandSpec(34,s));std::vector<uint8_t> payload;OpenReceiptPayload open{1,"1","DY000600-OP01-0000001",24,false};assert(buildDaisyOpenReceipt(open,payload,e)&&std::string(payload.begin(),payload.end())=="1,1,DY000600-OP01-0000001");assert(buildDatecsOpenReceipt(open,payload,e)&&std::string(payload.begin(),payload.end())=="1\t1\tDY000600-OP01-0000001\t24\t\t");open.unp="bad";assert(!buildDaisyOpenReceipt(open,payload,e)&&e=="DAISY_OPEN_FIELDS");SaleItemPayload item{"Coffee",'B',"2.65","3.000",2,"5.00",2,"pcs"};assert(buildDaisySaleItem(item,payload,e)&&payload[7]==0xC1&&std::string(payload.begin()+8,payload.end())=="2.65*3.000,-5.00");assert(buildDatecsSaleItem(item,payload,e)&&std::string(payload.begin(),payload.end())=="Coffee\t2\t2.65\t3.000\t2\t5.00\t2\tpcs\t");PaymentPayload pay{"10.00",'P',0};assert(buildDaisyPayment(pay,payload,e)&&std::string(payload.begin(),payload.end())=="\tP10.00");assert(buildDatecsPayment(pay,payload,e)&&std::string(payload.begin(),payload.end())=="0\t10.00\t");assert(buildDaisyDailyReport(true,payload)&&std::string(payload.begin(),payload.end())=="0");assert(buildDatecsDailyReport(false,payload)&&std::string(payload.begin(),payload.end())=="X\t");CashMovementPayload cash{"50.00",true};assert(buildDaisyCashMovement(cash,payload,e)&&std::string(payload.begin(),payload.end())=="-50.00,\t$EUR");assert(buildDatecsCashMovement(cash,payload,e)&&std::string(payload.begin(),payload.end())=="1\t50.00\t");DatecsResult result;assert(parseDatecsResult({'0','\t','R','\t','5','.','0','9','\t'},result,e)&&result.errorCode==0&&result.fields.size()==2&&result.fields[0]=="R");assert(!parseDatecsResult({'1','\t'},result,e));uint32_t all=0,fiscal=0;assert(parseDaisyOpenReceiptResult({'0','0','0','0','0','2',',','0','0','0','0','0','1'},all,fiscal,e)&&all==2&&fiscal==1);ReceiptStatusResult status;assert(parseDaisyReceiptStatus({'1',',','2',',','5','.','0','0',',','2','.','0','0',',','3','.','0','0'},status,e)&&status.openState==1&&status.items==2);assert(parseDatecsReceiptStatus({'0','\t','1','\t','5','1','7','\t','2','\t','5','.','0','0','\t','2','.','0','0','\t'},status,e)&&status.openState==1&&status.receiptNumber==517&&status.items==2);SubtotalPayload subtotal{true,false,2,"10.00"};assert(buildDatecsSubtotal(subtotal,payload,e)&&std::string(payload.begin(),payload.end())=="1\t0\t2\t10.00\t");subtotal.discountType=5;assert(!buildDatecsSubtotal(subtotal,payload,e)&&e=="DATECS_SUBTOTAL_FIELDS");assert(buildDatecsLastFiscalEntry(3,payload,e)&&std::string(payload.begin(),payload.end())=="3\t");assert(!buildDatecsLastFiscalEntry(4,payload,e));TaxRatesResult taxes;std::string taxRaw="0\t1\t00.00\t20.00\t20.00\t09.00\t100.00\t100.00\t100.00\t100.00\t01-01-26\t";assert(parseDatecsTaxRates(std::vector<uint8_t>(taxRaw.begin(),taxRaw.end()),taxes,e)&&taxes.firstZReport==1&&taxes.rates[1]=="20.00");SubtotalResult subtotalResult;std::string subtotalRaw="0\t473\t35.77\t21.46\t14.31\t0.00\t0.00\t0.00\t0.00\t0.00\t0.00\t";assert(parseDatecsSubtotal(std::vector<uint8_t>(subtotalRaw.begin(),subtotalRaw.end()),subtotalResult,e)&&subtotalResult.slipNumber==473&&subtotalResult.subtotal=="35.77");std::string clock;std::string clockRaw="0\t14-05-26 11:32:13 DST\t";assert(parseDatecsDateTime(std::vector<uint8_t>(clockRaw.begin(),clockRaw.end()),clock,e)&&clock=="14-05-26 11:32:13 DST");assert(!parseDatecsDateTime({'0','\t','2','0','2','6','/','0','5','/','1','4','\t'},clock,e));LastFiscalEntryResult last;std::string lastRaw="0\t6\t0.00\t20.00\t0.00\t0.00\t0.00\t0.00\t0.00\t0.00\t08-05-26\t";assert(parseDatecsLastFiscalEntry(std::vector<uint8_t>(lastRaw.begin(),lastRaw.end()),last,e)&&last.reportNumber==6&&last.taxValues[1]=="20.00");}
