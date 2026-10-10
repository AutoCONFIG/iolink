# Camera HTTP F1 audit fix evidence

Worktree: `/media/yun/706bc403-c76c-4fdd-8a3f-d954b6189048/iolink/.omo/teams/team-75bb9f76/worktrees/A`

## Platform ADMIN mini camera isolation

Command:

```text
IOLINK_TEST_PG_DSN='postgres://iolink_test:***@127.0.0.1:32904/postgres?sslmode=disable' GIN_MODE=release go test ./cmd/iolinkd -run '^TestCameraHTTPRealPGPlatformAdminMiniCameraForbidden$' -count=1 -v
```

Observed output:

```text
=== RUN   TestCameraHTTPRealPGPlatformAdminMiniCameraForbidden
    camera_http_auth_test.go:60: method=GET path=/api/v1/cameras status=403 redacted_response={"code":"forbidden","message":"forbidden"}
    camera_http_auth_test.go:60: method=GET path=/api/v1/cameras/1701 status=403 redacted_response={"code":"forbidden","message":"forbidden"}
--- PASS: TestCameraHTTPRealPGPlatformAdminMiniCameraForbidden (0.55s)
PASS
ok   git.hyhy.fun/rsplab/iolink/cmd/iolinkd 0.562s
```

The fixture uses a real PostgreSQL ADMIN with an active support tenant and farm grant. Both mini read routes return the constant 403 body, and the test compares `video_cameras` row count before and after both requests.

## Ordinary user and live-scope regression

Command:

```text
IOLINK_TEST_PG_DSN='postgres://iolink_test:***@127.0.0.1:32904/postgres?sslmode=disable' GIN_MODE=release go test ./cmd/iolinkd -run 'TestCameraHTTPRealPG(PlatformAdminMiniCameraForbidden|ScopesByCurrentRoleFarmAndTenant|RevocationIsLive)' -count=1 -v
```

Observed output:

```text
--- PASS: TestCameraHTTPRealPGScopesByCurrentRoleFarmAndTenant (0.92s)
    --- PASS: .../owner_mini_false
    --- PASS: .../owner_mini_true
    --- PASS: .../admin_mini_false
    --- PASS: .../admin_mini_true
    --- PASS: .../member_mini_false
    --- PASS: .../member_mini_true
    --- PASS: .../viewer_mini_false
    --- PASS: .../viewer_mini_true
    --- PASS: .../support_mini_false
    --- PASS: .../support_mini_true
--- PASS: TestCameraHTTPRealPGRevocationIsLive (0.49s)
PASS
ok   git.hyhy.fun/rsplab/iolink/cmd/iolinkd 1.958s
```

Ordinary USER roles retain their expected list/detail visibility, and live farm revocation remains enforced.

## Race, vet, format, and contracts

Commands and outputs:

```text
GIN_MODE=release go test -race -shuffle=on -count=1 ./internal/adminapi ./internal/appapi ./internal/core ./cmd/iolinkd
ok   git.hyhy.fun/rsplab/iolink/internal/adminapi 10.980s
ok   git.hyhy.fun/rsplab/iolink/internal/appapi 1.050s
ok   git.hyhy.fun/rsplab/iolink/internal/core 1.045s
ok   git.hyhy.fun/rsplab/iolink/cmd/iolinkd 1.033s

go vet ./internal/appapi ./internal/core ./cmd/iolinkd
exit 0

gofmt -l internal/appapi/server.go internal/appapi/cameras.go internal/appapi/wechat.go internal/core/users.go cmd/iolinkd/camera_http_fixture_test.go cmd/iolinkd/camera_http_auth_test.go
no output (all formatted)

/media/yun/706bc403-c76c-4fdd-8a3f-d954b6189048/iolink/.venv/contracts/bin/python scripts/check_contracts.py
docs/api/video-openapi.yaml: standard schema and 22 operation fixtures PASS
96 target operations, 568 synthetic request/response fixtures PASS; live handler verification is R02.c
```

`make verify-contracts` could not find the worktree-local `.venv/contracts/bin/python`; the same checker completed successfully through the existing repository absolute virtualenv shown above.

## Final rerun after fail-closed authority context guard

```text
GIN_MODE=release go test -race -shuffle=on -count=1 ./internal/adminapi ./internal/appapi ./internal/core ./cmd/iolinkd
ok   git.hyhy.fun/rsplab/iolink/internal/adminapi 10.849s
ok   git.hyhy.fun/rsplab/iolink/internal/appapi 1.044s
ok   git.hyhy.fun/rsplab/iolink/internal/core 1.041s
ok   git.hyhy.fun/rsplab/iolink/cmd/iolinkd 1.036s

go vet ./internal/appapi ./internal/core ./cmd/iolinkd
exit 0

gofmt -l <owned files>
no output

absolute contracts checker
docs/api/video-openapi.yaml: standard schema and 22 operation fixtures PASS
96 target operations, 568 synthetic request/response fixtures PASS; live handler verification is R02.c
```

The route now returns 503 when the production authority port is absent, and 403 only after a live authority lookup identifies platform ADMIN.
