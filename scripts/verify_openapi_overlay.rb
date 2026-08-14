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
  "$.paths['/workstations/{workstation_id}/sessions/{session_id}:logout']",
  "$.paths['/audit-events'].get.parameters",
  "$.components.schemas.SaleLine.properties",
  "$.components.schemas.FiscalReceipt.properties",
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
  "$.paths['/minipos/orders'].post.requestBody.content['application/json'].schema",
  "$.components.schemas.ComplianceExportRequest.properties",
  "$.components.schemas.BleSession.properties",
  "$.components.schemas.BleSession.required"
]
targets = actions.map { |value| value.fetch("target") }
abort "reviewed correction target inventory drifted" unless targets == expected_targets && targets.uniq.length == targets.length
logout_action, audit_action = actions[0], actions[1]
abort "logout correction must be a 204 POST" unless logout_action.dig("update", "post", "responses", "204", "description") == "Durable operator logout"
audit_names = audit_action.fetch("update").map { |parameter| parameter["name"] }.compact
abort "Annex audit filters drifted" unless audit_names == %w[actor_id action object_type object_id unp from to]
discount_action = actions[2]
abort "canonical SaleLine unexpectedly gained discount; review/remove correction" if canonical.dig("components", "schemas", "SaleLine", "properties").key?("discount")
abort "Fiscal discount correction must use canonical Money" unless discount_action.dig("update", "discount", "$ref") == "#/components/schemas/Money"
receipt_action = actions[3]
abort "canonical FiscalReceipt unexpectedly gained fiscal-memory snapshot" if canonical.dig("components", "schemas", "FiscalReceipt", "properties").key?("fiscal_memory_number")
abort "receipt device snapshot must be closed and device-bound" unless receipt_action.dig("update", "fiscal_device", "additionalProperties") == false && receipt_action.dig("update", "fiscal_device", "required") == ["device_id"]
action = actions[4]

original = canonical.dig("paths", "/webhook-endpoints", "post", "responses", "201")
abort "canonical omission changed; review/remove correction" unless original == {"description" => "Registered"}
update = action.fetch("update")
schema = update.dig("content", "application/json", "schema")
abort "correction must be a closed object response" unless schema&.fetch("type", nil) == "object" && schema["additionalProperties"] == false
required = %w[id version url events status created_at updated_at secret]
abort "one-time response required fields drifted" unless schema["required"] == required
abort "one-time secret must be write-only and exactly 32-byte base64url" unless schema.dig("properties", "secret") == {"type" => "string", "minLength" => 43, "maxLength" => 43, "writeOnly" => true}
abort "response property inventory drifted" unless schema.fetch("properties").keys == required

resource_requests = actions[5, 9]
resource_requests.each do |request_action|
  ref = request_action.dig("update", "$ref")
  abort "resource command correction must select only the business allOf branch" unless ref&.match?(%r{\A#/components/schemas/(Organization|Location|Register|Operator|Device)/allOf/1\z})
end

product_create, product_update = actions[14], actions[15]
employee_create, employee_update = actions[16], actions[17]
abort "MiniPOS product create/update schemas must remain identical" unless product_create["update"] == product_update["update"]
abort "MiniPOS employee create/update schemas must remain identical" unless employee_create["update"] == employee_update["update"]
abort "MiniPOS command schemas must reject unknown fields" unless [product_create, employee_create, actions[18]].all? { |value| value.dig("update", "additionalProperties") == false }
abort "MiniPOS product command required fields drifted" unless product_create.dig("update", "required") == %w[sku name price tax_group]
abort "MiniPOS employee command required fields drifted" unless employee_create.dig("update", "required") == %w[first_name last_name operator_code]
abort "MiniPOS order create must accept only an open shift reference" unless actions[18].dig("update", "required") == ["shift_id"] && actions[18].dig("update", "properties").keys == ["shift_id"]
abort "compliance export historical filters drifted" unless actions[19].dig("update")&.keys == %w[location_id device_id]
abort "BLE session identity/security correction drifted" unless actions[20].dig("update")&.keys == %w[tenant_id location_id register_id security_mode]
mode = actions[20].dig("update", "security_mode")
abort "BLE security mode must explicitly fence the MVP exception" unless mode&.fetch("enum", nil) == %w[OPEN_MVP X25519_AES_GCM]
abort "BLE session required identity drifted" unless actions[21].fetch("update") == %w[ble_session_id tenant_id edge_id device_id location_id register_id service_uuid security_mode signed_session_ticket expires_at]

puts "OpenAPI overlay OK: workstation logout, Annex audit filters and 20 reviewed canonical corrections"
