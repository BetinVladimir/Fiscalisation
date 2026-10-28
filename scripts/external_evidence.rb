# Shared fail-closed external reviewer attestation. No production decision here.
require_relative 'verify_hil_evidence'
module ExternalEvidence
  V = HilEvidence
  module_function
  def signed(directory, registry_path, requirements_path, filename, signature_name, role, expected_keys, now)
    root = File.realpath(V.string(directory))
    registry_file = V.external(registry_path, root)
    requirements = V.json(File.binread(V.external(requirements_path, root)))
    registry = V.json(File.binread(registry_file))
    V.object(registry, %w[schema_version reviewers]); V.check(registry['schema_version'] == 1, 'unsupported external registry schema')
    reviewers = V.inventory(registry['reviewers'], 'reviewer_id')
    reviewers.each_value do |r|
      V.object(r, %w[reviewer_id organization roles jurisdictions public_key_path public_key_sha256 revoked valid_from valid_until])
      V.string(r['organization']); V.digest(r['public_key_sha256'])
      %w[roles jurisdictions].each { |k| V.unique(V.list(r[k])); r[k].each { |v| V.string(v) } }
      V.check([true, false].include?(r['revoked']) && V.instant(r['valid_from']) < V.instant(r['valid_until']), 'invalid reviewer validity/revocation')
    end
    V.check(!File.exist?(File.join(root, 'trusted-legal-reviewer-public.pem')) && !File.exist?(File.join(root, 'trusted-activation-reviewer-public.pem')), 'packaged authority key forbidden')
    raw = File.binread(V.artifact_path(root, filename)); manifest = V.json(raw)
    V.object(manifest, expected_keys)
    V.check(manifest['schema_version'] == 1, 'unsupported signed manifest schema')
    reviewer = reviewers.fetch(V.string(manifest['reviewer_id']))
    V.check(!reviewer['revoked'] && reviewer['roles'].include?(role), 'reviewer revoked or unauthorized role')
    key_raw = File.binread(V.external(File.expand_path(V.string(reviewer['public_key_path']), File.dirname(registry_file)), root))
    V.check(key_raw.start_with?('-----BEGIN PUBLIC KEY-----'), 'public key PEM required')
    key = OpenSSL::PKey.read(key_raw)
    V.check(key.oid == 'ED25519' && Digest::SHA256.hexdigest(key.public_to_der) == reviewer['public_key_sha256'], 'external reviewer key mismatch')
    signature = File.binread(V.artifact_path(root, signature_name))
    V.check(signature.bytesize == 64 && key.verify(nil, signature, raw), 'invalid exact external signature')
    issued = V.instant(manifest['issued_at']); expiry = V.instant(manifest['valid_until'])
    V.check(V.instant(reviewer['valid_from']) <= issued && issued <= now && now < expiry && issued < expiry && expiry <= V.instant(reviewer['valid_until']), 'expired/future external attestation')
    [root, manifest, requirements, reviewer]
  end
  def subject(value)
    V.object(value, %w[jurisdiction normative_baseline product_version source_commit build_artifacts dossier_artifacts])
    %w[jurisdiction normative_baseline product_version].each { |k| V.string(value[k]) }
    V.check(V.string(value['source_commit']).match?(/\A[0-9a-f]{40}\z/), 'full source commit required')
    ids = []
    %w[build_artifacts dossier_artifacts].each do |kind|
      inventory = V.inventory(value[kind], 'artifact_id')
      inventory.each_value { |a| V.object(a, %w[artifact_id sha256]); V.digest(a['sha256']); ids << a['artifact_id'] }
    end
    V.unique(ids)
    value
  end
  def same_subject(left, right)
    subject(left); subject(right)
    %w[jurisdiction normative_baseline product_version source_commit].each { |k| V.check(left[k] == right[k], "subject #{k} mismatch") }
    %w[build_artifacts dossier_artifacts].each do |k|
      V.check(left[k].sort_by { |a| a['artifact_id'] } == right[k].sort_by { |a| a['artifact_id'] }, "subject #{k} mismatch")
    end
  end
end
