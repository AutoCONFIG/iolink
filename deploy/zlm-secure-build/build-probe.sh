#!/bin/sh
set -eu

root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
source="$root/source"
tool="$root/tool"
bin="$root/bin"
mkdir -p "$source" "$tool/patches" "$bin"

git init -q "$source"
git -C "$source" config user.email probe@example.invalid
git -C "$source" config user.name probe
printf '%s\n' before > "$source/tracked.txt"
git -C "$source" add tracked.txt
git -C "$source" commit -q -m base
printf '%s\n' after > "$source/tracked.txt"
git -C "$source" diff > "$tool/patches/change.patch"
git -C "$source" checkout -- tracked.txt
cp "$(dirname "$0")/build.sh" "$tool/build.sh"
chmod +x "$tool/build.sh"
cat > "$bin/cmake" <<'EOF'
#!/bin/sh
exit 0
EOF
cat > "$bin/ctest" <<'EOF'
#!/bin/sh
exit 0
EOF
chmod +x "$bin/cmake" "$bin/ctest"

source_commit=$(git -C "$source" rev-parse HEAD)
patch_hash=$(sha256sum "$tool/patches/change.patch" | awk '{print $1}')
python3 - "$tool/build-manifest.json" "$source_commit" "$patch_hash" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
path.write_text(json.dumps({"schema": 1, "status": "approved", "source": {"commit": sys.argv[2], "submodules": {}}, "patches": [{"id": "change", "path": "patches/change.patch", "sha256": sys.argv[3], "required": True}], "build": {"base_image_digest": "sha256:probe"}}), encoding="utf-8")
PY

PATH="$bin:$PATH" "$tool/build.sh" "$source" "$root/build"
grep -qx 'after' "$source/tracked.txt"
git -C "$source" reset --hard -q HEAD

printf '%s\n' dirty > "$source/dirty.txt"
if PATH="$bin:$PATH" "$tool/build.sh" "$source" "$root/build-dirty" >"$root/dirty.out" 2>&1; then
    echo "dirty source was accepted" >&2
    exit 1
fi
grep -q 'source checkout is dirty' "$root/dirty.out"
rm "$source/dirty.txt"

cp "$tool/patches/change.patch" "$root/change.patch"
printf '%s\n' 'not a patch' > "$tool/patches/change.patch"
patch_hash=$(sha256sum "$tool/patches/change.patch" | awk '{print $1}')
python3 - "$tool/build-manifest.json" "$patch_hash" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
data = json.loads(path.read_text(encoding="utf-8"))
data["patches"][0]["sha256"] = sys.argv[2]
path.write_text(json.dumps(data), encoding="utf-8")
PY
if PATH="$bin:$PATH" "$tool/build.sh" "$source" "$root/build-unapplied" >"$root/unapplied.out" 2>&1; then
    echo "unapplied patch was accepted" >&2
    exit 1
fi
grep -q 'patch does not apply' "$root/unapplied.out"
cp "$root/change.patch" "$tool/patches/change.patch"

python3 - "$tool/build-manifest.json" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
data = json.loads(path.read_text(encoding="utf-8"))
data["status"] = "external_blocked"
path.write_text(json.dumps(data), encoding="utf-8")
PY
if PATH="$bin:$PATH" "$tool/build.sh" "$source" "$root/build-blocked" >"$root/blocked.out" 2>&1; then
    echo "unapproved manifest was accepted" >&2
    exit 1
fi
grep -q 'manifest is not approved' "$root/blocked.out"
grep -qx 'before' "$source/tracked.txt"
printf '%s\n' 'zlm build probe: approved patch applied and reverse-checked; unapplied, dirty, and unapproved inputs rejected before build/fetch.'
