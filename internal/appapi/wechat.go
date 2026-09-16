package appapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	iolinkcontractsdomain "git.hyhy.fun/rsplab/iolink/internal/domain"
)

// WechatConfig holds mini-program credentials. Empty AppID keeps the stub
// exchanger (dev); set both to enable real code2session.
type WechatConfig struct {
	AppID  string
	Secret string
}

// wechatSession is the code2session response.
type wechatSession struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

// RealWechatExchanger returns an exchanger calling the official
// jscode2session endpoint. Only wired when WechatConfig is complete.
func RealWechatExchanger(cfg WechatConfig) func(string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	return func(code string) (string, error) {
		q := url.Values{
			"appid":      {cfg.AppID},
			"secret":     {cfg.Secret},
			"js_code":    {code},
			"grant_type": {"authorization_code"},
		}
		resp, err := client.Get("https://api.weixin.qq.com/sns/jscode2session?" + q.Encode())
		if err != nil {
			return "", fmt.Errorf("wechat http: %w", err)
		}
		defer resp.Body.Close()
		var ws wechatSession
		if err := json.NewDecoder(resp.Body).Decode(&ws); err != nil {
			return "", fmt.Errorf("wechat decode: %w", err)
		}
		if ws.ErrCode != 0 || ws.OpenID == "" {
			return "", fmt.Errorf("wechat code2session: code=%d msg=%s", ws.ErrCode, ws.ErrMsg)
		}
		return ws.OpenID, nil
	}
}

// SetWechatExchanger overrides the code->openid exchange (used by tests;
// production config does this implicitly in New).
func (s *Server) SetWechatExchanger(f func(code string) (openID string, err error)) {
	s.wx = f
}

// usersRepo implements UserStore over the users table (wired by main via
// core's pool). Living here keeps the auth concern inside appapi; it only
// gets the pool handle, never other modules.
type usersRepo struct{ q Queryer }

// Queryer is the minimal DB surface appapi needs (pgx pool satisfies it).
type Queryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) Row
}

// Row mirrors pgx.Row so appapi does not import pgx directly.
type Row interface {
	Scan(dest ...any) error
}

func (r *usersRepo) FindByOpenID(ctx context.Context, openID string) (*iolinkcontractsdomain.User, error) {
	var u iolinkcontractsdomain.User
	err := r.q.QueryRow(ctx,
		`SELECT id, open_id, coalesce(nickname,'') FROM users WHERE open_id=$1`, openID).
		Scan(&u.ID, &u.OpenID, &u.Nickname)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *usersRepo) EnsureUser(ctx context.Context, openID string) (*iolinkcontractsdomain.User, error) {
	const q = `INSERT INTO users (open_id) VALUES ($1)
		ON CONFLICT (open_id) DO UPDATE SET open_id = EXCLUDED.open_id
		RETURNING id, open_id, coalesce(nickname,'')`
	var u iolinkcontractsdomain.User
	err := r.q.QueryRow(ctx, q, openID).Scan(&u.ID, &u.OpenID, &u.Nickname)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
