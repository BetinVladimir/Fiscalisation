#!/usr/bin/env ruby
require "json"
require "openssl"
require "time"
require "rbconfig"

root = File.expand_path("..", __dir__)
trace = JSON.parse(File.read(File.join(root, "contracts/supto-annex29-trace.json")))
rows = trace.fetch("requirements")
blocked = rows.select { |row| row["status"] == "FAIL" || (%w[PARTIAL EXTERNAL_BLOCKED].include?(row["status"]) && row["gap"].to_s !~ /(external|physical|HIL|production|trusted|IdP)/i) }
abort "SUPTO software acceptance blocked by #{blocked.length} Annex rows: #{blocked.map { |row| row['id'] }.join(', ')}" unless blocked.empty?

def verify_external!(root, env_name, script)
  dir = ENV[env_name].to_s
  abort "#{env_name} is required" if dir.empty?
  runtime = ENV["EVIDENCE_RUBY"] || ENV["HIL_RUBY"] || RbConfig.ruby
  ok = system(runtime, File.join(root, "scripts", script), dir)
  abort "#{env_name} verification failed" unless ok
end
verify_external!(root, "HIL_EVIDENCE_DIR", "verify_hil_evidence.rb")
verify_external!(root, "RELEASE_EVIDENCE_DIR", "verify_release_evidence.rb")
verify_external!(root, "LEGAL_EVIDENCE_DIR", "verify_legal_evidence.rb")

# Activation repeats proof verification and binds all manifests to one candidate.
# The current release verifier only recognizes PROD_NO_GO; activation therefore
# remains blocked until a legitimate production release contract is implemented.
verify_external!(root, "ACTIVATION_EVIDENCE_DIR", "verify_activation_evidence.rb")
abort "production activation contract unavailable"
