#!/usr/bin/env ruby
require "yaml"

root = File.expand_path("..", __dir__)
contract = YAML.safe_load(File.read(File.join(root, "contracts/openapi-runtime-v1.yaml")), aliases: true)
abort "OpenAPI 3.1 required" unless contract["openapi"] == "3.1.0"

required = {
  "/minipos/configuration" => ["get", "patch"],
  "/minipos/shifts" => ["post"],
  "/minipos/shifts/{shift_id}/close" => ["post"],
  "/fiscal-webhooks" => ["post"],
  "/internal/v1/final-device" => ["get"],
  "/internal/v1/commands" => ["post"],
  "/internal/v1/storage" => ["get"],
}
operations = []
required.each do |path, methods|
  item = contract.fetch("paths").fetch(path) { abort "missing OpenAPI path #{path}" }
  methods.each do |method|
    operation = item.fetch(method) { abort "missing #{method.upcase} #{path}" }
    id = operation["operationId"]
    abort "missing operationId for #{method.upcase} #{path}" if id.to_s.empty?
    abort "duplicate operationId #{id}" if operations.include?(id)
    operations << id
  end
end

sources = {
  "beeminipos-backend/internal/api/handler.go" => [
    "/public/v1/minipos/configuration", "/public/v1/minipos/shifts", "/public/v1/minipos/shifts/", "/public/v1/fiscal-webhooks"
  ],
  "edge-agent/localapi/handler.go" => ["/internal/v1/final-device", "/internal/v1/commands", "/internal/v1/storage"],
}
sources.each do |relative, routes|
  source = File.read(File.join(root, relative))
  routes.each { |route| abort "runtime route disappeared: #{route}" unless source.include?(route) }
end

puts "runtime OpenAPI coverage OK: #{operations.length} operations"
