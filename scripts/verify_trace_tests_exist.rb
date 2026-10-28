#!/usr/bin/env ruby
# verify_trace_tests_exist.rb — NAP requirement → test traceability (TRT-001..002)
# Exit 0 = clean; exit 1 = violations; exit 3 = environment missing
require 'json'
require 'set'

ROOT = File.expand_path('..', __dir__)
TRACE_DIR = File.join(ROOT, 'docs', 'MVP1', 'fixtures')
TESTS_DIR = File.join(ROOT, 'tests', 'mvp1')

unless Dir.exist?(TESTS_DIR)
  warn "TRT-ENV: tests/mvp1/ directory not found"
  exit 3
end

errors = []

# TRT-001: each NAP-* requirement has at least one trace test
nap_reqs = Set.new
Dir.glob(File.join(ROOT, 'docs', 'MVP1', 'fixes', 'NAP-*.md')) do |f|
  nap_reqs << File.basename(f, '.md')
end
Dir.glob(File.join(ROOT, 'docs', 'MVP1', 'fixed', 'NAP-*.md')) do |f|
  nap_reqs << File.basename(f, '.md')
end

test_files = Dir.glob(File.join(TESTS_DIR, 'test_*.py')).map { |f| File.basename(f) }
trace_json = File.join(TRACE_DIR, 'nap_trace.json')

if File.exist?(trace_json)
  trace = JSON.parse(File.read(trace_json)) rescue {}
  nap_reqs.each do |req|
    lower = req.downcase.tr('-', '_')
    # Check trace file or test file
    covered = trace.key?(req) ||
              test_files.any? { |t| t.include?(lower) }
    unless covered
      errors << "TRT-001: #{req} has no trace test (add to nap_trace.json or test_#{lower}_*.py)"
    end
  end
else
  # Without trace file, just check that all NAP reqs have a test file
  nap_reqs.each do |req|
    lower = req.downcase.tr('-', '_')
    unless test_files.any? { |t| t.include?(lower) }
      errors << "TRT-001: #{req} has no acceptance test in tests/mvp1/"
    end
  end
end

# TRT-002: nap_trace.json references existing test files (if it exists)
if File.exist?(trace_json)
  trace = JSON.parse(File.read(trace_json)) rescue {}
  trace.each do |req, files|
    Array(files).each do |tf|
      full = File.join(ROOT, tf.to_s)
      unless File.exist?(full)
        errors << "TRT-002: trace for #{req} references non-existent file #{tf}"
      end
    end
  end
end

if errors.empty?
  puts "TRACE-TESTS OK: #{nap_reqs.size} NAP requirements checked"
  exit 0
else
  errors.each { |e| warn e }
  exit 1
end
