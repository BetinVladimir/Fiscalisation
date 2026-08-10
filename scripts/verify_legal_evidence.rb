#!/usr/bin/env ruby
require "json"
require "openssl"

dir = ARGV[0].to_s
abort "EVIDENCE_DIR is required" if dir.empty?
manifest_path = File.join(dir, "legal-acceptance.json")
signature_path = File.join(dir, "legal-acceptance.sig")
key_path = File.join(dir, "trusted-legal-reviewer-public.pem")
abort "trusted legal manifest/signature/key missing" unless [manifest_path, signature_path, key_path].all? { |path| File.file?(path) }
raw = File.binread(manifest_path)
manifest = JSON.parse(raw)
abort "external legal/service acceptance required" unless manifest["result"] == "ACCEPTED" && manifest["reviewer_organization"].to_s != "Beeloy"
key = OpenSSL::PKey.read(File.read(key_path))
abort "untrusted legal signature" unless key.verify(nil, File.binread(signature_path), raw)
puts "trusted external legal evidence OK"
