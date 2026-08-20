#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
test_dir="$root/tests/e2e/full-fiscal"
fiscal_base=${E2E_FISCAL_URL:-http://localhost:18000}
mini_base=${E2E_MINIPOS_URL:-http://localhost:18001}
api_version=2026-08-07
auth_key=${E2E_AUTH_KEY:-full-e2e-auth-signing-key-32-bytes}
export E2E_CREDENTIAL_KEY=${E2E_CREDENTIAL_KEY:-MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=}
suffix="$(date +%s)-$$"
email="full-e2e-$suffix@example.test"
tax=$(printf '20%07d' $((($$ + $(date +%s)) % 10000000)))

fiscal_compose="docker compose -p beeloy-full-e2e-fiscal -f $root/compose.fiscalisation.yaml -f $test_dir/compose.fiscal.override.yaml"
mini_compose="docker compose -p beeloy-full-e2e-minipos -f $root/compose.minipos.yaml -f $test_dir/compose.minipos.override.yaml"
mock_compose="docker compose -p beeloy-full-e2e-mocks -f $test_dir/compose.yaml"

cleanup() {
  cleanup_status=$?
  if [ "${KEEP_E2E:-0}" = 1 ]; then
    printf '%s\n' 'KEEP_E2E=1: окружение оставлено запущенным'
    return "$cleanup_status"
  fi
  $mini_compose down -v --remove-orphans >/dev/null 2>&1 || true
  $fiscal_compose down -v --remove-orphans >/dev/null 2>&1 || true
  $mock_compose down -v --remove-orphans >/dev/null 2>&1 || true
  return "$cleanup_status"
}
trap cleanup EXIT INT TERM

wait_url() {
  url=$1; label=$2; i=0
  until curl --max-time 3 -fsS "$url" >/dev/null 2>&1; do
    i=$((i+1)); [ "$i" -lt 90 ] || { printf 'Timeout: %s (%s)\n' "$label" "$url" >&2; return 1; }
    sleep 2
  done
}

otp_after() {
  recipient=$1; minimum_id=$2; i=0
  while [ "$i" -lt 60 ]; do
    value=$(curl --max-time 3 -fsS "http://localhost:18080/messages?to=$recipient" | jq -r --argjson n "$minimum_id" '[.items[] | select(.id > $n) | .body | capture("(?<code>[0-9]{6})").code] | last // empty')
    [ -n "$value" ] && { printf '%s' "$value"; return; }
    i=$((i+1)); sleep 1
  done
  printf 'OTP not captured for %s\n' "$recipient" >&2; return 1
}

jwt() {
  JWT_KEY="$auth_key" JWT_TENANT="$1" JWT_ROLE="$2" JWT_SCOPE="$3" node -e '
    const c=require("crypto"), enc=x=>Buffer.from(JSON.stringify(x)).toString("base64url");
    const h=enc({alg:"HS256",typ:"JWT"}), p=enc({sub:"full-e2e-user",iss:"local-e2e",tenant_id:process.env.JWT_TENANT,roles:[process.env.JWT_ROLE],scope:process.env.JWT_SCOPE,exp:Math.floor(Date.now()/1000)+3600});
    const s=h+"."+p; process.stdout.write(s+"."+c.createHmac("sha256",process.env.JWT_KEY).update(s).digest("base64url"));'
}

api() {
  method=$1; url=$2; token=$3; key=$4; body=${5:-}; if_match=${6:-}
  response_file=$(mktemp "${TMPDIR:-/tmp}/beeloy-e2e-response.XXXXXX")
  if [ -n "$body" ] && [ -n "$if_match" ]; then
    response_status=$(curl --max-time 20 -sS -o "$response_file" -w '%{http_code}' -X "$method" "$url" -H "Authorization: Bearer $token" -H "X-Api-Version: $api_version" -H "Idempotency-Key: $key" -H "If-Match: $if_match" -H 'Content-Type: application/json' --data "$body") || { cat "$response_file" >&2; rm -f "$response_file"; return 1; }
  elif [ -n "$body" ]; then
    response_status=$(curl --max-time 20 -sS -o "$response_file" -w '%{http_code}' -X "$method" "$url" -H "Authorization: Bearer $token" -H "X-Api-Version: $api_version" -H "Idempotency-Key: $key" -H 'Content-Type: application/json' --data "$body") || { cat "$response_file" >&2; rm -f "$response_file"; return 1; }
  else
    response_status=$(curl --max-time 20 -sS -o "$response_file" -w '%{http_code}' -X "$method" "$url" -H "Authorization: Bearer $token" -H "X-Api-Version: $api_version" -H "Idempotency-Key: $key") || { cat "$response_file" >&2; rm -f "$response_file"; return 1; }
  fi
  case "$response_status" in
    2??) ;;
    *) printf 'HTTP %s %s %s\n' "$response_status" "$method" "$url" >&2; cat "$response_file" >&2; rm -f "$response_file"; return 1 ;;
  esac
  cat "$response_file"
  rm -f "$response_file"
}

