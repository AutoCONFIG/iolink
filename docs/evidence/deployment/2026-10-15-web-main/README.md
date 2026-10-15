# Web main pointer release

Release candidate: base `ea8a438` with `web` pinned to
`406903678625fee26c5850b2ccaf56b8027bfc67`, tree
`b05e34563b850ecd43c9014e5ef1cf9914a867d9`. Remote `origin/main`
was fetched and fast-forwarded from `39299df` to `a474e93` on 2026-10-15,
then the count animation fix was committed on web main.

The already closed backend fixes through v0.0.40 are ancestors of this
candidate. No additional closed backend fix remains unstaged. Playback
HTTP drafts, AGENTS.md and unrelated historical evidence are excluded.
M7a playback HTTP remains unpublished and awaiting independent review.

Executed in the web checkout, all exit 0:

- `npm ci --prefix web`
- `npm test --prefix web`: 11 files, 68 tests passed (`tests.log`).
- `npm run typecheck --prefix web`
- `npm run build --prefix web`: Vite production build passed.

The first independent review found that an old animation could overwrite
a count newly reset to zero. Cancellation now precedes the unchanged-value
return. The new lifecycle regression fails with the original implementation
(`count-up-red.log`) and passes with the fix. Both independent reviewers
review the final web tree above; reports are archived alongside this file.

Build warnings: Zod PURE annotations and bundle chunks above 500 kB.
`npm audit --prefix web --json` reports one existing transitive high
advisory in source-map-js <1.2.2 (GHSA-68fv-2mgg-jv7q). This pointer
update does not change package.json or package-lock.json. Browser visual
acceptance remains user-owned. Remote publication is pending.
