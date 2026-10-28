#!/usr/bin/env ruby
# check_plan_coverage.rb — Validate docs/MVP1/fixtures/plan_coverage.json
# Exit 0: valid; Exit 1: schema/structure error; Exit 2: missing task IDs in plan.json
#
# Usage: ruby scripts/check_plan_coverage.rb [--plan PATH] [--coverage PATH]
require 'json'
require 'optparse'
require 'set'

root = File.expand_path('..', __dir__)

options = {
  plan:     File.join(root, 'docs/MVP1/fixes/plan.json'),
  coverage: File.join(root, 'docs/MVP1/fixtures/plan_coverage.json'),
}
OptionParser.new do |opts|
  opts.on('--plan PATH')     { |v| options[:plan]     = v }
  opts.on('--coverage PATH') { |v| options[:coverage] = v }
end.parse!

errors = []

# Load files
begin
  coverage = JSON.parse(File.read(options[:coverage]))
rescue => e
  abort "ERROR: cannot parse plan_coverage.json: #{e}"
end

begin
  plan = JSON.parse(File.read(options[:plan]))
rescue => e
  abort "ERROR: cannot parse plan.json: #{e}"
end

# Schema check
unless coverage['schema'] == 1
  errors << "coverage.schema must be 1, got #{coverage['schema'].inspect}"
end
unless coverage['groups'].is_a?(Array) && coverage['groups'].length > 0
  errors << "coverage.groups must be a non-empty array"
end

# Collect all plan task IDs
plan_ids = Set.new((plan['tasks'] || []).map { |t| t['id'] })

required_group_fields = %w[id name description task_ids evidence_gate]
required_groups = %w[P40 P41 P42 P43 P44]
found_groups = []

(coverage['groups'] || []).each do |grp|
  gid = grp['id']
  found_groups << gid

  required_group_fields.each do |field|
    unless grp.key?(field)
      errors << "Group #{gid}: missing required field '#{field}'"
    end
  end

  task_ids = grp['task_ids']
  unless task_ids.is_a?(Array) && task_ids.length > 0
    errors << "Group #{gid}: task_ids must be a non-empty array"
    next
  end

  task_ids.each do |tid|
    unless plan_ids.include?(tid)
      errors << "Group #{gid}: task_id '#{tid}' not found in plan.json"
    end
  end

  if grp['evidence_gate'].to_s.strip.empty?
    errors << "Group #{gid}: evidence_gate must not be empty"
  end
end

required_groups.each do |gid|
  unless found_groups.include?(gid)
    errors << "Missing required group '#{gid}'"
  end
end

if errors.empty?
  puts "PLAN-CHECK OK: plan_coverage.json is valid (#{coverage['groups'].length} groups)"
  exit 0
else
  errors.each { |e| puts "FAIL: #{e}" }
  exit 1
end
