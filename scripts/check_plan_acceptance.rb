#!/usr/bin/env ruby
# check_plan_acceptance.rb — every acceptance command references an existing file/target
# Exit 0 = clean; exit 1 = violations; exit 3 = environment missing
require 'json'

ROOT = File.expand_path('..', __dir__)
FIXES_DIR = File.join(ROOT, 'docs/MVP1/fixes')

unless Dir.exist?(FIXES_DIR)
  warn "ACCEPT-ENV: fixes dir not found"
  exit 3
end

errors = []
checked = 0

Dir.glob(File.join(FIXES_DIR, '*.md')).sort.each do |f|
  base = File.basename(f, '.md')
  next if %w[README INDEX REVIEW E2E_COVERAGE].include?(base)
  content = File.read(f)
  # Extract "Предложенная команда:" lines
  content.scan(/Предложенная команда:\s*`([^`]+)`/).each do |m|
    cmd = m[0]
    checked += 1
    # Check that the main script/file in the command exists
    if cmd =~ /python3 -m unittest/
      # acceptance tests – not checked here (they're generated)
    elsif cmd =~ /go test (\.\/[^\s]+)/
      # go package – check dir exists
      pkg = $1.gsub(/\.\.\./, '').sub(%r{^./}, '')
      dir = File.join(ROOT, pkg)
      unless Dir.exist?(dir) || File.exist?(dir)
        errors << "ACCEPT-001: #{base}: go pkg #{pkg} not found"
      end
    elsif cmd =~ %r{bash\s+(scripts/[^\s]+\.sh)}
      sh = File.join(ROOT, $1)
      unless File.exist?(sh)
        errors << "ACCEPT-001: #{base}: script #{$1} not found"
      end
    elsif cmd =~ %r{ruby\s+(scripts/[^\s]+\.rb)}
      rb = File.join(ROOT, $1)
      unless File.exist?(rb)
        errors << "ACCEPT-001: #{base}: script #{$1} not found"
      end
    end
  end
end

if errors.empty?
  puts "ACCEPT-CHECK OK: #{checked} acceptance commands checked"
  exit 0
else
  errors.each { |e| warn e }
  exit 1
end
