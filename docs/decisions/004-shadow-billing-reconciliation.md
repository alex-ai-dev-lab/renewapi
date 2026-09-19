# ADR-004: Durable Shadow Billing Balance Reconciliation

Status: Accepted

Date: 2026-08-17

## Context

RenewAPI keeps `BILLING_LEDGER_MODE=shadow` as the default for compatibility
and observation. The previous shadow path applied wallet, subscription, and
token changes through separate legacy calls, then marked the in-memory
`BillingSession` terminal. A failure between those calls could leave a user
balance and the ledger permanently inconsistent, while a process restart lost
the retry context.

## Decision

Shadow ledger reservations and all later balance transitions use the existing
`BillingLedger` transaction and outbox path. The ledger applies wallet,
subscription, and token balance legs atomically for `shadow` and `enforce`
modes. `enforce` remains the owner of usage/request counters; persistent
shadow and enforce sessions own async task terminal insertion, while shadow
retains its existing compatibility counter path and explicit `off` retains the
legacy task path.

Each ledger records funding, token, subscription, and statistics component
states. A transition is terminal only after all applicable balance legs commit.
The existing `request_id` uniqueness and ledger row lock provide idempotency;
reconciliation replays the signed delta from `AppliedQuota` to `DesiredQuota`.

The default mode is not changed to `enforce`. Explicit `off` remains a legacy
compatibility escape hatch and is outside the durable ledger guarantee.

## Compatibility and migration

- The change is additive: component-state columns are added by
  `billing-ledger:v2` and have defaults for existing rows.
- Existing shadow ledgers already store the legacy pre-consumed amount in
  `AppliedQuota`; a later transition adjusts only the delta from that value,
  preventing replay of the initial reservation.
- SQLite, MySQL, and PostgreSQL use GORM `AutoMigrate`; no database-specific
  SQL or lock hint is introduced.
- Existing API responses and product version remain unchanged.

## Validation

Focused SQLite tests cover atomic rollback when the second balance leg fails,
retry after restoring the failed component, duplicate settle/refund, repeated
reconciler delivery, concurrent terminal transitions, subscription extra
reserve/refund, missing-row `RowsAffected`, legacy-ledger compatibility, and
the default shadow `BillingSession` and async pending-task paths. External
MySQL/PostgreSQL execution and application-process crash/restart checks remain
release gates; adding CI steps without an inspected run is not pass evidence.
