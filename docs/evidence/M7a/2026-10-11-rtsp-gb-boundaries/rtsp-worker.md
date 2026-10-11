# M7a RTSP admission and DNS pinning worker report

Date: 2026-10-11
Scope: `internal/media/rtsp/*.go`

## Implemented behavior

- `New` rejects a nil resolver, an empty CIDR allowlist, an empty port allowlist, invalid prefixes, and zero ports.
- `Pin` accepts only `rtsp` hierarchical URIs without userinfo, query, fragment, control characters, malformed authorities, or invalid ports. The implicit port is 554 and is still checked against the configured port allowlist.
- The resolver is called with `network="ip"` for every `Pin` call. Every returned A/AAAA answer must be valid, allowed by a configured CIDR, and not unspecified, multicast, loopback, link-local, or a known cloud metadata address. Mixed allowed and denied answers fail closed.
- The selected result is deterministic and numeric. `Endpoint.URI()` emits only `rtsp://numeric-address:port` plus the escaped original path; it contains no credentials or hostname. No socket or provider operation is performed.
- Resolver errors are reduced to a constant sentinel. Context cancellation is returned without exposing resolver details.

## Test evidence

The tests cover missing boundaries, default and configured ports, userinfo/query/fragment/traversal and nested encoded traversal, encoded path preservation, IPv4/IPv6 answers, duplicate answers, mixed DNS answers, metadata and special address rejection, DNS rebinding across calls, resolver call count, cancellation, and 64 concurrent calls.

Commands and observed output:

```text
go test -race -shuffle=on -count=1 ./internal/media/rtsp
ok   git.hyhy.fun/rsplab/iolink/internal/media/rtsp  1.012s

go vet ./internal/media/rtsp
ok

go test -race -shuffle=on -count=1 ./internal/media/...
?    git.hyhy.fun/rsplab/iolink/internal/media       [no test files]
ok   git.hyhy.fun/rsplab/iolink/internal/media/rtp  1.012s
ok   git.hyhy.fun/rsplab/iolink/internal/media/rtsp 1.012s
ok   git.hyhy.fun/rsplab/iolink/internal/media/zlm  1.012s

go vet ./internal/media/...
ok

gofumpt -l internal/media/rtsp/admission.go internal/media/rtsp/admission_test.go
(no output)

goimports -local git.hyhy.fun/rsplab/iolink -d internal/media/rtsp/admission.go internal/media/rtsp/admission_test.go
(no output)

git diff --check -- internal/media/rtsp/admission.go internal/media/rtsp/admission_test.go
(no output)
```

Pure LOC: `admission.go` 181, `admission_test.go` 156. No commit or push was performed. The package is not wired into runtime composition and does not claim real RTSP or ZLM acceptance.
