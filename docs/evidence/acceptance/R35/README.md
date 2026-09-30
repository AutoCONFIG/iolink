# R35 M6a compatibility projection acceptance evidence

Status: software passed on the final frozen worktree after two independent approvals. External status: not applicable.

The water compatibility path keeps `sensor_data` and v1 reads, writes a model-versioned generic `telemetry` row atomically, and carries `device_shadows.model_version`. Fixture-only backfill uses `telemetry_backfill_checkpoints` with ordered `(device_no, ts)` high-water progress and can resume after a batch interruption.

Evidence:

- `.omo/evidence/todo10-m6a-water-projection.md`
- `internal/core/m6a_telemetry_integration_test.go::TestGenericTelemetryBackfillResumesFromCheckpoint`
- `.omo/evidence/todo10-m6a-code-review-r7.md`
- `.omo/evidence/todo10-m6a-gate-review-final-alt.md`

Final checks passed: Go race/shuffle integration, MQTT/history/backfill probes, contracts, architecture manifests, web/web-mini checks, Playwright, and diff check.
The final architecture gate was rerun after adding the missing R03/R04 evidence anchors: `python3 scripts/check_architecture_manifests.py --gate M6a` passed.

Production customer data migration is not claimed.
