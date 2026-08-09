#!/usr/bin/env ruby
require "date"
require "yaml"

root = File.expand_path("..", __dir__)
canonical_path = File.expand_path("../BeeloyBackend/docs/Fiscal/api/openapi-public-v1.yaml", root)
overlay_path = File.join(root, "contracts/openapi-corrections-v1.yaml")
load_yaml = ->(path) { YAML.safe_load(File.read(path), permitted_classes: [Date, Time], aliases: true) }
canonical = load_yaml.call(canonical_path)
overlay = load_yaml.call(overlay_path)

abort "OpenAPI Overlay 1.0 required" unless overlay["overlay"] == "1.0.0"
abort "overlay must extend the locked canonical path" unless overlay["extends"] == "../../BeeloyBackend/docs/Fiscal/api/openapi-public-v1.yaml"
actions = overlay.fetch("actions")
expected_targets = [
  "$.paths['/webhook-endpoints'].post.responses['201']",
  "$.paths['/organizations'].patch.requestBody.content['application/json'].schema",
  "$.paths['/locations'].post.requestBody.content['application/json'].schema",
  "$.paths['/locations/{location_id}'].patch.requestBody.content['application/json'].schema",
  "$.paths['/registers'].post.requestBody.content['application/json'].schema",
  "$.paths['/registers/{register_id}'].patch.requestBody.content['application/json'].schema",
  "$.paths['/operators'].post.requestBody.content['application/json'].schema",
  "$.paths['/operators/{operator_id}'].patch.requestBody.content['application/json'].schema",
  "$.paths['/devices'].post.requestBody.content['application/json'].schema",
  "$.paths['/devices/{device_id}'].patch.requestBody.content['application/json'].schema",
  "$.paths['/minipos/products'].post.requestBody.content['application/json'].schema",
  "$.paths['/minipos/products/{product_id}'].patch.requestBody.content['application/json'].schema",
  "$.paths['/minipos/employees'].post.requestBody.content['application/json'].schema",
  "$.paths['/minipos/employees/{employee_id}'].patch.requestBody.content['application/json'].schema",
  "$.paths['/minipos/orders'].post.requestBody.content['application/json'].schema"
]
targets = actions.map { |value| value.fetch("target") }
abort "reviewed correction target inventory drifted" unless targets == expected_targets && targets.uniq.length == targets.length
action = actions.first

original = canonical.dig("paths", "/webhook-endpoints", "post", "responses", "201")
abort "canonical omission changed; review/remove correction" unless original == {"description" => "Registered"}
update = action.fetch("update")
schema = update.dig("content", "application/json", "schema")
abort "correction must be a closed object response" unless schema&.fetch("type", nil) == "object" && schema["additionalProperties"] == false
required = %w[id version url events status created_at updated_at secret]
abort "one-time response required fields drifted" unless schema["required"] == required
abort "one-time secret must be write-only and exactly 32-byte base64url" unless schema.dig("properties", "secret") == {"type" => "string", "minLength" => 43, "maxLength" => 43, "writeOnly" => true}
abort "response property inventory drifted" unless schema.fetch("properties").keys == required

resource_requests = actions[1, 9]
resource_requests.each do |request_action|
  ref = request_action.dig("update", "$ref")
  abort "resource command correction must select only the business allOf branch" unless ref&.match?(%r{\A#/components/schemas/(Organization|Location|Register|Operator|Device)/allOf/1\z})
end

product_create, product_update = actions[10], actions[11]
employee_create, employee_update = actions[12], actions[13]
abort "MiniPOS product create/update schemas must remain identical" unless product_create["update"] == product_update["update"]
abort "MiniPOS employee create/update schemas must remain identical" unless employee_create["update"] == employee_update["update"]
abort "MiniPOS command schemas must reject unknown fields" unless [product_create, employee_create, actions[14]].all? { |value| value.dig("update", "additionalProperties") == false }
abort "MiniPOS product command required fields drifted" unless product_create.dig("update", "required") == %w[sku name price tax_group]
abort "MiniPOS employee command required fields drifted" unless employee_create.dig("update", "required") == %w[first_name last_name operator_code]
abort "MiniPOS order create must accept only an open shift reference" unless actions[14].dig("update", "required") == ["shift_id"] && actions[14].dig("update", "properties").keys == ["shift_id"]

puts "OpenAPI overlay OK: 1 response omission and 14 server-owned request-schema defects corrected"
