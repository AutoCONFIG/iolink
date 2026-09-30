# R33 software evidence

`scripts/capacity_smoke.py` passed the bounded error-budget and p95 checks against health, history and latest HTTP surfaces with 20 concurrent clients and 20 requests (0% transport errors, p95 0.014 seconds; unauthenticated history/latest probes correctly returned 401). The required 24-hour, 500-device, 90-day fixture and 13-month disk-budget measurement require a sustained isolated load environment and remain `external_blocked`.
