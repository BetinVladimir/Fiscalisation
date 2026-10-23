# MVP gates and formal exclusions

## Проверка контуров и статусы — 2026-10-21

[Текущий срез, протоколы и112 критериев unit-тестов](docs/fiscal-contour-review-20261021/README.md). На этом срезе contract-lock **VERIFIED_FAIL**: runtime OpenAPI расходится с lock/script; остальные прежние PASS требуют source/scope сверки и здесь не перезапускались. MiniPOS iOS native dependencies расходятся с package.json; полный demo/native/device цикл не принят. Это дополнение к подробному remediation, не отмена его требований. Исторические сводные количества endpoint/tests и blanket readiness ниже не переносить на текущую версию без повторного gate.


## Readiness update — 2026-10-17

Current target MiniPOS/S3/BlueCash readiness: **SOFTWARE_GAPS_OPEN**.
The [runtime audit, remediation tasks and acceptance tests](docs/remediation/2026-10-17/README.md)
supersede earlier blanket software-complete claims. Hardware, vendor and
production gates remain separate; this audit changed documentation only.


This register prevents a software simulator from being mistaken for Bulgarian production readiness.

| Gate | MVP disposition | Closure evidence |
|---|---|---|
| Daisy SMART S vendor fiscal/payment/barcode SDK and protocol | `STUB_ONLY`; hard-disabled in PROD | Vendor SDK, license, supported firmware matrix and bench evidence |
| Daisy Compact S 01 USB host role, power, VID/PID and EUR firmware applicability | `DOCUMENTED_NOT_APPROVED` | Electrical prototype record, enumerated USB descriptors, approved firmware/protocol statement and HIL report |
| DP-150 MX electrical interface and EUR fiscal firmware | `DOCUMENTED_NOT_APPROVED` | Level/pin/power review, device firmware statement and HIL report |
| BlueCash-50 fiscal/payment integration | `EXCLUDED_FROM_PROD` | Vendor/acquirer SDK terms, test credentials/device and approval evidence |
| BluePad-50 Plus pairing/payment use | Optional and `UNSUPPORTED` until approval | Written vendor/acquirer permission, pairing protocol and certified test evidence |
| НАП/legal certification | Release blocker, never implied by automated tests | Signed compliance matrix, device/service dossiers and regulator/certification evidence |

Software development may continue against deterministic simulators and stable interfaces. No gate above may be converted to `SUPPORTED` by configuration, undocumented reverse engineering or successful simulator tests.
