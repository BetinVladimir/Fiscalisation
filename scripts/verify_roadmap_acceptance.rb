#!/usr/bin/env ruby
require "json"

root = File.expand_path("..", __dir__)
register = JSON.parse(File.read(File.join(root, "contracts/roadmap-stage-acceptance.json")))
abort "unexpected roadmap acceptance schema" unless register.fetch("schema_version") == 1
abort "unexpected MVP profile" unless register.fetch("profile") == "BG_MVP_FUNCTIONAL_NONPROD"

roadmap = File.expand_path(register.fetch("roadmap"), root)
abort "canonical roadmap missing" unless File.file?(roadmap)
stages = register.fetch("stages")
abort "roadmap stages must be exactly 1..25" unless stages.map { |row| row.fetch("stage") } == (1..25).to_a

allowed = %w[SOFTWARE_PASS FORMALLY_EXCLUDED_EXTERNAL PENDING_EXTERNAL_ACCEPTANCE]
makefile = File.read(File.join(root, "Makefile"))
stages.each do |row|
  id = "stage #{row.fetch('stage')}"
  abort "#{id}: invalid status" unless allowed.include?(row.fetch("status"))
  abort "#{id}: milestone missing" if row.fetch("milestone").strip.empty?
  evidence = row.fetch("evidence")
  checks = row.fetch("verification")
  abort "#{id}: evidence missing" unless evidence.is_a?(Array) && !evidence.empty?
  abort "#{id}: verification missing" unless checks.is_a?(Array) && !checks.empty?
  evidence.each { |path| abort "#{id}: missing evidence #{path}" unless File.file?(File.join(root, path)) }
  checks.each { |target| abort "#{id}: unknown Make target #{target}" unless makefile.match?(/^#{Regexp.escape(target)}:/) }
  if row.fetch("status") == "SOFTWARE_PASS"
    abort "#{id}: SOFTWARE_PASS cannot declare a gap" if row.key?("gap")
  else
    abort "#{id}: non-PASS stage requires explicit gap" if row.fetch("gap", "").strip.length < 24
  end
end

acceptance = JSON.parse(File.read(File.join(root, "contracts/mvp-acceptance-v1.json")))
pending = stages.select { |row| row.fetch("status") == "PENDING_EXTERNAL_ACCEPTANCE" }
abort "stage 25 must remain pending while signatures are external" unless pending.map { |row| row.fetch("stage") } == [25]
abort "acceptance must remain ready for human signature" unless acceptance.fetch("acceptance_state") == "READY_FOR_HUMAN_SIGNATURE"
abort "production cannot be GO with external stages" unless acceptance.fetch("production_status") == "NO_GO"

counts = stages.group_by { |row| row.fetch("status") }.transform_values(&:length)
puts "roadmap acceptance OK: 25/25 stages; " + allowed.map { |status| "#{status}=#{counts.fetch(status, 0)}" }.join(", ")
