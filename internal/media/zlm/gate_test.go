package zlm_test

import (
	"context"
	"errors"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/media/zlm"
)

const (
	patchDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	imageDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

type fakeRuntime struct {
	identity    zlm.BuildIdentity
	probes      zlm.Capabilities
	identityErr error
	probeErr    error
}

func (f fakeRuntime) BuildIdentity(context.Context) (zlm.BuildIdentity, error) {
	return f.identity, f.identityErr
}

func (f fakeRuntime) ProbeCapabilities(context.Context) (zlm.Capabilities, error) {
	return f.probes, f.probeErr
}

func approval() zlm.Approval {
	return zlm.Approval{SourceSHA: zlm.PinnedSourceSHA, PatchManifestSHA256: patchDigest, ImageDigest: imageDigest}
}

func identity() zlm.BuildIdentity {
	a := approval()
	return zlm.BuildIdentity{SourceSHA: a.SourceSHA, PatchManifestSHA256: a.PatchManifestSHA256, ImageDigest: a.ImageDigest}
}

func capabilities() zlm.Capabilities {
	return zlm.Capabilities{ProbeVersion: zlm.CapabilityProbeVersion, RejectsRedirects: true, PinsControlURIs: true, RedactsSourceCredentials: true, DisablesProviderRetries: true, PrivateAPI: true, HLS: true}
}

func TestCheckAcceptsMatchingBuildAndLiveProbe(t *testing.T) {
	got := zlm.Check(context.Background(), fakeRuntime{identity: identity(), probes: capabilities()}, approval())
	if got != nil {
		t.Fatalf("matching provider rejected: %v", got)
	}
}

func TestCheckRejectsUnapprovedIdentityBeforeProbe(t *testing.T) {
	for name, mutate := range map[string]func(*zlm.BuildIdentity){
		"source": func(v *zlm.BuildIdentity) { v.SourceSHA = "1111111111111111111111111111111111111111" },
		"patch":  func(v *zlm.BuildIdentity) { v.PatchManifestSHA256 = patchDigest[:63] + "b" },
		"image":  func(v *zlm.BuildIdentity) { v.ImageDigest = "sha256:" + patchDigest },
	} {
		t.Run(name, func(t *testing.T) {
			got := identity()
			mutate(&got)
			if err := zlm.Check(context.Background(), fakeRuntime{identity: got, probes: capabilities()}, approval()); !errors.Is(err, zlm.ErrUnapprovedBuild) {
				t.Fatalf("error = %v, want ErrUnapprovedBuild", err)
			}
		})
	}
}

func TestCheckRejectsMalformedApproval(t *testing.T) {
	for name, mutate := range map[string]func(*zlm.Approval){
		"source": func(v *zlm.Approval) { v.SourceSHA = "main" },
		"patch":  func(v *zlm.Approval) { v.PatchManifestSHA256 = "" },
		"image":  func(v *zlm.Approval) { v.ImageDigest = "latest" },
	} {
		t.Run(name, func(t *testing.T) {
			got := approval()
			mutate(&got)
			if err := zlm.Check(context.Background(), fakeRuntime{identity: identity(), probes: capabilities()}, got); !errors.Is(err, zlm.ErrInvalidApproval) {
				t.Fatalf("error = %v, want ErrInvalidApproval", err)
			}
		})
	}
}

func TestCheckRejectsEveryMissingCapability(t *testing.T) {
	base := capabilities()
	mutations := map[string]func(*zlm.Capabilities){
		"probe version": func(v *zlm.Capabilities) { v.ProbeVersion = "zlm-security-v0" },
		"redirect":      func(v *zlm.Capabilities) { v.RejectsRedirects = false },
		"origin":        func(v *zlm.Capabilities) { v.PinsControlURIs = false },
		"redaction":     func(v *zlm.Capabilities) { v.RedactsSourceCredentials = false },
		"retry":         func(v *zlm.Capabilities) { v.DisablesProviderRetries = false },
		"private api":   func(v *zlm.Capabilities) { v.PrivateAPI = false },
		"hls":           func(v *zlm.Capabilities) { v.HLS = false },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			got := base
			mutate(&got)
			if err := zlm.Check(context.Background(), fakeRuntime{identity: identity(), probes: got}, approval()); !errors.Is(err, zlm.ErrCapabilityFailure) {
				t.Fatalf("error = %v, want ErrCapabilityFailure", err)
			}
		})
	}
}

func TestCheckFailsClosedOnProviderErrorsAndCancellation(t *testing.T) {
	if err := zlm.Check(context.Background(), fakeRuntime{identityErr: errors.New("provider details must not escape")}, approval()); !errors.Is(err, zlm.ErrUnavailable) {
		t.Fatalf("identity error = %v, want ErrUnavailable", err)
	}
	if err := zlm.Check(context.Background(), fakeRuntime{identity: identity(), probeErr: errors.New("raw probe details")}, approval()); !errors.Is(err, zlm.ErrCapabilityFailure) {
		t.Fatalf("probe error = %v, want ErrCapabilityFailure", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := zlm.Check(ctx, fakeRuntime{identity: identity(), probes: capabilities()}, approval()); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled check = %v, want context.Canceled", err)
	}
}
