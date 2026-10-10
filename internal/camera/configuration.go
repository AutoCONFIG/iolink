package camera

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

var ErrInvalid = errors.New("invalid camera request")

// SourceInput is a write-only boundary value. Configuration prevents accidentally
// serializing secrets after parsing them.
type SourceInput struct {
	Kind        domain.VideoSourceKind
	URI         string
	Credentials *Credentials
	DeviceID    int64
	ChannelID   string
}
type Credentials struct{ Username, Password string }

func (SourceInput) String() string               { return "camera source [redacted]" }
func (SourceInput) GoString() string             { return "camera source [redacted]" }
func (SourceInput) MarshalJSON() ([]byte, error) { return nil, domain.ErrVideoSecretSerialization }
func (Credentials) String() string               { return "camera credentials [redacted]" }
func (Credentials) GoString() string             { return "camera credentials [redacted]" }
func (Credentials) MarshalJSON() ([]byte, error) { return nil, domain.ErrVideoSecretSerialization }

type Configuration struct {
	pondID     int64
	name       string
	source     domain.VideoSource
	credential []byte
}

func ParseConfiguration(pondID int64, name string, input SourceInput) (Configuration, error) {
	if pondID <= 0 || !utf8.ValidString(name) || utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 128 || strings.TrimSpace(name) == "" || strings.ContainsFunc(name, unicode.IsControl) {
		return Configuration{}, ErrInvalid
	}
	cfg := Configuration{pondID: pondID, name: name}
	var err error
	switch input.Kind {
	case domain.VideoRTSP:
		if input.DeviceID != 0 || input.ChannelID != "" {
			return Configuration{}, ErrInvalid
		}
		cfg.source, err = domain.NewRTSPVideoSource(input.URI)
		if err != nil {
			return Configuration{}, ErrInvalid
		}
		if input.Credentials != nil {
			for _, value := range []string{input.Credentials.Username, input.Credentials.Password} {
				if !utf8.ValidString(value) || utf8.RuneCountInString(value) < 1 || utf8.RuneCountInString(value) > 128 || strings.ContainsFunc(value, unicode.IsControl) {
					return Configuration{}, ErrInvalid
				}
			}
			cfg.credential, err = json.Marshal(struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}{input.Credentials.Username, input.Credentials.Password})
		}
	case domain.VideoGB28181:
		if input.URI != "" || input.Credentials != nil {
			return Configuration{}, ErrInvalid
		}
		cfg.source, err = domain.NewGBVideoSource(input.DeviceID, input.ChannelID)
	default:
		return Configuration{}, ErrInvalid
	}
	if err != nil {
		return Configuration{}, ErrInvalid
	}
	return cfg, nil
}
func (c Configuration) PondID() int64              { return c.pondID }
func (c Configuration) Name() string               { return c.name }
func (c Configuration) Source() domain.VideoSource { return c.source }
func (Configuration) String() string               { return "camera configuration [redacted]" }
func (Configuration) GoString() string             { return "camera configuration [redacted]" }
func (Configuration) MarshalJSON() ([]byte, error) { return nil, domain.ErrVideoSecretSerialization }
