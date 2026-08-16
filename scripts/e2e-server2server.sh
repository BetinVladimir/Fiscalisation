#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
base=${BEEFISCAL_E2E_URL:-http://localhost:8080}
auth_key=${BEEFISCAL_E2E_AUTH_KEY:-server2server-e2e-signing-key-32-bytes}
suffix=$(date +%s)

admin_token=$(AUTH_KEY="$auth_key" node -e 'const c=require("crypto"),e=v=>Buffer.from(JSON.stringify(v)).toString("base64url"),h=e({alg:"HS256",typ:"JWT"}),p=e({sub:"server2server-e2e",iss:"local-e2e",tenant_id:"platform",roles:["PLATFORM_INTEGRATION_ADMIN"],scope:"beefiscal.platform",exp:Math.floor(Date.now()/1000)+1800}),s=h+"."+p;process.stdout.write(s+"."+c.createHmac("sha256",process.env.AUTH_KEY).update(s).digest("base64url"))')
system_body=$(curl --max-time 10 -fsS -X POST "$base/platform/v1/external-systems" -H "Authorization: Bearer $admin_token" -H "Idempotency-Key: create-system-$suffix" -H 'Content-Type: application/json' --data "{\"code\":\"E2E_$suffix\",\"display_name\":\"Server2Server E2E\",\"webhook_url\":\"https://webhook.invalid/beefiscal-e2e\",\"webhook_events\":[\"integration.command.updated\"]}")
system_token=$(printf '%s' "$system_body" | jq -er .bootstrap_token)
tax=$(printf '%09d' $((suffix%1000000000)))
email="e2e-$suffix@example.com"
enroll_body=$(curl --max-time 10 -fsS -X POST "$base/integration/v1/enrollments" -H "Authorization: Bearer $system_token" -H "Idempotency-Key: enroll-start-$suffix" -H 'Content-Type: application/json' --data "{\"email\":\"$email\",\"source_company_id\":\"company-$suffix\",\"company\":{\"legal_name\":\"E2E Company $suffix\",\"tax_identifier\":{\"country\":\"BG\",\"type\":\"EIK\",\"value\":\"$tax\"},\"address\":\"Sofia\"}}")
temporary_token=$(printf '%s' "$enroll_body" | jq -er .temporary_token)
otp=$(docker compose -f "$root/compose.fiscalisation.yaml" -f "$root/compose.fiscalisation.dev.yaml" exec -T postgres psql -U fiscal -d fiscal -Atc "select right(body_text,6) from fiscal_email_outbox where recipient='$email' order by created_at desc limit 1")
verify_body=$(curl --max-time 10 -fsS -X POST "$base/integration/v1/enrollments:verify" -H "Authorization: Bearer $temporary_token" -H "Idempotency-Key: enroll-verify-$suffix" -H 'Content-Type: application/json' --data "{\"code\":\"$otp\"}")
tenant_token=$(printf '%s' "$verify_body" | jq -er .access_token)
tenant_id=$(printf '%s' "$verify_body" | jq -er .tenant_id)

BEEFISCAL_BASE_URL="$base" BEEFISCAL_TENANT_TOKEN="$tenant_token" node "$root/docs/integration-kit/conformance/check.mjs"
organization_count=$(docker compose -f "$root/compose.fiscalisation.yaml" -f "$root/compose.fiscalisation.dev.yaml" exec -T postgres psql -U fiscal -d fiscal -Atc "select count(*) from fiscal_runtime_resources where tenant_id='$tenant_id' and kind='organization'")
test "$organization_count" = 1
printf '%s\n' 'Server-to-server E2E passed'
