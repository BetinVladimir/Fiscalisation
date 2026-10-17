#!/usr/bin/env ruby
require_relative 'verify_legal_evidence'
require 'open3'
require 'rbconfig'
module ActivationEvidence
  V = HilEvidence
  module_function
  def verify(directory, env = ENV, now = Time.now.utc)
    root, manifest, requirements, reviewer = ExternalEvidence.signed(directory, env['ACTIVATION_TRUST_REGISTRY'], env['ACTIVATION_REQUIREMENTS'], 'bg014-activation.json', 'bg014-activation.sig', 'ACTIVATION_REVIEWER', %w[schema_version profile result reviewer_id issued_at valid_until subject proofs], now)
    V.object(requirements, %w[schema_version reviewer_organization subject]); V.check(requirements['schema_version'] == 1, 'unsupported activation requirements schema')
    V.check(manifest['profile'] == 'BG_SUPTO_FULL' && manifest['result'] == 'PASS', 'activation profile/decision invalid')
    ExternalEvidence.same_subject(manifest['subject'], requirements['subject'])
    V.check(reviewer['organization'] == requirements['reviewer_organization'] && reviewer['jurisdictions'].include?(manifest['subject']['jurisdiction']), 'activation organization/jurisdiction mismatch')
    V.object(manifest['proofs'], %w[hil_manifest_sha256 release_manifest_sha256 legal_manifest_sha256])
    hil_root = File.realpath(V.string(env['HIL_EVIDENCE_DIR'])); legal_root = File.realpath(V.string(env['LEGAL_EVIDENCE_DIR'])); release_root = File.realpath(V.string(env['RELEASE_EVIDENCE_DIR']))
    [[hil_root, 'hil-manifest.json', 'hil_manifest_sha256'], [legal_root, 'legal-acceptance.json', 'legal_manifest_sha256'], [release_root, 'release-manifest.json', 'release_manifest_sha256']].each do |dir, name, key|
      V.digest(manifest['proofs'][key]); V.check(Digest::SHA256.file(V.artifact_path(dir, name)).hexdigest == manifest['proofs'][key], 'activation proof manifest hash mismatch')
    end
    V.check(HilEvidence.verify(hil_root, env, now), 'HIL proof invalid')
    ExternalEvidence.same_subject(LegalEvidence.verify(legal_root, env, now), manifest['subject'])
    hil = V.json(File.binread(V.artifact_path(hil_root, 'hil-manifest.json')))
    V.check(hil['source_commit'] == manifest['subject']['source_commit'], 'HIL candidate source mismatch')
    builds = hil['artifacts'].select { |a| a['kind'] == 'BUILD' }.map { |a| {'artifact_id' => a['artifact_id'], 'sha256' => a['sha256']} }
    V.check(builds.sort_by { |a| a['artifact_id'] } == manifest['subject']['build_artifacts'].sort_by { |a| a['artifact_id'] }, 'HIL candidate build mismatch')
    release_path = V.artifact_path(release_root, 'release-manifest.json')
    release = V.json(File.binread(release_path))
    runtime = env['EVIDENCE_RUBY'] || env['HIL_RUBY'] || RbConfig.ruby
    out, err, status = Open3.capture3(env.to_h, runtime, File.join(__dir__, 'verify_release_evidence.rb'), release_root)
    V.check(status.success?, "release evidence verification failed: #{err}#{out}")
    V.check(release.dig('source', 'git_commit') == manifest['subject']['source_commit'], 'release candidate source mismatch')
    # Current verifier ONLY recognizes PROD_NO_GO and has no production subject
    # contract. Never infer production eligibility from a successful NO_GO check.
    raise V::Invalid, "production release contract unavailable (#{release['release_decision']}); activation remains blocked"
  rescue KeyError, JSON::ParserError, SystemCallError, ArgumentError, OpenSSL::OpenSSLError => e
    raise V::Invalid, e.message
  end
end
if $PROGRAM_NAME == __FILE__
  begin
    ActivationEvidence.verify(ARGV[0].to_s)
  rescue HilEvidence::Invalid => e
    warn "Activation evidence rejected: #{e.message}"; exit 1
  end
end