printf '%s\n' '[1/8] Запуск SMTP и device mocks'
$mock_compose up -d --build --wait

printf '%s\n' '[2/8] Запуск Fiscal dev stack'
APP_ENV=e2e AUTH_HMAC_KEY="$auth_key" FISCAL_HTTP_PORT=18000 FISCAL_HTTPS_PORT=18443 FISCAL_POSTGRES_PUBLISH_PORT=15433 REDIS_PORT=16380 RABBITMQ_AMQP_PORT=15674 RABBITMQ_MANAGEMENT_PORT=25674 EMQX_MQTT_PORT=11884 EMQX_WS_PORT=28083 EMQX_WSS_PORT=28084 EMQX_MQTTS_PORT=18884 EMQX_DASHBOARD_PORT=28085 $fiscal_compose up -d postgres redis rabbitmq emqx
# Docker Compose <2.24 merges multi-network lists and on some Docker Desktop
# versions attaches only the last network. Explicit connects keep the runner
# compatible with the repository's currently supported Compose 2.19.
for service in redis rabbitmq emqx; do
  container=$($fiscal_compose ps -q "$service")
  docker network connect beeloy-full-e2e-fiscal_private "$container" 2>/dev/null || true
done
APP_ENV=e2e AUTH_HMAC_KEY="$auth_key" FISCAL_HTTP_PORT=18000 FISCAL_HTTPS_PORT=18443 FISCAL_POSTGRES_PUBLISH_PORT=15433 REDIS_PORT=16380 RABBITMQ_AMQP_PORT=15674 RABBITMQ_MANAGEMENT_PORT=25674 EMQX_MQTT_PORT=11884 EMQX_WS_PORT=28083 EMQX_WSS_PORT=28084 EMQX_MQTTS_PORT=18884 EMQX_DASHBOARD_PORT=28085 $fiscal_compose up -d --build fiscal-backend caddy
fiscal_caddy=$($fiscal_compose ps -q caddy)
docker network connect beeloy-full-e2e-fiscal_private "$fiscal_caddy" 2>/dev/null || true
wait_url "$fiscal_base/healthz" Fiscal

admin_token=$(jwt platform PLATFORM_INTEGRATION_ADMIN beefiscal.platform)
system=$(api POST "$fiscal_base/platform/v1/external-systems" "$admin_token" "system-$suffix" "{\"code\":\"FULL_E2E_$suffix\",\"display_name\":\"Full E2E\",\"webhook_url\":\"https://webhook.invalid/full-e2e\",\"webhook_events\":[\"integration.command.updated\"]}")
export E2E_FISCAL_SYSTEM_TOKEN=$(printf '%s' "$system" | jq -er .bootstrap_token)

