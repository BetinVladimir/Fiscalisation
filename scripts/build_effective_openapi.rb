#!/usr/bin/env ruby
require "date"
require "yaml"

root = File.expand_path("..", __dir__)
output = ARGV.fetch(0)
load_yaml = ->(path) { YAML.safe_load(File.read(path), permitted_classes: [Date, Time], aliases: true) }
document = load_yaml.call(File.expand_path("../BeeloyBackend/docs/Fiscal/api/openapi-public-v1.yaml", root))
overlay = load_yaml.call(File.join(root, "contracts/openapi-corrections-v1.yaml"))

def resolve_pointer(document, pointer)
  pointer.delete_prefix("#/").split("/").reduce(document) { |node, key| node.fetch(key.gsub("~1", "/").gsub("~0", "~")) }
end

overlay.fetch("actions").each do |action|
  target = action.fetch("target")
  update = action.fetch("update")
  if (component = target.match(/\A\$\.components\.schemas\.([A-Za-z0-9_]+)\.properties\z/))
	    document.dig("components", "schemas", component[1], "properties").merge!(update)
  elsif (component = target.match(/\A\$\.components\.schemas\.([A-Za-z0-9_]+)\.required\z/))
	    document.dig("components", "schemas", component[1])["required"] = update
  elsif (match = target.match(/\A\$\.paths\['([^']+)'\]\.(get|post|put|patch|delete)\.requestBody\.content\['application\/json'\]\.schema\z/))
	    operation = document.dig("paths", match[1], match[2])
	    request = operation.fetch("requestBody")
	    if request["$ref"]
	      request = Marshal.load(Marshal.dump(resolve_pointer(document, request.fetch("$ref"))))
	      operation["requestBody"] = request
	    end
	    request.dig("content", "application/json")["schema"] = update
  elsif target == "$.paths['/webhook-endpoints'].post.responses['201']"
    document.dig("paths", "/webhook-endpoints", "post", "responses")["201"] = update
  elsif target == "$.paths['/audit-events'].get.parameters"
    document.dig("paths", "/audit-events", "get")["parameters"] = update
  elsif target == "$.paths['/workstations/{workstation_id}/sessions/{session_id}:logout']"
    document["paths"]["/workstations/{workstation_id}/sessions/{session_id}:logout"] = update
  else
    abort "unsupported effective OpenAPI overlay target: #{target}"
  end
end

File.write(output, YAML.dump(document))
