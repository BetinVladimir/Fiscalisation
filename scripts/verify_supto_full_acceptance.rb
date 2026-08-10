#!/usr/bin/env ruby
require "json"

root = File.expand_path("..", __dir__)
trace = JSON.parse(File.read(File.join(root, "contracts/supto-annex29-trace.json")))
blocked = trace.fetch("requirements").reject { |row| %w[PASS NOT_APPLICABLE].include?(row.fetch("status")) }
abort "SUPTO full acceptance blocked by #{blocked.length} Annex rows: #{blocked.map { |row| row['id'] }.join(', ')}" unless blocked.empty?
abort "BG-014 activation requires trusted external HIL/release/legal evidence and signed activation manifest"
