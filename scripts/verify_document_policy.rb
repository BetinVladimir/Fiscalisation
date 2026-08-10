#!/usr/bin/env ruby
require "json"

root = File.expand_path("..", __dir__)
inventory = JSON.parse(File.read(File.join(root, "contracts/document-template-inventory.json")))
abort "template authority must be server-only" unless inventory["authority"] == "FISCAL_CORE_SERVER_ONLY"
abort "customer-editable template code is forbidden" unless inventory["customer_editable_template_code"] == false
templates = inventory.fetch("templates")
ids = templates.map { |row| row.fetch("id") }
abort "duplicate template id" unless ids.uniq.length == ids.length
%w[fiscal-receipt fiscal-reversal service-cash-in service-cash-out].each { |id| abort "missing #{id}" unless ids.include?(id) }
templates.each do |row|
  abort "uncontrolled template #{row['id']}" unless row["status"] == "CONTROLLED"
  if row["customer_facing"]
    abort "customer document is not fiscal: #{row['id']}" unless %w[FISCAL_RECEIPT FISCAL_REVERSAL].include?(row["class"])
  end
  if row["class"] == "OPERATIONAL_NON_FISCAL"
    abort "non-fiscal marking missing: #{row['id']}" unless row.fetch("required_marking").include?("не е")
  end
end
puts "document policy OK: #{templates.length} controlled server templates"
