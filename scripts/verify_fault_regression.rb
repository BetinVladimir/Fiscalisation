#!/usr/bin/env ruby
require 'json'

root = File.expand_path('..', __dir__)
matrix = JSON.parse(File.read(File.join(root, 'contracts/fault-regression-matrix.json')))
cases = matrix.fetch('cases')
abort 'fault matrix must contain at least 20 executable cases' if cases.length < 20
ids = cases.map { |item| item.fetch('id') }
abort 'duplicate fault IDs' unless ids.uniq.length == ids.length
required = %w[device ble authority restart sync storage retention clock routing]
missing_categories = required - cases.map { |item| item.fetch('category') }.uniq
abort "missing fault categories: #{missing_categories.join(', ')}" unless missing_categories.empty?

cases.each do |item|
  source = File.join(root, item.fetch('source'))
  abort "missing fault source #{item['source']}" unless File.file?(source)
  content = File.read(source)
  abort "missing executable test #{item['id']}: #{item['test']}" unless content.include?(item.fetch('test'))
  abort "missing invariant #{item['id']}" if item.fetch('invariant').strip.empty?
end

puts "fault regression matrix OK: #{cases.length} executable cases, #{required.length} categories"
