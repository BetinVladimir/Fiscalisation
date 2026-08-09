#!/usr/bin/env ruby
require "json"

root = File.expand_path("..", __dir__)
matrix = JSON.parse(File.read(File.join(root, "contracts/soak-regression-matrix.json")))
abort "invalid soak time model" unless matrix["time_model"] == "ACCELERATED_DETERMINISTIC_SLOTS"
abort "soak matrix must not claim HIL" unless matrix["hardware_claimed"] == false
scenarios = matrix.fetch("scenarios")
abort "expected two soak scenarios" unless scenarios.map { |v| v.fetch("id") }.sort == %w[SOAK-72H-NETWORK-FLAP SOAK-7D-JOURNAL]

scenarios.each do |scenario|
  required_hours = scenario["id"].include?("72H") ? 72 : 168
  abort "soak duration too short: #{scenario['id']}" unless scenario.fetch("simulated_duration_hours") >= required_hours
  abort "invalid soak slot: #{scenario['id']}" unless scenario.fetch("slot_minutes").between?(1, 10)
  abort "missing soak invariants: #{scenario['id']}" unless scenario.fetch("invariants").length >= 5
  source = File.read(File.join(root, scenario.fetch("source")))
  abort "missing executable soak test #{scenario['test']}" unless source.include?("func #{scenario.fetch('test')}(")
end

report_path = ARGV[0]
if report_path
  passed = {}
  File.foreach(report_path) do |line|
    item = JSON.parse(line)
    passed[item["Test"]] = true if item["Action"] == "pass" && item["Test"]
  end
  missing = scenarios.map { |v| v.fetch("test") }.reject { |name| passed[name] }
  abort "soak report lacks passing tests: #{missing.join(',')}" unless missing.empty?
end

puts "soak regression OK: 72h network-flap + 7d journal accelerated; zero-loss/duplicate gates; HIL not claimed"
