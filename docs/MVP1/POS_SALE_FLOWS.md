# POS Sale Flows — UNP Timing (Annex 29 §9/9.1)

Annex 29 §9/9.1 requires generating and displaying the UNP (Unique Number of Fiscal Document)
when the first product is added to the receipt. This document maps each sale scenario to its
UNP generation moment, the responsible component, and the covering test.

## UNP Timing by Scenario

| Scenario | UNP Moment | Component | Test |
|---|---|---|---|
| Retail (immediate checkout) | On `openSaleWithLine` (first item added) | pos-app → fiscal-backend `POST /public/v1/sales:open-with-line` | `HttpFiscalApiClient.routing.test.ts` — `openSaleWithLine routes to fiscalBase` |
| Restaurant / table order | On first line addition (`openSaleWithLine`) before any payment | pos-app workflow `LocalLineOperations` → fiscal-backend | `LocalLineOperations.test.ts` — line addition triggers fiscal open |
| ERP / online order (via erp-connector) | On outbox drain publish of `fiscal.sale.open.v1` event | erp-connector orchestration → fiscal-backend | `erp-connector/internal/orchestration` Go tests |
| VOID / reversal (storno) | UNP from original sale's fiscal document; no new UNP generated | pos-gateway reversal handler, `idle.go` `originalFiscalReference` | `refund.originalReference.test.ts` — originalFiscalReference not substituted |
| Refund (partial) | UNP from original sale; refund uses `originalFiscalOperationId` | pos-gateway `reversal_handler.go` | `reversal_handler_test.go` — `TestHasReversalPermission` |

## Path A vs Path B

| Path | Description | UNP appears | Compliance status |
|---|---|---|---|
| Path A | pos-app calls `openSaleWithLine` at first item | Before payment (Annex 29 §9 compliant) | COMPLIANT |
| Path B | gateway finalizes whole check at checkout | After payment (potentially non-compliant) | TO VERIFY via ARCH_06 |

**Status:** Path A (openSaleWithLine) is implemented and compliant. Path B usage must be verified
per ARCH_06 legal interpretation. If any flow uses Path B only, it requires a task to add
openSaleWithLine call at line-1 addition.

## Notes

- Empty UNP in FISCALIZED response → no print, NEEDS_ATTENTION state (BUG-053).
- UNP must NOT be substituted with `fiscalReference` or `fiscalNumber` (invariant #6, BUG-053).
- UNP is only set from FU response; no synthetic UNP is generated (invariant #6).
