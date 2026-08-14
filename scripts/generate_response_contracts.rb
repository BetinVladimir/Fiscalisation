#!/usr/bin/env ruby
require "date"
require "fileutils"
require "json"
require "yaml"

root = File.expand_path("..", __dir__)
canonical_path = File.expand_path("../BeeloyBackend/docs/Fiscal/api/openapi-public-v1.yaml", root)
runtime_path = File.join(root, "contracts/openapi-runtime-v1.yaml")
load_yaml = ->(path) { YAML.safe_load(File.read(path), permitted_classes: [Date, Time], aliases: true) }
canonical = load_yaml.call(canonical_path)
runtime = load_yaml.call(runtime_path)
overlay = load_yaml.call(File.join(root, "contracts/openapi-corrections-v1.yaml"))
overlay.fetch("actions").each do |action|
	target = action.fetch("target")
	match = target.match(/\A\$\.components\.schemas\.([A-Za-z0-9_]+)\.properties\z/)
	canonical.dig("components", "schemas", match[1], "properties").merge!(action.fetch("update")) if match
	required = target.match(/\A\$\.components\.schemas\.([A-Za-z0-9_]+)\.required\z/)
	canonical.dig("components", "schemas", required[1])["required"] = action.fetch("update") if required
	canonical.dig("paths", "/audit-events", "get")["parameters"] = action.fetch("update") if target == "$.paths['/audit-events'].get.parameters"
	canonical["paths"]["/workstations/{workstation_id}/sessions/{session_id}:logout"] = action.fetch("update") if target == "$.paths['/workstations/{workstation_id}/sessions/{session_id}:logout']"
end

def resolve_pointer(document, pointer)
  pointer.delete_prefix("#/").split("/").reduce(document) do |node, key|
    node.is_a?(Array) ? node.fetch(Integer(key, 10)) : node.fetch(key)
  end
end

def resolve(document, value)
  return value unless value.is_a?(Hash) && value["$ref"]&.start_with?("#/")
  resolve_pointer(document, value["$ref"])
end

def dereference_schema(document, node, stack = [])
  case node
  when Hash
    if node["$ref"]&.start_with?("#/")
      pointer = node["$ref"]
      raise "cyclic response schema #{pointer}" if stack.include?(pointer)
      target = resolve_pointer(document, pointer)
      merged = dereference_schema(document, target, stack + [pointer])
      siblings = node.reject { |key, _| key == "$ref" }
      return merged.merge(dereference_schema(document, siblings, stack)) if merged.is_a?(Hash)
      return merged
    end
    node.to_h { |key, value| [key, dereference_schema(document, value, stack)] }
  when Array
    node.map { |value| dereference_schema(document, value, stack) }
  else
    node
  end
end

corrected_webhook_schema = overlay.fetch("actions").find { |action| action["target"] == "$.paths['/webhook-endpoints'].post.responses['201']" }.dig("update", "content", "application/json", "schema")
request_corrections = {}
overlay.fetch("actions").each do |action|
  match = action.fetch("target").match(/\A\$\.paths\['([^']+)'\]\.(get|post|put|patch|delete)\.requestBody\.content\['application\/json'\]\.schema\z/)
  request_corrections[[match[1], match[2]]] = action.fetch("update") if match
end
problem_schema = JSON.generate(dereference_schema(canonical, canonical.fetch("components").fetch("schemas").fetch("Problem")))

