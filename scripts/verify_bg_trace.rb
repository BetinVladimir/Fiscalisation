#!/usr/bin/env ruby
require "json"

root = File.expand_path("..", __dir__)
trace = JSON.parse(File.read(File.join(root, "contracts/bg-requirements-trace.json")))
rows = trace.fetch("requirements")
expected = (1..25).map { |n| format("BG-%03d", n) }
ids = rows.map { |row| row.fetch("id") }
abort "BG IDs must be exactly BG-001..BG-025" unless ids == expected
abort "duplicate rule IDs" unless rows.map { |row| row.fetch("rule") }.uniq.length == rows.length

allowed = %w[PASS PARTIAL EXTERNAL_BLOCKED EXCLUDED_MVP]
rows.each do |row|
  abort "#{row['id']}: invalid status" unless allowed.include?(row["status"])
  %w[api tests evidence].each do |field|
    abort "#{row['id']}: #{field} must be non-empty" unless row[field].is_a?(Array) && !row[field].empty?
  end
  row["tests"].each do |path|
    abort "#{row['id']}: missing test/evidence executable #{path}" unless File.file?(File.join(root, path))
  end
  if row["status"] != "PASS"
    abort "#{row['id']}: non-PASS row requires an explicit gap" if row.fetch("gap", "").strip.empty?
  elsif row.key?("gap")
    abort "#{row['id']}: PASS cannot declare a gap"
  end
end

counts = rows.group_by { |row| row["status"] }.transform_values(&:length)
puts "BG trace OK: #{rows.length}/25 rows; " + allowed.map { |s| "#{s}=#{counts.fetch(s, 0)}" }.join(", ")
