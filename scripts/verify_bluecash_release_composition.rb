#!/usr/bin/env ruby

root = File.expand_path("..", __dir__)
activity = File.read(File.join(root, "SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/MainActivity.kt"))

required = %w[DatecsAndroidFiscalPort DatecsAndroidPaymentPort BoricaPinpadCodec]
missing = required.reject { |symbol| activity.include?(symbol) }
abort "BlueCash release composition lacks: #{missing.join(', ')}" unless missing.empty?

forbidden = %w[MissingVendorFiscalAdapter MissingVendorCardAdapter MissingDatecsPinpadCodec]
selected = forbidden.select { |symbol| activity.include?(symbol) }
abort "BlueCash release selects fail-closed placeholder: #{selected.join(', ')}" unless selected.empty?

puts "BlueCash release composition OK: Android fiscal socket + Android pinpad socket + audited BORICA codec"
