#!/usr/bin/env ruby
require "json"

# Static source evidence is necessary, not browser/HIL or protocol acceptance.
# Each extracted module and its caller wiring are independently required.
class UiAcceptanceVerifier
  CATEGORIES = %w[touch accessibility test_ids unknown final_device web_ble public_api readiness reports admin_editors platforms interaction_e2e].freeze
  def initialize(root, matrix)
    @root = File.expand_path(root)
    @matrix = matrix
  end

  def errors
    result = []
    cases = @matrix.fetch("cases")
    result << "UI matrix must cover both apps and all platforms" unless @matrix.fetch("platforms").sort == %w[android ios web]
    result << "UI matrix requires at least 12 cases" if cases.length < 12
    ids = cases.map { |item| item.fetch("id") }
    result << "duplicate UI case id" unless ids.uniq.length == ids.length
    %w[BeeMiniPOS BeeFiscalApp both].each do |app|
      result << "UI matrix missing #{app}" unless cases.any? { |item| item.fetch("app") == app }
    end
    CATEGORIES.each do |category|
      result << "UI matrix missing category #{category}" unless cases.any? { |item| item.fetch("category") == category }
    end
    cases.each do |item|
      id = item.fetch("id")
      result << "#{id}: invariant is too weak" if item.fetch("invariant").length < 24
      blocks = [item] + Array(item["evidence"])
      blocks.each do |block|
        relative = block.fetch("source")
        source = File.expand_path(relative, @root)
        unless source.start_with?(@root + File::SEPARATOR)
          result << "#{id}: source outside repository #{relative}"
          next
        end
        unless File.file?(source)
          result << "#{id}: source missing #{relative}"
          next
        end
        anchors = block.fetch("anchors")
        if !anchors.is_a?(Array) || anchors.empty? || anchors.any? { |a| !a.is_a?(String) || a.strip.empty? }
          result << "#{id}: source #{relative} needs nonempty required anchors"
          next
        end
        body = File.read(source).gsub(/\s+/, "")
        anchors.each do |anchor|
          result << "#{id}: #{relative}: missing anchor #{anchor.inspect}" unless body.include?(anchor.gsub(/\s+/, ""))
        end
        Array(block["forbidden"]).each do |anchor|
          result << "#{id}: #{relative}: forbidden anchor #{anchor.inspect}" if body.include?(anchor.gsub(/\s+/, ""))
        end
      end
      if item["status"] == "OPEN"
        gap = item.fetch("gap", "")
        result << "#{id}: OPEN requires a concrete gap description" if gap.length < 24
        result << "#{id}: OPEN: #{gap}"
      elsif item.key?("status")
        result << "#{id}: unsupported status #{item['status'].inspect}"
      end
    end
    result
  end
end

if $PROGRAM_NAME == __FILE__
  begin
    root = File.expand_path("..", __dir__)
    matrix = JSON.parse(File.read(File.join(root, "contracts/ui-acceptance-matrix.json")))
    errors = UiAcceptanceVerifier.new(root, matrix).errors
    abort errors.join("\n") unless errors.empty?
    puts "UI acceptance matrix static evidence OK: #{matrix.fetch('cases').length} cases, Android/iOS/Web, both apps; runtime/browser/HIL evidence is separate"
  rescue KeyError, JSON::ParserError, TypeError, ArgumentError => e
    abort "Invalid UI matrix: #{e.message}"
  end
end
