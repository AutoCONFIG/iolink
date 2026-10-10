The reviewed ZLMediaKit source seams are pinned in `../build-manifest.json` and
`docs/evidence/M7a/2026-10-11-foundation/rtsp-build-seams.md`.

The two required security patches are intentionally absent until the RTSP
redirect, origin-control, logging, and retry changes have been implemented and
reviewed against the pinned source. `build.sh` refuses to build while the
manifest status, base image digest, patch paths, or patch hashes are missing.
