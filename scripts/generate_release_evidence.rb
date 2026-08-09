#!/usr/bin/env ruby
require "digest"
require "fileutils"
require "json"
require "open3"
require "pathname"
require "time"

ROOT = Pathname.new(__dir__).join("..").realpath
output = Pathname.new(ARGV.fetch(0, ROOT.join("artifacts/evidence/mvp-dev").to_s)).expand_path
abort "refusing evidence output outside project or /tmp" unless output.to_s.start_with?(ROOT.to_s + "/") || output.to_s.start_with?("/tmp/")
FileUtils.mkdir_p(output)
%w[sbom.cdx.json provenance.intoto.json vulnerability-report.json release-manifest.json release-manifest.sig release-public-key.pem checksums.sha256].each do |name|
  FileUtils.rm_f(output.join(name))
end

components = {}
Dir[ROOT.join("{fiscal-backend,beeminipos-backend,edge-agent}/go.sum")].sort.each do |path|
  File.foreach(path) do |line|
    name, version = line.split
    next unless name && version && !version.end_with?("/go.mod")
    version = version.sub(/\/go\.mod\z/, "")
    components[["golang", name, version]] = {"type" => "library", "name" => name, "version" => version, "purl" => "pkg:golang/#{name}@#{version}"}
  end
end
Dir[ROOT.join("{BeeMiniPOS,BeeFiscalApp}/package-lock.json")].sort.each do |path|
  lock = JSON.parse(File.read(path))
  (lock["packages"] || {}).each do |key, value|
    next unless key.start_with?("node_modules/") && value["version"]
    name = key.sub("node_modules/", "")
    version = value["version"]
    components[["npm", name, version]] = {"type" => "library", "name" => name, "version" => version, "purl" => "pkg:npm/#{name.gsub("@", "%40")}@#{version}"}
  end
end
Dir[ROOT.join("compose.*.yaml")].sort.each do |path|
  File.foreach(path) do |line|
    next unless (match = line.match(/^\s*image:\s*["']?([^"'\s]+)["']?/))
    image = match[1]
    name, version = image.split(":", 2)
    next unless version
    components[["container", name, version]] = {"type" => "container", "name" => name, "version" => version, "purl" => "pkg:docker/#{name}@#{version}"}
  end
end

created_at = ENV.fetch("RELEASE_CREATED_AT", Time.now.utc.iso8601)
begin
  created_at = Time.iso8601(created_at).utc.iso8601
rescue ArgumentError
  abort "RELEASE_CREATED_AT must be ISO 8601"
end
serial_hex = Digest::SHA256.hexdigest(components.keys.sort.inspect)[0, 32]
serial = "urn:uuid:#{serial_hex[0, 8]}-#{serial_hex[8, 4]}-#{serial_hex[12, 4]}-#{serial_hex[16, 4]}-#{serial_hex[20, 12]}"
sbom = {
  "bomFormat" => "CycloneDX", "specVersion" => "1.6", "serialNumber" => serial, "version" => 1,
  "metadata" => {"timestamp" => created_at, "component" => {"type" => "application", "name" => "beeloy-fiscalisation-mvp", "version" => "2026-08-07"}},
  "components" => components.values.sort_by { |c| [c["type"], c["name"], c["version"]] }
}
sbom_path = output.join("sbom.cdx.json")
File.write(sbom_path, JSON.pretty_generate(sbom) + "\n")
sbom_sha256 = Digest::SHA256.file(sbom_path).hexdigest

evidence_files = Dir[
  ROOT.join("contracts/**/*.{yaml,md}"), ROOT.join("database/**/*.sql"), ROOT.join("compose.*.yaml"),
  ROOT.join("TOOLCHAIN.md"), ROOT.join("MVP_GATES.md"), ROOT.join("IMPLEMENTATION_STATUS.md")
].select { |path| File.file?(path) }.sort
hashes = evidence_files.to_h { |path| [Pathname.new(path).relative_path_from(ROOT).to_s, Digest::SHA256.file(path).hexdigest] }
git_head, = Open3.capture2("git", "rev-parse", "HEAD", chdir: ROOT.to_s)
git_status, = Open3.capture2("git", "status", "--porcelain", "--untracked-files=normal", chdir: ROOT.to_s)
provenance = {
  "_type" => "https://in-toto.io/Statement/v1",
  "subject" => [{"name" => "sbom.cdx.json", "digest" => {"sha256" => sbom_sha256}}],
  "predicateType" => "https://slsa.dev/provenance/v1",
  "predicate" => {
    "buildDefinition" => {
      "buildType" => "https://beeloy.example/build-types/fiscalisation-release-evidence/v1",
      "externalParameters" => {"release" => "mvp-dev-2026-08-07", "required_command" => "make full-regression"},
      "internalParameters" => {"contracts" => "2026-08-07", "country_policy" => "BG-2026-EUR"},
      "resolvedDependencies" => [{"uri" => "git+local:fiscalisation", "digest" => {"gitCommit" => git_head.strip}}]
    },
    "runDetails" => {
      "builder" => {"id" => "https://beeloy.example/builders/release-evidence/v1"},
      "metadata" => {"invocationId" => serial, "startedOn" => created_at, "finishedOn" => created_at}
    }
  }
}
provenance_path = output.join("provenance.intoto.json")
File.write(provenance_path, JSON.pretty_generate(provenance) + "\n")

