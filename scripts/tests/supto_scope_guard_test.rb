require "minitest/autorun"
require "tmpdir"
require "fileutils"
require "json"
require "open3"
require "rbconfig"

class SuptoScopeGuardTest < Minitest::Test
  ROOT = File.expand_path("../..", __dir__)
  def setup
    @dir = Dir.mktmpdir("supto-scope-")
    paths = %w[scripts/verify_supto_trace.rb contracts/supto-annex29-trace.json contracts/supto-feature-scope.json contracts/bg-requirements-trace.json]
    trace = JSON.parse(File.read(File.join(ROOT, paths[1])))
    paths += trace.fetch("requirements").flat_map { |row| row.fetch("evidence") }
    paths += Dir.glob(File.join(ROOT, "docs/decisions/ADR-SUPTO-*.md")).map { |p| p.delete_prefix(ROOT + "/") }
    paths += Dir.glob(File.join(ROOT, "minipos/BeeMiniPOS/{App.tsx,src/**/*.ts}")).map { |p| p.delete_prefix(ROOT + "/") }
    paths.uniq.each do |p|
      target = File.join(@dir, p)
      FileUtils.mkdir_p(File.dirname(target))
      FileUtils.cp(File.join(ROOT, p), target)
    end
  end
  def teardown
    FileUtils.remove_entry(@dir)
  end
  def mutate(path)
    p = File.join(@dir, path)
    data = JSON.parse(File.read(p))
    yield data
    File.write(p, JSON.generate(data))
  end
  def verify
    Open3.capture3(RbConfig.ruby, File.join(@dir, "scripts/verify_supto_trace.rb"))
  end
  def test_honest_open_scope_is_consistent
    out, err, status = verify
    assert status.success?, err
    assert_includes out, "OPEN_SOFTWARE_GAPS"
    assert_includes out, "FAIL=1"
  end
  def test_electronic_receipt_cannot_be_removed
    mutate("contracts/supto-feature-scope.json") { |d| d["features"]["electronic_fiscal_receipts"]["in_product_scope"] = false }
    _, err, status = verify
    refute status.success?
    assert_includes err, "silently removed"
  end
  def test_unimplemented_feature_cannot_claim_implemented
    mutate("contracts/supto-feature-scope.json") { |d| d["features"]["electronic_fiscal_receipts"]["status"] = "SOFTWARE_IMPLEMENTED_HIL_REQUIRED" }
    _, err, status = verify
    refute status.success?
    assert_includes err, "production block"
  end
  def test_annex_gap_cannot_become_not_applicable
    mutate("contracts/supto-annex29-trace.json") { |d| d["requirements"][23]["status"] = "NOT_APPLICABLE" }
    _, err, status = verify
    refute status.success?
    assert_includes err, "scope mismatch"
  end
  def test_gap_cannot_lose_production_block
    mutate("contracts/supto-annex29-trace.json") { |d| d["requirements"][23]["production_blocked"] = false }
    _, err, status = verify
    refute status.success?
    assert_includes err, "not production-blocked"
  end
end
