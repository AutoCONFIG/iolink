package zlm

import (
	"context"
	"errors"
	"regexp"
)

const (
	PinnedSourceSHA        = "46220e6a866592c140d719ca2981bd2276344f5e"
	CapabilityProbeVersion = "zlm-security-v1"
)

var (
	ErrUnavailable       = errors.New("zlm provider unavailable")
	ErrUnapprovedBuild   = errors.New("zlm provider build is not approved")
	ErrCapabilityFailure = errors.New("zlm provider capability probe failed")
	ErrInvalidApproval   = errors.New("zlm provider approval is invalid")
)

var (
	sha1Pattern   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type BuildIdentity struct {
	SourceSHA           string
	PatchManifestSHA256 string
	ImageDigest         string
}

type Capabilities struct {
	ProbeVersion             string
	RejectsRedirects         bool
	PinsControlURIs          bool
	RedactsSourceCredentials bool
	DisablesProviderRetries  bool
	PrivateAPI               bool
	HLS                      bool
}

type Runtime interface {
	BuildIdentity(context.Context) (BuildIdentity, error)
	ProbeCapabilities(context.Context) (Capabilities, error)
}

type Approval struct {
	SourceSHA           string
	PatchManifestSHA256 string
	ImageDigest         string
}

func Check(ctx context.Context, runtime Runtime, approval Approval) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if runtime == nil {
		return ErrUnavailable
	}
	if err := approval.validate(); err != nil {
		return err
	}

	identity, err := runtime.BuildIdentity(ctx)
	if err != nil {
		return ErrUnavailable
	}
	if !sameIdentity(identity, approval) {
		return ErrUnapprovedBuild
	}

	capabilities, err := runtime.ProbeCapabilities(ctx)
	if err != nil {
		return ErrCapabilityFailure
	}
	if !capabilities.approved() {
		return ErrCapabilityFailure
	}
	return nil
}

func (a Approval) validate() error {
	if a.SourceSHA != PinnedSourceSHA || !sha1Pattern.MatchString(a.SourceSHA) || !sha256Pattern.MatchString(a.PatchManifestSHA256) || !digestPattern.MatchString(a.ImageDigest) {
		return ErrInvalidApproval
	}
	return nil
}

func sameIdentity(identity BuildIdentity, approval Approval) bool {
	return identity.SourceSHA == approval.SourceSHA &&
		identity.PatchManifestSHA256 == approval.PatchManifestSHA256 &&
		identity.ImageDigest == approval.ImageDigest
}

func (c Capabilities) approved() bool {
	return c.ProbeVersion == CapabilityProbeVersion &&
		c.RejectsRedirects &&
		c.PinsControlURIs &&
		c.RedactsSourceCredentials &&
		c.DisablesProviderRetries &&
		c.PrivateAPI &&
		c.HLS
}