rows = []
request_rows = []
[[canonical, nil], [runtime, nil]].each do |document, _default_owner|
  document.fetch("paths").each do |path, item|
    item.each do |method, operation|
      next unless operation.is_a?(Hash) && operation["operationId"]
      owner = if path.start_with?("/internal/") || path.start_with?("/local/")
        :edge
      elsif path.start_with?("/minipos/") || path == "/fiscal-webhooks"
        :minipos
      else
        :fiscal
      end
      raw_request = operation["requestBody"]
      parameters = (item.fetch("parameters", []) + operation.fetch("parameters", [])).map do |raw_parameter|
        parameter = resolve(document, raw_parameter)
        {
          "name" => parameter.fetch("name"),
          "in" => parameter.fetch("in"),
          "required" => parameter["required"] == true,
          "schema" => dereference_schema(document, parameter.fetch("schema", {}))
        }
      end
      if raw_request
        request = resolve(document, raw_request)
        request_media = request.fetch("content", {}).keys.sort
        request_schema = request_media.empty? ? nil : request.dig("content", request_media.first, "schema")
        request_schema = request_corrections.fetch([path, method], request_schema) if document.equal?(canonical)
        resolved_request_schema = request_schema && dereference_schema(document, request_schema)
        request_rows << {owner: owner, method: method.upcase, path: path, operation: operation["operationId"], required: request["required"] == true, media: request_media, schema: resolved_request_schema && JSON.generate(resolved_request_schema), parameters: JSON.generate(parameters)}
      else
        request_rows << {owner: owner, method: method.upcase, path: path, operation: operation["operationId"], required: false, media: [], schema: nil, parameters: JSON.generate(parameters)}
      end
      operation.fetch("responses").each do |status, raw_response|
        next unless status.to_s.match?(/^2\d\d$/)
        response = resolve(document, raw_response)
        media = response.fetch("content", {}).keys.sort
        schema = media.empty? ? nil : response.dig("content", media.first, "schema")
        if operation["operationId"] == "createWebhookEndpoint" && status.to_s == "201"
          media = ["application/json"]
          schema = corrected_webhook_schema
        end
        resolved_schema = schema && dereference_schema(document, schema)
        rows << {owner: owner, method: method.upcase, path: path, operation: operation["operationId"], status: status.to_i, media: media, schema: resolved_schema && JSON.generate(resolved_schema)}
      end
    end
  end
end

output_root = ENV.fetch("RESPONSE_CONTRACT_OUTPUT_ROOT", root)
targets = {
  fiscal: File.join(output_root, "fiscal-backend/internal/api/response_contracts_gen.go"),
  minipos: File.join(output_root, "minipos/beeminipos-backend/internal/api/response_contracts_gen.go"),
  edge: File.join(output_root, "edge-agent/localapi/response_contracts_gen.go")
}
targets.each do |owner, output|
  package_name = owner == :edge ? "localapi" : "api"
  body = +"// Code generated by scripts/generate_response_contracts.rb; DO NOT EDIT.\npackage #{package_name}\n\n"
  body << "var generatedSuccessResponses = []successResponseContract{\n"
  rows.select { |row| row[:owner] == owner }.sort_by { |row| [row[:path], row[:method], row[:status]] }.each do |row|
    medias = row[:media].map { |media| %Q{"#{media}"} }.join(", ")
    schema = row[:schema] ? %Q{`#{row[:schema]}`} : '""'
    body << %Q{\t{Method: "#{row[:method]}", Path: "#{row[:path]}", Operation: "#{row[:operation]}", Status: #{row[:status]}, Media: []string{#{medias}}, Schema: #{schema}},\n}
  end
  body << "}\n"
  body << "\nvar generatedRequestContracts = []requestContract{\n"
  request_rows.select { |row| row[:owner] == owner }.sort_by { |row| [row[:path], row[:method]] }.each do |row|
    medias = row[:media].map { |media| %Q{"#{media}"} }.join(", ")
    schema = row[:schema] ? %Q{`#{row[:schema]}`} : '""'
    body << %Q{\t{Method: "#{row[:method]}", Path: "#{row[:path]}", Operation: "#{row[:operation]}", Required: #{row[:required]}, Media: []string{#{medias}}, Schema: #{schema}, Parameters: `#{row[:parameters]}`},\n}
  end
  body << "}\n"
  body << %Q{\nconst generatedProblemResponseSchema = `#{problem_schema}`\n}
  FileUtils.mkdir_p(File.dirname(output))
  File.write(output, body)
  validator = File.read(File.join(root, "scripts/response_schema_validator.go.txt")).sub("__PACKAGE__", package_name)
  File.write(File.join(File.dirname(output), "response_schema_validator_gen.go"), validator)
end

puts "generated #{rows.length} successful responses and #{request_rows.length} request contracts across #{targets.length} services"
