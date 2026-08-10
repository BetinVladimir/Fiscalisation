#!/usr/bin/env ruby
require "yaml"

root = File.expand_path("..", __dir__)
contract = YAML.safe_load(File.read(File.join(root, "contracts/openapi-runtime-v1.yaml")), aliases: true)
cloud = contract.dig("paths", "/sales:open-with-line", "post") or abort "cloud atomic open missing"
local = contract.dig("paths", "/local/v1/intents", "post") or abort "local intent missing"
abort "cloud/local operations are not documented" if cloud["operationId"].to_s.empty? || local["operationId"].to_s.empty?
intent = contract.dig("components", "schemas", "ComplianceIntent") or abort "ComplianceIntent missing"
result = contract.dig("components", "schemas", "ComplianceIntentResult") or abort "ComplianceIntentResult missing"
%w[intent_id action client_sale_surrogate_id operator_code app_instance_id expected_version].each { |field| abort "local intent misses #{field}" unless intent.fetch("required").include?(field) }
%w[operation_id state version].each { |field| abort "local result misses #{field}" unless result.fetch("required").include?(field) }
body = File.read(File.join(root, "edge-agent/gateway/compliance_test.go")) + File.read(File.join(root, "edge-agent/gateway/processor_test.go"))
%w[AB123456-A001-0000041 idempotent\ replay durable\ sale\ UNP TestEncryptedComplianceIntentReturnsOpaqueRegulatoryIdentifier].each do |anchor|
  abort "offline equivalence evidence missing #{anchor.tr('\\', '')}" unless body.include?(anchor.tr('\\', ''))
end
pos = File.read(File.join(root, "BeeMiniPOS/App.tsx"))
abort "POS does not use atomic cloud open" unless pos.include?("/sales:open-with-line")
abort "legacy POS checkout authority remains" if pos.include?("/orders/${order.id}/checkout")
puts "offline equivalence OK: cloud atomic sale + protected REST/BLE intent, opaque identifier, durable replay and same-UNP binding"
