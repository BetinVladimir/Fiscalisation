require "minitest/autorun"
require "tmpdir"
require "fileutils"
require "open3"
require "rbconfig"
class ProductBoundaryFixtureTest < Minitest::Test
  ROOT=File.expand_path("../..",__dir__)
  def setup
    @dir=Dir.mktmpdir("boundary-fixture-")
    paths=%w[scripts/verify_product_boundary.rb compose.fiscalisation.yaml compose.minipos.yaml compose.minipos.prod.yaml compose.fiscalisation.e2e.yaml compose.minipos.e2e.yaml]
    paths+=Dir.glob(File.join(ROOT,"minipos/beeminipos-backend/{cmd,internal}/**/*.go")).map { |p| p.delete_prefix(ROOT+"/") }
    paths.each { |p| dest=File.join(@dir,p);FileUtils.mkdir_p(File.dirname(dest));FileUtils.cp(File.join(ROOT,p),dest) }
  end
  def teardown;FileUtils.remove_entry(@dir);end
  def change(path,from,to)
    p=File.join(@dir,path);raw=File.read(p);assert_includes raw,from;File.write(p,raw.sub(from,to))
  end
  def verify;Open3.capture3(RbConfig.ruby,File.join(@dir,"scripts/verify_product_boundary.rb"));end
  def test_current_external_base_and_isolated_e2e
    _,err,status=verify;assert status.success?,err
  end
  def test_missing_e2e_schema_bootstrap_fails
    change("compose.minipos.e2e.yaml","${MINIPOS_E2E_INIT_DIR:?run scripts/e2e-two-compose.sh}:/docker-entrypoint-initdb.d:ro","./empty:/docker-entrypoint-initdb.d:ro")
    _,err,status=verify;refute status.success?;assert_includes err,"database ownership drift"
  end
  def test_cross_product_e2e_schema_fails
    change("compose.minipos.e2e.yaml","minipos_db_socket:/var/run/postgresql","./database/fiscal:/foreign:ro")
    _,err,status=verify;refute status.success?;assert_includes err,"crossed into Fiscal"
  end
  def test_tenant_reader_cannot_use_owner
    change("compose.fiscalisation.yaml","postgres://beefiscal_tenant:","postgres://fiscal:")
    _,err,status=verify;refute status.success?;assert_includes err,"tenant reader role drift"
  end
end
