#!/usr/bin/env ruby
require 'json'
require 'openssl'
require 'digest'
require 'time'
require 'pathname'
require 'rubygems/version'

module HilEvidence
  class Invalid < StandardError; end
  module_function
  def check(condition, message)
    raise Invalid, message unless condition
  end
  def object(value, keys)
    check(value.is_a?(Hash) && value.keys.sort == keys.sort, "invalid schema: expected #{keys.join(',')}")
    value
  end
  def string(value)
    check(value.is_a?(String) && !value.strip.empty? && !value.include?("\0"), 'nonempty string required')
    value
  end
  def list(value)
    check(value.is_a?(Array) && !value.empty?, 'nonempty array required')
    value
  end
  def unique(values)
    check(values.uniq.size == values.size, 'duplicate inventory entry')
    values
  end
  def digest(value)
    check(value.is_a?(String) && value.match?(/\A[0-9a-f]{64}\z/), 'invalid SHA256')
    value
  end
  def instant(value)
    check(value.is_a?(String) && value.match?(/\A\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ\z/), 'UTC timestamp required')
    parsed = Time.iso8601(value)
    check(parsed.utc.strftime('%Y-%m-%dT%H:%M:%SZ') == value, 'invalid calendar timestamp')
    parsed
  end
  def inside?(path, root)
    path == root || path.start_with?(root + File::SEPARATOR)
  end
  def external(path, root)
    resolved = File.realpath(string(path))
    check(File.file?(resolved) && File.stat(resolved).nlink == 1 && !inside?(resolved, root), 'trust/requirements/key must be outside evidence tree')
    resolved
  end
  def json(raw)
    check(Gem::Version.new(JSON::VERSION) >= Gem::Version.new('2.18.0'), 'JSON >= 2.18 required for duplicate-key rejection')
    JSON.parse(raw, allow_duplicate_key: false)
  rescue JSON::ParserError => e
    raise Invalid, e.message
  end
  def inventory(values, key)
    list(values)
    values.each { |item| check(item.is_a?(Hash), 'inventory object required') }
    unique(values.map { |item| string(item.fetch(key)) })
    values.to_h { |item| [item.fetch(key), item] }
  end
  def artifact_path(root, path)
    string(path)
    check(!Pathname.new(path).absolute? && !path.include?('\\') && path.split('/').none? { |part| ['', '.', '..'].include?(part) }, 'unsafe artifact path')
    real = File.realpath(File.join(root, path))
    check(inside?(real, root) && File.file?(real), 'artifact escapes evidence tree')
    real
  end
  def verify(directory, env = ENV, now = Time.now.utc)
    check(OpenSSL::PKey.respond_to?(:generate_key), 'Ruby OpenSSL with Ed25519 support required')
    root = File.realpath(string(directory))
    check(File.directory?(root), 'evidence directory required')
    check(!File.exist?(File.join(root, 'trusted-reviewer-public.pem')), 'bundle-contained reviewer key forbidden')
    registry = json(File.binread(external(env['HIL_TRUST_REGISTRY'], root)))
    requirements = json(File.binread(external(env['HIL_REQUIREMENTS'], root)))
    object(registry, %w[schema_version reviewers]); check(registry['schema_version'] == 1, 'unsupported registry schema')
    reviewers = inventory(registry['reviewers'], 'reviewer_id')
    reviewers.each_value do |r|
      object(r, %w[reviewer_id public_key_path public_key_sha256 revoked valid_from valid_until])
      check([true, false].include?(r['revoked']), 'revoked must be boolean')
      digest(r['public_key_sha256']); check(instant(r['valid_from']) < instant(r['valid_until']), 'invalid reviewer validity')
    end
    raw = File.binread(artifact_path(root, 'hil-manifest.json'))
    manifest = json(raw)
    object(manifest, %w[schema_version evidence_type result reviewer_id source_commit issued_at valid_until devices artifacts cases])
    check(manifest['schema_version'] == 1 && manifest['evidence_type'] == 'PHYSICAL_HIL' && manifest['result'] == 'PASS', 'physical PASS evidence required')
    reviewer = reviewers.fetch(string(manifest['reviewer_id']))
    check(!reviewer['revoked'], 'reviewer revoked')
    registry_dir = File.dirname(File.realpath(env.fetch('HIL_TRUST_REGISTRY')))
    key_path = external(File.expand_path(string(reviewer['public_key_path']), registry_dir), root)
    key_raw = File.binread(key_path)
    check(key_raw.start_with?('-----BEGIN PUBLIC KEY-----'), 'public key PEM required')
    key = OpenSSL::PKey.read(key_raw)
    check(key.oid == 'ED25519', 'Ed25519 reviewer key required')
    check(Digest::SHA256.hexdigest(key.public_to_der) == reviewer['public_key_sha256'], 'reviewer fingerprint mismatch')
    signature = File.binread(artifact_path(root, 'hil-manifest.sig'))
    check(signature.bytesize == 64 && key.verify(nil, signature, raw), 'invalid exact manifest signature')
    issued = instant(manifest['issued_at']); expiry = instant(manifest['valid_until'])
    check(issued <= now && now < expiry && issued < expiry, 'expired/future HIL evidence')
    check(instant(reviewer['valid_from']) <= issued && now < instant(reviewer['valid_until']) && expiry <= instant(reviewer['valid_until']), 'reviewer validity does not cover evidence')
    object(requirements, %w[schema_version source_commit execution_valid_from execution_valid_until max_evidence_age_seconds device_profiles build_artifacts required_cases])
    check(requirements['schema_version'] == 1, 'unsupported requirements schema')
    check(string(requirements['source_commit']).match?(/\A[0-9a-f]{40}\z/) && manifest['source_commit'] == requirements['source_commit'], 'source commit mismatch')
    window_start = instant(requirements['execution_valid_from']); window_end = instant(requirements['execution_valid_until'])
    age = requirements['max_evidence_age_seconds']
    check(window_start < window_end && age.is_a?(Integer) && age > 0, 'invalid approved execution window/age')
    profiles = inventory(requirements['device_profiles'], 'profile_id')
    profiles.each_value do |p|
      object(p, %w[profile_id manufacturer model firmware_version serial_numbers])
      %w[manufacturer model firmware_version].each { |k| string(p[k]) }
      unique(list(p['serial_numbers'])); p['serial_numbers'].each { |v| string(v) }
    end
    devices = inventory(manifest['devices'], 'device_id')
    devices.each_value do |d|
      object(d, %w[device_id profile_id manufacturer model serial_number firmware_version])
      d.each_value { |v| string(v) }
      p = profiles.fetch(d['profile_id'])
      check(d['manufacturer'] == p['manufacturer'] && d['model'] == p['model'] && d['firmware_version'] == p['firmware_version'] && p['serial_numbers'].include?(d['serial_number']), 'device model/firmware mismatch')
      check(d.values.none? { |v| v.match?(/\b(simulator|stub|emulator)\b/i) }, 'physical device metadata required')
    end
    unique(devices.values.map { |d| [d['manufacturer'], d['model'], d['serial_number']] })
    artifacts = inventory(manifest['artifacts'], 'artifact_id')
    realpaths = artifacts.values.map do |a|
      object(a, %w[artifact_id path sha256 kind role]); digest(a['sha256']); string(a['role'])
      check(%w[BUILD LOG CAPTURE REPORT].include?(a['kind']), 'invalid artifact kind')
      path = artifact_path(root, a['path'])
      check(Digest::SHA256.file(path).hexdigest == a['sha256'], 'artifact hash mismatch')
      path
    end
    unique(realpaths)
    builds = inventory(requirements['build_artifacts'], 'artifact_id')
    builds.each_value do |b|
      object(b, %w[artifact_id sha256]); digest(b['sha256'])
      a = artifacts.fetch(b['artifact_id'])
      check(a['kind'] == 'BUILD' && a['sha256'] == b['sha256'], 'expected build mismatch')
    end
    expected = inventory(requirements['required_cases'], 'case_id')
    cases = inventory(manifest['cases'], 'case_id')
    check(cases.keys.sort == expected.keys.sort, 'case inventory mismatch')
    expected.each_value do |e|
      object(e, %w[case_id profile os transports device_profile_ids build_artifact_ids evidence_roles])
      %w[profile os].each { |k| string(e[k]) }
      %w[transports device_profile_ids build_artifact_ids].each { |k| unique(list(e[k])); e[k].each { |v| string(v) } }
      check((e['transports'] - %w[USB COM BLE MQTT HTTP]).empty?, 'unsupported transport')
      e['device_profile_ids'].each { |id| profiles.fetch(id) }; e['build_artifact_ids'].each { |id| builds.fetch(id) }
      roles = inventory(e['evidence_roles'], 'role')
      roles.each_value { |r| object(r, %w[role kind]); check(%w[LOG CAPTURE REPORT].include?(r['kind']), 'invalid required evidence kind') }
      c = cases.fetch(e['case_id'])
      object(c, %w[case_id profile os transports device_ids build_artifact_ids result started_at completed_at artifact_ids])
      check(c['result'] == 'PASS' && c['profile'] == e['profile'] && c['os'] == e['os'], 'case profile/OS/result mismatch')
      %w[transports device_ids build_artifact_ids artifact_ids].each { |k| unique(list(c[k])); c[k].each { |v| string(v) } }
      check(c['transports'].sort == e['transports'].sort && c['build_artifact_ids'].sort == e['build_artifact_ids'].sort, 'case transport/build mismatch')
      actual_profiles = c['device_ids'].map { |id| devices.fetch(id)['profile_id'] }
      check(actual_profiles.sort == e['device_profile_ids'].sort, 'case physical devices mismatch')
      evidence = c['artifact_ids'].map { |id| artifacts.fetch(id) }
      check(evidence.map { |a| [a['role'], a['kind']] }.sort == roles.values.map { |r| [r['role'], r['kind']] }.sort, 'case evidence role/kind mismatch')
      started = instant(c['started_at']); completed = instant(c['completed_at'])
      check(started <= completed && completed <= issued && started >= instant(reviewer['valid_from']) && completed <= instant(reviewer['valid_until']) && started >= window_start && completed <= window_end && started >= now - age, 'case outside approved reviewer/execution/age window')
    end
    true
  rescue KeyError, JSON::ParserError, SystemCallError, ArgumentError, OpenSSL::OpenSSLError => e
    raise Invalid, e.message
  end
end

if $PROGRAM_NAME == __FILE__
  begin
    HilEvidence.verify(ARGV[0].to_s)
    puts 'externally trusted physical HIL evidence verified (not production/legal approval)'
  rescue HilEvidence::Invalid => e
    warn "HIL evidence rejected: #{e.message}"
    exit 1
  end
end
