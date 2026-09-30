package wechat

import (
	"context"
	"errors"
	"git.hyhy.fun/rsplab/iolink/internal/notifications"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func testDelivery() notifications.Delivery {
	return notifications.Delivery{AlarmID: 7, RecipientID: "owner-open-id", DeviceNo: "device-1", PondID: 3, Metric: "temperature", CurrentValue: 31.25, Threshold: 30, Level: "warning", Message: "temperature high", CreatedAt: time.Date(2026, 9, 27, 8, 30, 0, 0, time.UTC)}
}

func TestWeChatSenderDisabledWithoutCredentials(t *testing.T) {
	s := NewSender(Config{})
	if !errors.Is(s.Send(context.Background(), testDelivery()), notifications.ErrDisabled) {
		t.Fatal("missing credentials must disable delivery")
	}
}

func TestWeChatSenderRejectsHTTPAndMalformedResponses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{name: "http 500", status: http.StatusInternalServerError, body: "error"},
		{name: "http 400", status: http.StatusBadRequest, body: "error", want: notifications.ErrPermanent},
		{name: "malformed token", status: http.StatusOK, body: "{", want: notifications.ErrRetryable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSender(Config{AppID: "app", Secret: "secret", TemplateID: "template", TokenURL: "https://token.test", SendURL: "https://send.test", HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return response(tc.status, tc.body), nil
			})}})
			err := s.Send(context.Background(), testDelivery())
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("error=%v want %v", err, tc.want)
			}
			if tc.want == nil && err == nil {
				t.Fatal("HTTP 500 must not be reported as success")
			}
		})
	}
}

func TestWeChatSenderRefreshesExpiredTokenOnce(t *testing.T) {
	var tokenCalls, sendCalls atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "token.test" {
			n := tokenCalls.Add(1)
			return response(http.StatusOK, `{"access_token":"token-`+string(rune('0'+n))+`","expires_in":3600}`), nil
		}
		sendCalls.Add(1)
		if r.URL.Query().Get("access_token") == "token-1" {
			return response(http.StatusOK, `{"errcode":40001,"errmsg":"expired"}`), nil
		}
		return response(http.StatusOK, `{"errcode":0,"errmsg":"ok"}`), nil
	})}
	s := NewSender(Config{AppID: "app", Secret: "secret", TemplateID: "template", TokenURL: "https://token.test", SendURL: "https://send.test", HTTP: client})
	if err := s.Send(context.Background(), testDelivery()); err != nil {
		t.Fatal(err)
	}
	if tokenCalls.Load() != 2 || sendCalls.Load() != 2 {
		t.Fatalf("token calls=%d send calls=%d", tokenCalls.Load(), sendCalls.Load())
	}
}

func TestWeChatSenderNonzeroProviderCodeFails(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "token.test" {
			return response(http.StatusOK, `{"access_token":"token","expires_in":3600}`), nil
		}
		return response(http.StatusOK, `{"errcode":40003,"errmsg":"bad openid"}`), nil
	})}
	s := NewSender(Config{AppID: "app", Secret: "secret", TemplateID: "template", TokenURL: "https://token.test", SendURL: "https://send.test", HTTP: client})
	if err := s.Send(context.Background(), testDelivery()); !errors.Is(err, notifications.ErrPermanent) {
		t.Fatalf("error=%v want permanent", err)
	}
}

func TestWeChatSenderMissingSendCodeIsUnknown(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "token.test" {
			return response(http.StatusOK, `{"access_token":"token","expires_in":3600}`), nil
		}
		return response(http.StatusOK, `{}`), nil
	})}
	s := NewSender(Config{AppID: "app", Secret: "secret", TemplateID: "template", TokenURL: "https://token.test", SendURL: "https://send.test", HTTP: client})
	if err := s.Send(context.Background(), testDelivery()); !errors.Is(err, notifications.ErrUnknown) {
		t.Fatalf("error=%v want unknown", err)
	}
}

func TestWeChatSenderTokenTransportIsRetryable(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("temporary network failure")
	})}
	s := NewSender(Config{AppID: "app", Secret: "secret", TemplateID: "template", TokenURL: "https://token.test", SendURL: "https://send.test", HTTP: client})
	if err := s.Send(context.Background(), testDelivery()); !errors.Is(err, notifications.ErrRetryable) {
		t.Fatalf("error=%v want retryable", err)
	}
}

func TestWeChatSenderUsesConfiguredTemplateFields(t *testing.T) {
	var body string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "token.test" {
			return response(http.StatusOK, `{"access_token":"token","expires_in":3600}`), nil
		}
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		return response(http.StatusOK, `{"errcode":0}`), nil
	})}
	s := NewSender(Config{AppID: "app", Secret: "secret", TemplateID: "template", MessageField: "msg", ValueField: "val", TimeField: "when", TokenURL: "https://token.test", SendURL: "https://send.test", HTTP: client})
	if err := s.Send(context.Background(), testDelivery()); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"msg"`, `"val"`, `"when"`} {
		if !strings.Contains(body, key) {
			t.Fatalf("payload=%s missing %s", body, key)
		}
	}
}

func TestWeChatSenderRejectsDuplicateTemplateFields(t *testing.T) {
	s := NewSender(Config{AppID: "app", Secret: "secret", TemplateID: "template", MessageField: "same", ValueField: "same", TimeField: "when"})
	if err := s.Send(context.Background(), testDelivery()); !errors.Is(err, notifications.ErrPermanent) {
		t.Fatalf("error=%v want permanent", err)
	}
}
