#!/usr/bin/env ruby
require "json"

root = File.expand_path("..", __dir__)
register = JSON.parse(File.read(File.join(root, "contracts/implementation-governance.json")))
abort "unexpected governance schema" unless register.fetch("schema_version") == 1
abort "unexpected governance profile" unless register.fetch("profile") == "BG_MVP_FUNCTIONAL_NONPROD"

products = register.fetch("products")
abort "BeeFiscal and BeeMiniPOS must be the only governed products" unless products.map { |row| row.fetch("id") }.sort == %w[BEEFISCAL BEEMINIPOS]
abort "products must have independent databases" unless products.map { |row| row.fetch("database") }.uniq.length == products.length
products.each do |row|
  abort "#{row['id']}: Caddy must be the only declared ingress" unless row.fetch("ingress") == "Caddy"
  abort "#{row['id']}: compose missing" unless File.file?(File.join(root, row.fetch("compose")))
  abort "#{row['id']}: owner missing" if row.fetch("owner_role").strip.empty?
end

required_modules = %w[PUBLIC_CONTRACTS FISCAL_CORE MINIPOS_BACKEND EDGE_RUNTIME DAISY_DEVICE DATECS_DEVICE PAYMENTS QUALITY SECURITY COMPLIANCE]
owners = register.fetch("module_owners")
abort "module owner matrix incomplete" unless owners.map { |row| row.fetch("module") }.sort == required_modules.sort
owners.each do |row|
  abort "#{row['module']}: owner missing" if row.fetch("owner_role").strip.empty?
  paths = row.fetch("paths")
  abort "#{row['module']}: evidence missing" if paths.empty?
  paths.each { |path| abort "#{row['module']}: missing #{path}" unless File.file?(File.join(root, path)) }
end

identifiers = register.fetch("identifier_policy")
%w[requirement security_test fault_test ui_test defect release].each do |name|
  value = identifiers.fetch(name)
  begin
    Regexp.new("\\A(?:#{value})\\z")
  rescue RegexpError
    abort "#{name}: invalid identifier regex"
  end
end

decisions = register.fetch("p0_decisions")
expected = (1..13).map { |id| format("P0-%02d", id) } + ["P0-AUTH-001"]
abort "P0 decision register incomplete" unless decisions.map { |row| row.fetch("id") }.sort == expected.sort
decisions.each do |row|
  abort "#{row['id']}: owner missing" if row.fetch("owner_role").strip.empty?
  abort "#{row['id']}: disposition missing" if row.fetch("disposition").strip.empty?
  abort "#{row['id']}: external P0 cannot be production-unblocked" unless row.fetch("production_blocked") == true
  evidence = row.fetch("closure_evidence")
  abort "#{row['id']}: closure evidence missing" if evidence.empty?
  evidence.each { |path| abort "#{row['id']}: missing #{path}" unless File.file?(File.join(root, path)) }
end

policy = register.fetch("release_policy")
abort "release acceptance record missing" unless File.file?(File.join(root, policy.fetch("acceptance_record")))
abort "external signatures must gate production" unless policy.fetch("production_requires_external_signatures") == true

puts "governance gate OK: #{products.length} products, #{owners.length} owned modules, #{decisions.length} P0 decisions"
