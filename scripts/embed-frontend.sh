#!/bin/sh
# Mirror the web/ submodule's built frontend (web/dist) into internal/web/dist
# so go:embed can pick it up — embed patterns cannot reach outside the package
# directory. Falls back to a generated placeholder page when the submodule has
# no dist yet, keeping every entry point (make, CI, docker build) green.
set -e

src=web/dist
dst=internal/web/dist

rm -rf "$dst"
mkdir -p "$dst"

if [ -f "$src/index.html" ]; then
    cp -r "$src"/. "$dst/"
else
    cat > "$dst/index.html" <<'EOF'
<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>IoLink 管理后台</title></head>
<body style="font-family:system-ui;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;background:#0f172a;color:#94a3b8">
<div style="text-align:center">
<h1 style="color:#e2e8f0">IoLink 管理后台</h1>
<p>前端构建产物尚未部署(web/dist 缺失,此为构建时兜底页)。</p>
<p>接口契约见仓库 docs/api/admin-openapi.yaml</p>
</div></body></html>
EOF
fi
