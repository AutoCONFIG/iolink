#!/bin/sh
set -eu
: "${IOLINKD_IMAGE:?set IOLINKD_IMAGE to a locally available image}"
: "${IOLINK_DB_IMAGE:?set IOLINK_DB_IMAGE to a locally available image}"
out=${1:-dist/iolink-offline}
source_revision=$(git rev-parse HEAD)
rm -rf "$out"
mkdir -p "$out/images" "$out/deploy" "$out/docs" "$out/frontend" "$out/migrations" "$out/metadata"
docker image inspect "$IOLINKD_IMAGE" >/dev/null
docker image inspect "$IOLINK_DB_IMAGE" >/dev/null
docker image inspect "$IOLINKD_IMAGE" --format '{{.Id}}' >"$out/images/iolinkd.id"
docker image inspect "$IOLINK_DB_IMAGE" --format '{{.Id}}' >"$out/images/db.id"
docker image inspect "$IOLINKD_IMAGE" >"$out/images/iolinkd.manifest.json"
docker image inspect "$IOLINK_DB_IMAGE" >"$out/images/db.manifest.json"
docker image inspect "$IOLINKD_IMAGE" --format '{{json .RepoDigests}}' >"$out/metadata/iolinkd.repo-digests.json"
docker image inspect "$IOLINK_DB_IMAGE" --format '{{json .RepoDigests}}' >"$out/metadata/db.repo-digests.json"
test "$(docker image inspect "$IOLINKD_IMAGE" --format '{{.Os}}/{{.Architecture}}')" = linux/amd64
test "$(docker image inspect "$IOLINK_DB_IMAGE" --format '{{.Os}}/{{.Architecture}}')" = linux/amd64
printf 'IOLINKD_IMAGE=%s\nIOLINK_DB_IMAGE=%s\n' "$IOLINKD_IMAGE" "$IOLINK_DB_IMAGE" >"$out/images/refs.env"
docker save "$IOLINKD_IMAGE" | gzip -n >"$out/images/iolinkd.tar.gz"
docker save "$IOLINK_DB_IMAGE" | gzip -n >"$out/images/db.tar.gz"
cp deploy/docker-compose.offline.yaml "$out/deploy/docker-compose.yaml"
cp scripts/install-offline.sh "$out/install.sh"
cp scripts/uninstall-offline.sh "$out/uninstall.sh"
cp scripts/offline-smoke.sh "$out/smoke.sh"
cp docs/DEPLOY.md docs/design-m6c.md "$out/docs/"
cp deploy/.env.example "$out/deploy/"
cp deploy/reverse-proxy.optional.yaml deploy/streaming.optional.yaml "$out/deploy/"
cp -a internal/migrate/sql "$out/migrations/"
go list -m all >"$out/metadata/go-dependencies.txt"
go list -m -json all | python3 -c '
import json, sys
packages = []
decoder = json.JSONDecoder()
raw = sys.stdin.read()
pos = 0
while pos < len(raw):
    while pos < len(raw) and raw[pos].isspace():
        pos += 1
    if pos >= len(raw):
        break
    item, end = decoder.raw_decode(raw, pos)
    packages.append({"name": item.get("Path", ""), "version": item.get("Version", "") or "NOASSERTION", "licenseConcluded": "NOASSERTION"})
    pos = end
json.dump({"spdxVersion":"SPDX-2.3","dataLicense":"CC0-1.0","SPDXID":"SPDXRef-DOCUMENT","name":"iolink-offline-go-dependencies","documentNamespace":"https://iolink.invalid/sbom/" + "'"$source_revision"'" ,"packages":packages}, sys.stdout, indent=2)
print()
' >"$out/metadata/sbom.spdx.json"
printf '%s\n' 'License expressions are NOASSERTION; review upstream notices before redistribution.' >"$out/metadata/licenses.txt"
if test -d web/dist; then
  cp -a web/dist "$out/frontend/admin-dist"
else
  printf '%s\n' 'Admin web assets are embedded in the iolinkd image; web/dist was not present in the source checkout.' >"$out/frontend/README.txt"
fi
printf '%s\n' "IOLINKD_IMAGE=$IOLINKD_IMAGE" "IOLINK_DB_IMAGE=$IOLINK_DB_IMAGE" "SOURCE_REVISION=$(git rev-parse HEAD)" >"$out/VERSION"
(cd "$out" && {
  printf '{"source_revision":%s,"architecture":"linux/amd64","application_image_id":%s,"database_image_id":%s,"artifacts":["deploy/docker-compose.yaml","deploy/.env.example","deploy/reverse-proxy.optional.yaml","deploy/streaming.optional.yaml","migrations/sql","metadata/go-dependencies.txt","metadata/licenses.txt","metadata/sbom.spdx.json","smoke.sh","images/iolinkd.manifest.json","images/db.manifest.json"]}\n' \
    "$(printf '%s' "$source_revision" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read().strip()))')" \
    "$(printf '%s' "$(cat images/iolinkd.id)" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read().strip()))')" \
    "$(printf '%s' "$(cat images/db.id)" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read().strip()))')" > metadata/manifest.json
})
(cd "$out" && find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum > SHA256SUMS)
