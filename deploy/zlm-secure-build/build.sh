#!/bin/sh
set -eu

manifest_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
source_dir=${1:?usage: build.sh SOURCE_CHECKOUT [BUILD_DIR]}
build_dir=${2:-"$source_dir/build"}
manifest="$manifest_dir/build-manifest.json"

command -v git >/dev/null 2>&1 || { echo "git is required" >&2; exit 1; }

expected_source=$(python3 - "$manifest" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as f:
    print(json.load(f)["source"]["commit"])
PY
)
actual_source=$(git -C "$source_dir" rev-parse HEAD)
[ "$actual_source" = "$expected_source" ] || { echo "unapproved ZLMediaKit source revision" >&2; exit 1; }

python3 - "$manifest" "$manifest_dir" <<'PY'
import hashlib, json, pathlib, sys
manifest_path = pathlib.Path(sys.argv[1])
manifest_dir = pathlib.Path(sys.argv[2])
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
if manifest.get("status") != "approved":
    raise SystemExit("ZLMediaKit security manifest is not approved")
if not manifest["build"].get("base_image_digest"):
    raise SystemExit("ZLMediaKit base image digest is not pinned")
for patch in manifest["patches"]:
    if patch.get("required") and not patch.get("sha256"):
        raise SystemExit("required ZLMediaKit security patch is not hashed")
    path = manifest_dir / patch["path"]
    if not path.is_file():
        raise SystemExit(f"missing required ZLMediaKit security patch: {patch['path']}")
    actual = hashlib.sha256(path.read_bytes()).hexdigest()
    if actual != patch["sha256"]:
        raise SystemExit(f"ZLMediaKit security patch digest mismatch: {patch['id']}")
PY

git -C "$source_dir" diff --quiet || { echo "ZLMediaKit source checkout is dirty" >&2; exit 1; }
git -C "$source_dir" diff --cached --quiet || { echo "ZLMediaKit source index is dirty" >&2; exit 1; }
[ -z "$(git -C "$source_dir" status --porcelain --untracked-files=all)" ] || { echo "ZLMediaKit source checkout is dirty" >&2; exit 1; }

required_patches=$(python3 - "$manifest" <<'PY'
import json, pathlib, sys
manifest = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
for patch in manifest["patches"]:
    if patch.get("required"):
        print(patch["path"])
PY
)

for patch in $required_patches; do
	patch_path="$manifest_dir/$patch"
	git -C "$source_dir" apply --check --whitespace=error "$patch_path" || {
		echo "required ZLMediaKit security patch does not apply: $patch" >&2
		exit 1
	}
done

for patch in $required_patches; do
	patch_path="$manifest_dir/$patch"
	git -C "$source_dir" apply --whitespace=error "$patch_path" || {
		echo "failed to apply required ZLMediaKit security patch: $patch" >&2
		exit 1
	}
	git -C "$source_dir" apply --check --reverse --whitespace=error "$patch_path" || {
		echo "required ZLMediaKit security patch application could not be verified: $patch" >&2
		exit 1
	}
done

git -C "$source_dir" submodule update --init --recursive
git -C "$source_dir" submodule status --recursive | grep '^-' >/dev/null 2>&1 && {
	echo "uninitialized ZLMediaKit submodule" >&2
	exit 1
}

python3 - "$manifest" "$source_dir" <<'PY'
import json, pathlib, subprocess, sys
manifest = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
source = pathlib.Path(sys.argv[2])
paths = {"ZLToolKit": "3rdpart/ZLToolKit", "jsoncpp": "3rdpart/jsoncpp", "media-server": "3rdpart/media-server", "pybind11": "3rdpart/pybind11", "webassist": "www/webassist"}
for name, expected in manifest["source"]["submodules"].items():
    path = paths[name]
    gitlink = subprocess.check_output(["git", "-C", str(source), "rev-parse", f"HEAD:{path}"], text=True).strip()
    if gitlink != expected:
        raise SystemExit(f"ZLMediaKit submodule gitlink mismatch: {name}")
    checkout = subprocess.check_output(["git", "-C", str(source / path), "rev-parse", "HEAD"], text=True).strip()
    if checkout != expected:
        raise SystemExit(f"ZLMediaKit submodule checkout mismatch: {name}")
PY

command -v cmake >/dev/null 2>&1 || { echo "cmake is required" >&2; exit 1; }

cmake -S "$source_dir" -B "$build_dir" -DCMAKE_BUILD_TYPE=Release -DENABLE_FFMPEG=OFF -DENABLE_WEBRTC=OFF -DENABLE_TESTS=ON
cmake --build "$build_dir" --parallel
ctest --test-dir "$build_dir" --output-on-failure
