#!/usr/bin/env ruby
require "json"
require "openssl"
require "time"

dir = ARGV[0].to_s
abort "EVIDENCE_DIR is required" if dir.empty?
manifest_path = File.join(dir, "hil-manifest.json")
signature_path = File.join(dir, "hil-manifest.sig")
key_path = File.join(dir, "trusted-reviewer-public.pem")
abort "trusted HIL manifest/signature/key missing" unless [manifest_path, signature_path, key_path].all? { |path| File.file?(path) }
raw = File.binread(manifest_path)
manifest = JSON.parse(raw)
abort "simulator/STUB cannot satisfy HIL" if %w[SIMULATOR STUB].include?(manifest["evidence_type"])
abort "physical device evidence required" unless manifest["evidence_type"] == "PHYSICAL_HIL" && manifest["result"] == "PASS"
abort "HIL evidence expired" unless Time.parse(manifest.fetch("valid_until")) > Time.now.utc
key = OpenSSL::PKey.read(File.read(key_path))
abort "untrusted HIL signature" unless key.verify(nil, File.binread(signature_path), raw)
puts "trusted physical HIL evidence OK"
