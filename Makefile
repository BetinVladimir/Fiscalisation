.PHONY: deps generate-openapi test test-race vet governance-test postgres-integration typecheck contract-test roadmap-acceptance-test bg-trace-test driver-coverage-test fault-regression-test soak-regression-test security-test ui-acceptance-test ui-interaction-test handover-test web-build native-bundle android-build ios-build native-regression smart-device-test iot-test compose-check compose-e2e regression full-regression
deps:
	cd BeeMiniPOS && npm ci --cache /tmp/beeminipos-npm-cache --no-audit --no-fund
	cd BeeFiscalApp && npm ci --cache /tmp/beefiscalapp-npm-cache --no-audit --no-fund
generate-openapi:
	BeeMiniPOS/node_modules/.bin/openapi-typescript ../BeeloyBackend/docs/Fiscal/api/openapi-public-v1.yaml --output contracts/generated/openapi-public-v1.d.ts
	BeeMiniPOS/node_modules/.bin/openapi-typescript contracts/openapi-runtime-v1.yaml --output contracts/generated/openapi-runtime-v1.d.ts
test:
	cd fiscal-backend && GOCACHE=/tmp/beefiscal-go-cache go test ./...
	cd beeminipos-backend && GOCACHE=/tmp/beeminipos-go-cache go test ./...
	cd edge-agent && GOCACHE=/tmp/edge-go-cache go test ./...
test-race:
	cd fiscal-backend && GOCACHE=/tmp/beefiscal-go-cache go test -race ./...
	cd beeminipos-backend && GOCACHE=/tmp/beeminipos-go-cache go test -race ./...
	cd edge-agent && GOCACHE=/tmp/edge-go-cache go test -race ./...
vet:
	cd fiscal-backend && GOCACHE=/tmp/beefiscal-go-vet-cache go vet ./...
	cd beeminipos-backend && GOCACHE=/tmp/beeminipos-go-vet-cache go vet ./...
	cd edge-agent && GOCACHE=/tmp/edge-go-vet-cache go vet ./...
governance-test:
	ruby scripts/verify_governance.rb
postgres-integration:
	./scripts/postgres-integration.sh
typecheck:
	cd BeeMiniPOS && npx tsc --noEmit
	cd BeeMiniPOS && npm test
	cd BeeFiscalApp && npx tsc --noEmit
contract-test:
	./scripts/verify-contract-lock.sh
	./scripts/verify-generated-openapi.sh
	./scripts/verify-generated-response-contracts.sh
	cd BeeMiniPOS && npx tsc --noEmit
	cd BeeMiniPOS && npx tsc --noEmit --skipLibCheck --module esnext --moduleResolution bundler ../contracts/generated/contract-smoke.ts
	ruby scripts/verify_runtime_openapi.rb
	ruby scripts/verify_openapi_overlay.rb
	ruby scripts/verify_contract_surface.rb

roadmap-acceptance-test:
	ruby scripts/verify_roadmap_acceptance.rb

bg-trace-test:
	ruby scripts/verify_bg_trace.rb

driver-coverage-test:
	ruby scripts/verify_driver_coverage.rb
fault-regression-test:
	ruby scripts/verify_fault_regression.rb

soak-regression-test:
	./scripts/run_soak_regression.sh /tmp/beeloy-soak-regression.jsonl
security-test:
	ruby scripts/verify_security_regression.rb
	./scripts/verify_sensitive_data.sh
ui-acceptance-test:
	ruby scripts/verify_ui_acceptance.rb
handover-test:
	ruby scripts/verify_handover.rb

ui-interaction-test:
	cd BeeMiniPOS && EXPO_PUBLIC_APP_ENV=dev EXPO_PUBLIC_MINIPOS_API_URL=http://minipos-api.test/public/v1/minipos EXPO_PUBLIC_FISCAL_API_URL=http://fiscal-api.test/public/v1 npx expo export --clear --platform web --output-dir .ui-e2e-web
	cd BeeMiniPOS && EXPO_PUBLIC_APP_ENV=prod EXPO_PUBLIC_MINIPOS_AUTH_TOKEN=forbidden-static-token EXPO_PUBLIC_FISCAL_AUTH_TOKEN=forbidden-fiscal-static-token EXPO_PUBLIC_MINIPOS_API_URL=http://minipos-api.test/public/v1/minipos npx expo export --clear --platform web --output-dir .ui-e2e-prod-web
	cd BeeFiscalApp && EXPO_PUBLIC_APP_ENV=dev EXPO_PUBLIC_FISCAL_API_URL=http://fiscal-admin.test/public/v1 npx expo export --platform web --output-dir .ui-e2e-web
	cd BeeFiscalApp && EXPO_PUBLIC_APP_ENV=prod EXPO_PUBLIC_FISCAL_AUTH_TOKEN=forbidden-fiscal-admin-static-token EXPO_PUBLIC_FISCAL_API_URL=http://fiscal-admin.test/public/v1 npx expo export --clear --platform web --output-dir .ui-e2e-prod-web
	cd BeeMiniPOS && npm run test:web-interaction

evidence-test:
	ruby scripts/generate_release_evidence.rb /tmp/beeloy-release-evidence-test
	ruby scripts/verify_release_evidence.rb /tmp/beeloy-release-evidence-test
	./scripts/test-release-evidence-signing.sh
