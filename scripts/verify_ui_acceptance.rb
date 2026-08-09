#!/usr/bin/env ruby
require "json"

root = File.expand_path("..", __dir__)
matrix_path = File.join(root, "contracts/ui-acceptance-matrix.json")
matrix = JSON.parse(File.read(matrix_path))
cases = matrix.fetch("cases")
abort "UI matrix must cover both apps and all platforms" unless matrix.fetch("platforms").sort == %w[android ios web]
abort "UI matrix requires at least 12 cases" if cases.length < 12
ids = cases.map { |item| item.fetch("id") }
abort "duplicate UI case id" unless ids.uniq.length == ids.length
%w[BeeMiniPOS BeeFiscalApp both].each do |app|
  abort "UI matrix missing #{app}" unless cases.any? { |item| item.fetch("app") == app }
end
%w[touch accessibility test_ids unknown final_device web_ble public_api readiness reports admin_editors platforms].each do |category|
  abort "UI matrix missing category #{category}" unless cases.any? { |item| item.fetch("category") == category }
end

cases.each do |item|
  source = File.join(root, item.fetch("source"))
  abort "#{item.fetch('id')}: source missing #{source}" unless File.file?(source)
  body = File.read(source)
  item.fetch("anchors").each do |anchor|
    abort "#{item.fetch('id')}: missing anchor #{anchor.inspect}" unless body.include?(anchor)
  end
  Array(item["forbidden"]).each do |anchor|
    abort "#{item.fetch('id')}: forbidden anchor #{anchor.inspect}" if body.include?(anchor)
  end
  abort "#{item.fetch('id')}: invariant is too weak" if item.fetch("invariant").length < 24
end

puts "UI acceptance matrix OK: #{cases.length} cases, Android/iOS/Web, both apps"
