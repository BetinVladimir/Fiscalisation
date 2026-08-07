#!/usr/bin/env ruby
require "yaml"
require "date"

ROOT = File.expand_path("..", __dir__)
CANONICAL = "/Users/freelancer/Documents/Beeloy/BeeloyBackend/docs/Fiscal/api/openapi-public-v1.yaml"
ASYNCAPI = "/Users/freelancer/Documents/Beeloy/BeeloyBackend/docs/Fiscal/events/asyncapi-device-v1.yaml"
RUNTIME = File.join(ROOT, "contracts/openapi-runtime-v1.yaml")

def load_yaml(path)
  YAML.safe_load(File.read(path), permitted_classes: [Date, Time], aliases: true)
rescue StandardError => e
  abort "cannot parse #{path}: #{e.message}"
end

def resolve_pointer(document, pointer)
  pointer.delete_prefix("#/").split("/").reduce(document) do |node, token|
    key = token.gsub("~1", "/").gsub("~0", "~")
    node.is_a?(Hash) ? node[key] : nil
  end
end

def validate_refs(node, document, location = "#")
  case node
  when Hash
    if node["$ref"]&.start_with?("#/") && resolve_pointer(document, node["$ref"]).nil?
      abort "unresolved ref #{node['$ref']} at #{location}"
    end
    node.each { |key, value| validate_refs(value, document, "#{location}/#{key}") }
  when Array
    node.each_with_index { |value, index| validate_refs(value, document, "#{location}/#{index}") }
  end
end

def operations(document)
  document.fetch("paths", {}).flat_map do |path, item|
    item.each_with_object([]) do |(method, operation), rows|
      next unless operation.is_a?(Hash) && operation["operationId"]
      rows << [method.upcase, path, operation.fetch("operationId")]
    end
  end
end

def registration_path(path)
  full = "/public/v1#{path}"
  return full unless full.include?("{")
  full[0...full.index("{")]
end

canonical = load_yaml(CANONICAL)
runtime = load_yaml(RUNTIME)
asyncapi = load_yaml(ASYNCAPI)
[canonical, runtime, asyncapi].each { |document| validate_refs(document, document) }
abort "AsyncAPI version missing" unless asyncapi["asyncapi"].to_s.start_with?("3.")

all = operations(canonical) + operations(runtime)
duplicates = all.group_by(&:last).select { |_id, rows| rows.length > 1 }
abort "duplicate operationId(s): #{duplicates.keys.join(', ')}" unless duplicates.empty?

fiscal_router = File.read(File.join(ROOT, "fiscal-backend/internal/api/handler.go"))
minipos_router = File.read(File.join(ROOT, "beeminipos-backend/internal/api/handler.go"))
edge_router = File.read(File.join(ROOT, "edge-agent/localapi/handler.go"))
missing = []
all.each do |method, path, operation_id|
  if path.start_with?("/internal/")
    source = edge_router
    expected = path.gsub(/\{[^}]+\}/, "")
  elsif path.start_with?("/minipos/") || path == "/fiscal-webhooks"
    source = minipos_router
    expected = registration_path(path)
  else
    source = fiscal_router
    expected = registration_path(path)
  end
  missing << "#{method} #{path} (#{operation_id}) -> #{expected}" unless source.include?(%Q{"#{expected}"})
end
abort "OpenAPI operations without router registration:\n#{missing.join("\n")}" unless missing.empty?

puts "contract surface OK: #{operations(canonical).length} canonical + #{operations(runtime).length} runtime operations; AsyncAPI #{asyncapi['asyncapi']}"
