package wechat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/notifications"
)

type Config struct {
	AppID        string
	Secret       string
	TemplateID   string
	Page         string
	MessageField string
	ValueField   string
	TimeField    string
	HTTP         *http.Client
	TokenURL     string
	SendURL      string
	Now          func() time.Time
}

type Sender struct {
	appID, secret, templateID, page     string
	messageField, valueField, timeField string
	invalidMapping                      bool
	http                                *http.Client
	tokenURL, sendURL                   string
	now                                 func() time.Time
	mu                                  sync.Mutex
	token                               string
	expires                             time.Time
}

func NewSender(cfg Config) *Sender {
	if cfg.HTTP == nil {
		cfg.HTTP = &http.Client{Timeout: 5 * time.Second}
	}
	if cfg.TokenURL == "" {
		cfg.TokenURL = "https://api.weixin.qq.com/cgi-bin/token"
	}
	if cfg.SendURL == "" {
		cfg.SendURL = "https://api.weixin.qq.com/cgi-bin/message/subscribe/send"
	}
	if cfg.Page == "" {
		cfg.Page = "pages/alarms/index"
	}
	if cfg.MessageField == "" {
		cfg.MessageField = "thing1"
	}
	if cfg.ValueField == "" {
		cfg.ValueField = "number2"
	}
	if cfg.TimeField == "" {
		cfg.TimeField = "time3"
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Sender{appID: cfg.AppID, secret: cfg.Secret, templateID: cfg.TemplateID, page: cfg.Page, messageField: cfg.MessageField, valueField: cfg.ValueField, timeField: cfg.TimeField, invalidMapping: cfg.MessageField == cfg.ValueField || cfg.MessageField == cfg.TimeField || cfg.ValueField == cfg.TimeField, http: cfg.HTTP, tokenURL: cfg.TokenURL, sendURL: cfg.SendURL, now: cfg.Now}
}

func (s *Sender) Send(ctx context.Context, d notifications.Delivery) error {
	if s.appID == "" || s.secret == "" || s.templateID == "" {
		return notifications.ErrDisabled
	}
	if s.invalidMapping {
		return fmt.Errorf("%w: template fields must be distinct", notifications.ErrPermanent)
	}
	tok, err := s.accessToken(ctx)
	if err != nil {
		return err
	}
	err = s.sendWithToken(ctx, tok, d)
	if !errors.Is(err, errTokenExpired) {
		return err
	}
	s.invalidate(tok)
	tok, err = s.accessToken(ctx)
	if err != nil {
		return err
	}
	err = s.sendWithToken(ctx, tok, d)
	if errors.Is(err, errTokenExpired) {
		return fmt.Errorf("%w: token rejected after refresh", notifications.ErrPermanent)
	}
	return err
}

var errTokenExpired = errors.New("wechat access token expired")

func (s *Sender) accessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if s.token != "" && now.Before(s.expires) {
		return s.token, nil
	}
	u := s.tokenURL + "?grant_type=client_credential&appid=" + urlQuery(s.appID) + "&secret=" + urlQuery(s.secret)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: token request failed", notifications.ErrRetryable)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", classifyHTTPStatus(resp.StatusCode)
	}
	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     *int   `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body); err != nil {
		return "", fmt.Errorf("%w: token response: %v", notifications.ErrRetryable, err)
	}
	if body.ErrCode != nil && (*body.ErrCode == -1 || *body.ErrCode == 45009 || *body.ErrCode == 45011) {
		return "", fmt.Errorf("%w: temporary token provider error", notifications.ErrRetryable)
	}
	if body.ErrCode != nil && *body.ErrCode != 0 {
		return "", fmt.Errorf("%w: token provider error", notifications.ErrPermanent)
	}
	if body.AccessToken == "" || body.ExpiresIn <= 0 {
		return "", fmt.Errorf("%w: token response missing fields", notifications.ErrPermanent)
	}
	d := time.Duration(body.ExpiresIn) * time.Second
	if d > 5*time.Minute {
		d -= 5 * time.Minute
	}
	s.token, s.expires = body.AccessToken, now.Add(d)
	return s.token, nil
}

func (s *Sender) invalidate(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token == token {
		s.token, s.expires = "", time.Time{}
	}
}

func (s *Sender) sendWithToken(ctx context.Context, token string, d notifications.Delivery) error {
	payload := map[string]any{
		"touser": d.RecipientID, "template_id": s.templateID, "page": s.page,
		"data": map[string]any{
			s.messageField: map[string]string{"value": truncate(d.Message, 20)},
			s.valueField:   map[string]string{"value": fmt.Sprintf("%.2f", d.CurrentValue)},
			s.timeField:    map[string]string{"value": d.CreatedAt.Format("2006-01-02 15:04")},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%w: encode request: %v", notifications.ErrPermanent, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.sendURL+"?access_token="+urlQuery(token), bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: send transport failed", notifications.ErrUnknown)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return classifyHTTPStatus(resp.StatusCode)
	}
	var body struct {
		ErrCode *int   `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body); err != nil {
		return fmt.Errorf("%w: send response: %v", notifications.ErrUnknown, err)
	}
	if body.ErrCode == nil {
		return fmt.Errorf("%w: missing send errcode", notifications.ErrUnknown)
	}
	if *body.ErrCode == 40001 || *body.ErrCode == 40014 || *body.ErrCode == 42001 {
		return errTokenExpired
	}
	if *body.ErrCode != 0 {
		if *body.ErrCode == 45009 || *body.ErrCode == 45011 || *body.ErrCode == -1 {
			return fmt.Errorf("%w: wechat temporary provider error", notifications.ErrRetryable)
		}
		return fmt.Errorf("%w: wechat provider error", notifications.ErrPermanent)
	}
	return nil
}

func classifyHTTPStatus(status int) error {
	if status >= 500 || status == http.StatusTooManyRequests {
		return fmt.Errorf("provider http %d: retryable", status)
	}
	return fmt.Errorf("%w: provider http %d", notifications.ErrPermanent, status)
}

func urlQuery(s string) string {
	return url.QueryEscape(s)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
