#!/usr/bin/env ruby
require "json"

root = File.expand_path("..", __dir__)
trace = JSON.parse(File.read(File.join(root, "contracts/supto-annex29-trace.json")))
abort "software gate declaration missing" unless trace["software_gate"] == "make supto-software-acceptance"

ddl = File.read(File.join(root, "database/fiscal/017_supto_full_event_model.sql"))
%w[country_identifier_schemes sale_events operator_security_events supto_evidence_manifests device_compliance_profiles].each do |table|
  abort "missing typed SUPTO table #{table}" unless ddl.match?(/CREATE TABLE IF NOT EXISTS #{table}\b/i)
end
%w[sale_events operator_security_events regulatory_identifier_bindings].each do |table|
  abort "#{table} is not append-only" unless ddl.include?("#{table}_append_only")
end

api = File.read(File.join(root, "fiscal-backend/internal/api/response_contracts_gen.go"))
%w[action object_type object_id actor_id unp from to].each { |field| abort "audit filter #{field} missing" unless api.include?(%Q{"name":"#{field}"}) }
abort "workstation logout contract missing" unless api.include?("logoutWorkstationSession")

repository = File.read(File.join(root, "fiscal-backend/internal/domain/repository.go"))
%w[LOGIN_SUCCEEDED LOGIN_FAILED LOGOUT OPERATOR_CREATED OPERATOR_CHANGED OPERATOR_ROLE_CHANGED OPERATOR_DEACTIVATED WORKSTATION_STARTED EDGE_INTENT_ACCEPTED].each do |event|
  sources = repository + File.read(File.join(root, "fiscal-backend/internal/domain/service.go"))
  abort "operator/audit event #{event} missing" unless sources.include?(event)
end

sync = File.read(File.join(root, "fiscal-backend/internal/domain/edge_sync.go"))
%w[FISCAL_SALE_OPEN FISCAL_SALE_LINE FISCAL_SALE_LINE_CHANGE FISCAL_SALE_LINE_CANCEL FISCAL_SALE_CANCEL].each { |command| abort "offline projection #{command} missing" unless sync.include?(command) }

export_tests = File.read(File.join(root, "fiscal-backend/internal/domain/exports_test.go"))
%w[SUPTO_18_1 SUPTO_18_2 SUPTO_18_3 SUPTO_18_4 SUPTO_18_5 SUPTO_18_9].each { |kind| abort "export fixture #{kind} missing" unless export_tests.include?(kind) }
abort "JSON/CSV/XLSX fixture matrix missing" unless %w[JSON CSV XLSX].all? { |format| export_tests.include?(format) }

puts "BG SUPTO software acceptance PASS: behavioral suites + DDL/API/event/export invariants"
