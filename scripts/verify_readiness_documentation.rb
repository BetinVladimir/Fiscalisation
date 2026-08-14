#!/usr/bin/env ruby
require 'json'

root = File.expand_path('..', __dir__)
acceptance = JSON.parse(File.read(File.join(root, 'contracts/mvp-acceptance-v1.json')))
surface = acceptance.fetch('contract_surface')
trace = acceptance.fetch('requirement_trace')

expected = [
  'SOFTWARE_COMPLETE_HIL_PENDING',
  "#{surface.fetch('canonical_operations')} effective canonical + #{surface.fetch('runtime_operations')} runtime operations",
  "#{surface.fetch('request_contracts')} generated request and #{surface.fetch('successful_response_contracts')} success contracts",
  "#{trace.fetch('pass')} PASS, #{trace.fetch('partial_external_only')} PARTIAL, #{trace.fetch('external_blocked')} EXTERNAL_BLOCKED, #{trace.fetch('excluded_mvp')} EXCLUDED_MVP"
]

audit = File.read(File.join(root, 'MVP_COMPLETION_AUDIT.md'))
mvp = File.read(File.join(root, 'docs/MVP1/README.md'))
abort 'readiness documentation drift: MVP1 status marker missing' unless mvp.include?(expected[0])
expected.each do |marker|
  abort "readiness documentation drift: missing #{marker.inspect}" unless audit.include?(marker)
end

forbidden = ['73 canonical + 19 runtime', '92 generated request', '5 EXTERNAL_BLOCKED, 1 EXCLUDED_MVP']
forbidden.each do |marker|
  abort "readiness documentation drift: stale marker #{marker.inspect}" if audit.include?(marker)
end

puts "readiness documentation OK: #{surface['canonical_operations']}+#{surface['runtime_operations']} operations, #{surface['request_contracts']}/#{surface['successful_response_contracts']} contracts, external-only gaps"
