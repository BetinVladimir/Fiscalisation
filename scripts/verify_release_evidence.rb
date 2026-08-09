#!/usr/bin/env ruby
require "digest"
require "json"
require "open3"
require "pathname"
require "time"

root = Pathname.new(ARGV.fetch(0)).expand_path
sbom_path = root.join("sbom.cdx.json")
manifest_path = root.join("release-manifest.json")
checksums_path = root.join("checksums.sha256")
[sbom_path, manifest_path, checksums_path].each { |path| abort "missing #{path}" unless path.file? }
sbom = JSON.parse(sbom_path.read)
abort "not CycloneDX 1.6" unless sbom["bomFormat"] == "CycloneDX" && sbom["specVersion"] == "1.6"
abort "SBOM has too few components" unless sbom.fetch("components").length >= 20
purls = sbom.fetch("components").map { |component| component["purl"] }
abort "duplicate SBOM purl" unless purls.uniq.length == purls.length
manifest = JSON.parse(manifest_path.read)
abort "unsafe release decision" unless manifest["release_decision"] == "PROD_NO_GO"
signature_status = manifest.dig("verification", "signature_status")
case signature_status
when "UNSIGNED_NO_GO"
  abort "unsigned release not gated" unless manifest.fetch("no_go_reasons").include?("release artifacts are not signed")
when "SIGNED_ED25519"
  trusted_key = ENV["RELEASE_TRUSTED_PUBLIC_KEY"].to_s
  abort "signed evidence requires RELEASE_TRUSTED_PUBLIC_KEY" if trusted_key.empty?
  trusted_key_path = Pathname.new(trusted_key).expand_path
  signature_path = root.join("release-manifest.sig")
  packaged_key_path = root.join("release-public-key.pem")
  abort "missing trusted release public key" unless trusted_key_path.file?
  abort "missing release-manifest.sig" unless signature_path.file?
  abort "missing packaged release public key" unless packaged_key_path.file?
  trusted_pem = trusted_key_path.read
  abort "packaged release key differs from trusted key" unless packaged_key_path.read == trusted_pem
  abort "release key fingerprint mismatch" unless manifest.dig("verification", "public_key_sha256") == Digest::SHA256.hexdigest(trusted_pem)
  signature_helper = Pathname.new(__dir__).join("release-signature.mjs").realpath
  _stdout, verify_error, verify_status = Open3.capture3(
    "node", signature_helper.to_s, "verify", trusted_key_path.to_s, manifest_path.to_s, signature_path.to_s
  )
  abort "release manifest signature invalid: #{verify_error}" unless verify_status.success?
else
  abort "unknown signature status"
end
abort "SBOM hash mismatch" unless manifest.dig("artifacts", "sbom_sha256") == Digest::SHA256.file(sbom_path).hexdigest
provenance_path = root.join(manifest.dig("artifacts", "provenance").to_s)
abort "missing provenance" unless provenance_path.file?
abort "provenance hash mismatch" unless manifest.dig("artifacts", "provenance_sha256") == Digest::SHA256.file(provenance_path).hexdigest
provenance = JSON.parse(provenance_path.read)
abort "not an in-toto v1 statement" unless provenance["_type"] == "https://in-toto.io/Statement/v1"
abort "not SLSA provenance v1" unless provenance["predicateType"] == "https://slsa.dev/provenance/v1"
subject = provenance.fetch("subject").find { |entry| entry["name"] == "sbom.cdx.json" }
abort "provenance does not bind the SBOM" unless subject&.dig("digest", "sha256") == Digest::SHA256.file(sbom_path).hexdigest
abort "provenance source commit mismatch" unless provenance.dig("predicate", "buildDefinition", "resolvedDependencies", 0, "digest", "gitCommit") == manifest.dig("source", "git_commit")

vulnerability_name = manifest.dig("artifacts", "vulnerability_report")
if vulnerability_name
  abort "vulnerability status is inconsistent" unless manifest.dig("verification", "vulnerability_status") == "ATTACHED_REQUIRES_VERIFICATION"
  abort "unsafe vulnerability report path" unless Pathname.new(vulnerability_name).basename.to_s == vulnerability_name
  vulnerability_path = root.join(vulnerability_name)
  abort "missing vulnerability report" unless vulnerability_path.file?
  abort "vulnerability report hash mismatch" unless manifest.dig("artifacts", "vulnerability_report_sha256") == Digest::SHA256.file(vulnerability_path).hexdigest
  report = JSON.parse(vulnerability_path.read)
  abort "unsupported vulnerability report schema" unless report["schema_version"] == "1.0"
  abort "vulnerability report is not bound to SBOM" unless report.dig("subject", "sbom_sha256") == Digest::SHA256.file(sbom_path).hexdigest
  abort "vulnerability scanner did not complete" unless report.dig("scanner", "status") == "COMPLETED"
  database_updated_at = Time.iso8601(report.dig("scanner", "database_updated_at").to_s)
  abort "vulnerability database is older than 72 hours" if Time.now.utc - database_updated_at > 72 * 60 * 60
  %w[critical high medium low unknown].each do |severity|
    count = report.dig("summary", severity)
    abort "invalid vulnerability count for #{severity}" unless count.is_a?(Integer) && count >= 0
  end
  abort "release has critical/high vulnerabilities" unless report.dig("summary", "critical").zero? && report.dig("summary", "high").zero?
  abort "vulnerability finding count mismatch" unless report.fetch("findings").length == report.fetch("summary").values.sum
else
  abort "vulnerability status is inconsistent" unless manifest.dig("verification", "vulnerability_status") == "REQUIRES_RELEASE_SCAN"
end
checksum_names = []
checksums_path.each_line do |line|
  hash, name = line.split
  abort "unsafe checksum path" unless name && Pathname.new(name).basename.to_s == name
  abort "duplicate checksum entry" if checksum_names.include?(name)
  checksum_names << name
  path = root.join(name)
  abort "checksum mismatch for #{name}" unless path.file? && Digest::SHA256.file(path).hexdigest == hash
end
expected_checksums = %w[sbom.cdx.json provenance.intoto.json release-manifest.json]
expected_checksums << vulnerability_name if vulnerability_name
expected_checksums += %w[release-manifest.sig release-public-key.pem] if signature_status == "SIGNED_ED25519"
abort "incomplete or unexpected checksum inventory" unless checksum_names.sort == expected_checksums.sort
puts "release evidence OK: #{sbom.fetch("components").length} components, #{signature_status}, PROD_NO_GO enforced"
