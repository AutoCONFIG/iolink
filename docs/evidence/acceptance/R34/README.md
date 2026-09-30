# R34 M6a product/model acceptance evidence

Status: software passed on the final frozen worktree after two independent approvals. External status: not applicable.

Verified on 2026-09-28:

- Product/model persistence and tenant isolation: `.omo/evidence/todo10-m6a-backend-verification.md`.
- Generic telemetry validation, model version history, and scoped history: `.omo/evidence/todo10-m6a-water-projection.md` plus the M6a integration test.
- Generic event ingestion now accepts JSON properties for a published model and updates the model-versioned shadow in the same transaction.
- Admin product/model routes and Products page: `.omo/evidence/todo10-m6a-admin-ui.txt`.
- Browser and MQTT surface evidence: `.omo/evidence/todo10-m6a-browser-mqtt-r2.md` and the three `todo10-m6a-products-*.png` screenshots.
- Independent approvals: `.omo/evidence/todo10-m6a-code-review-r7.md` and `.omo/evidence/todo10-m6a-gate-review-final-alt.md`.

Final checks passed: Go race/shuffle integration, contracts, architecture manifests, web/web-mini checks, Playwright, and diff check.
The final architecture gate was rerun after adding the missing R03/R04 evidence anchors: `python3 scripts/check_architecture_manifests.py --gate M6a` passed.
