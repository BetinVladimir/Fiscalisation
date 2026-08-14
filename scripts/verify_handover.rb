#!/usr/bin/env ruby
require 'json'
require 'yaml'
require 'date'
require 'tmpdir'

root = File.expand_path('..', __dir__)
record_path = ENV.fetch('ACCEPTANCE_RECORD_PATH', File.join(root, 'contracts/mvp-acceptance-v1.json'))
record = JSON.parse(File.read(record_path))

def assert(condition, message)
  abort("handover gate failed: #{message}") unless condition
end

assert(record['schema_version'] == '1.0', 'acceptance schema version')
assert(record['profile'] == 'BG_MVP_FUNCTIONAL_NONPROD', 'unexpected acceptance profile')
assert(record['acceptance_state'] == 'READY_FOR_HUMAN_SIGNATURE', 'software must not fabricate human acceptance')
assert(record['base_profile_status'] == 'PASS', 'base software profile is not PASS')
assert(record['pilot_status'] == 'NO_GO' && record['production_status'] == 'NO_GO', 'external gates must remain NO_GO')
assert(record.dig('regression', 'status') == 'PASS', 'regression evidence missing')
assert(record.dig('regression', 'observed_consecutive_clean_runs').to_i >= record.dig('regression', 'required_consecutive_clean_runs').to_i, 'repeatability threshold not met')
http_methods = %w[get post put patch delete]
operation_count = lambda do |path|
  document = YAML.safe_load(File.read(path), permitted_classes: [Date, Time], aliases: true)
  document.fetch('paths', {}).sum do |_route, item|
    item.count { |method, operation| http_methods.include?(method) && operation.is_a?(Hash) && !operation['operationId'].to_s.empty? }
  end
end
canonical_source = '/Users/freelancer/Documents/Beeloy/BeeloyBackend/docs/Fiscal/api/openapi-public-v1.yaml'
canonical_count = Dir.mktmpdir('beeloy-handover-openapi') do |dir|
  effective = File.join(dir, 'openapi-public-v1.yaml')
  assert(system('ruby', File.join(root, 'scripts/build_effective_openapi.rb'), effective,
                out: File::NULL), 'effective canonical OpenAPI build failed')
  operation_count.call(effective)
end
runtime_count = operation_count.call(File.join(root, 'contracts/openapi-runtime-v1.yaml'))
surface = record.fetch('contract_surface')
assert(surface['canonical_operations'] == canonical_count, "canonical contract count drift: acceptance=#{surface['canonical_operations']} actual=#{canonical_count}")
assert(surface['runtime_operations'] == runtime_count, "runtime contract count drift: acceptance=#{surface['runtime_operations']} actual=#{runtime_count}")

generated = %w[
  fiscal-backend/internal/api/response_contracts_gen.go
  minipos/beeminipos-backend/internal/api/response_contracts_gen.go
  edge-agent/localapi/response_contracts_gen.go
].map { |relative| File.read(File.join(root, relative)) }
success_count = generated.sum { |body| body.split('var generatedRequestContracts =', 2).first.scan(/^\s*\{Method:/).length }
request_count = generated.sum { |body| body.split('var generatedRequestContracts =', 2).fetch(1).scan(/^\s*\{Method:/).length }
assert(surface['request_contracts'] == request_count, "generated request contract drift: acceptance=#{surface['request_contracts']} actual=#{request_count}")
assert(surface['successful_response_contracts'] == success_count, "generated response contract drift: acceptance=#{surface['successful_response_contracts']} actual=#{success_count}")
assert(request_count == canonical_count + runtime_count && success_count == canonical_count + runtime_count, 'generated contract surface is incomplete')

trace = JSON.parse(File.read(File.join(root, 'contracts/bg-requirements-trace.json'))).fetch('requirements')
trace_counts = trace.group_by { |row| row.fetch('status') }.transform_values(&:length)
record_trace = record.fetch('requirement_trace')
assert(record_trace['total'] == trace.length, 'BG trace total drift')
assert(record_trace['pass'] == trace_counts.fetch('PASS', 0), 'BG PASS count drift')
assert(record_trace['partial_external_only'] == trace_counts.fetch('PARTIAL', 0), 'BG PARTIAL count drift')
assert(record_trace['external_blocked'] == trace_counts.fetch('EXTERNAL_BLOCKED', 0), 'BG EXTERNAL_BLOCKED count drift')
assert(record_trace['excluded_mvp'] == trace_counts.fetch('EXCLUDED_MVP', 0), 'BG EXCLUDED_MVP count drift')
assert(record.fetch('external_no_go_gates').length >= 8, 'external NO-GO inventory incomplete')
assert(record.fetch('signoffs').map { |x| x['role'] }.sort == %w[COMPLIANCE ENGINEERING PRODUCT QA SECURITY], 'required signoff roles missing')
assert(record.fetch('signoffs').all? { |x| x['status'] == 'PENDING_EXTERNAL_SIGNATURE' }, 'unsigned local artifact must not claim a signature')

required = {
  'RELEASE_NOTES.md' => ['## Known limitations and NO-GO', '## Rollback'],
  'docs/operations-runbook.md' => ['## PROD prerequisites', '## Backup and restore', '## Incident actions'],
  'docs/rollback-plan.md' => ['## Rollback triggers', '## Procedure', '## Forbidden rollback actions'],
  'docs/support-guide.md' => ['## Severity', '## Minimum ticket evidence', '## Escalation ownership'],
  'MVP_GATES.md' => ['Closure evidence'],
  'MVP_COMPLETION_AUDIT.md' => ['Production Bulgaria: **NO-GO**']
}
required.each do |path, markers|
  body = File.read(File.join(root, path))
  markers.each { |marker| assert(body.include?(marker), "#{path} missing #{marker}") }
end

puts "handover gate OK: #{canonical_count}+#{runtime_count} OpenAPI operations and #{request_count}/#{success_count} generated contracts match acceptance; human signatures and PILOT/PROD external gates remain NO-GO"
