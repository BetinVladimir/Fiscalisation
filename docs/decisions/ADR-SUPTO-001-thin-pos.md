# ADR-SUPTO-001: Thin POS boundary

BeeMiniPOS generates only surrogate UUID intents and renders authoritative projections. Fiscal rules, totals, regulatory identifiers, route selection, vendor commands and retry/reconciliation policy belong to Compliance Gateway/Core. This decision supersedes the checkout-owned fiscal client for `BG_SUPTO_FULL`; migration is incomplete while `SUPTO-29-02`, `09`, `12` and `22` remain `FAIL`.
