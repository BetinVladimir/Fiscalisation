# GAP015: explicit fiscal execution configuration and money preflight

## Configuration contract and migration

Signed composite binding schema 3 requires `fiscal_endpoint.execution`. Schema 2 remains readable for diagnosis and migration; monetary operations fail with `FISCAL_CONFIG_REQUIRED`. No credentials, operator, till or card payment code are guessed.

Datecs example (a declaration of installed device settings):

```json
{"operator_number":2,"operator_count":30,"operator_password_ref":"fiscal-pass","device_configuration_id":"installed-config-7","till_number":24,"payment_mapping":{"CASH":0,"CARD":2}}
```

Daisy example:

```json
{"operator_number":2,"operator_count":30,"operator_password_ref":"fiscal-pass","device_configuration_id":"installed-config-7","payment_mapping":{"CASH":"P","CARD":"C"}}
```

Datecs operator is 1–30, till 1–99999; the new configuration requires an eight-digit password. Daisy password is 1–6 digits. Daisy operator count must be declared from the device configuration (command 128 parameter P21); this implementation supports counts up to 30, matching the current protocol builder. Daisy has no till field; supplying one is rejected. CARD mapping is optional when no card tender is supported; attempting a CARD operation without a mapping fails before POS purchase.

**Correction to the initial audit assumption:** Datecs payment 1 is credit card and 2 is debit card; neither is a universal CARD code. Cash is 0. Daisy P is cash; N/C/D/U/B/E select programmed payment slots, not intrinsically verified card types. `device_configuration_id` and the mapping are declarations, not readback or hardware proof. Verify the programmed payment name/type during installation and HIL.

`RuntimeConfigLoader::provision_fiscal_password(binding,password)` is a trusted local provisioning API. It writes only the referenced password to the existing `edge-secrets` NVS store and commits it. Runtime reads it through the existing protected-NVS API, validates profile-specific digits, and passes it directly to physical IO. Passwords are never included in the signed binding, route snapshots or logs. Missing/invalid passwords block startup for schema 3; direct physical/preflight calls also fail closed.

The authenticated delivery/UI/factory caller for the new secret and execution fields remains to be implemented and integrated. The local API does not establish an authenticated remote secret-provisioning channel. Existing NVS encryption, flash protection and physical extraction resistance require separate deployment evidence; naming an NVS namespace protected does not establish those properties.

## Money and vendor commands

Preflight runs before POS purchase/refund and before fiscal open. Decimal strings use checked fixed-point arithmetic: cents × thousandths, half-up rounding per line `(price_cents * quantity_milli + 500) / 1000`, followed by subtraction of the absolute line discount. This matches backend `service.go` `lineTotalCents` and `discountedLineTotalCents`. Payment sum must equal the discounted total exactly. No change/overpayment is inferred. Duplicate line/payment identifiers, overflow, malformed values, unsupported tax groups, excessive discounts, mismatched payment minor units and invalid vendor payloads fail before side effects.

Line `discount` is absolute EUR Money, not percentage. Positive discounts emit Datecs command 49 DiscountType=4/DiscountValue and Daisy `$-Netto`. Zero discount emits no discount modifier. Configured payment slots and operator/password/till are used for both normal receipt and reversal.

After fiscal lines, command 51 reads the device subtotal without printing/displaying or applying a second discount: Datecs `0\t0\t\t\t`, Daisy `00`. Vendor-specific parsers validate the response; observed cents must equal the preflight payment total before command 53. Missing, malformed or mismatched subtotal returns UNKNOWN and sends neither fiscal payment nor close. Approved POS references remain available for durable recovery; no automatic success or unproven rejection is invented.

External POS purchase still precedes fiscal open in the existing saga. Subtotal failure can therefore require recovery/compensation. Reordering fiscal open/lines before POS purchase is a separate saga change. Vendor fractional rounding and maximum lengths/encoding still require HIL confirmation. Zero-price lines are currently outside the supported physical profile because vendor builders reject them.

## Evidence and acceptance

`tests/run_money_host.sh` compiles actual production profile executor, orchestrator, durable SQLite storage, binding decoder and runtime config against actual protocol builders and actual installed cJSON. Serial session is replaced only at its exchange boundary; expected command payloads and device subtotal responses are literal vendor vectors, not generated from production request encoders. NVS is a host fake testing the actual provisioning/load API, not an encryption test.

Coverage includes both vendors; literal open/discount/payment/subtotal commands; Datecs credit/debit selection; cash reversal; malformed/failed/mismatched subtotal; preserved POS reference/RRN/auth in actual SQLite payment rows; exact totals and rounding boundary; overflow; invalid name/tax/payment units; missing configuration/secret; invalid binding fields; schema 2 migration; secret provision/load/missing cases. Existing profile orchestration tests use complete valid receipt/config fixtures.

Sources: Datecs DP-150 MX programmers manual §§4.11–4.15 (commands 48/49/51/53), extracted `datecs.txt` pp10–15; Daisy protocol V2.0-4 2026 commands 48/49/51/53 and command128 P21, extracted `daisy.txt` pp16–22 and parameter table. Operator/password and physical payment mappings must be checked against the actual configured model/firmware during HIL.
