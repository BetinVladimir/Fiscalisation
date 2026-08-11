#!/usr/bin/env ruby
require "yaml"

root = File.expand_path("..", __dir__)
contract = YAML.safe_load(File.read(File.join(root, "contracts/openapi-runtime-v1.yaml")), aliases: true)
abort "OpenAPI 3.1 required" unless contract["openapi"] == "3.1.0"

http_methods = %w[get post put patch delete]
operations = []
contract.fetch("paths").each do |path, item|
  methods = item.keys & http_methods
  abort "runtime path has no HTTP operation: #{path}" if methods.empty?
  methods.each do |method|
    operation = item.fetch(method) { abort "missing #{method.upcase} #{path}" }
    id = operation["operationId"]
    abort "missing operationId for #{method.upcase} #{path}" if id.to_s.empty?
    abort "duplicate operationId #{id}" if operations.include?(id)
    operations << id
  end
end

sources = {
  "fiscal-backend/internal/api/handler.go" => [
    "/public/v1/sales:open-with-line", "/public/v1/workstations/", "/public/v1/exports/periodized", "/public/v1/exports/", "activation-tokens"
  ],
  "beeminipos-backend/internal/api/handler.go" => [
    "/public/v1/minipos/configuration", "/public/v1/minipos/employees/", "/public/v1/minipos/operator-session",
    "/public/v1/minipos/shifts", "/public/v1/minipos/shifts/", "/public/v1/minipos/orders/",
    "identity-binding", "checkout-batch", "reversals", "/public/v1/fiscal-webhooks"
  ],
  "edge-agent/localapi/handler.go" => ["/internal/v1/final-device", "/internal/v1/commands", "/internal/v1/storage", "/local/v1/intents"],
}
sources.each do |relative, routes|
  source = File.read(File.join(root, relative))
  routes.each { |route| abort "runtime route disappeared: #{route}" unless source.include?(route) }
end

abort "runtime OpenAPI operation count changed without verifier review: #{operations.length}" unless operations.length == 33
puts "runtime OpenAPI coverage OK: #{operations.length} operations"
