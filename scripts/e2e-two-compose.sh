#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
fiscal_project="beefiscal_e2e_$$"
minipos_project="beeminipos_e2e_$$"
shared_ingress="beeloy_e2e_ingress_$$"
client_name="beeloy_e2e_client_$$"
fiscal_publish_port=${E2E_FISCAL_PORT:-0}
minipos_publish_port=${E2E_MINIPOS_PORT:-0}
fiscal_base_url=http://fiscal-public/public/v1
api_version=2026-08-07
fiscal_secret=e2e-fiscal-db-password
minipos_secret=e2e-minipos-db-password
fiscal_rls_secret=e2e-fiscal-rls-password
minipos_rls_secret=e2e-minipos-rls-password
webhook_key=e2e-webhook-signing-key-32-bytes
ble_key=e2e-ble-signing-key-32-bytes-000
auth_key=e2e-auth-signing-key-32-bytes-000
auth_token=$(AUTH_KEY="$auth_key" ruby -rjson -ropenssl -rbase64 -e 'enc=->(v){Base64.urlsafe_encode64(v,padding:false)};h=enc.call({alg:"HS256",typ:"JWT"}.to_json);p=enc.call({sub:"e2e-admin",tenant_id:"e2e-tenant",roles:["ADMIN"],scope:"fiscal.base",exp:Time.now.to_i+7200}.to_json);s="#{h}.#{p}";puts "#{s}.#{enc.call(OpenSSL::HMAC.digest("SHA256",ENV.fetch("AUTH_KEY"),s))}"')
fiscal_backup=/tmp/beeloy-fiscal-backup-$$.dump
minipos_backup=/tmp/beeloy-minipos-backup-$$.dump

fiscal_compose() {
  APP_ENV=dev FISCAL_HTTP_PORT="$fiscal_publish_port" FISCAL_HTTPS_PORT=0 FISCAL_DB_PASSWORD="$fiscal_secret" FISCAL_RLS_DB_PASSWORD="$fiscal_rls_secret" FISCAL_DB_HOST="${fiscal_db_host:-postgres}" FISCAL_UPSTREAM="${fiscal_upstream:-fiscal-backend:8080}" WEBHOOK_SIGNING_KEY="$webhook_key" BLE_SIGNING_KEY="$ble_key" AUTH_HMAC_KEY="$auth_key" docker compose -p "$fiscal_project" -f "$root/compose.fiscalisation.yaml" -f "$root/compose.fiscalisation.dev.yaml" -f "$root/compose.fiscalisation.e2e.yaml" "$@"
}
minipos_compose() {
  APP_ENV=dev MINIPOS_HTTP_PORT="$minipos_publish_port" MINIPOS_HTTPS_PORT=0 MINIPOS_DB_PASSWORD="$minipos_secret" MINIPOS_RLS_DB_PASSWORD="$minipos_rls_secret" MINIPOS_DB_HOST="${minipos_db_host:-postgres}" MINIPOS_UPSTREAM="${minipos_upstream:-beeminipos-backend:8081}" FISCAL_PUBLIC_BASE_URL="$fiscal_base_url" FISCAL_AUTH_TOKEN="$auth_token" AUTH_HMAC_KEY="$auth_key" WEBHOOK_VERIFICATION_KEY="$webhook_key" docker compose -p "$minipos_project" -f "$root/compose.minipos.yaml" -f "$root/compose.minipos.dev.yaml" -f "$root/compose.minipos.e2e.yaml" "$@"
}
cleanup() {
  docker rm -f "$client_name" >/dev/null 2>&1 || true
  fiscal_compose down -v --remove-orphans >/dev/null 2>&1 || true
  minipos_compose down -v --remove-orphans >/dev/null 2>&1 || true
  docker network rm "$shared_ingress" >/dev/null 2>&1 || true
  rm -f "$fiscal_backup" "$minipos_backup"
}
failed() {
  status=$?
  if [ "$status" -ne 0 ]; then
    echo "E2E failed at step: ${step:-bootstrap}" >&2
    fiscal_compose ps >&2 || true
    minipos_compose ps >&2 || true
    fiscal_compose logs --no-color --tail=120 >&2 || true
    minipos_compose logs --no-color --tail=120 >&2 || true
  fi
  cleanup
  exit "$status"
}
trap failed EXIT INT TERM

wait_http() {
  url=$1
  attempts=0
  until client_curl --max-time 3 --silent --show-error --fail "$url" >/dev/null 2>&1; do
    attempts=$((attempts+1))
    if [ "$attempts" -ge 90 ]; then
      echo "timeout waiting for $url" >&2
      return 1
    fi
    sleep 1
  done
}

client_curl() {
  docker exec "$client_name" curl --connect-timeout 3 --max-time 30 -H "Authorization: Bearer $auth_token" "$@"
}

