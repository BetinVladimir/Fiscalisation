require 'minitest/autorun'
require 'tmpdir'
require 'fileutils'
require 'open3'
require 'rbconfig'
require_relative '../verify_hil_evidence'

# Cryptographic verifier fixtures only. These are NOT physical HIL evidence.
class HilEvidenceTest < Minitest::Test
  def setup
    @tmp = Dir.mktmpdir('hil-verifier-unit-'); @bundle = File.join(@tmp, 'bundle'); Dir.mkdir(@bundle)
    @key = OpenSSL::PKey.generate_key('ED25519'); File.write(File.join(@tmp, 'reviewer.pem'), @key.public_to_pem)
    @now = Time.now.utc
    t = ->(offset) { (@now + offset).iso8601 }
    @registry = {schema_version: 1, reviewers: [{reviewer_id: 'unit-reviewer', public_key_path: 'reviewer.pem', public_key_sha256: Digest::SHA256.hexdigest(@key.public_to_der), revoked: false, valid_from: t[-3600], valid_until: t[7200]}]}
    File.write(File.join(@bundle, 'build.bin'), 'unit build'); File.write(File.join(@bundle, 'run.log'), 'unit execution fixture')
    hash = ->(f) { Digest::SHA256.file(File.join(@bundle, f)).hexdigest }
    @requirements = {schema_version: 1, source_commit: 'a' * 40, execution_valid_from: t[-600], execution_valid_until: t[600], max_evidence_age_seconds: 600, device_profiles: [{profile_id: 'fiscal', manufacturer: 'unit-vendor', model: 'unit-model', firmware_version: '1.2', serial_numbers: ['fixture-123']}], build_artifacts: [{artifact_id: 'app', sha256: hash['build.bin']}], required_cases: [{case_id: 'lost-response', profile: 'iot-datecs', os: 'ANDROID_16', transports: ['COM', 'BLE'], device_profile_ids: ['fiscal'], build_artifact_ids: ['app'], evidence_roles: [{role: 'COM_CAPTURE', kind: 'CAPTURE'}]}]}
    @manifest = {schema_version: 1, evidence_type: 'PHYSICAL_HIL', result: 'PASS', reviewer_id: 'unit-reviewer', source_commit: 'a' * 40, issued_at: t[-10], valid_until: t[3600], devices: [{device_id: 'device1', profile_id: 'fiscal', manufacturer: 'unit-vendor', model: 'unit-model', serial_number: 'fixture-123', firmware_version: '1.2'}], artifacts: [{artifact_id: 'app', path: 'build.bin', sha256: hash['build.bin'], kind: 'BUILD', role: 'BUILD'}, {artifact_id: 'log', path: 'run.log', sha256: hash['run.log'], kind: 'CAPTURE', role: 'COM_CAPTURE'}], cases: [{case_id: 'lost-response', profile: 'iot-datecs', os: 'ANDROID_16', transports: ['COM', 'BLE'], device_ids: ['device1'], build_artifact_ids: ['app'], result: 'PASS', started_at: t[-30], completed_at: t[-20], artifact_ids: ['log']}]}
    @env = {'HIL_TRUST_REGISTRY' => File.join(@tmp, 'registry.json'), 'HIL_REQUIREMENTS' => File.join(@tmp, 'requirements.json')}
  end
  def teardown
    FileUtils.remove_entry(@tmp)
  end
  def write_fixture
    File.write(@env['HIL_TRUST_REGISTRY'], JSON.generate(@registry)); File.write(@env['HIL_REQUIREMENTS'], JSON.generate(@requirements))
    raw = JSON.generate(@manifest); File.write(File.join(@bundle, 'hil-manifest.json'), raw)
    File.binwrite(File.join(@bundle, 'hil-manifest.sig'), @key.sign(nil, raw))
  end
  def verify
    write_fixture
    HilEvidence.verify(@bundle, @env, @now)
  end
  def reject
    assert_raises(HilEvidence::Invalid) { verify }
  end
  def test_hardlinked_external_inputs_rejected
    write_fixture
    File.link(@env['HIL_TRUST_REGISTRY'], File.join(@bundle, 'registry-alias'))
    assert_raises(HilEvidence::Invalid) { HilEvidence.verify(@bundle, @env, @now) }
  end
  def test_hardlinked_external_key_rejected
    File.link(File.join(@tmp, 'reviewer.pem'), File.join(@bundle, 'key-alias')); reject
  end
  def test_symlink_external_input_alias_into_bundle_rejected
    write_fixture
    File.write(File.join(@bundle, 'inside.json'), File.read(@env['HIL_TRUST_REGISTRY']))
    File.delete(@env['HIL_TRUST_REGISTRY']); File.symlink(File.join(@bundle, 'inside.json'), @env['HIL_TRUST_REGISTRY'])
    assert_raises(HilEvidence::Invalid) { HilEvidence.verify(@bundle, @env, @now) }
  end
  def test_unapproved_manufacturer_and_serial
    @manifest[:devices][0][:manufacturer] = 'foreign'; reject
    @manifest[:devices][0][:manufacturer] = 'unit-vendor'; @manifest[:devices][0][:serial_number] = 'unapproved'; reject
  end
  def test_execution_before_reviewer_authority
    @registry[:reviewers][0][:valid_from] = (@now - 15).iso8601; reject
  end
  def test_execution_outside_plan_window_or_age
    @requirements[:execution_valid_from] = (@now - 25).iso8601; reject
    @requirements[:execution_valid_from] = (@now - 600).iso8601
    @requirements[:max_evidence_age_seconds] = 25; reject
  end
  def test_calendar_normalization_rejected
    assert_raises(HilEvidence::Invalid) { HilEvidence.instant('2026-02-30T00:00:00Z') }
  end
  def test_generic_log_cannot_replace_capture
    @manifest[:artifacts][1][:kind] = 'LOG'; reject
    @manifest[:artifacts][1][:kind] = 'CAPTURE'; @manifest[:artifacts][1][:role] = 'GENERIC'; reject
  end
  def test_full_acceptance_uses_selected_hil_runtime_and_blocks_without_release
    write_fixture
    fixture_root = File.join(@tmp, 'full-gate'); FileUtils.mkdir_p(File.join(fixture_root, 'scripts')); FileUtils.mkdir_p(File.join(fixture_root, 'contracts'))
    %w[verify_supto_full_acceptance.rb verify_hil_evidence.rb].each { |f| FileUtils.cp(File.expand_path("../#{f}", __dir__), File.join(fixture_root, 'scripts', f)) }
    File.write(File.join(fixture_root, 'contracts', 'supto-annex29-trace.json'), JSON.generate(requirements: []))
    marker = File.join(@tmp, 'selected-runtime'); wrapper = File.join(@tmp, 'hil-ruby')
    File.write(wrapper, "#!#{RbConfig.ruby}\nrequire 'rbconfig'\nFile.write(#{marker.inspect}, RbConfig.ruby)\nexec RbConfig.ruby, *ARGV\n"); File.chmod(0700, wrapper)
    env = @env.merge('HIL_EVIDENCE_DIR' => @bundle, 'RELEASE_EVIDENCE_DIR' => nil, 'LEGAL_EVIDENCE_DIR' => nil, 'HIL_RUBY' => wrapper)
    out, err, status = Open3.capture3(env, RbConfig.ruby, File.join(fixture_root, 'scripts', 'verify_supto_full_acceptance.rb'))
    refute status.success?; assert_equal RbConfig.ruby, File.read(marker)
    assert_includes out, 'externally trusted physical HIL evidence verified'
    assert_includes err, 'RELEASE_EVIDENCE_DIR is required'
    refute_includes out, 'full acceptance PASS'
    # Omission inherits the running interpreter, not the macOS system ruby.
    env['HIL_RUBY'] = nil
    out, err, status = Open3.capture3(env, RbConfig.ruby, File.join(fixture_root, 'scripts', 'verify_supto_full_acceptance.rb'))
    refute status.success?; assert_includes out, 'externally trusted physical HIL evidence verified'; assert_includes err, 'RELEASE_EVIDENCE_DIR is required'
  end
  def test_valid_external_registry_and_complete_matrix
    assert verify
  end
  def test_self_signed_bundle_rejected
    File.write(File.join(@bundle, 'trusted-reviewer-public.pem'), @key.public_to_pem); reject
  end
  def test_registry_inside_bundle_rejected
    @env['HIL_TRUST_REGISTRY'] = File.join(@bundle, 'registry.json'); reject
  end
  def test_requirements_inside_bundle_rejected
    @env['HIL_REQUIREMENTS'] = File.join(@bundle, 'requirements.json'); reject
  end
  def test_key_inside_bundle_rejected
    File.write(File.join(@bundle, 'key.pem'), @key.public_to_pem); @registry[:reviewers][0][:public_key_path] = File.join(@bundle, 'key.pem'); reject
  end
  def test_foreign_signing_key
    @key = OpenSSL::PKey.generate_key('ED25519'); reject
  end
  def test_revoked_reviewer
    @registry[:reviewers][0][:revoked] = true; reject
  end
  def test_wrong_fingerprint
    @registry[:reviewers][0][:public_key_sha256] = '0' * 64; reject
  end
  def test_omitted_case
    @manifest[:cases] = []; reject
  end
  def test_duplicate_case
    @manifest[:cases] *= 2; reject
  end
  def test_wrong_build
    @requirements[:build_artifacts][0][:sha256] = '0' * 64; reject
  end
  def test_wrong_device_firmware
    @manifest[:devices][0][:firmware_version] = 'wrong'; reject
  end
  def test_wrong_model
    @manifest[:devices][0][:model] = 'wrong'; reject
  end
  def test_wrong_source
    @manifest[:source_commit] = 'b' * 40; reject
  end
  def test_wrong_transport_and_os
    @manifest[:cases][0][:transports] = ['USB']; reject
    @manifest[:cases][0][:transports] = ['COM', 'BLE']; @manifest[:cases][0][:os] = 'IOS_18'; reject
  end
  def test_expired
    @manifest[:valid_until] = (@now - 1).iso8601; reject
  end
  def test_expired_reviewer
    @registry[:reviewers][0][:valid_until] = (@now - 1).iso8601; reject
  end
  def test_artifact_tamper
    File.write(File.join(@bundle, 'run.log'), 'tampered'); reject
  end
  def test_path_traversal
    @manifest[:artifacts][1][:path] = '../reviewer.pem'; reject
  end
  def test_symlink_escape
    File.symlink(File.join(@tmp, 'reviewer.pem'), File.join(@bundle, 'outside')); @manifest[:artifacts][1][:path] = 'outside'; reject
  end
  def test_unknown_manifest_field
    @manifest[:approval] = true; reject
  end
  def test_exact_raw_signature_and_duplicate_json_keys
    write_fixture
    File.open(File.join(@bundle, 'hil-manifest.json'), 'a') { |f| f.write("\n") }
    assert_raises(HilEvidence::Invalid) { HilEvidence.verify(@bundle, @env, @now) }
    assert_raises(HilEvidence::Invalid) { HilEvidence.json('{"x":1,"x":2}') }
  end
  def test_simulator_metadata
    @manifest[:devices][0][:serial_number] = 'SIMULATOR'; reject
  end
  def test_missing_external_inputs_cli_fails_closed
    write_fixture
    require 'open3'
    _, err, status = Open3.capture3({'HIL_TRUST_REGISTRY' => nil, 'HIL_REQUIREMENTS' => nil}, RbConfig.ruby, File.expand_path('../verify_hil_evidence.rb', __dir__), @bundle)
    refute status.success?; assert_includes err, 'HIL evidence rejected'
  end
end
