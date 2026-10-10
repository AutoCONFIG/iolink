# M7a domain and credential foundation — member A

Date: 2026-10-11 Asia/Shanghai. Requirements: R43 foundation only; design authority docs/design/M7a-video.md and docs/api/video-openapi.yaml. This report does not mark R43, R36.b/R37.b/R39.b, or the M7a phase complete.

## Scope and implementation

Owned files: internal/domain/video*.go and internal/videocredential/*.go. Shared checkout, no commit/push, no unrelated code edits. Retained the interrupted attempt's typed RTSP/GB source, playback-window, credential-binding, and AES-GCM foundation. Resume edits add explicit bracketed-IPv6 validation using netip, three extra malformed-authority cases, and independent HMAC HKDF Extract/Expand interoperability in both encryption directions. Secret RTSP values reject JSON serialization and redact standard formatting. Cipher uses a camera-specific 32-byte HKDF-SHA256 key, random 12-byte nonce + 16-byte tag envelope, and purpose/tenant/entity/source-version AAD.

Plaintext bound is 4096 bytes, ciphertext bound is 4124 bytes. The 128-character username/password contract is exercised both as actual 4-byte Unicode scalars and escaped UTF-16 surrogate JSON. These cipher limits bound the encrypted envelope; HTTP character validation remains the future application boundary's responsibility.

This scope does not wire video into runtime routes or composition, create provider connections, or alter default configuration. SSRF allowlist/DNS/IP pinning, provider hardening, camera authorization, session state transitions, persistence, real RTSP/GB/media/browser/WeChat operations remain future work. This is a foundation report only.

## Fresh direct evidence

No earlier report was used as proof. Installed toolchain and owned-file SHA256 snapshot are in environment.log. Parent HEAD: de86a55383697726ba6403386d6afe8c51cbe8fa; owned files remain uncommitted. No active currentAttemptDir was available: `omo-agent-toolkit ulw-loop status --json` returned ULW_LOOP_PLAN_MISSING. Evidence is recorded under .omo/evidence/m7a-domain-crypto-20261011/.

Invocations actually run:

- T0: `go test -race -shuffle=on -count=1 -v ./internal/domain ./internal/videocredential` before resume edits; exit 0, baseline.log. Installed Go 1.26.8 already rejects the prior bracketed non-IP/raw IPv6 test cases; no fail-before claim is made.
- T1: `go test -race -shuffle=on -count=1 -v ./internal/domain ./internal/videocredential` after final source edits; exit 0, scoped-tests.log. Both packages emit PASS and ok; no skips.
- V1: `go vet ./internal/domain ./internal/videocredential`; exit 0, scoped-vet.log.
- F1: `gofumpt -l internal/domain/video*.go internal/videocredential/*.go` and `goimports -local git.hyhy.fun/rsplab/iolink -l internal/domain/video*.go internal/videocredential/*.go`; no filenames emitted; format.log records the result.

All paths below are relative to .omo/evidence/m7a-domain-crypto-20261011/. Every T1 row is a real named scenario executed by the complete T1 invocation, with assertions against the public constructors/cipher.

| Criterion / exact scenario | Invocation | Binary observable | Captured artifact |
|---|---|---|---|
| Unsafe RTSP syntax: TestVideoRTSPSourceRejectsUnsafeSyntax, cases 0–24; includes userinfo/query/fragment/ports, decoded traversal/control/backslash/double escapes, bracketed DNS/IPv4, raw IPv6, zone, suffix, empty port | T1 | 25 cases PASS; each returns ErrInvalidVideoSource | scoped-tests.log |
| Valid RTSP sources: TestVideoRTSPSourcePreservesValidEncodedPath; DNS, IPv4, bracketed IPv6 with/without port, encoded space | T1 | PASS; raw URI preserved and RTSP kind returned | scoped-tests.log |
| Secret serialization: TestVideoRTSPSourceDoesNotSerializeOrFormatURI | T1 | PASS; json.Marshal returns ErrVideoSecretSerialization; %s/%v/%+v/%#v omit host | scoped-tests.log |
| GB identity: TestVideoGBSourceRejectsInvalidIdentity; TestVideoGBSourceKeepsDeviceAndChannel | T1 | PASS; invalid IDs reject, valid device/channel preserved | scoped-tests.log |
| TTL: TestVideoPlaybackWindowEnforcesFiveMinuteMaximum; TestVideoPlaybackActiveAtChecksExactBoundariesAndTerminalStates; TestVideoPlaybackZeroWindowIsInactive | T1 | PASS; >300s/nonpositive/zero creation reject; now >= expiresAt and failed/revoked/expired/unknown states inactive; zero window inactive | scoped-tests.log |
| Codec baseline: TestVideoCodecsRejectImplicitTranscoding | T1 | PASS; H264+AAC or no audio true; H265/G711/missing video false | scoped-tests.log |
| Random authenticated envelope and reconstruction: TestVideoCredentialRoundTrip | T1 | PASS; ciphertext lacks plaintext; size plaintext+28; new same-root instance recovers exact plaintext | scoped-tests.log |
| Independent cryptographic format: TestVideoCredentialEnvelopeUsesAES256GCMWithIndependentHKDFKey; TestVideoCredentialOpensIndependentAES256GCMEnvelope | T1 | PASS; independent HMAC HKDF + AES256GCM decrypts adapter output, direct root key fails, independently generated nonce/AAD envelope opens | scoped-tests.log |
| Unicode contract: TestVideoCredentialRoundTripMaximumUnicodeFields; TestVideoCredentialRoundTripEscapedMaximumUnicodeFields | T1 | PASS; both 128-scalar username and password round trip byte-for-byte | scoped-tests.log |
| Bounds: TestVideoCredentialRoundTripAtSizeBoundaries/1 and /4096; TestVideoCredentialFailsClosedOnInvalidInputs; TestVideoCredentialRejectsOversizedEnvelope | T1 | PASS; 1 and 4096 plaintext round trip; nil/empty/4097 seal reject; 4125-byte open rejects with nil plaintext | scoped-tests.log |
| AAD and key isolation: TestVideoCredentialRejectsWrongKeyAndBinding | T1 | PASS; different root, tenant, entity, version, camera/GB namespace all return ErrVideoCredentialUnavailable and nil plaintext | scoped-tests.log |
| Integrity/truncation: TestVideoCredentialRejectsTamperedAndTruncatedEnvelope | T1 | PASS; each envelope byte flip and every shorter prefix rejects with nil plaintext | scoped-tests.log |
| Nonce sampling: TestVideoCredentialSealUsesDistinctRandomNonces | T1 | PASS; 32 repeated same-key/plaintext encryptions have distinct 12-byte nonce prefixes | scoped-tests.log |
| Zero/invalid configuration: TestVideoCredentialRejectsZeroInstancesAndBinding; TestVideoCredentialFailsClosedOnInvalidInputs | T1 | PASS; nil/zero cipher, zero binding, short root, unknown purpose, nonpositive binding components fail closed | scoped-tests.log |
| Cancellation: TestVideoCredentialHonorsCanceledContext; TestVideoCredentialHonorsExpiredDeadline | T1 | PASS; Seal/Open return Canceled or DeadlineExceeded and no result | scoped-tests.log |
| Build-time static checks | V1 | exit 0 | scoped-vet.log |
| Formatting | F1 | no filenames | format.log |

## Architectural self-review

All 8 owned files are under 200 pure lines (range 37–164), captured in source-review.log. Responsibilities are source values, playback window, AAD binding/port, credential encryption, and three distinct credential test concerns. Untrusted URI/identity/time/binding inputs use constructors and private fields. Playback state dispatch lists all current variants and rejects unknown states; purpose checks validate closed membership without variant-dependent behavior. Zero cipher/binding checks are necessary Go zero-value boundary checks. No logging, network I/O, goroutines, clock acquisition, arbitrary maps, panic, or SDK dependency is added. All introduced behavior has a named public-surface test. Independent key helper is reused by both interoperability tests. The existing four-argument binding constructor intentionally exposes the complete security binding (purpose plus tenant/entity/version); the test fixture mirrors that tuple plus testing.T. The uint64 conversion in AssociatedData encodes already-positive ID components as fixed-width big-endian values, and adapters reject zero bindings before use. No redundant destructive verification or negative-name flags were added.

## Limitations and outstanding gates

- golangci-lint and nilaway are absent in the local PATH; not run and not claimed passed. No global toolchain/dependency/config edits were made.
- Full make verify, real PostgreSQL/Timescale migration/history tests, and contract checks are being captured by the leader/member B; this member reports no database/integration completion.
- External RTSP/GB hardware, media provider safety build, browser/WeChat playback are external_blocked or unimplemented at this foundation milestone; not claimed passed.
- Two independent non-editing reviewers and any phase acceptance remain the leader's gate; this report is executor evidence only.
