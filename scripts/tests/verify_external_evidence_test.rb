# All keys/evidence here are ephemeral verifier fixtures, never real approvals.
require_relative 'verify_hil_evidence_test'
require_relative '../verify_legal_evidence'
require_relative '../verify_activation_evidence'
class ExternalEvidenceTest < Minitest::Test
  def setup
    @hil = HilEvidenceTest.new('fixture_only'); @hil.setup; @hil.write_fixture
    @tmp = @hil.instance_variable_get(:@tmp); @now = @hil.instance_variable_get(:@now); @key = @hil.instance_variable_get(:@key)
    @env = @hil.instance_variable_get(:@env).dup; @env['HIL_EVIDENCE_DIR'] = @hil.instance_variable_get(:@bundle)
    @legal = File.join(@tmp, 'legal'); @activation = File.join(@tmp, 'activation'); @release = File.join(@tmp, 'release'); [@legal,@activation,@release].each { |d| Dir.mkdir(d) }
    t = ->(offset) { (@now + offset).iso8601 }
    @registry = {schema_version: 1, reviewers: [{reviewer_id: 'external', organization: 'Approved Independent Lab', roles: ['LEGAL_REVIEWER','ACTIVATION_REVIEWER'], jurisdictions: ['BG'], public_key_path: 'reviewer.pem', public_key_sha256: Digest::SHA256.hexdigest(@key.public_to_der), revoked: false, valid_from: t[-3600], valid_until: t[7200]}]}
    @subject = {jurisdiction: 'BG', normative_baseline: 'approved-baseline-2026', product_version: 'unit-v1', source_commit: 'a'*40, build_artifacts: [{artifact_id: 'app', sha256: Digest::SHA256.hexdigest('unit build')}], dossier_artifacts: [{artifact_id: 'dossier', sha256: Digest::SHA256.hexdigest('unit dossier')}]}
    @requirements = {schema_version: 1, reviewer_organization: 'Approved Independent Lab', subject: @subject}
    @manifest = {schema_version: 1, result: 'ACCEPTED', reviewer_id: 'external', issued_at: t[-5], valid_until: t[3600], subject: @subject, artifacts: [{artifact_id: 'app', path: 'build.bin', sha256: Digest::SHA256.hexdigest('unit build')}, {artifact_id: 'dossier', path: 'dossier.txt', sha256: Digest::SHA256.hexdigest('unit dossier')}]}
    File.write(File.join(@legal,'build.bin'),'unit build'); File.write(File.join(@legal,'dossier.txt'),'unit dossier')
    @env.merge!('LEGAL_TRUST_REGISTRY'=>File.join(@tmp,'external-registry.json'),'LEGAL_REQUIREMENTS'=>File.join(@tmp,'legal-plan.json'),'ACTIVATION_TRUST_REGISTRY'=>File.join(@tmp,'external-registry.json'),'ACTIVATION_REQUIREMENTS'=>File.join(@tmp,'activation-plan.json'),'LEGAL_EVIDENCE_DIR'=>@legal,'RELEASE_EVIDENCE_DIR'=>@release)
    @activation_manifest = {schema_version: 1, profile: 'BG_SUPTO_FULL', result: 'PASS', reviewer_id: 'external', issued_at: t[-5], valid_until: t[3600], subject: @subject, proofs: {}}
  end
  def teardown
    @hil.teardown
  end
  def write_fixture
    File.write(@env['LEGAL_TRUST_REGISTRY'],JSON.generate(@registry)); File.write(@env['LEGAL_REQUIREMENTS'],JSON.generate(@requirements)); File.write(@env['ACTIVATION_REQUIREMENTS'],JSON.generate(@requirements))
    raw = JSON.generate(@manifest); File.write(File.join(@legal,'legal-acceptance.json'),raw); File.binwrite(File.join(@legal,'legal-acceptance.sig'),@key.sign(nil,raw))
  end
  def verify
    write_fixture; LegalEvidence.verify(@legal,@env,@now)
  end
  def reject
    assert_raises(HilEvidence::Invalid) { verify }
  end
  def test_valid_independently_pinned_legal_subject
    assert_equal JSON.parse(JSON.generate(@subject)), verify
  end
  def test_packaged_selftrust_key_rejected
    File.write(File.join(@legal,'trusted-legal-reviewer-public.pem'),@key.public_to_pem); reject
  end
  def test_forged_organization_cannot_override_registry
    @manifest[:reviewer_organization]='Fake'; reject
  end
  def test_registry_wrong_organization_or_jurisdiction
    @registry[:reviewers][0][:organization]='Foreign'; reject
    @registry[:reviewers][0][:organization]='Approved Independent Lab'; @registry[:reviewers][0][:jurisdictions]=['DE']; reject
  end
  def test_revoked_foreign_and_wrongrole
    @registry[:reviewers][0][:revoked]=true; reject
    @registry[:reviewers][0][:revoked]=false; @registry[:reviewers][0][:roles]=['ACTIVATION_REVIEWER']; reject
    @registry[:reviewers][0][:roles]=['LEGAL_REVIEWER']; @key=OpenSSL::PKey.generate_key('ED25519'); reject
  end
  def test_external_hardlink_rejected
    write_fixture; File.link(@env['LEGAL_REQUIREMENTS'],File.join(@legal,'plan-alias')); reject
  end
  def test_expired_and_invalid_calendar
    @manifest[:valid_until]=(@now-1).iso8601; reject
    @manifest[:valid_until]='2026-02-30T12:00:00Z'; reject
  end
  def test_wrong_candidate_baseline_and_version
    @manifest[:subject]=Marshal.load(Marshal.dump(@subject)); @manifest[:subject][:source_commit]='b'*40; reject
    @manifest[:subject][:source_commit]='a'*40; @manifest[:subject][:normative_baseline]='foreign'; reject
    @manifest[:subject][:normative_baseline]=@subject[:normative_baseline]; @manifest[:subject][:product_version]='v-other'; reject
  end
  def test_omission_duplicate_and_artifact_tamper
    @manifest[:artifacts].pop; reject
    @manifest[:artifacts] *= 2; reject
  end
  def test_dossier_tamper_and_path_escape
    File.write(File.join(@legal,'dossier.txt'),'tampered'); reject
    @manifest[:artifacts][1][:path]='../reviewer.pem'; reject
  end
  def make_release_no_go
    sbom={bomFormat:'CycloneDX',specVersion:'1.6',components:(1..20).map { |i| {purl:"pkg:generic/unit-#{i}"} }}
    File.write(File.join(@release,'sbom.cdx.json'),JSON.generate(sbom)); hash=Digest::SHA256.file(File.join(@release,'sbom.cdx.json')).hexdigest
    provenance={_type:'https://in-toto.io/Statement/v1',predicateType:'https://slsa.dev/provenance/v1',subject:[{name:'sbom.cdx.json',digest:{sha256:hash}}],predicate:{buildDefinition:{resolvedDependencies:[{digest:{gitCommit:'a'*40}}]}}}
    File.write(File.join(@release,'provenance.intoto.json'),JSON.generate(provenance))
    release={release_decision:'PROD_NO_GO',source:{git_commit:'a'*40},verification:{signature_status:'UNSIGNED_NO_GO',vulnerability_status:'REQUIRES_RELEASE_SCAN'},no_go_reasons:['release artifacts are not signed'],artifacts:{sbom_sha256:hash,provenance:'provenance.intoto.json',provenance_sha256:Digest::SHA256.file(File.join(@release,'provenance.intoto.json')).hexdigest}}
    File.write(File.join(@release,'release-manifest.json'),JSON.generate(release))
    File.write(File.join(@release,'checksums.sha256'),%w[sbom.cdx.json provenance.intoto.json release-manifest.json].map { |f| "#{Digest::SHA256.file(File.join(@release,f)).hexdigest}  #{f}\n" }.join)
  end
  def activation_fixture
    write_fixture; make_release_no_go
    @activation_manifest[:proofs]={hil_manifest_sha256:Digest::SHA256.file(File.join(@env['HIL_EVIDENCE_DIR'],'hil-manifest.json')).hexdigest,legal_manifest_sha256:Digest::SHA256.file(File.join(@legal,'legal-acceptance.json')).hexdigest,release_manifest_sha256:Digest::SHA256.file(File.join(@release,'release-manifest.json')).hexdigest}
  end
  def activate
    raw=JSON.generate(@activation_manifest);File.write(File.join(@activation,'bg014-activation.json'),raw);File.binwrite(File.join(@activation,'bg014-activation.sig'),@key.sign(nil,raw))
    ActivationEvidence.verify(@activation,@env,@now)
  end
  def test_real_verified_no_go_release_cannot_activate
    activation_fixture
    _,err,status=Open3.capture3(RbConfig.ruby,File.expand_path('../verify_release_evidence.rb',__dir__),@release)
    assert status.success?,err
    error=assert_raises(HilEvidence::Invalid){activate};assert_includes error.message,'production release contract unavailable (PROD_NO_GO)'
  end
  def test_activation_requires_exact_proof_hashes
    activation_fixture; @activation_manifest[:proofs][:hil_manifest_sha256]='0'*64
    error=assert_raises(HilEvidence::Invalid){activate};assert_includes error.message,'proof manifest hash mismatch'
  end
  def test_activation_legal_candidate_mismatch
    activation_fixture; @activation_manifest[:subject]=Marshal.load(Marshal.dump(@subject));@activation_manifest[:subject][:product_version]='other'
    assert_raises(HilEvidence::Invalid){activate}
  end
  def test_activation_packaged_authority_rejected
    activation_fixture;File.write(File.join(@activation,'trusted-activation-reviewer-public.pem'),@key.public_to_pem)
    assert_raises(HilEvidence::Invalid){activate}
  end
  def test_actual_full_gate_cannot_pass_valid_no_go_candidate
    activation_fixture; assert_raises(HilEvidence::Invalid) { activate }
    root=File.join(@tmp,'full-external-gate');FileUtils.mkdir_p(File.join(root,'scripts'));FileUtils.mkdir_p(File.join(root,'contracts'))
    %w[verify_supto_full_acceptance.rb verify_hil_evidence.rb verify_legal_evidence.rb verify_activation_evidence.rb external_evidence.rb verify_release_evidence.rb release-signature.mjs].each { |f| FileUtils.cp(File.expand_path("../#{f}",__dir__),File.join(root,'scripts',f)) }
    File.write(File.join(root,'contracts','supto-annex29-trace.json'),JSON.generate(requirements:[]))
    out,err,status=Open3.capture3(@env.merge('HIL_RUBY'=>RbConfig.ruby,'ACTIVATION_EVIDENCE_DIR'=>@activation),RbConfig.ruby,File.join(root,'scripts','verify_supto_full_acceptance.rb'))
    refute status.success?;assert_includes out,'PROD_NO_GO enforced';assert_includes out,'externally trusted legal evidence verified';assert_includes err,'production release contract unavailable';refute_includes out,'full acceptance PASS'
  end

end
