# Product and thing model console

## Scope

This milestone aligns the product and thing model page with the ThingsPanel resource workflow: searchable product catalog, list/card views, selected resource details, version history, field definition review, draft publication, and device model binding. Read-only tenant roles can inspect products and models; only owner/admin roles receive write controls.

## Verification

- `web/npm test -- --run`: passed; 10 files and 65 tests.
- `web/npm run typecheck`: passed.
- `web/npm run build`: passed; existing zod annotation and chunk-size warnings only.
- `web/npx playwright test e2e/admin-pages.spec.ts -g '产品页可创建'`: passed.
- Full `web/npx playwright test e2e/admin-pages.spec.ts`: passed; 5 tests.
- `git -C web diff --check`: passed.
- API adapter regression covers snake_case field ranges/enums, omission of empty enum values, and readable/writable/nullable flags.

## Interaction decisions

- Product creation is a separate dialog from resource browsing.
- Model creation is a separate dialog that queues fields and submits one immutable version.
- Version details, publication and device binding are separated into `概览`, `物模型` and `设备绑定` tabs.
- Published versions are read-only; a new version is required for schema changes.
- Binding requires a published version and explicitly identifies the target device.
- Field options expose readable, writable and nullable flags; empty enum values are omitted from requests so ordinary string fields remain valid.
- Publishing updates both the selected detail and the catalog's current-version summary.

## Review and acceptance boundary

Independent code and gate review receipts are attached to this milestone after the initial review findings were fixed. Browser visual acceptance remains user-owned. Real PostgreSQL/Timescale and external device assignment remain environment-dependent and are not claimed by this frontend milestone.
