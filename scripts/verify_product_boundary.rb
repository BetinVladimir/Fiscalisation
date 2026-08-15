#!/usr/bin/env ruby
root = File.expand_path("..", __dir__)
fiscal = File.read(File.join(root, "compose.fiscalisation.yaml"))
minipos = File.read(File.join(root, "compose.minipos.yaml"))
minipos_prod = File.read(File.join(root, "compose.minipos.prod.yaml"))

abort "Compose product names must be independent" unless fiscal.include?("name: beefiscal-") && minipos.include?("name: beeminipos-")
abort "MiniPOS Compose crossed into Fiscal private services" if minipos.match?(/^\s{2}fiscal-backend:/) || minipos.include?("database/fiscal")
abort "Fiscal Compose crossed into MiniPOS private services" if fiscal.match?(/^\s{2}beeminipos-backend:/) || fiscal.include?("database/minipos")
abort "MiniPOS database ownership drift" unless minipos.include?("/minipos?sslmode=disable") && minipos.include?("./database/minipos:")
abort "Fiscal database ownership drift" unless fiscal.include?("/fiscal?sslmode=disable") && fiscal.include?("./database/fiscal:")
abort "MiniPOS must receive only FISCAL_PUBLIC_BASE_URL" unless minipos.include?('FISCAL_PUBLIC_BASE_URL: "${FISCAL_PUBLIC_BASE_URL:-http://host.docker.internal:8080/public/v1}"')
abort "MiniPOS PROD must require FISCAL_PUBLIC_BASE_URL" unless minipos_prod.include?('FISCAL_PUBLIC_BASE_URL: "${FISCAL_PUBLIC_BASE_URL:?required}"')

def service_block(body, service)
  match = body.match(/^  #{Regexp.escape(service)}:\n(?<block>.*?)(?=^  [a-zA-Z0-9_-]+:\n|^networks:)/m)
  match && match[:block]
end

{
  "MiniPOS postgres" => [minipos, "postgres", "networks: [private]"],
  "MiniPOS backend" => [minipos, "beeminipos-backend", "networks: [private]"],
  "MiniPOS Caddy" => [minipos, "caddy", "networks: [private, ingress]"],
  "Fiscal postgres" => [fiscal, "postgres", "networks: [private]"],
  "Fiscal backend" => [fiscal, "fiscal-backend", "networks: [private]"],
  "Fiscal Caddy" => [fiscal, "caddy", "networks: [private, ingress]"]
}.each do |name, (body, service, expected)|
  block = service_block(body, service)
  abort "#{name} service missing or network boundary drift" unless block&.include?(expected)
end

mini_sources = Dir.glob(File.join(root, "minipos/beeminipos-backend", "{cmd,internal}", "**", "*.go")).reject { |path| path.end_with?("_test.go") }
mini_body = mini_sources.map { |path| File.read(path) }.join("\n")
abort "MiniPOS imports Fiscal private backend code" if mini_body.include?("fiscal-backend/")
abort "MiniPOS calls an Edge/Fiscal internal HTTP API" if mini_body.include?("/internal/v1")

config_body = File.read(File.join(root, "minipos/beeminipos-backend/internal/config/config.go"))
abort "Exact /public/v1 Fiscal base validation is missing" unless config_body.include?('parsed.Path == "/public/v1"') && config_body.include?("parsed.RawQuery == \"\"")

service_body = File.read(File.join(root, "minipos/beeminipos-backend/internal/domain/service.go"))
allowed_prefixes = %w[/sales /operations /registers]
literal_paths = service_body.scan(/s\.call(?:WithIfMatch)?\("(?:GET|POST|PATCH|DELETE)",\s*"([^"+]+)["+]/).flatten
bad = literal_paths.reject { |path| allowed_prefixes.any? { |prefix| path.start_with?(prefix) } }
abort "MiniPOS downstream path leaves public Fiscal surface: #{bad.inspect}" unless bad.empty?
abort "MiniPOS Fiscal client ignores request-construction failure" unless service_body.include?("build Fiscal public API request")

puts "product boundary OK: independent Compose/DB/private networks; MiniPOS downstream restricted to exact public /public/v1 API"