request() {
  method=$1
  url=$2
  key=$3
  body=${4:-}
  match=${5:-}
  if [ -n "$body" ]; then
    if [ -n "$match" ]; then
      raw=$(client_curl --silent --show-error --write-out '\n%{http_code}' -X "$method" -H "X-Api-Version: $api_version" -H "Idempotency-Key: $key" -H "If-Match: $match" -H 'Content-Type: application/json' --data "$body" "$url")
    else
      raw=$(client_curl --silent --show-error --write-out '\n%{http_code}' -X "$method" -H "X-Api-Version: $api_version" -H "Idempotency-Key: $key" -H 'Content-Type: application/json' --data "$body" "$url")
    fi
  else
    raw=$(client_curl --silent --show-error --write-out '\n%{http_code}' -X "$method" -H "X-Api-Version: $api_version" -H "Idempotency-Key: $key" "$url")
  fi
  status=$(printf '%s\n' "$raw" | tail -n 1)
  response=$(printf '%s\n' "$raw" | sed '$d')
  echo "E2E HTTP $status $url" >&2
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "E2E response: $response" >&2
    return 1
  fi
  printf '%s' "$response"
}

assert_json_eq() {
  json=$1
  expression=$2
  expected=$3
  actual=$(printf '%s' "$json" | jq -er "$expression") || {
    echo "invalid response for $step: $json" >&2
    return 1
  }
  if [ "$actual" != "$expected" ]; then
    echo "unexpected response for $step: expected $expression=$expected, actual=$actual; body=$json" >&2
    return 1
  fi
}

step="start fiscal stack"
docker network create "$shared_ingress" >/dev/null
fiscal_compose up -d postgres
fiscal_db_host=$(docker inspect -f "{{(index .NetworkSettings.Networks \"${fiscal_project}_private\").IPAddress}}" "$(fiscal_compose ps -q postgres)")
fiscal_compose up -d --build fiscal-backend
docker network connect "$shared_ingress" "$(fiscal_compose ps -q fiscal-backend)"
fiscal_upstream=$(docker inspect -f "{{(index .NetworkSettings.Networks \"${shared_ingress}\").IPAddress}}:8080" "$(fiscal_compose ps -q fiscal-backend)")
fiscal_compose up -d caddy
docker network connect --alias fiscal-public "$shared_ingress" "$(fiscal_compose ps -q caddy)"
fiscal_public_ip=$(docker inspect -f "{{(index .NetworkSettings.Networks \"${shared_ingress}\").IPAddress}}" "$(fiscal_compose ps -q caddy)")
step="start MiniPOS stack"
minipos_compose up -d postgres
minipos_db_host=$(docker inspect -f "{{(index .NetworkSettings.Networks \"${minipos_project}_private\").IPAddress}}" "$(minipos_compose ps -q postgres)")
fiscal_base_url="http://${fiscal_public_ip}/public/v1"
minipos_compose up -d --build beeminipos-backend
docker network connect "$shared_ingress" "$(minipos_compose ps -q beeminipos-backend)"
minipos_upstream=$(docker inspect -f "{{(index .NetworkSettings.Networks \"${shared_ingress}\").IPAddress}}:8081" "$(minipos_compose ps -q beeminipos-backend)")
minipos_compose up -d caddy
docker network connect --alias minipos-public "$shared_ingress" "$(minipos_compose ps -q caddy)"
docker run -d --name "$client_name" --network "$shared_ingress" --entrypoint sleep curlimages/curl:8.12.1 600 >/dev/null
members=$(docker network inspect -f '{{range .Containers}}{{.Name}} {{end}}' "$shared_ingress")
member_count=$(printf '%s' "$members" | wc -w | tr -d ' ')
test "$member_count" = 5
case "$members" in
  *postgres*) echo "private database attached to shared ingress: $members" >&2; exit 1 ;;
esac
step="wait for health endpoints"
minipos_public_ip=$(docker inspect -f "{{(index .NetworkSettings.Networks \"${shared_ingress}\").IPAddress}}" "$(minipos_compose ps -q caddy)")
wait_http "http://${fiscal_public_ip}/healthz"
wait_http "http://${minipos_public_ip}/healthz"

