#!/usr/bin/env ruby
require "digest"
require "json"
require "pathname"

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
abort "unsigned release not gated" unless manifest.dig("verification", "signature_status") == "UNSIGNED_NO_GO"
abort "SBOM hash mismatch" unless manifest.dig("artifacts", "sbom_sha256") == Digest::SHA256.file(sbom_path).hexdigest
checksums_path.each_line do |line|
  hash, name = line.split
  path = root.join(name)
  abort "checksum mismatch for #{name}" unless path.file? && Digest::SHA256.file(path).hexdigest == hash
end
puts "release evidence OK: #{sbom.fetch("components").length} components, PROD_NO_GO enforced"