scan_source = ENV["RELEASE_VULNERABILITY_REPORT"].to_s
if !scan_source.empty?
  scan_source_path = Pathname.new(scan_source).expand_path
  abort "vulnerability report does not exist" unless scan_source_path.file?
  vulnerability_path = output.join("vulnerability-report.json")
  FileUtils.cp(scan_source_path, vulnerability_path)
else
  vulnerability_path = nil
end
signing_key = ENV["RELEASE_SIGNING_PRIVATE_KEY"].to_s
if !signing_key.empty?
  signing_key_path = Pathname.new(signing_key).expand_path
  abort "release signing key does not exist" unless signing_key_path.file?
  abort "release signing key must not be group/world accessible" unless (signing_key_path.stat.mode & 0o077).zero?
  public_pem, public_error, public_status = Open3.capture3("node", ROOT.join("scripts/release-signature.mjs").to_s, "public", signing_key_path.to_s)
  abort "cannot derive release public key: #{public_error}" unless public_status.success?
  public_key_path = output.join("release-public-key.pem")
  File.write(public_key_path, public_pem)
  public_fingerprint = Digest::SHA256.hexdigest(public_pem)
else
  public_key_path = nil
  public_fingerprint = nil
end

manifest = {
  "schema_version" => "1.0", "created_at" => created_at, "release" => "mvp-dev-2026-08-07",
  "source" => {"git_commit" => git_head.strip, "dirty" => !git_status.empty?},
  "contracts" => {"openapi" => "2026-08-07", "asyncapi" => "3.0.0", "ble" => "v1", "country_policy" => "BG-2026-EUR"},
  "artifacts" => {
    "sbom" => sbom_path.basename.to_s,
    "sbom_sha256" => sbom_sha256,
    "provenance" => provenance_path.basename.to_s,
    "provenance_sha256" => Digest::SHA256.file(provenance_path).hexdigest,
    "vulnerability_report" => vulnerability_path&.basename&.to_s,
    "vulnerability_report_sha256" => vulnerability_path && Digest::SHA256.file(vulnerability_path).hexdigest,
    "file_hashes" => hashes
  }.compact,
  "verification" => {
    "required_command" => "make full-regression",
    "signature_status" => signing_key.empty? ? "UNSIGNED_NO_GO" : "SIGNED_ED25519",
    "signature_algorithm" => signing_key.empty? ? nil : "Ed25519",
    "public_key_sha256" => public_fingerprint,
    "vulnerability_status" => vulnerability_path ? "ATTACHED_REQUIRES_VERIFICATION" : "REQUIRES_RELEASE_SCAN"
  }.compact,
  "release_decision" => "PROD_NO_GO",
  "no_go_reasons" => [
    "hardware/vendor/legal gates in MVP_GATES.md",
    ("release artifacts are not signed" if signing_key.empty?),
    ("release vulnerability scan is not attached" unless vulnerability_path)
  ].compact
}
manifest_path = output.join("release-manifest.json")
File.write(manifest_path, JSON.pretty_generate(manifest) + "\n")
signed_files = [sbom_path, provenance_path, manifest_path]
signed_files << vulnerability_path if vulnerability_path
unless signing_key.empty?
  signature_path = output.join("release-manifest.sig")
  _stdout, sign_error, sign_status = Open3.capture3(
    "node", ROOT.join("scripts/release-signature.mjs").to_s, "sign",
    Pathname.new(signing_key).expand_path.to_s, manifest_path.to_s, signature_path.to_s
  )
  abort "cannot sign release manifest: #{sign_error}" unless sign_status.success?
  signed_files.concat([signature_path, public_key_path])
end
File.write(output.join("checksums.sha256"), signed_files.map { |path| "#{Digest::SHA256.file(path).hexdigest}  #{path.basename}" }.join("\n") + "\n")
puts output
