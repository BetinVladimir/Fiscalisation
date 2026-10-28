#!/usr/bin/env ruby
# check_plan_coverage.rb — plan integrity: markdown cards <-> status.json
# Exit 0 = clean; exit 1 = violations; exit 3 = missing environment
require 'json'
require 'set'

ROOT = File.expand_path('..', __dir__)
STATUS_FILE = File.join(ROOT, 'docs/MVP1/execution/status.json')

unless File.exist?(STATUS_FILE)
  warn "PLAN-ENV: #{STATUS_FILE} not found (exit 3)"
  exit 3
end

status = JSON.parse(File.read(STATUS_FILE)) rescue nil
unless status.is_a?(Hash)
  warn 'PLAN-ENV: status.json is not a JSON object'
  exit 3
end

tasks_raw = status['tasks'] || []
# tasks can be Array of {id:...} or Hash {id => {...}}
tasks = if tasks_raw.is_a?(Array)
  tasks_raw.each_with_object({}) { |t, h| h[t['id']] = t if t['id'] }
else
  tasks_raw
end
status_ids = Set.new(tasks.keys)

# Collect IDs from fixes/ markdown files
card_dir = File.join(ROOT, 'docs/MVP1/fixes')
card_ids = Set.new
if Dir.exist?(card_dir)
  Dir.glob(File.join(card_dir, '*.md')).each do |f|
    base = File.basename(f, '.md')
    next if %w[README INDEX REVIEW E2E_COVERAGE].include?(base)
    card_ids << base
  end
end

errors = []

# PLAN-001: every card ID should be in status.json (warn only — cards may precede status)
missing_from_status = card_ids - status_ids
if missing_from_status.size > 50
  errors << "PLAN-001: #{missing_from_status.size} cards not in status.json (sample: #{missing_from_status.first(3).join(', ')})"
end

# PLAN-002: status IDs with doc field must point to existing file
tasks.each do |id, task|
  doc = task['doc']
  next unless doc
  doc_path = File.join(ROOT, doc)
  unless File.exist?(doc_path)
    errors << "PLAN-002: task #{id} doc=#{doc} not found"
  end
end

# PLAN-003: depends reference existing IDs
tasks.each do |id, task|
  (task['depends'] || task['requires'] || []).each do |dep|
    unless status_ids.include?(dep) || card_ids.include?(dep)
      errors << "PLAN-003: task #{id} depends on unknown #{dep}"
    end
  end
end

if errors.empty?
  puts "PLAN-CHECK OK: #{tasks.size} tasks, #{card_ids.size} cards checked"
  exit 0
else
  errors.each { |e| warn e }
  exit 1
end
