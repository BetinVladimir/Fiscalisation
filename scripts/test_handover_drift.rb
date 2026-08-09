#!/usr/bin/env ruby
require 'json'
require 'open3'
require 'tmpdir'

root = File.expand_path('..', __dir__)
source = JSON.parse(File.read(File.join(root, 'contracts/mvp-acceptance-v1.json')))

Dir.mktmpdir('beeloy-handover-drift') do |dir|
  path = File.join(dir, 'acceptance.json')
  source['contract_surface']['request_contracts'] -= 1
  File.write(path, JSON.pretty_generate(source))
  _output, status = Open3.capture2e(
    {'ACCEPTANCE_RECORD_PATH' => path},
    'ruby', File.join(root, 'scripts/verify_handover.rb')
  )
  abort 'handover drift test failed: stale generated contract count was accepted' if status.success?
end

puts 'handover drift test OK: stale acceptance contract counts fail closed'
