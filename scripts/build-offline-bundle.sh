#!/bin/sh
set -eu
: "${IOLINKD_IMAGE:?set IOLINKD_IMAGE to a locally available image}"
: "${IOLINK_DB_IMAGE:?set IOLINK_DB_IMAGE to a locally available image}"
out=${1:-dist/iolink-offline}
rm -rf "$out"
mkdir -p "$out/images" "$out/deploy" "$out/docs" "$out/frontend"
docker image inspect "$IOLINKD_IMAGE" >/dev/null
docker image inspect "$IOLINK_DB_IMAGE" >/dev/null
docker image inspect "$IOLINKD_IMAGE" --format '{{.Id}}' >"$out/images/iolinkd.id"
docker image inspect "$IOLINK_DB_IMAGE" --format '{{.Id}}' >"$out/images/db.id"
docker image inspect "$IOLINKD_IMAGE" >"$out/images/iolinkd.manifest.json"
docker image inspect "$IOLINK_DB_IMAGE" >"$out/images/db.manifest.json"
test "$(docker image inspect "$IOLINKD_IMAGE" --format '{{.Os}}/{{.Architecture}}')" = linux/amd64
test "$(docker image inspect "$IOLINK_DB_IMAGE" --format '{{.Os}}/{{.Architecture}}')" = linux/amd64
printf 'IOLINKD_IMAGE=%s\nIOLINK_DB_IMAGE=%s\n' "$IOLINKD_IMAGE" "$IOLINK_DB_IMAGE" >"$out/images/refs.env"
docker save "$IOLINKD_IMAGE" | gzip -n >"$out/images/iolinkd.tar.gz"
docker save "$IOLINK_DB_IMAGE" | gzip -n >"$out/images/db.tar.gz"
cp deploy/docker-compose.offline.yaml "$out/deploy/docker-compose.yaml"
cp scripts/install-offline.sh "$out/install.sh"
cp scripts/uninstall-offline.sh "$out/uninstall.sh"
cp docs/DEPLOY.md docs/design-m6c.md "$out/docs/"
if test -d web/dist; then
  cp -a web/dist "$out/frontend/admin-dist"
else
  printf '%s\n' 'Admin web assets are embedded in the iolinkd image; web/dist was not present in the source checkout.' >"$out/frontend/README.txt"
fi
printf '%s\n' "IOLINKD_IMAGE=$IOLINKD_IMAGE" "IOLINK_DB_IMAGE=$IOLINK_DB_IMAGE" "SOURCE_REVISION=$(git rev-parse HEAD)" >"$out/VERSION"
(cd "$out" && find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum > SHA256SUMS)
