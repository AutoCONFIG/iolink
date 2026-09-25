package platform

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// LoadConfig validates configuration before opening sockets. Errors identify
// the field, never echo DSNs, passwords, JWT keys or WeChat credentials.
func LoadConfig(getenv func(string) string, serving bool) (Config, error) {
	get := func(k, def string) string {
		if v := getenv(k); v != "" {
			return v
		}
		return def
	}
	c := Config{HTTPAddr: get("IOLINK_HTTP_ADDR", ":8080"), MQTTAddr: get("IOLINK_MQTT_ADDR", ":1883"), PgDSN: getenv("IOLINK_PG_DSN"), PgMaxConns: 20, QueryTimeout: 5 * time.Second, SecretKey: getenv("IOLINK_SECRET_KEY"), WXAppID: getenv("IOLINK_WX_APPID"), WXSecret: getenv("IOLINK_WX_SECRET"), WXTemplateID: getenv("IOLINK_WX_TEMPLATE_ID")}
	// Requiring a URL keeps connection error redaction and deployment behavior unambiguous.
	u, err := url.Parse(c.PgDSN)
	if err != nil || u == nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" || u.Path == "" || u.Path == "/" {
		return c, errors.New("IOLINK_PG_DSN must be a PostgreSQL URL with host and database")
	}
	for _, p := range []struct{ name, addr string }{{"IOLINK_HTTP_ADDR", c.HTTPAddr}, {"IOLINK_MQTT_ADDR", c.MQTTAddr}} {
		_, port, e := net.SplitHostPort(p.addr)
		n, pe := strconv.Atoi(port)
		if e != nil || pe != nil || n < 1 || n > 65535 {
			return c, fmt.Errorf("%s must be host:port (1..65535)", p.name)
		}
	}
	interval, err := strconv.Atoi(get("IOLINK_REPORT_INTERVAL", "60"))
	if err != nil || (interval != 60 && interval != 300) {
		return c, errors.New("IOLINK_REPORT_INTERVAL must be 60 or 300 seconds")
	}
	c.ReportInterval = time.Duration(interval) * time.Second
	grace, err := strconv.Atoi(get("IOLINK_OFFLINE_GRACE", "3"))
	if err != nil || grace < 1 || grace > 10 {
		return c, errors.New("IOLINK_OFFLINE_GRACE must be between 1 and 10")
	}
	c.OfflineGrace = grace
	if serving && (len(c.SecretKey) < 32 || strings.TrimSpace(c.SecretKey) != c.SecretKey || c.SecretKey == strings.Repeat(string(c.SecretKey[0]), len(c.SecretKey))) {
		return c, errors.New("IOLINK_SECRET_KEY must be a non-default random string of at least 32 bytes")
	}
	if (c.WXAppID == "") != (c.WXSecret == "") {
		return c, errors.New("IOLINK_WX_APPID and IOLINK_WX_SECRET must be configured together")
	}
	if c.WXTemplateID != "" && c.WXAppID == "" {
		return c, errors.New("IOLINK_WX_TEMPLATE_ID requires WeChat credentials")
	}
	return c, nil
}