web-build:
	cd BeeMiniPOS && EXPO_PUBLIC_APP_ENV=dev npx expo export --clear --platform web --output-dir .regression-web && test -f .regression-web/index.html
	cd BeeFiscalApp && npx expo export --platform web --output-dir .regression-web && test -f .regression-web/index.html
native-bundle:
	./scripts/test-minipos-metro-resolution.sh .regression-native
	test -f BeeMiniPOS/.regression-native/index.html
	test -d BeeMiniPOS/.regression-native/_expo/static/js/android
	test -d BeeMiniPOS/.regression-native/_expo/static/js/ios
android-build:
	cd BeeMiniPOS/android && ./gradlew assembleDebug --no-daemon
	test -f BeeMiniPOS/android/app/build/outputs/apk/debug/app-debug.apk
ios-build:
	cd BeeMiniPOS/ios && pod install
	cd BeeMiniPOS/ios && xcodebuild -workspace BeeMiniPOS.xcworkspace -scheme BeeMiniPOS -configuration Debug -sdk iphonesimulator -destination 'generic/platform=iOS Simulator' CODE_SIGNING_ALLOWED=NO build
native-regression: native-bundle android-build ios-build
smart-device-test:
	cd BeeMiniPOS/android && ./gradlew -p ../../SmartDevices :daisy-smart-stub:testDebugUnitTest :daisy-smart-stub:assembleDebug :daisy-smart-stub:assembleRelease --no-daemon
	test -f SmartDevices/daisy-smart-stub/build/outputs/apk/debug/daisy-smart-stub-debug.apk
	test -f SmartDevices/daisy-smart-stub/build/outputs/apk/release/daisy-smart-stub-release-unsigned.apk
	rg -q 'BuildConfig.STUB_ADAPTER && BuildConfig.DEBUG' SmartDevices/daisy-smart-stub/src/main/kotlin/com/beeloy/fiscal/daisy/DaisySmartStub.kt
iot-test:
	c++ -std=c++17 -Wall -Wextra -Werror -IIoT/protocol-abstraction/include IoT/protocol-abstraction/src/FrameCodec.cpp IoT/protocol-abstraction/src/AllCommands.cpp IoT/protocol-abstraction/src/CommandRegistry.cpp IoT/protocol-abstraction/src/CommandPayload.cpp IoT/protocol-abstraction/tests/FrameCodecTest.cpp -o /tmp/beefiscal-frame-codec-test
	/tmp/beefiscal-frame-codec-test
compose-check:
	docker compose --env-file .env.example -f compose.fiscalisation.yaml -f compose.fiscalisation.dev.yaml config >/dev/null
	docker compose --env-file .env.example -f compose.minipos.yaml -f compose.minipos.dev.yaml config >/dev/null
	FISCAL_DB_PASSWORD=test FISCAL_RLS_DB_PASSWORD=test-rls WEBHOOK_SIGNING_KEY=dev-key docker compose -f compose.fiscalisation.yaml -f compose.fiscalisation.dev.yaml config >/dev/null
	FISCAL_DB_PASSWORD=test FISCAL_RLS_DB_PASSWORD=test-rls WEBHOOK_SIGNING_KEY=production-webhook-key-at-least-32-bytes OIDC_ISSUER=https://id.example.test OIDC_AUDIENCE=beefiscal OIDC_JWKS_URL=https://id.example.test/jwks BLE_SIGNING_KEY=production-ble-key-at-least-32-bytes FISCAL_SITE=https://fiscal.example.test FISCAL_PUBLIC_BASE_URL=https://fiscal.example.test/public/v1 FISCAL_CORS_ALLOWED_ORIGINS=https://admin.example.test,https://pos.example.test docker compose -f compose.fiscalisation.yaml -f compose.fiscalisation.prod.yaml config >/dev/null
	MINIPOS_DB_PASSWORD=test MINIPOS_RLS_DB_PASSWORD=test-rls FISCAL_PUBLIC_BASE_URL=http://fiscal.test/public/v1 WEBHOOK_VERIFICATION_KEY=dev-key docker compose -f compose.minipos.yaml -f compose.minipos.dev.yaml config >/dev/null
	MINIPOS_DB_PASSWORD=test MINIPOS_RLS_DB_PASSWORD=test-rls FISCAL_PUBLIC_BASE_URL=https://fiscal.example.test/public/v1 FISCAL_OAUTH_TOKEN_URL=https://id.example.test/token FISCAL_OAUTH_CLIENT_ID=minipos FISCAL_OAUTH_CLIENT_SECRET=production-client-secret OIDC_ISSUER=https://id.example.test OIDC_AUDIENCE=beeminipos OIDC_JWKS_URL=https://id.example.test/jwks WEBHOOK_VERIFICATION_KEY=production-webhook-key-at-least-32-bytes MINIPOS_SITE=https://pos.example.test MINIPOS_CORS_ALLOWED_ORIGINS=https://pos.example.test docker compose -f compose.minipos.yaml -f compose.minipos.prod.yaml config >/dev/null
compose-e2e:
	./scripts/e2e-two-compose.sh
regression: test-race vet governance-test typecheck contract-test roadmap-acceptance-test bg-trace-test driver-coverage-test fault-regression-test soak-regression-test security-test ui-acceptance-test ui-interaction-test handover-test evidence-test web-build native-bundle smart-device-test iot-test compose-check
full-regression: regression postgres-integration compose-e2e