printf '%s\n' '[3/8] Запуск MiniPOS и регистрация компании по OTP'
APP_ENV=e2e AUTH_HMAC_KEY="$auth_key" MINIPOS_HTTP_PORT=18001 MINIPOS_HTTPS_PORT=18444 $mini_compose up -d --build
mini_caddy=$($mini_compose ps -q caddy)
docker network connect beeloy-full-e2e-minipos_private "$mini_caddy" 2>/dev/null || true
wait_url "$mini_base/healthz" MiniPOS
before=$(curl -fsS http://localhost:18080/messages | jq '.items | length')
curl -fsS -X POST "$mini_base/public/v1/minipos/auth/request-code" -H "X-Api-Version: $api_version" -H 'Content-Type: application/json' --data "{\"email\":\"$email\",\"language\":\"ru\"}" >/dev/null
mini_otp=$(otp_after "$email" "$before")
auth=$(curl -fsS -X POST "$mini_base/public/v1/minipos/auth/verify-code" -H "X-Api-Version: $api_version" -H 'Content-Type: application/json' --data "{\"email\":\"$email\",\"code\":\"$mini_otp\"}")
onboarding_token=$(printf '%s' "$auth" | jq -er .onboarding_token)
tokens=$(curl -fsS -X POST "$mini_base/public/v1/minipos/auth/onboarding" -H "X-Api-Version: $api_version" -H 'Content-Type: application/json' --data "{\"onboarding_token\":\"$onboarding_token\",\"company_name\":\"E2E Company $suffix\",\"address\":\"Sofia, E2E 1\",\"tax_identifier\":\"$tax\",\"full_name\":\"E2E Administrator\"}")
mini_token=$(printf '%s' "$tokens" | jq -er .access_token)
company_id=$(JWT_VALUE="$mini_token" node -e 'const p=process.env.JWT_VALUE.split(".")[1];process.stdout.write(JSON.parse(Buffer.from(p,"base64url")).tenant_id)')

printf '%s\n' '[4/8] Связывание MiniPOS → Fiscal и проверка ACTIVE tenant'
before=$(curl -fsS http://localhost:18080/messages | jq '.items | length')
enroll_key=$(node -e 'console.log(require("crypto").randomUUID())')
api POST "$mini_base/public/v1/minipos/fiscal-enrollment" "$mini_token" "$enroll_key" "{\"email\":\"$email\",\"source_company_id\":\"$company_id\",\"legal_name\":\"E2E Company $suffix\",\"tax_country\":\"BG\",\"tax_type\":\"EIK\",\"tax_value\":\"$tax\",\"address\":\"Sofia, E2E 1\"}" >/dev/null
fiscal_otp=$(otp_after "$email" "$before")
mini_verify_key=$(node -e 'console.log(require("crypto").randomUUID())')
enrollment=$(api POST "$mini_base/public/v1/minipos/fiscal-enrollment:verify" "$mini_token" "$mini_verify_key" "{\"enrollment_idempotency_key\":\"$enroll_key\",\"code\":\"$fiscal_otp\"}")
tenant_id=$(printf '%s' "$enrollment" | jq -er .tenant_id)
active=$($fiscal_compose exec -T postgres psql -U fiscal -d fiscal -Atc "select count(*) from tenant_source_bindings where tenant_id='$tenant_id' and source_company_id='$company_id' and status='ACTIVE'")
[ "$active" = 1 ] || { printf '%s\n' 'Fiscal tenant binding is not ACTIVE' >&2; exit 1; }

printf '%s\n' '[5/8] Синхронизация конфигурации и оператора из MiniPOS'
location_source=$(node -e 'console.log(require("crypto").randomUUID())')
register_source=$(node -e 'console.log(require("crypto").randomUUID())')
adapter_id=$(node -e 'console.log(require("crypto").randomUUID())')
configuration_key=$(node -e 'console.log(require("crypto").randomUUID())')
api PATCH "$mini_base/public/v1/minipos/configuration" "$mini_token" "$configuration_key" "{\"location_name\":\"E2E Shop\",\"location_address\":\"Sofia\",\"workstation_name\":\"E2E Register\",\"location_id\":\"$location_source\",\"fiscal_register_id\":\"$register_source\",\"fiscal_adapter_id\":\"$adapter_id\",\"binding_generation\":1,\"adapter_base_url\":\"http://edge-agent-s3-mock/beeloy/local/v1\"}" >/dev/null
employees=$(api GET "$mini_base/public/v1/minipos/employees" "$mini_token" "read-employees-$suffix")
employee=$(printf '%s' "$employees" | jq -c '.items[0]')
employee_id=$(printf '%s' "$employee" | jq -er .id)
employee_version=$(printf '%s' "$employee" | jq -er .version)
employee_body=$(printf '%s' "$employee" | jq -c '{first_name,last_name,operator_code:"E001",roles,status:"ACTIVE"}')
employee_key=$(node -e 'console.log(require("crypto").randomUUID())')
curl -fsS -X PATCH "$mini_base/public/v1/minipos/employees/$employee_id" -H "Authorization: Bearer $mini_token" -H "X-Api-Version: $api_version" -H "Idempotency-Key: $employee_key" -H "If-Match: $employee_version" -H 'Content-Type: application/json' --data "$employee_body" >/dev/null

printf '%s\n' '[6/8] Реальные Fiscal CASH/CARD операции, чек и возврат'
tenant_token=$(jwt "$tenant_id" ADMIN fiscal.base)
device_data="{\"kind\":\"SMART_DEVICE\",\"status\":\"DRAFT\",\"serial\":\"E2E-$suffix\",\"fiscal_device_number\":\"12345678\",\"fiscal_memory_number\":\"87654321\",\"vendor\":\"E2E\",\"model\":\"COMBINED\",\"environment\":\"DEV\",\"simulated\":true}"
device=$(api POST "$fiscal_base/public/v1/devices" "$tenant_token" "device-$suffix" "$device_data")
device_id=$(printf '%s' "$device" | jq -er '.id // .device_id')
pending_data=$(printf '%s' "$device_data" | jq -c '.status="PENDING_SERVICE_ACTIVATION"')
pending=$(curl -fsS -X PATCH "$fiscal_base/public/v1/devices/$device_id" -H "Authorization: Bearer $tenant_token" -H "X-Api-Version: $api_version" -H "Idempotency-Key: device-pending-$suffix" -H 'If-Match: 1' -H 'Content-Type: application/json' --data "$pending_data")
pending_version=$(printf '%s' "$pending" | jq -er .version)
active_data=$(printf '%s' "$device_data" | jq -c '.status="ACTIVE"')
device=$(curl -fsS -X PATCH "$fiscal_base/public/v1/devices/$device_id" -H "Authorization: Bearer $tenant_token" -H "X-Api-Version: $api_version" -H "Idempotency-Key: device-active-$suffix" -H "If-Match: $pending_version" -H 'Content-Type: application/json' --data "$active_data")
location=$(api POST "$fiscal_base/public/v1/locations" "$tenant_token" "location-$suffix" "{\"code\":\"L$suffix\",\"name\":\"Fiscal E2E Shop\",\"address\":\"Sofia\",\"status\":\"ACTIVE\"}")
location_id=$(printf '%s' "$location" | jq -er .id)
register=$(api POST "$fiscal_base/public/v1/registers" "$tenant_token" "register-$suffix" "{\"code\":\"R$suffix\",\"location_id\":\"$location_id\",\"status\":\"ACTIVE\"}")
register_id=$(printf '%s' "$register" | jq -er .id)
api POST "$fiscal_base/public/v1/registers/$register_id/bindings" "$tenant_token" "bind-fiscal-$suffix" "{\"device_id\":\"$device_id\",\"role\":\"FISCAL_DEVICE\",\"active_from\":\"2020-01-01T00:00:00Z\"}" >/dev/null
api POST "$fiscal_base/public/v1/registers/$register_id/bindings" "$tenant_token" "bind-payment-$suffix" "{\"device_id\":\"$device_id\",\"role\":\"OPTIONAL_PAYMENT_TERMINAL\",\"active_from\":\"2020-01-01T00:00:00Z\"}" >/dev/null
operator_ready=0
operator_wait=0
while [ "$operator_wait" -lt 60 ]; do
  operator_ready=$($fiscal_compose exec -T postgres psql -U fiscal -d fiscal -Atc "select count(*) from fiscal_runtime_resources where tenant_id='$tenant_id' and kind='operator' and data->>'code'='E001'")
  [ "$operator_ready" = 1 ] && break
  operator_wait=$((operator_wait+1))
  sleep 1
done
[ "$operator_ready" = 1 ] || { printf '%s\n' 'Synced Fiscal operator E001 was not projected' >&2; exit 1; }

sale_flow() {
  payment=$1; serial=$2
  sale=$(api POST "$fiscal_base/public/v1/sales" "$tenant_token" "sale-$serial-$suffix" "{\"external_id\":\"sale-$serial-$suffix\",\"register_id\":\"$register_id\",\"operator_id\":\"E001\"}")
  sale_id=$(printf '%s' "$sale" | jq -er .sale_id)
  line=$(curl -fsS -X POST "$fiscal_base/public/v1/sales/$sale_id/lines" -H "Authorization: Bearer $tenant_token" -H "X-Api-Version: $api_version" -H "Idempotency-Key: line-$serial-$suffix" -H 'If-Match: 1' -H 'Content-Type: application/json' --data "{\"line_id\":\"line-$serial\",\"name\":\"E2E item\",\"quantity\":\"1.000\",\"unit_price\":{\"amount\":\"2.50\",\"currency\":\"EUR\"},\"tax_group\":\"B\"}")
  version=$(printf '%s' "$line" | jq -er .version)
  payment_key="payment-$serial-$suffix"
  payment_body="{\"payment_id\":\"payment-$serial\",\"type\":\"$payment\",\"amount\":{\"amount\":\"2.50\",\"currency\":\"EUR\"},\"terminal_policy\":\"REQUIRED\"}"
  operation=$(api POST "$fiscal_base/public/v1/sales/$sale_id/payments" "$tenant_token" "$payment_key" "$payment_body")
  [ "$(printf '%s' "$operation" | jq -r .state)" = FISCALIZED ]
  # A client may lose the first response after the device has already printed.
  # Replaying the same durable key must return the same operation, never charge
  # or fiscalise a second time.
  replay_operation=$(api POST "$fiscal_base/public/v1/sales/$sale_id/payments" "$tenant_token" "$payment_key" "$payment_body")
  [ "$(printf '%s' "$operation" | jq -r .operation_id)" = "$(printf '%s' "$replay_operation" | jq -r .operation_id)" ] || {
    printf '%s\n' 'Idempotency replay created a different fiscal operation' >&2
    exit 1
  }
  receipt=$(api GET "$fiscal_base/public/v1/sales/$sale_id/receipt" "$tenant_token" "receipt-$serial-$suffix")
  printf '%s' "$receipt" | jq -e '.fiscal_reference != null or .regulatory_identifiers != null' >/dev/null
  fiscal_reference=$(printf '%s' "$operation" | jq -er .fiscal_reference)
  sale_version=$(api GET "$fiscal_base/public/v1/sales/$sale_id" "$tenant_token" "sale-version-$serial-$suffix" | jq -er .version)
  reversal=$(api POST "$fiscal_base/public/v1/sales/$sale_id:reverse" "$tenant_token" "reverse-$serial-$suffix" "{\"reason_code\":\"CUSTOMER_RETURN\",\"original_fiscal_reference\":\"$fiscal_reference\"}" "$sale_version")
  printf '%s' "$reversal" | jq -e '.state == "FISCALIZED"' >/dev/null
}
sale_flow CASH cash
sale_flow CARD card

# Optimistic concurrency is checked against the real persistence layer. A
# stale editor must not overwrite a register updated by another client.
conflict_file=$(mktemp "${TMPDIR:-/tmp}/beeloy-e2e-conflict.XXXXXX")
conflict_status=$(curl --max-time 20 -sS -o "$conflict_file" -w '%{http_code}' -X PATCH "$fiscal_base/public/v1/registers/$register_id" \
  -H "Authorization: Bearer $tenant_token" -H "X-Api-Version: $api_version" \
  -H "Idempotency-Key: stale-register-$suffix" -H 'If-Match: 0' \
  -H 'Content-Type: application/json' \
  --data "{\"code\":\"R$suffix\",\"location_id\":\"$location_id\",\"status\":\"ACTIVE\"}")
case "$conflict_status" in
  409|412) ;;
  *) printf 'Expected stale If-Match conflict, got HTTP %s: ' "$conflict_status" >&2; cat "$conflict_file" >&2; rm -f "$conflict_file"; exit 1 ;;
