#!/usr/bin/env ruby
require_relative 'external_evidence'
module LegalEvidence
  V = HilEvidence
  module_function
  def verify(directory, env = ENV, now = Time.now.utc)
    root, manifest, requirements, reviewer = ExternalEvidence.signed(directory, env['LEGAL_TRUST_REGISTRY'], env['LEGAL_REQUIREMENTS'], 'legal-acceptance.json', 'legal-acceptance.sig', 'LEGAL_REVIEWER', %w[schema_version result reviewer_id issued_at valid_until subject artifacts], now)
    V.object(requirements, %w[schema_version reviewer_organization subject]); V.check(requirements['schema_version'] == 1, 'unsupported legal requirements schema')
    V.check(manifest['result'] == 'ACCEPTED', 'external legal acceptance required')
    ExternalEvidence.same_subject(manifest['subject'], requirements['subject'])
    V.check(reviewer['organization'] == V.string(requirements['reviewer_organization']) && reviewer['organization'] != 'Beeloy' && reviewer['jurisdictions'].include?(manifest['subject']['jurisdiction']), 'external legal organization/jurisdiction mismatch')
    expected = (requirements['subject']['build_artifacts'] + requirements['subject']['dossier_artifacts']).to_h { |a| [a['artifact_id'], a['sha256']] }
    actual = V.inventory(manifest['artifacts'], 'artifact_id')
    V.check(actual.keys.sort == expected.keys.sort, 'legal artifact inventory mismatch')
    paths = actual.values.map do |a|
      V.object(a, %w[artifact_id path sha256]); V.digest(a['sha256'])
      path = V.artifact_path(root, a['path'])
      V.check(a['sha256'] == expected.fetch(a['artifact_id']) && Digest::SHA256.file(path).hexdigest == a['sha256'], 'legal artifact hash mismatch')
      path
    end
    V.unique(paths)
    manifest['subject']
  rescue KeyError, JSON::ParserError, SystemCallError, ArgumentError, OpenSSL::OpenSSLError => e
    raise V::Invalid, e.message
  end
end
if $PROGRAM_NAME == __FILE__
  begin
    LegalEvidence.verify(ARGV[0].to_s)
    puts 'externally trusted legal evidence verified (not production activation)'
  rescue HilEvidence::Invalid => e
    warn "Legal evidence rejected: #{e.message}"; exit 1
  end
end
