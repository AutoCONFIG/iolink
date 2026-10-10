# M7a hardened ZLMediaKit build and capability gate

Date: 2026-10-11 Asia/Shanghai. Scope: pinned-source build boundary and
fail-closed provider admission contract. Video remains disabled; no ZLM process,
RTSP source, GB runtime or playback route is assembled.

## Implemented artifact

- `internal/media/zlm/gate.go` defines the narrow adapter boundary. It admits a
  runtime only when the approved source SHA, patch-manifest SHA-256 and image
  digest match, then requires a live `zlm-security-v1` probe with redirect
  rejection, same-origin control URI pinning, source/credential redaction,
  provider retry disablement, private API and HLS capabilities.
- `internal/media/zlm/gate_test.go` exercises the matching path, every identity
  mismatch, malformed approvals, each missing capability, provider errors and
  context cancellation.
- `deploy/zlm-secure-build/build-manifest.json` records the fixed upstream SHA,
  all five submodule gitlinks, CMake flags, required security patch seams and
  private-network runtime constraints.
- `deploy/zlm-secure-build/build.sh` is the reproducible source-build recipe;
  it refuses to run unless the manifest is approved, the base image is pinned,
  every required patch exists and its digest matches.

## Evidence ledger

| Scenario | Invocation | Binary observable | Artifact |
|---|---|---|---|
| Matching approved identity and all live capabilities | `go test -race -count=1 ./internal/media/zlm` | package exits 0; `TestCheckAcceptsMatchingBuildAndLiveProbe` passes | [gate-race.log](gate-race.log) |
| Source, patch and image digest mismatch fail closed | same invocation | three subtests return `ErrUnapprovedBuild` before capability probing | [gate-race.log](gate-race.log) |
| Malformed approval values fail closed | same invocation | source, patch and image malformed cases return `ErrInvalidApproval` | [gate-race.log](gate-race.log) |
| Missing probe version or any required security capability fails closed | same invocation | seven subtests return `ErrCapabilityFailure` | [gate-race.log](gate-race.log) |
| Provider identity/probe errors and canceled context | same invocation | typed unavailable/probe errors and `context.Canceled`; raw provider text is not returned | [gate-race.log](gate-race.log) |
| Static Go checks | `go vet ./internal/media/zlm`; `gofmt -l internal/media/zlm/*.go` | vet exits 0 and formatter emits no paths | [gate-vet.log](gate-vet.log), [gate-format.log](gate-format.log) |
| Recipe guard | `deploy/zlm-secure-build/build.sh /tmp/iolink-rtsp-build-seams/zlm /tmp/iolink-zlm-security-build` | exits 1 before any submodule fetch because the manifest is not approved | [build-recipe.log](build-recipe.log) |
| Manifest guard | Python manifest check in [manifest-gate.log](manifest-gate.log) | exits 1 with `FAIL_CLOSED: manifest status is not approved`; required patch SHA values and base image digest are null | [manifest-gate.log](manifest-gate.log) |

## External blocker

Docker Engine 29.9.0 is present, but a hardened ZLM build is not runnable from
this increment: the required source patches have not been implemented or
reviewed, the manifest intentionally has no approved patch or base-image
digest, and the host has no `cmake`. No Docker/ZLM build or runtime success is
claimed. The exact source investigation and patch seams are in
[rtsp-build-seams.md](../2026-10-11-foundation/rtsp-build-seams.md).

The implementation tree and evidence files are hashed in
[source-sha256sums.txt](source-sha256sums.txt); no camera credentials, runtime
provider URLs or unapproved image digests were recorded.
