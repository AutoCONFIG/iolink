# Web main pointer corrected candidate gate

recommendation: APPROVE
blockers: []

originalIntent: Update the web UI pointer to latest main, include previously closed fixes, then tag and push.

desiredOutcome: Root release based on `ea8a438`, including closed backend fixes and the latest web UI with the discovered stale-count regression fixed.

## Exact reviewed snapshot

Root base `ea8a438`; web upstream base `a474e93e441cebb744a19e5ad153eb87a6f70a59` plus web tree `b05e34563b850ecd43c9014e5ef1cf9914a867d9`, assigned final web commit `406903678625fee26c5850b2ccaf56b8027bfc67`. Verified `git -C web write-tree` equals that tree. Its only additions over upstream are `src/directives/countUp.ts` (four changed lines) and `tests/count-up.test.ts` (27 added lines).

This supersedes my original a474e93-only gate. Approval requires committing the reviewed web tree, selecting that commit in the root gitlink, and excluding shared AGENTS/playback drafts.

## Criteria and user outcome review

- C1, latest fetched UI: verified upstream origin/main is a474e93; corrected candidate descends from that upstream content.
- C2, previous closed fixes included: `29c85ed` and `95025f0` are ancestors of root `ea8a438`.
- C3, working build and tests: independently ran `npm test && npm run typecheck && npm run build`, session `52436`, exit 0; 11 files and 68 tests passed, typecheck and Vite production build passed.
- C4, stale-count bug repaired: cancellation now occurs before the equality return. Inspected the queued-frame lifecycle test and archived RED log demonstrating original code fails. The test drives mount(5), update(0), and remaining queued callbacks; corrected code keeps the latest text at zero.
- C5, scoped release: only the web pointer and release evidence belong in the root release. Playback drafts and AGENTS changes have no approval here.
- C6, independent double review: this is the final independent gate approval of the above tree; separate code review must identify this same tree. No push or tag result is claimed by this report.

## Direct skill and overfit/slop check

Consulted programming SKILL.md with TypeScript reference and remove-ai-slops SKILL.md. Inspected upstream diff and corrected production/test diff directly. No new runtime abstraction, parser, normalization, broad catch, any assertion, or suppression is introduced by the correction. The regression drives real directive lifecycle and queued animation callbacks, fails against the original implementation, and checks resulting text; it is neither removal-only nor tautological. No business API or authorization code changed.

NOTE: The test fixture uses HTMLElement/DirectiveBinding assertions and `null!` for unused Vue lifecycle arguments. These are narrow test setup escapes, not runtime state handling; they do not invalidate the behavior observed. The existing UI SSR identity test asserts rendered labels and does not establish animation or visual acceptance.

## Checked artifacts

- Root gitlink diff, web HEAD/origin/main, reflog and staged tree.
- Web upstream source/test diff from 39299df to a474e93.
- `web/src/directives/countUp.ts` and `web/tests/count-up.test.ts`.
- `docs/evidence/deployment/2026-10-15-web-main/README.md`.
- `docs/evidence/deployment/2026-10-15-web-main/tests.log`, `typecheck.log`, `build.log`.
- `docs/evidence/deployment/2026-10-15-web-main/count-up-red.log`.
- `.omo/evidence/web-main-pointer-code-review.md` (original REQUEST_CHANGES report; its stale-frame finding is repaired by reviewed tree).
- `.omo/evidence/web-main-pointer-code-review-final.md`: explicit APPROVE for the identical b05e345 tree; independently reproduces the latest-count outcome and explicitly covers programming/remove-ai-slops and overfit criteria.
- Independent execution sessions 42779 (original) and 52436 (corrected).

## Exact evidence limits and release notes

Browser acceptance remains user-owned; none was run. Existing source-map-js audit high advisory is unchanged because package and lock files are unchanged; no source-map input path is introduced by the pointer/cancellation patch. Build warns about Zod annotations and large chunks but exits successfully. Root tag/main and corrected web commit publication remain executor actions requiring their own push receipts. No external hardware, License or playback acceptance is claimed.
