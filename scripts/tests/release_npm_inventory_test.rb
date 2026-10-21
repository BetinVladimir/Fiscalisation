#!/usr/bin/env ruby
require 'minitest/autorun'
require 'tmpdir'
require 'fileutils'
require 'json'
require 'open3'
require 'digest'
require 'rbconfig'

class ReleaseNpmInventoryTest < Minitest::Test
  LOCKS = %w[BeeFiscalApp/package-lock.json minipos/BeeMiniPOS/package-lock.json minipos/miniposweb/package-lock.json].freeze
  def setup
    @root=File.realpath(Dir.mktmpdir('release-inventory-'))
    FileUtils.mkdir_p(File.join(@root,'scripts'))
    FileUtils.cp(File.expand_path('../generate_release_evidence.rb',__dir__),File.join(@root,'scripts/generate_release_evidence.rb'))
    %w[fiscal-backend/go.sum minipos/beeminipos-backend/go.sum].each { |f| write(f,'example.test/library v1.0.0 hash') }
    LOCKS.each_with_index { |f,i| lock(f,{"node_modules/client-#{i}"=>{'version'=>'1.0.0'}}) }
  end
  def teardown; FileUtils.remove_entry(@root);end
  def write(f,body);p=File.join(@root,f);FileUtils.mkdir_p(File.dirname(p));File.write(p,body);end
  def lock(f,packages,version=3);write(f,JSON.generate({'lockfileVersion'=>version,'packages'=>packages}));end
  def generate
    _,err,status=Open3.capture3({'RELEASE_CREATED_AT'=>'2026-10-21T00:00:00Z'},RbConfig.ruby,File.join(@root,'scripts/generate_release_evidence.rb'),File.join(@root,'evidence'))
    [status,err]
  end
  def sbom;JSON.parse(File.read(File.join(@root,'evidence/sbom.cdx.json')));end
  def test_all_three_clients_and_exact_input_hashes
    status,err=generate;assert status.success?,err
    assert_equal %w[client-0 client-1 client-2],sbom['components'].select { |c| c['purl'].start_with?('pkg:npm/') }.map { |c| c['name'] }.sort
    manifest=JSON.parse(File.read(File.join(@root,'evidence/release-manifest.json')))
    LOCKS.each { |f| assert_equal Digest::SHA256.file(File.join(@root,f)).hexdigest,manifest.dig('artifacts','file_hashes',f) }
    assert_equal 'PROD_NO_GO',manifest['release_decision']
    assert_equal 'UNSIGNED_NO_GO',manifest.dig('verification','signature_status')
    assert_includes sbom.dig('metadata','properties'),{'name'=>'beeloy:inventory:coverage','value'=>'PARTIAL_LOCKFILE_INVENTORY'}
  end
  def test_nested_scoped_alias_and_version_identity
    lock(LOCKS.first,{
      'node_modules/parent/node_modules/@scope/leaf'=>{'version'=>'2.0.0'},
      'node_modules/@scope/leaf'=>{'version'=>'2.0.0'},
      'node_modules/parent/node_modules/leaf'=>{'version'=>'1.0.0'},
      'node_modules/leaf'=>{'version'=>'3.0.0'},
      'node_modules/alias'=>{'name'=>'actual-name','version'=>'4.0.0'},
      'node_modules/local'=>{'link'=>true,'resolved'=>'../local'},
      ''=>{'name'=>'root','version'=>'0.0.0'}
    })
    status,err=generate;assert status.success?,err
    npm=sbom['components'].select { |c| c['purl'].start_with?('pkg:npm/') }
    assert_includes npm.map { |c| c['purl'] },'pkg:npm/%40scope/leaf@2.0.0'
    assert_equal 1,npm.count { |c| c['name']=='@scope/leaf' }
    assert_equal %w[1.0.0 3.0.0],npm.select { |c| c['name']=='leaf' }.map { |c| c['version'] }.sort
    assert_includes npm.map { |c| c['purl'] },'pkg:npm/actual-name@4.0.0'
    refute npm.any? { |c| c['name'].include?('node_modules') || c['name']=='local' || c['name']=='root' }
  end
  def test_every_known_lock_is_required
    (LOCKS + %w[fiscal-backend/go.sum minipos/beeminipos-backend/go.sum]).each do |f|
      raw=File.read(File.join(@root,f));File.delete(File.join(@root,f));status,err=generate
      refute status.success?,"silently accepted missing #{f}";assert_includes err,f
      write(f,raw)
    end
  end
  def test_unsupported_or_missing_package_inventory_fails_closed
    lock(LOCKS.last,{},1);status,err=generate;refute status.success?;assert_match(/lockfileVersion/,err)
    write(LOCKS.last,JSON.generate({'lockfileVersion'=>3}));status,err=generate;refute status.success?;assert_match(/packages/,err)
  end
  def test_non_link_component_without_version_is_not_silently_dropped
    lock(LOCKS.last,{'node_modules/unknown'=>{}});status,err=generate;refute status.success?;assert_match(/version/,err)
  end
end
