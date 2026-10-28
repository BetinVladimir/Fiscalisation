require "minitest/autorun"
require "tmpdir"
require_relative "verify_ui_acceptance"

class UiAcceptanceVerifierTest < Minitest::Test
  def setup
    @root = Dir.mktmpdir("ui-matrix-")
    @matrix = {"platforms"=>%w[android ios web], "cases"=>UiAcceptanceVerifier::CATEGORIES.each_with_index.map do |category,i|
      {"id"=>"CASE-#{i}","app"=>%w[BeeMiniPOS BeeFiscalApp both][i%3],"category"=>category,"source"=>"App.tsx","anchors"=>["<Child onAction={perform} />"],"invariant"=>"The owner gate and action callback remain wired to the actual implementation."}
    end}
    File.write(File.join(@root,"App.tsx"),"<Child onAction={perform} />")
    @matrix['cases'][0]['evidence']=[{'source'=>'Child.tsx','anchors'=>['allowed.includes(action)'],'forbidden'=>['repeatPayment()']}]
    File.write(File.join(@root,"Child.tsx"),'allowed.includes(action)')
  end
  def teardown
    FileUtils.remove_entry(@root)
  end
  def errors
    UiAcceptanceVerifier.new(@root,@matrix).errors
  end
  def test_split_source_and_caller_wiring_pass
    assert_empty errors
  end
  def test_extracted_implementation_cannot_replace_missing_caller_wiring
    File.write(File.join(@root,"App.tsx"),'<Child />')
    assert errors.any?{|e|e.include?('CASE-0: App.tsx: missing anchor')}
  end
  def test_extracted_required_guard_is_independently_required
    File.write(File.join(@root,"Child.tsx"),'render action')
    assert errors.any?{|e|e.include?('CASE-0: Child.tsx: missing anchor')}
  end
  def test_forbidden_repeat_remains_rejected_in_extracted_module
    File.write(File.join(@root,"Child.tsx"),'allowed.includes(action); repeatPayment()')
    assert errors.any?{|e|e.include?('CASE-0: Child.tsx: forbidden anchor')}
  end
  def test_open_gap_never_becomes_static_acceptance_success
    @matrix['cases'][0].merge!('status'=>'OPEN','gap'=>'The command response still lacks authenticated device proof.')
    assert errors.any?{|e|e.include?('CASE-0: OPEN:')}
  end
  def test_unknown_status_cannot_suppress_open_gap
    @matrix['cases'][0]['status']='IGNORE'
    assert errors.any?{|e|e.include?('unsupported status')}
  end
  def test_all_missing_case_evidence_is_reported
    @matrix['cases'][0]['source']='missing-one.tsx'
    @matrix['cases'][1]['source']='missing-two.tsx'
    assert errors.any?{|e|e.include?('CASE-0: source missing')}
    assert errors.any?{|e|e.include?('CASE-1: source missing')}
  end
  def test_empty_required_anchors_are_rejected
    @matrix['cases'][0]['anchors']=[]
    assert errors.any?{|e|e.include?('needs nonempty required anchors')}
  end
  def test_source_cannot_escape_repository
    @matrix['cases'][0]['source']='../external.tsx'
    assert errors.any?{|e|e.include?('source outside repository')}
  end
  def test_existing_global_requirements_remain_required
    @matrix['platforms']=['web'];@matrix['cases'].pop
    out=errors
    assert out.any?{|e|e.include?('all platforms')}
    assert out.any?{|e|e.include?('at least 12 cases')}
    assert out.any?{|e|e.include?('missing category interaction_e2e')}
  end
end
