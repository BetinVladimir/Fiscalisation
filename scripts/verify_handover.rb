#!/usr/bin/env ruby
require 'json'

root = File.expand_path('..', __dir__)
record_path = File.join(root, 'contracts/mvp-acceptance-v1.json')
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
assert(record.dig('contract_surface', 'canonical_operations') + record.dig('contract_surface', 'runtime_operations') == 90, 'contract count mismatch')
assert(record.dig('requirement_trace', 'total') == 25, 'BG trace count mismatch')
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

puts 'handover gate OK: base software PASS; human signatures and PILOT/PROD external gates remain NO-GO'
