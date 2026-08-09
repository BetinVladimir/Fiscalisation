#!/usr/bin/env ruby
require "json"

root = File.expand_path("..", __dir__)
source = File.read(File.join(root, "IoT/protocol-abstraction/src/AllCommands.cpp"))
registry = File.read(File.join(root, "IoT/protocol-abstraction/src/CommandRegistry.cpp"))
payload = File.read(File.join(root, "IoT/protocol-abstraction/src/CommandPayload.cpp"))
coverage = JSON.parse(File.read(File.join(root, "contracts/driver-semantic-coverage.json")))

def command_array(source, vendor, expected)
  match = source.match(/const uint16_t #{vendor}AllCommands\[#{expected}\]=\{([^}]*)\}/)
  abort "missing #{vendor} command inventory" unless match
  values = match[1].split(",").map(&:to_i)
  abort "#{vendor}: expected #{expected}, got #{values.length}" unless values.length == expected
  abort "#{vendor}: command inventory must be sorted and unique" unless values == values.sort.uniq
  values
end

daisy = command_array(source, "Daisy", 88)
datecs = command_array(source, "Datecs", 73)
abort "registry functions missing" unless registry.include?("daisyCommandSpec") && registry.include?("datecsCommandSpec")

datecs_disposition = registry[registry.index("Disposition datecsDisposition")..]
datecs_dispositions = %w[optional privileged excluded].to_h do |name|
  match = datecs_disposition.match(/const uint16_t #{name}\[\]=\{([^}]*)\}/)
  abort "missing Datecs #{name} disposition inventory" unless match
  [name, match[1].split(",").map(&:to_i)]
end
classified_non_supported = datecs_dispositions.values.flatten
abort "Datecs disposition inventories overlap" unless classified_non_supported.length == classified_non_supported.uniq.length
datecs_supported = datecs - classified_non_supported

daisy_disposition = registry[registry.index("Disposition daisyDisposition")...registry.index("RetryClass datecsRetry")]
daisy_dispositions = %w[optional privileged excluded].to_h do |name|
  match = daisy_disposition.match(/const uint16_t #{name}\[\]=\{([^}]*)\}/)
  abort "missing Daisy #{name} disposition inventory" unless match
  [name, match[1].split(",").map(&:to_i)]
end
daisy_non_supported = daisy_dispositions.values.flatten
abort "Daisy disposition inventories overlap" unless daisy_non_supported.length == daisy_non_supported.uniq.length
daisy_supported = daisy - daisy_non_supported

functions = %w[buildDaisyOpenReceipt buildDatecsOpenReceipt buildDaisySaleItem buildDatecsSaleItem buildDaisyPayment buildDatecsPayment buildDaisyDailyReport buildDatecsDailyReport buildDaisyCashMovement buildDatecsCashMovement buildDatecsSubtotal buildDatecsLastFiscalEntry buildDatecsErrorLookup buildDatecsDeviceIdentity buildDatecsDailyTaxation buildDatecsDiagnosticInfo buildDatecsItemGroupInfo buildDatecsDepartmentInfo buildDatecsFiscalMemoryTest buildDatecsAdditionalDailyInfo buildDatecsOperatorInfo buildDatecsCurrencyConversion buildDatecsFiscalMemoryRead buildDatecsDeviceInfo buildDatecsReceiptPeriodSearch buildDatecsEjDocumentSelector buildDatecsEjCsvRange buildDatecsEjCsvRead buildDatecsFiscalMemoryStructured buildDatecsModemInfo buildDatecsOpenReversal buildDatecsPcConnectionMode buildDatecsProgrammedItem buildDatecsGeneralInfo buildDatecsFiscalMemoryDateReport buildDatecsFiscalMemoryNumberReport buildDatecsOperatorReport buildDatecsPluReport buildDatecsOptionalNoParameters buildDatecsDisplayText buildDatecsPaperFeed buildDatecsSound buildDatecsBarcode buildDatecsSeparator buildDatecsDrawer buildDatecsInvoiceData buildDatecsClientInfo buildDatecsClientProgram buildDatecsClientDelete buildDatecsClientRead buildDatecsClientSeek buildDatecsClientNext buildDatecsClientFindByTaxNumber buildDatecsClientFindUnprogrammed parseDatecsTaxRates parseDatecsSubtotal parseDatecsDateTime parseDatecsLastFiscalEntry parseDatecsTaxNumber parseDatecsErrorDescription parseDatecsCurrentReceipt parseDatecsDeviceIdentity parseDatecsDailyTaxation parseDatecsReportsLeft parseDatecsLastFiscalRecordDateTime parseDatecsDiagnosticInfo parseDatecsItemGroupInfo parseDatecsDepartmentInfo parseDatecsFiscalMemoryTest parseDatecsDailyPayments parseDatecsDailySales parseDatecsDailyDualCountSum parseDatecsDailyCashMovements parseDatecsOperatorInfo parseDatecsCurrencyConversion parseDatecsFiscalMemoryRead parseDatecsDevicePowerNetwork parseDatecsLastFiscalReceiptInfo parseDatecsDeviceInfoVerification parseDatecsDeviceBattery parseDatecsReceiptPeriodSearch parseDatecsEjDocumentInfo parseDatecsEjTextLine parseDatecsEjBase64Data parseDatecsEjAcknowledge parseDatecsEjCsvRow parseDatecsFiscalMemoryCapacity parseDatecsFiscalMemoryZRecord parseDatecsFiscalMemoryValueEvent parseDatecsFiscalMemoryDateEvent parseDatecsFiscalMemoryVatEvent parseDatecsFiscalMemoryCounterEvent parseDatecsFiscalMemoryKlenEvent parseDatecsModemIdentifier parseDatecsModemStatus parseDatecsSlipNumber parseDatecsAcknowledge parseDatecsNapConnection parseDatecsClientInfo parseDatecsClientRecord parseDatecsClientIndex parseDaisyReceiptStatus parseDatecsReceiptStatus]
functions.each { |name| abort "missing semantic implementation #{name}" unless payload.include?("#{name}(") }
%w[buildDaisyTaxRatesPeriod buildDaisySubtotal buildDaisyFiscalTotals parseDaisyTaxRates parseDaisySubtotal parseDaisyDateTime parseDaisyLastFiscalRecord parseDaisyCurrentFiscalTotals parseDaisyFiscalization parseDaisyFreeFiscalRecords].each do |name|
  abort "missing Daisy semantic implementation #{name}" unless payload.include?("#{name}(")
end
%w[buildDaisyDiagnostic buildDaisyErrorDescription buildDaisyCurrencyTransition parseDaisyDiagnostic parseDaisyCurrentTaxRates parseDaisyTaxNumber parseDaisyReceiptInfo parseDaisyLastDocumentNumber parseDaisyFirstUnsentReceipt parseDaisyFirmware parseDaisyErrorDescription parseDaisyCurrencyTransitionInfo parseDaisyCurrencyTransitionDate].each do |name|
  abort "missing Daisy device/currency semantic implementation #{name}" unless payload.include?("#{name}(")
end
%w[buildDaisySaleAndDisplay buildDaisyProgrammedItem buildDaisyFiscalMemoryNumberReport buildDaisyFiscalMemoryDateReport buildDaisyDailyReportOption buildDaisyPluReport buildDaisyDepartmentReport buildDaisySupportedNoData parseDaisyPassFail parseDaisyDailyReport].each do |name|
  abort "missing Daisy sale/report semantic implementation #{name}" unless payload.include?("#{name}(")
end
%w[buildDaisyCurrentDay buildDaisyOperatorInfo buildDaisyFmInformationNumber buildDaisyQrDocument buildDaisyIssuedDocument buildDaisyFmInformationDate buildDaisyDepartmentSale parseDaisyCurrentDay parseDaisyOperatorInfo parseDaisyFmInformation parseDaisyQrDocument parseDaisyIssuedDocument parseDaisyConstants parseDaisyTextReportLine].each do |name|
  abort "missing Daisy Supported semantic implementation #{name}" unless payload.include?("#{name}(")
end
%w[buildDaisyOptionalNoData buildDaisyDisplayText buildDaisyPaperFeed buildDaisyPaperCut buildDaisyDisplayLine buildDaisyInvoiceCustomer buildDaisyBarcode buildDaisyCustomerQr buildDaisyDrawer buildDaisyDisplayConfiguration buildDaisyCustomerProgram buildDaisyCustomerDelete buildDaisyCustomerRead buildDaisyCustomerIteration buildDaisyBattery parseDaisyNoData parseDaisyCustomerQr parseDaisyCustomerRecord parseDaisyBattery].each do |name|
  abort "missing Daisy Optional semantic implementation #{name}" unless payload.include?("#{name}(")
end

{"daisy" => daisy, "datecs" => datecs}.each do |vendor, inventory|
  required = coverage.fetch("execution_critical").fetch(vendor)
  missing = required - inventory
  abort "#{vendor}: semantic command absent from inventory: #{missing.join(',')}" unless missing.empty?
  missing_evidence = required.reject { |code| coverage.fetch("semantic_evidence").key?(code.to_s) }
  abort "#{vendor}: semantic commands lack evidence: #{missing_evidence.join(',')}" unless missing_evidence.empty?
end
missing_supported = datecs_supported - coverage.fetch("execution_critical").fetch("datecs")
abort "Datecs Supported commands lack semantic evidence: #{missing_supported.join(',')}" unless missing_supported.empty?
missing_daisy_supported = daisy_supported - coverage.fetch("execution_critical").fetch("daisy")
abort "Daisy Supported commands lack semantic evidence: #{missing_daisy_supported.join(',')}" unless missing_daisy_supported.empty?
optional_semantic = coverage.fetch("optional_semantic").fetch("datecs")
abort "optional semantic list contains non-Optional Datecs command" unless (optional_semantic - datecs_dispositions.fetch("optional")).empty?
abort "core and optional semantic inventories overlap" unless (optional_semantic & coverage.fetch("execution_critical").fetch("datecs")).empty?
missing_optional_evidence = optional_semantic.reject { |code| coverage.fetch("semantic_evidence").key?(code.to_s) }
abort "Datecs Optional commands lack evidence: #{missing_optional_evidence.join(',')}" unless missing_optional_evidence.empty?
daisy_optional_semantic = coverage.fetch("optional_semantic").fetch("daisy")
abort "Daisy optional semantic list contains non-Optional command" unless (daisy_optional_semantic - daisy_dispositions.fetch("optional")).empty?
abort "Daisy core and optional semantic inventories overlap" unless (daisy_optional_semantic & coverage.fetch("execution_critical").fetch("daisy")).empty?
missing_daisy_optional = daisy_dispositions.fetch("optional") - daisy_optional_semantic
abort "Daisy Optional commands lack semantic coverage: #{missing_daisy_optional.join(',')}" unless missing_daisy_optional.empty?
missing_daisy_optional_evidence = daisy_optional_semantic.reject { |code| coverage.fetch("semantic_evidence").key?(code.to_s) }
abort "Daisy Optional commands lack evidence: #{missing_daisy_optional_evidence.join(',')}" unless missing_daisy_optional_evidence.empty?
test = File.join(root, coverage.fetch("golden_test"))
abort "missing driver golden test" unless File.file?(test)
abort "hardware state must remain non-approved before HIL" unless coverage.fetch("hardware_state") == "DOCUMENTED_NOT_HARDWARE_VERIFIED"

puts "driver coverage OK: Daisy 88 classified/#{coverage['execution_critical']['daisy'].length} core + #{daisy_optional_semantic.length} optional semantic; Datecs 73 classified/#{coverage['execution_critical']['datecs'].length} core + #{optional_semantic.length} optional semantic; HIL not claimed"
