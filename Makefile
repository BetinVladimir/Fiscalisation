.PHONY: deps test test-race postgres-integration typecheck contract-test bg-trace-test security-test web-build native-bundle android-build ios-build native-regression smart-device-test iot-test compose-check compose-e2e regression full-regression
deps:
	cd BeeMiniPOS && npm ci --cache /tmp/beeminipos-npm-cache --no-audit --no-fund
	cd BeeFiscalApp && npm ci --cache /tmp/beefiscalapp-npm-cache --no-audit --no-fund
test:
	cd fiscal-backend && GOCACHE=/tmp/beefiscal-go-cache go test ./...
	cd beeminipos-backend && GOCACHE=/tmp/beeminipos-go-cache go test ./...
	cd edge-agent && GOCACHE=/tmp/edge-go-cache go test ./...
test-race:
	cd fiscal-backend && GOCACHE=/tmp/beefiscal-go-cache go test -race ./...
	cd beeminipos-backend && GOCACHE=/tmp/beeminipos-go-cache go test -race ./...
	cd edge-agent && GOCACHE=/tmp/edge-go-cache go test -race ./...
postgres-integration:
	./scripts/postgres-integration.sh
typecheck:
	cd BeeMiniPOS && npx tsc --noEmit
	cd BeeMiniPOS && npm test
	cd BeeFiscalApp && npx tsc --noEmit
contract-test:
	./scripts/verify-contract-lock.sh
	ruby scripts/verify_runtime_openapi.rb
	ruby scripts/verify_contract_surface.rb

bg-trace-test:
	ruby scripts/verify_bg_trace.rb
security-test:
	./scripts/verify_sensitive_data.sh

evidence-test:
	ruby scripts/generate_release_evidence.rb /tmp/beeloy-release-evidence-test
	ruby scripts/verify_release_evidence.rb /tmp/beeloy-release-evidence-test
web-build:
	cd BeeMiniPOS && npx expo export --platform web --output-dir .regression-web && test -f .regression-web/index.html
	cd BeeFiscalApp && npx expo export --platform web --output-dir .regression-web && test -f .regression-web/index.html
native-bundle:
	cd BeeMiniPOS && npx expo export --platform all --output-dir .regression-native
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
	c++ -std=c++17 -Wall -Wextra -Werror -IIoT/protocol-abstraction/include IoT/protocol-abstraction/src/FrameCodec.cpp IoT/protocol-abstraction/src/AllCommands.cpp IoT/protocol-abstraction/src/CommandRegistry.cpp IoT/protocol-abstraction/tests/FrameCodecTest.cpp -o /tmp/beefiscal-frame-codec-test
	/tmp/beefiscal-frame-codec-test
compose-check:
	FISCAL_DB_PASSWORD=test FISCAL_RLS_DB_PASSWORD=test-rls WEBHOOK_SIGNING_KEY=dev-key docker compose -f compose.fiscalisation.yaml -f compose.fiscalisation.dev.yaml config >/dev/null
	FISCAL_DB_PASSWORD=test FISCAL_RLS_DB_PASSWORD=test-rls WEBHOOK_SIGNING_KEY=prod-key OIDC_ISSUER=https://id.example.test OIDC_AUDIENCE=beefiscal OIDC_JWKS_URL=https://id.example.test/jwks BLE_SIGNING_KEY=production-ble-key-at-least-32-bytes FISCAL_SITE=https://fiscal.example.test docker compose -f compose.fiscalisation.yaml -f compose.fiscalisation.prod.yaml config >/dev/null
	MINIPOS_DB_PASSWORD=test MINIPOS_RLS_DB_PASSWORD=test-rls FISCAL_PUBLIC_BASE_URL=http://fiscal.test/public/v1 WEBHOOK_VERIFICATION_KEY=dev-key docker compose -f compose.minipos.yaml -f compose.minipos.dev.yaml config >/dev/null
	MINIPOS_DB_PASSWORD=test MINIPOS_RLS_DB_PASSWORD=test-rls FISCAL_PUBLIC_BASE_URL=https://fiscal.example.test/public/v1 FISCAL_OAUTH_TOKEN_URL=https://id.example.test/token FISCAL_OAUTH_CLIENT_ID=minipos FISCAL_OAUTH_CLIENT_SECRET=production-client-secret OIDC_ISSUER=https://id.example.test OIDC_AUDIENCE=beeminipos OIDC_JWKS_URL=https://id.example.test/jwks WEBHOOK_VERIFICATION_KEY=prod-key MINIPOS_SITE=https://pos.example.test docker compose -f compose.minipos.yaml -f compose.minipos.prod.yaml config >/dev/null
compose-e2e:
	./scripts/e2e-two-compose.sh
regression: test-race typecheck contract-test bg-trace-test security-test evidence-test web-build native-bundle smart-device-test iot-test compose-check
full-regression: regression postgres-integration compose-e2e
