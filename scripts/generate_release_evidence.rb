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

created_at = Time.now.utc.iso8601
serial_hex = Digest::SHA256.hexdigest(components.keys.sort.inspect)[0, 32]
serial = "urn:uuid:#{serial_hex[0, 8]}-#{serial_hex[8, 4]}-#{serial_hex[12, 4]}-#{serial_hex[16, 4]}-#{serial_hex[20, 12]}"
sbom = {
  "bomFormat" => "CycloneDX", "specVersion" => "1.6", "serialNumber" => serial, "version" => 1,
  "metadata" => {"timestamp" => created_at, "component" => {"type" => "application", "name" => "beeloy-fiscalisation-mvp", "version" => "2026-08-07"}},
  "components" => components.values.sort_by { |c| [c["type"], c["name"], c["version"]] }
}
sbom_path = output.join("sbom.cdx.json")
File.write(sbom_path, JSON.pretty_generate(sbom) + "\n")

evidence_files = Dir[
  ROOT.join("contracts/**/*.{yaml,md}"), ROOT.join("database/**/*.sql"), ROOT.join("compose.*.yaml"),
  ROOT.join("TOOLCHAIN.md"), ROOT.join("MVP_GATES.md"), ROOT.join("IMPLEMENTATION_STATUS.md")
].select { |path| File.file?(path) }.sort
hashes = evidence_files.to_h { |path| [Pathname.new(path).relative_path_from(ROOT).to_s, Digest::SHA256.file(path).hexdigest] }
git_head, = Open3.capture2("git", "rev-parse", "HEAD", chdir: ROOT.to_s)
git_status, = Open3.capture2("git", "status", "--porcelain", "--untracked-files=normal", chdir: ROOT.to_s)
manifest = {
  "schema_version" => "1.0", "created_at" => created_at, "release" => "mvp-dev-2026-08-07",
  "source" => {"git_commit" => git_head.strip, "dirty" => !git_status.empty?},
  "contracts" => {"openapi" => "2026-08-07", "asyncapi" => "3.0.0", "ble" => "v1", "country_policy" => "BG-2026-EUR"},
  "artifacts" => {"sbom" => sbom_path.basename.to_s, "sbom_sha256" => Digest::SHA256.file(sbom_path).hexdigest, "file_hashes" => hashes},
  "verification" => {"required_command" => "make full-regression", "signature_status" => "UNSIGNED_NO_GO", "vulnerability_status" => "REQUIRES_RELEASE_SCAN"},
  "release_decision" => "PROD_NO_GO",
  "no_go_reasons" => ["hardware/vendor/legal gates in MVP_GATES.md", "release artifacts are not signed", "release vulnerability scan is not attached"]
}
manifest_path = output.join("release-manifest.json")
File.write(manifest_path, JSON.pretty_generate(manifest) + "\n")
File.write(output.join("checksums.sha256"), [sbom_path, manifest_path].map { |path| "#{Digest::SHA256.file(path).hexdigest}  #{path.basename}" }.join("\n") + "\n")
puts output