pos="http://${minipos_public_ip}/public/v1/minipos"
fiscal="http://${fiscal_public_ip}/public/v1"
step="provision active fiscal register"
location=$(request POST "$fiscal/locations" fiscal-location-0001 '{"code":"E2E-SOF","name":"E2E Sofia","address":"1 Main","status":"ACTIVE"}')
location_id=$(printf '%s' "$location" | jq -er .id)
register=$(request POST "$fiscal/registers" fiscal-register-0001 "{\"location_id\":\"$location_id\",\"code\":\"E2E-R01\",\"status\":\"ACTIVE\"}")
register_id=$(printf '%s' "$register" | jq -er .id)
device=$(request POST "$fiscal/devices" fiscal-device-00001 '{"kind":"FISCAL_DEVICE","vendor":"Datecs","model":"DP-150 MX","serial":"E2E-DP150-001","status":"DRAFT","environment":"DEV","simulated":true}')
device_id=$(printf '%s' "$device" | jq -er .id)
device_version=$(printf '%s' "$device" | jq -er .version)
device=$(request PATCH "$fiscal/devices/$device_id" fiscal-device-pending-0001 '{"kind":"FISCAL_DEVICE","vendor":"Datecs","model":"DP-150 MX","serial":"E2E-DP150-001","status":"PENDING_SERVICE_ACTIVATION","environment":"DEV","simulated":true}' "$device_version")
device_version=$(printf '%s' "$device" | jq -er .version)
device=$(request PATCH "$fiscal/devices/$device_id" fiscal-device-active-00001 '{"kind":"FISCAL_DEVICE","vendor":"Datecs","model":"DP-150 MX","serial":"E2E-DP150-001","status":"ACTIVE","environment":"DEV","simulated":true}' "$device_version")
binding=$(request POST "$fiscal/registers/$register_id/bindings" fiscal-binding-0001 "{\"device_id\":\"$device_id\",\"role\":\"FISCAL_DEVICE\"}")
assert_json_eq "$binding" .role FISCAL_DEVICE
operator=$(request POST "$fiscal/operators" fiscal-operator-0001 '{"code":"A001","first_name":"Ada","last_name":"Lovelace","roles":["CASHIER"],"active_from":"2026-01-01T00:00:00Z"}')
assert_json_eq "$operator" .code A001
step="configure MiniPOS"
configuration=$(request PATCH "$pos/configuration" configuration-create-0001 "{\"location_name\":\"E2E Shop\",\"location_address\":\"Sofia\",\"workstation_name\":\"POS 01\",\"fiscal_register_id\":\"$register_id\"}")
assert_json_eq "$configuration" .version 1

step="create employee"
employee=$(request POST "$pos/employees" employee-create-0000001 '{"first_name":"Ada","last_name":"Lovelace","operator_code":"A001"}')
employee_id=$(printf '%s' "$employee" | jq -er .id)
step="create product"
product=$(request POST "$pos/products" product-create-00000001 '{"sku":"COFFEE","name":"Coffee","price":{"amount":"2.50","currency":"EUR"},"tax_group":"B"}')
product_id=$(printf '%s' "$product" | jq -er .id)
step="open shift"
shift=$(request POST "$pos/shifts" shift-open-0000000001 "{\"register_id\":\"$register_id\",\"employee_id\":\"$employee_id\"}")
shift_id=$(printf '%s' "$shift" | jq -er .id)
step="create order"
order=$(request POST "$pos/orders" order-create-00000001 "{\"shift_id\":\"$shift_id\"}")
order_id=$(printf '%s' "$order" | jq -er .id)
order_version=$(printf '%s' "$order" | jq -er .version)
line_id=00000000-0000-4000-8000-000000000101
step="add order line"
order=$(client_curl --silent --show-error --fail-with-body -X POST -H "X-Api-Version: $api_version" -H 'Idempotency-Key: order-line-0000000001' -H "If-Match: $order_version" -H 'Content-Type: application/json' --data "{\"line_id\":\"$line_id\",\"product_id\":\"$product_id\",\"name\":\"Coffee\",\"quantity\":\"1.000\",\"unit_price\":{\"amount\":\"2.50\",\"currency\":\"EUR\"},\"tax_group\":\"B\"}" "$pos/orders/$order_id/lines")
step="checkout cash order"
checkout=$(request POST "$pos/orders/$order_id/checkout" checkout-000000000001 '{"payment_id":"00000000-0000-4000-8000-000000000201","type":"CASH","amount":{"amount":"2.50","currency":"EUR"},"terminal_policy":"NONE"}')
test "$(printf '%s' "$checkout" | jq -r .state)" = COMPLETED
test -n "$(printf '%s' "$checkout" | jq -r .fiscal_operation_id)"

