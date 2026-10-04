# M6b D16 support-expiry precision evidence

This evidence covers the web-only support-expiry save path. It proves that an unchanged Shanghai-local display sends the original UTC RFC3339 value, including seconds and fractional seconds, and that an edited display converts to the requested new instant.

Environment: Linux 7.0.0-111031-tuxedo, Node v24.21.0, npm 11.19.0, Google Chrome 154.0.8037.92, captured 2026-10-03 UTC. The browser scenarios use the demo adapter and localStorage; assembled API/DB evidence is owned by the parent M6b validation.

Commands and artifacts:

- `npm ci --no-audit --no-fund` -> [`npm-ci.log`](npm-ci.log)
- `npm test` -> [`unit.log`](unit.log), 21 tests passed
- `npm run typecheck` -> [`typecheck-final.log`](typecheck-final.log)
- `npm run build` -> [`build-final.log`](build-final.log)
- `npm run e2e -- --config /tmp/iolink-d16-playwright.config.mjs` -> [`full-browser.log`](full-browser.log), 13 tests passed on the pre-fraction final view; [`support-browser.log`](support-browser.log), 9 final precision scenarios passed after fractional-second preservation
- Red regression: before the view fix, UTC `2099-10-02T05:00:37Z` was persisted as `2099-10-02T05:00:00Z`; [`red-browser.log`](red-browser.log) and its Playwright trace under `red-browser-results/` capture the failure.

The final browser run records unchanged whole-second and fractional-second JSON persistence attachments and screenshots at 375, 768, and 1280px under `support-browser-results/`. The final implementation is in the child web commit recorded by the parent handoff; no backend or status document was modified by this worktree.
