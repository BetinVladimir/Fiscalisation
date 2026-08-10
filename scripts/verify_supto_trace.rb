#!/usr/bin/env ruby
require "json"
root = File.expand_path("..", __dir__)
trace = JSON.parse(File.read(File.join(root, "contracts/supto-annex29-trace.json")))
scope = JSON.parse(File.read(File.join(root, "contracts/supto-feature-scope.json")))
abort "SUPTO trace country/profile drifted" unless trace["country_code"] == "BG" && trace["target_profile"] == "BG_SUPTO_FULL"
rows = trace.fetch("requirements")
expected = (1..24).map { |n| format("SUPTO-29-%02d", n) }
abort "SUPTO Annex 29 coverage drifted" unless rows.map { |r| r["id"] } == expected
allowed = %w[PASS PARTIAL FAIL EXTERNAL_BLOCKED NOT_APPLICABLE]
required_arrays = %w[invariants apis db_constraints unit_tests integration_tests e2e_tests hil_evidence legal_evidence evidence]
rows.each do |row|
  abort "invalid SUPTO status #{row["id"]}" unless allowed.include?(row["status"])
  abort "missing SUPTO owner/title #{row["id"]}" if row["owner"].to_s.empty? || row["title"].to_s.empty?
  required_arrays.each { |key| abort "missing #{key} #{row["id"]}" unless row[key].is_a?(Array) }
  row["evidence"].each { |path| abort "missing evidence #{path}" unless File.exist?(File.join(root, path)) }
  if %w[PARTIAL FAIL EXTERNAL_BLOCKED].include?(row["status"])
    abort "open gap not production-blocked #{row["id"]}" unless row["production_blocked"] == true && !row["gap"].to_s.empty?
  end
end
abort "cancelled Annex row must be N/A" unless rows[2]["status"] == "NOT_APPLICABLE"
features = scope.fetch("features")
%w[inventory supplies electronic_fiscal_receipts].each do |name|
  value = features.fetch(name)
  abort "feature scope #{name} is not fail-closed" unless value["in_product_scope"] == false && value["status"] == "NOT_IN_PRODUCT_SCOPE"
end
abort "electronic receipt requirement/scope mismatch" unless rows[23]["status"] == "NOT_APPLICABLE"
adrs = %w[ADR-SUPTO-001-thin-pos.md ADR-SUPTO-002-local-compliance-gateway.md ADR-SUPTO-003-fmin-unp.md ADR-SUPTO-004-offline-ranges.md]
adrs.each { |name| abort "missing SUPTO decision #{name}" unless File.exist?(File.join(root, "docs/decisions", name)) }
pos_sources = Dir.glob(File.join(root, "BeeMiniPOS", "{App.tsx,src/**/*.ts}")).reject { |path| path.end_with?(".test.ts") }.map { |path| File.read(path) }.join("\n")
forbidden_pos_authority = %w[setCart fiscalCheckout OfflineSaleInput buildOfflineSaleEnvelope chooseTransport sendFiscalCommand /orders/checkout]
forbidden_pos_authority.each { |token| abort "thin POS regained fiscal authority: #{token}" if pos_sources.include?(token) }
abort "first-tap atomic API disappeared from POS" unless pos_sources.include?("/sales:open-with-line")
abort "POS does not render opaque regulatory identifiers" unless pos_sources.include?("regulatory_identifiers")
bg = JSON.parse(File.read(File.join(root, "contracts/bg-requirements-trace.json"))).fetch("requirements").find { |r| r["id"] == "BG-014" }
abort "EPIC-00 must not fabricate BG-014 PASS" unless bg && bg["status"] == "EXCLUDED_MVP"
counts = rows.group_by { |r| r["status"] }.transform_values(&:length)
puts "SUPTO Annex 29 trace OK: 24/24; #{counts.sort.map { |k,v| "#{k}=#{v}" }.join(", ")}; BG-014 remains production-blocked"
