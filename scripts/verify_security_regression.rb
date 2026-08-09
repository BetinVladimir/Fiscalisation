#!/usr/bin/env ruby
require 'json'

root = File.expand_path('..', __dir__)
cases = JSON.parse(File.read(File.join(root, 'contracts/security-regression-matrix.json'))).fetch('cases')
abort 'security matrix must contain at least 18 executable cases' if cases.length < 18
ids = cases.map { |item| item.fetch('id') }
abort 'duplicate security IDs' unless ids.uniq.length == ids.length
required = %w[authentication authorization webhook ssrf replay revoke rate_limit idempotency production_guard secret_rotation ota supply_chain]
missing = required - cases.map { |item| item.fetch('category') }.uniq
abort "missing security categories: #{missing.join(', ')}" unless missing.empty?
cases.each do |item|
  path = File.join(root, item.fetch('source'))
  abort "missing security source #{item['source']}" unless File.file?(path)
  abort "missing executable security test #{item['id']}: #{item['test']}" unless File.read(path).include?(item.fetch('test'))
  abort "missing control description #{item['id']}" if item.fetch('control').strip.empty?
end
puts "security regression matrix OK: #{cases.length} executable cases, #{required.length} categories"