step="prepare ambiguous-outcome order"
unknown_order=$(request POST "$pos/orders" order-create-unknown-0001 "{\"shift_id\":\"$shift_id\"}")
unknown_order_id=$(printf '%s' "$unknown_order" | jq -er .id)
unknown_version=$(printf '%s' "$unknown_order" | jq -er .version)
unknown_line_id=00000000-0000-4000-8000-000000000102
client_curl --silent --show-error --fail-with-body -X POST -H "X-Api-Version: $api_version" -H 'Idempotency-Key: order-line-unknown-0001' -H "If-Match: $unknown_version" -H 'Content-Type: application/json' --data "{\"line_id\":\"$unknown_line_id\",\"product_id\":\"$product_id\",\"name\":\"Coffee\",\"quantity\":\"1.000\",\"unit_price\":{\"amount\":\"2.50\",\"currency\":\"EUR\"},\"tax_group\":\"B\"}" "$pos/orders/$unknown_order_id/lines" >/dev/null
step="verify ambiguous Fiscal outage is not reported as success"
fiscal_compose stop fiscal-backend >/dev/null
unknown_raw=$(client_curl --silent --show-error --write-out '\n%{http_code}' -X POST -H "X-Api-Version: $api_version" -H 'Idempotency-Key: checkout-unknown-00001' -H 'Content-Type: application/json' --data '{"payment_id":"00000000-0000-4000-8000-000000000202","type":"CASH","amount":{"amount":"2.50","currency":"EUR"},"terminal_policy":"NONE"}' "$pos/orders/$unknown_order_id/checkout")
unknown_status=$(printf '%s\n' "$unknown_raw" | tail -n 1)
test "$unknown_status" = 502
unknown_saved=$(request GET "$pos/orders/$unknown_order_id" order-read-unknown-0001 '')
assert_json_eq "$unknown_saved" .state UNKNOWN
fiscal_compose start fiscal-backend >/dev/null
wait_http "http://${fiscal_public_ip}/healthz"
replay_raw=$(client_curl --silent --show-error --write-out '\n%{http_code}' -X POST -H "X-Api-Version: $api_version" -H 'Idempotency-Key: checkout-unknown-00001' -H 'Content-Type: application/json' --data '{"payment_id":"00000000-0000-4000-8000-000000000202","type":"CASH","amount":{"amount":"2.50","currency":"EUR"},"terminal_policy":"NONE"}' "$pos/orders/$unknown_order_id/checkout")
replay_status=$(printf '%s\n' "$replay_raw" | tail -n 1)
test "$replay_status" = 502

step="restart backends"
fiscal_compose restart fiscal-backend >/dev/null
minipos_compose restart beeminipos-backend >/dev/null
wait_http "http://${fiscal_public_ip}/healthz"
wait_http "http://${minipos_public_ip}/healthz"
step="verify restart recovery"
restored=$(request GET "$pos/orders/$order_id" order-read-after-restart '')
test "$(printf '%s' "$restored" | jq -r .state)" = COMPLETED
restored_configuration=$(request GET "$pos/configuration" configuration-read-after-restart '')
test "$(printf '%s' "$restored_configuration" | jq -r .location_name)" = "E2E Shop"

step="backup and restore both isolated databases"
restore_started=$(date +%s)
fiscal_compose exec -T postgres pg_dump -U fiscal -d fiscal --format=custom > "$fiscal_backup"
fiscal_compose exec -T postgres createdb -U fiscal fiscal_restore
fiscal_compose exec -T postgres pg_restore -U fiscal -d fiscal_restore --exit-on-error < "$fiscal_backup"
fiscal_rows=$(fiscal_compose exec -T postgres psql -U fiscal -d fiscal_restore -Atc 'select count(*) from fiscal_runtime_sales')
test "$fiscal_rows" -gt 0
fiscal_mode=$(fiscal_compose exec -T postgres psql -U fiscal -d fiscal_restore -Atc 'select storage_mode from fiscal_state_meta where singleton=true')
test "$fiscal_mode" = 2
minipos_compose exec -T postgres pg_dump -U minipos -d minipos --format=custom > "$minipos_backup"
minipos_compose exec -T postgres createdb -U minipos minipos_restore
minipos_compose exec -T postgres pg_restore -U minipos -d minipos_restore --exit-on-error < "$minipos_backup"
minipos_rows=$(minipos_compose exec -T postgres psql -U minipos -d minipos_restore -Atc 'select count(*) from minipos_runtime_orders')
test "$minipos_rows" -gt 0
minipos_mode=$(minipos_compose exec -T postgres psql -U minipos -d minipos_restore -Atc 'select storage_mode from minipos_state_meta where singleton=true')
test "$minipos_mode" = 2
restore_elapsed=$(($(date +%s)-restore_started))
test "$restore_elapsed" -lt 120

step="verify MiniPOS database autonomy"
fiscal_compose stop postgres >/dev/null
independent=$(request GET "$pos/configuration" configuration-read-fiscal-down '')
test "$(printf '%s' "$independent" | jq -r .workstation_name)" = "POS 01"
fiscal_compose start postgres >/dev/null

printf '%s\n' "two-compose E2E passed: sale, UNKNOWN/no-retry outage, restart recovery, backup/restore, autonomous DB"
trap - EXIT INT TERM
cleanup
