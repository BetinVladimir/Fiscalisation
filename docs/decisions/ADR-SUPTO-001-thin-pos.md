# ADR-SUPTO-001: Thin POS boundary

BeeMiniPOS generates only surrogate UUID intents and renders authoritative projections. Fiscal rules, totals, regulatory identifiers, route selection, vendor commands and retry/reconciliation policy belong to Compliance Gateway/Core. This decision supersedes the checkout-owned fiscal client for `BG_SUPTO_FULL`. Implementation/evidence status is authoritative only in `contracts/supto-annex29-trace.json`; this ADR does not duplicate mutable `PASS/PARTIAL` values.
