# Device resource console and paged API

## Scope

This milestone aligns the device resource flow with the ThingsPanel resource pattern: a paged resource envelope, server-side search/filtering, list/card views, a tabbed detail drawer (overview/telemetry/configuration), lifecycle actions, one-time registration credentials, product/model navigation, and per-metric freshness presentation.

## Verification

- `make verify`: passed (Go build, vet, race tests, and package tests).
- `make verify-contracts`: passed; 96 target operations and 568 synthetic fixtures.
- `go test ./internal/adminapi ./internal/core`: passed.
- `web/npm test -- --run`: passed; 9 files and 61 tests.
- `web/npm run typecheck`: passed.
- `web/npm run build`: passed; only existing dependency annotation and chunk-size warnings.
- `git diff --check` and `git -C web diff --check`: passed.
- Real PostgreSQL/Timescale verification for `ListDevicesPage` was `external_blocked` because `IOLINK_TEST_PG_DSN` was not set in this environment.

## Contract decisions

- `/admin/v1/devices` and `/user/v1/devices` use the same `{list,total,page,page_size}` envelope.
- `status=active` is the default and hides disabled devices; `all` and `disabled` explicitly reveal them.
- `name` is limited to 128 Unicode characters; page offset overflow and invalid status values are rejected.
- Demo mode mutates device pond ownership and preserves the configured report interval.

## Reviews

Independent review receipts are stored with the task evidence. Browser visual acceptance remains user-owned; automated frontend checks above cover the changed code paths.
