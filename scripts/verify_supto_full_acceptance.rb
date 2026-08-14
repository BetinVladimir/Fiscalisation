#!/usr/bin/env ruby
require "json"
require "openssl"
require "time"

root = File.expand_path("..", __dir__)
trace = JSON.parse(File.read(File.join(root, "contracts/supto-annex29-trace.json")))
rows = trace.fetch("requirements")
blocked = rows.select { |row| row["status"] == "FAIL" || (%w[PARTIAL EXTERNAL_BLOCKED].include?(row["status"]) && row["gap"].to_s !~ /(external|physical|HIL|production|trusted|IdP)/i) }
abort "SUPTO software acceptance blocked by #{blocked.length} Annex rows: #{blocked.map { |row| row['id'] }.join(', ')}" unless blocked.empty?

def verify_external!(root, env_name, script)
  dir = ENV[env_name].to_s
  abort "#{env_name} is required" if dir.empty?
  ok = system("ruby", File.join(root, "scripts", script), dir)
  abort "#{env_name} verification failed" unless ok
end
verify_external!(root, "HIL_EVIDENCE_DIR", "verify_hil_evidence.rb")
verify_external!(root, "RELEASE_EVIDENCE_DIR", "verify_release_evidence.rb")
verify_external!(root, "LEGAL_EVIDENCE_DIR", "verify_legal_evidence.rb")

activation_dir = ENV["ACTIVATION_EVIDENCE_DIR"].to_s
abort "ACTIVATION_EVIDENCE_DIR is required" if activation_dir.empty?
manifest_path = File.join(activation_dir, "bg014-activation.json")
signature_path = File.join(activation_dir, "bg014-activation.sig")
key_path = File.join(activation_dir, "trusted-activation-reviewer-public.pem")
abort "trusted BG-014 activation evidence missing" unless [manifest_path, signature_path, key_path].all? { |path| File.file?(path) }
raw = File.binread(manifest_path)
manifest = JSON.parse(raw)
abort "BG-014 activation decision is not PASS" unless manifest["profile"] == "BG_SUPTO_FULL" && manifest["result"] == "PASS" && Time.parse(manifest.fetch("valid_until")) > Time.now.utc
key = OpenSSL::PKey.read(File.read(key_path))
abort "BG-014 activation signature invalid" unless key.verify(nil, File.binread(signature_path), raw)
puts "BG SUPTO full acceptance PASS"
