# R28 software evidence

`go test -race -shuffle=on ./internal/notifications ./internal/wechat -count=1` passed. The suite covers disabled credentials, HTTP errors, malformed provider responses, token refresh, bounded retries, unknown outcomes, owner reassignment, lease recovery and duplicate-worker claiming. Real WeChat delivery remains `external_blocked` because credentials, subscription consent and a device receipt are unavailable.
