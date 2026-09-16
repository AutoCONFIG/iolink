package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

// AlarmNotifier delivers an alarm outside the alarm center (e.g. WeChat
// subscribe message). Implementations must not block; SendAlarm is invoked
// on its own goroutine with pre-resolved recipients.
type AlarmNotifier interface {
	SendAlarm(a domain.Alarm, openIDs []string)
}

// SetNotifier wires an alarm notifier (nil = alarm center only).
func (s *Service) SetNotifier(n AlarmNotifier) {
	if n == nil {
		s.al.notifier = nil
		return
	}
	s.al.notifier = func(a domain.Alarm) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		openIDs, err := s.wechatUsersForPond(ctx, a.PondID)
		if err != nil {
			s.log.Warn("notify: resolve users", "err", err)
			return
		}
		if len(openIDs) == 0 {
			s.log.Info("notify: no wechat users bound to pond", "pond_id", a.PondID)
			return
		}
		go n.SendAlarm(a, openIDs)
	}
}

func (s *Service) wechatUsersForPond(ctx context.Context, pondID int64) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.open_id FROM users u
		JOIN farms f ON f.owner_id = u.id
		JOIN ponds p ON p.farm_id = f.id
		WHERE p.id = $1 AND u.open_id <> ''`, pondID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// WeChatNotifier sends WeChat subscribe messages for alarms. Unconfigured
// fields make it a no-op logger (dev environments).
type WeChatNotifier struct {
	AppID      string
	Secret     string
	TemplateID string
	Page       string // mini program landing page, e.g. "pages/alarms/index"

	log  *slog.Logger
	http *http.Client

	mu    sync.Mutex
	token string
	exp   time.Time
}

func NewWeChatNotifier(appID, secret, templateID, page string, log *slog.Logger) *WeChatNotifier {
	return &WeChatNotifier{
		AppID: appID, Secret: secret, TemplateID: templateID, Page: page,
		log: log, http: &http.Client{Timeout: 5 * time.Second},
	}
}

// SendAlarm implements AlarmNotifier. Best-effort: errors are logged.
func (n *WeChatNotifier) SendAlarm(a domain.Alarm, openIDs []string) {
	if len(openIDs) == 0 {
		return
	}
	if n.AppID == "" || n.Secret == "" || n.TemplateID == "" {
		n.log.Warn("wx notifier unconfigured, skip", "alarm_id", a.ID)
		return
	}
	tok, err := n.accessToken()
	if err != nil {
		n.log.Warn("wx access token", "err", err)
		return
	}
	page := n.Page
	if page == "" {
		page = "pages/alarms/index"
	}
	for _, openID := range openIDs {
		body := map[string]any{
			"touser":      openID,
			"template_id": n.TemplateID,
			"page":        page,
			"data": map[string]any{
				"thing1":  map[string]string{"value": truncate(a.Message, 20)},
				"number2": map[string]string{"value": fmt.Sprintf("%.2f", a.CurrentValue)},
				"time3":   map[string]string{"value": a.CreatedAt.Format("2006-01-02 15:04")},
			},
		}
		raw, _ := json.Marshal(body)
		url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/message/subscribe/send?access_token=%s", tok)
		resp, err := n.http.Post(url, "application/json", bytes.NewReader(raw))
		if err != nil {
			n.log.Warn("wx send", "err", err)
			continue
		}
		var r struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&r)
		resp.Body.Close()
		if r.ErrCode != 0 {
			n.log.Warn("wx send rejected", "errcode", r.ErrCode, "errmsg", r.ErrMsg)
		} else {
			MetricNotificationsSent.Inc()
			n.log.Info("wx alarm sent", "alarm_id", a.ID, "to", openID)
		}
	}
}

func (n *WeChatNotifier) accessToken() (string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.token != "" && time.Now().Before(n.exp) {
		return n.token, nil
	}
	url := fmt.Sprintf("https://api.weixin.qq.com/cgi/token?grant_type=client_credential&appid=%s&secret=%s", n.AppID, n.Secret)
	resp, err := n.http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var r struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}
	if r.AccessToken == "" {
		return "", fmt.Errorf("wx token: %s", r.ErrMsg)
	}
	n.token = r.AccessToken
	n.exp = time.Now().Add(time.Duration(r.ExpiresIn-300) * time.Second)
	return n.token, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
