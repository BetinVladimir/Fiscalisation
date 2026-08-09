#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
work=$(mktemp -d /tmp/beeloy-release-signing.XXXXXX)
cleanup() {
  rm -rf "$work"
}
trap cleanup EXIT INT TERM

test_signed_evidence_requires_trusted_ed25519_and_rejects_tamper() {
  private_key="$work/release-private.pem"
  public_key="$work/release-public.pem"
  evidence="$work/evidence"
  release_created_at="2026-08-09T12:00:00Z"

  node "$root/scripts/release-signature.mjs" generate "$private_key" "$public_key"
  unsigned="$work/unsigned"
  report="$work/vulnerability-report.json"
  RELEASE_CREATED_AT="$release_created_at" ruby "$root/scripts/generate_release_evidence.rb" "$unsigned" >/dev/null
  ruby -rjson -rtime -rdigest -e 'File.write(ARGV[1], JSON.pretty_generate({"schema_version"=>"1.0","scanner"=>{"name"=>"test-scanner","version"=>"1.0","database_updated_at"=>Time.now.utc.iso8601,"status"=>"COMPLETED"},"subject"=>{"sbom_sha256"=>Digest::SHA256.file(ARGV[0]).hexdigest},"summary"=>{"critical"=>0,"high"=>0,"medium"=>0,"low"=>0,"unknown"=>0},"findings"=>[]})+"\n")' "$unsigned/sbom.cdx.json" "$report"
  RELEASE_CREATED_AT="$release_created_at" RELEASE_SIGNING_PRIVATE_KEY="$private_key" RELEASE_VULNERABILITY_REPORT="$report" ruby "$root/scripts/generate_release_evidence.rb" "$evidence" >/dev/null
  RELEASE_TRUSTED_PUBLIC_KEY="$public_key" ruby "$root/scripts/verify_release_evidence.rb" "$evidence" >/dev/null

  cp "$evidence/release-manifest.json" "$work/release-manifest.original.json"
  sed 's/mvp-dev-2026-08-07/mvp-tampered-2026-08-07/' "$work/release-manifest.original.json" > "$evidence/release-manifest.json"
  if RELEASE_TRUSTED_PUBLIC_KEY="$public_key" ruby "$root/scripts/verify_release_evidence.rb" "$evidence" >/dev/null 2>&1; then
    echo "tampered signed release manifest was accepted" >&2
    exit 1
  fi

  cp "$work/release-manifest.original.json" "$evidence/release-manifest.json"
  cp "$evidence/provenance.intoto.json" "$work/provenance.original.json"
  sed 's/fiscalisation-release-evidence\/v1/fiscalisation-tampered\/v1/' "$work/provenance.original.json" > "$evidence/provenance.intoto.json"
  if RELEASE_TRUSTED_PUBLIC_KEY="$public_key" ruby "$root/scripts/verify_release_evidence.rb" "$evidence" >/dev/null 2>&1; then
    echo "tampered provenance was accepted" >&2
    exit 1
  fi

  ruby -rjson -e 'v=JSON.parse(File.read(ARGV[0])); v["summary"]["high"]=1; v["findings"]=[{"id"=>"CVE-TEST-HIGH","severity"=>"high"}]; File.write(ARGV[0],JSON.pretty_generate(v)+"\n")' "$report"
  RELEASE_CREATED_AT="$release_created_at" RELEASE_SIGNING_PRIVATE_KEY="$private_key" RELEASE_VULNERABILITY_REPORT="$report" ruby "$root/scripts/generate_release_evidence.rb" "$evidence" >/dev/null
  if RELEASE_TRUSTED_PUBLIC_KEY="$public_key" ruby "$root/scripts/verify_release_evidence.rb" "$evidence" >/dev/null 2>&1; then
    echo "high-severity vulnerability report was accepted" >&2
    exit 1
  fi

  ruby -rjson -e 'v=JSON.parse(File.read(ARGV[0])); v["summary"]["high"]=0; v["findings"]=[]; v["subject"]["sbom_sha256"]="0"*64; File.write(ARGV[0],JSON.pretty_generate(v)+"\n")' "$report"
  RELEASE_CREATED_AT="$release_created_at" RELEASE_SIGNING_PRIVATE_KEY="$private_key" RELEASE_VULNERABILITY_REPORT="$report" ruby "$root/scripts/generate_release_evidence.rb" "$evidence" >/dev/null
  if RELEASE_TRUSTED_PUBLIC_KEY="$public_key" ruby "$root/scripts/verify_release_evidence.rb" "$evidence" >/dev/null 2>&1; then
    echo "vulnerability report for another SBOM was accepted" >&2
    exit 1
  fi
}

test_signed_evidence_requires_trusted_ed25519_and_rejects_tamper

echo "release evidence signing OK: Ed25519 trust, provenance, SBOM-bound scan and tamper rejection"
