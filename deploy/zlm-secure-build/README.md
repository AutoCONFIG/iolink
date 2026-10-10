# Pinned ZLMediaKit security build boundary

This directory is a build boundary for the M7a provider. It does not enable
video or assemble a ZLMediaKit process. `build-manifest.json` pins the approved
upstream commit and all five recorded submodule gitlinks, and names the two
required source patch seams from the M7a review.

`build.sh` checks the upstream commit, initializes submodules, refuses an
unapproved manifest, requires a base image digest and requires a SHA-256 digest
for every required patch before running the release CMake build and tests. The
current manifest is deliberately `external_blocked`: the reviewed patches,
base image digest and final image digest are not available yet.

The runtime contract is `internal/media/zlm`. A future composition root must
call its gate with the private provider probe before enabling video. The gate
requires the pinned source, patch manifest digest, image digest, probe version,
redirect rejection, same-origin control URI pinning, source/credential log
redaction, provider retry disablement, private API and HLS capability.
