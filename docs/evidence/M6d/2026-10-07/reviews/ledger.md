# Independent review ledger

Each record applies only to its exact source/web snapshot. One rejection blocks
release, even if another lane approved. Later product changes require fresh lanes.

| Source | Web | Lane | Verdict | Durable report |
|---|---|---|---|---|
| d82aa8249e70302ee70e8cacd5cef15177b31ef5 | 207dcfa751545e11a63a3a013da7e60552fecd75 | m6d_d82_code | REQUEST_CHANGES; report persisted before platform interruption | [code](code-d82aa82.md) |
| d82aa8249e70302ee70e8cacd5cef15177b31ef5 | 207dcfa751545e11a63a3a013da7e60552fecd75 | m6d_d82_gate | INCONCLUSIVE | [gate](gate-d82aa82.md) |
| 865e9b28f531c1402bcf96a53c8663ad0e2dab89 | 3c1d860e2cca0b6ed8207142f5506784f5fef452 | m6d_865_code | REQUEST_CHANGES | [code](code-865e9b2.md) |
| 865e9b28f531c1402bcf96a53c8663ad0e2dab89 | 3c1d860e2cca0b6ed8207142f5506784f5fef452 | m6d_865_gate | APPROVE / HIGH | [gate](gate-865e9b2.md) |

865e9b2 is rejected for replay after429. Its single approval is historical and
does not approve the repaired 657807e product or any later snapshot.
