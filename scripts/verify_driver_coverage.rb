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
classified_non_supported = %w[optional privileged excluded].flat_map do |name|
  match = datecs_disposition.match(/const uint16_t #{name}\[\]=\{([^}]*)\}/)
  abort "missing Datecs #{name} disposition inventory" unless match
  match[1].split(",").map(&:to_i)
end
abort "Datecs disposition inventories overlap" unless classified_non_supported.length == classified_non_supported.uniq.length
datecs_supported = datecs - classified_non_supported

functions = %w[buildDaisyOpenReceipt buildDatecsOpenReceipt buildDaisySaleItem buildDatecsSaleItem buildDaisyPayment buildDatecsPayment buildDaisyDailyReport buildDatecsDailyReport buildDaisyCashMovement buildDatecsCashMovement buildDatecsSubtotal buildDatecsLastFiscalEntry buildDatecsErrorLookup buildDatecsDeviceIdentity buildDatecsDailyTaxation buildDatecsDiagnosticInfo buildDatecsItemGroupInfo buildDatecsDepartmentInfo buildDatecsFiscalMemoryTest buildDatecsAdditionalDailyInfo buildDatecsOperatorInfo buildDatecsCurrencyConversion buildDatecsFiscalMemoryRead buildDatecsDeviceInfo buildDatecsReceiptPeriodSearch buildDatecsEjDocumentSelector buildDatecsEjCsvRange buildDatecsEjCsvRead buildDatecsFiscalMemoryStructured buildDatecsModemInfo buildDatecsOpenReversal buildDatecsPcConnectionMode buildDatecsProgrammedItem buildDatecsGeneralInfo buildDatecsFiscalMemoryDateReport buildDatecsFiscalMemoryNumberReport buildDatecsOperatorReport buildDatecsPluReport parseDatecsTaxRates parseDatecsSubtotal parseDatecsDateTime parseDatecsLastFiscalEntry parseDatecsTaxNumber parseDatecsErrorDescription parseDatecsCurrentReceipt parseDatecsDeviceIdentity parseDatecsDailyTaxation parseDatecsReportsLeft parseDatecsLastFiscalRecordDateTime parseDatecsDiagnosticInfo parseDatecsItemGroupInfo parseDatecsDepartmentInfo parseDatecsFiscalMemoryTest parseDatecsDailyPayments parseDatecsDailySales parseDatecsDailyDualCountSum parseDatecsDailyCashMovements parseDatecsOperatorInfo parseDatecsCurrencyConversion parseDatecsFiscalMemoryRead parseDatecsDevicePowerNetwork parseDatecsLastFiscalReceiptInfo parseDatecsDeviceInfoVerification parseDatecsDeviceBattery parseDatecsReceiptPeriodSearch parseDatecsEjDocumentInfo parseDatecsEjTextLine parseDatecsEjBase64Data parseDatecsEjAcknowledge parseDatecsEjCsvRow parseDatecsFiscalMemoryCapacity parseDatecsFiscalMemoryZRecord parseDatecsFiscalMemoryValueEvent parseDatecsFiscalMemoryDateEvent parseDatecsFiscalMemoryVatEvent parseDatecsFiscalMemoryCounterEvent parseDatecsFiscalMemoryKlenEvent parseDatecsModemIdentifier parseDatecsModemStatus parseDatecsSlipNumber parseDatecsAcknowledge parseDatecsNapConnection parseDaisyReceiptStatus parseDatecsReceiptStatus]
functions.each { |name| abort "missing semantic implementation #{name}" unless payload.include?("#{name}(") }

{"daisy" => daisy, "datecs" => datecs}.each do |vendor, inventory|
  required = coverage.fetch("execution_critical").fetch(vendor)
  missing = required - inventory
  abort "#{vendor}: semantic command absent from inventory: #{missing.join(',')}" unless missing.empty?
end
missing_supported = datecs_supported - coverage.fetch("execution_critical").fetch("datecs")
abort "Datecs Supported commands lack semantic evidence: #{missing_supported.join(',')}" unless missing_supported.empty?
test = File.join(root, coverage.fetch("golden_test"))
abort "missing driver golden test" unless File.file?(test)
abort "hardware state must remain non-approved before HIL" unless coverage.fetch("hardware_state") == "DOCUMENTED_NOT_HARDWARE_VERIFIED"

puts "driver coverage OK: Daisy 88 classified/#{coverage['execution_critical']['daisy'].length} core semantic; Datecs 73 classified/#{coverage['execution_critical']['datecs'].length} core semantic; HIL not claimed"