esac
rm -f "$conflict_file"

printf '%s\n' '[7/8] Локальные операции edge-agent-s3 и bluecash-app mocks'
for endpoint in 19001 19002; do
  curl -fsS "http://localhost:$endpoint/beeloy/local/v1/readyz" -H 'Authorization: Bearer e2e-local-token' | jq -e '.state == "READY"' >/dev/null
done
edge=$(curl -fsS -X POST http://localhost:19001/beeloy/local/v1/intents -H 'Authorization: Bearer e2e-local-token' -H 'Idempotency-Key: edge-operation-e2e' -H 'Content-Type: application/json' --data '{"intent_id":"edge-intent","client_operation_id":"edge-operation-e2e","payment_type":"CASH","binding_generation":1}')
printf '%s' "$edge" | jq -e '.state == "FISCALIZED" and .profile == "edge-agent-s3"' >/dev/null
edge_id=$(printf '%s' "$edge" | jq -er .operation_id)
curl -fsS "http://localhost:19001/beeloy/local/v1/operations/$edge_id:reconcile" -H 'Authorization: Bearer e2e-local-token' | jq -e '.reconciled == true' >/dev/null
blue=$(curl -fsS -X POST http://localhost:19002/beeloy/local/v1/intents -H 'Authorization: Bearer e2e-local-token' -H 'Idempotency-Key: blue-operation-e2e' -H 'Content-Type: application/json' --data '{"intent_id":"blue-intent","client_operation_id":"blue-operation-e2e","payment_type":"CARD","binding_generation":1}')
printf '%s' "$blue" | jq -e '.state == "FISCALIZED" and .profile == "bluecash-app" and (.rrn | length) > 0' >/dev/null
replay=$(curl -fsS -X POST http://localhost:19002/beeloy/local/v1/intents -H 'Authorization: Bearer e2e-local-token' -H 'Idempotency-Key: blue-operation-e2e' -H 'Content-Type: application/json' --data '{"payment_type":"CARD"}')
[ "$(printf '%s' "$blue" | jq -r .operation_id)" = "$(printf '%s' "$replay" | jq -r .operation_id)" ]

printf '%s\n' '[8/8] Все полные E2E проверки пройдены'
printf 'company_id=%s fiscal_tenant_id=%s email=%s\n' "$company_id" "$tenant_id" "$email"
