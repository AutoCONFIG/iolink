package platform

import (
	"strings"
	"testing"
	"time"
)

func validEnv() map[string]string {
	return map[string]string{"IOLINK_PG_DSN": "postgres://user:secret@localhost:5432/test", "IOLINK_SECRET_KEY": "test-only-key-at-least-32-bytes-long"}
}
func TestConfig(t *testing.T) {
	env := validEnv()
	cfg, err := LoadConfig(func(k string) string { return env[k] }, true)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ReportInterval != time.Minute || cfg.OfflineGrace != 3 {
		t.Fatal("bad defaults")
	}
	env["IOLINK_REPORT_INTERVAL"] = "300"
	env["IOLINK_OFFLINE_GRACE"] = "4"
	cfg, err = LoadConfig(func(k string) string { return env[k] }, true)
	if err != nil || cfg.ReportInterval != 5*time.Minute || cfg.OfflineGrace != 4 {
		t.Fatal(cfg, err)
	}
}
func TestConfigRejectsInvalidWithoutLeakingValues(t *testing.T) {
	for _, tc := range []struct{ k, v string }{{"IOLINK_SECRET_KEY", ""}, {"IOLINK_SECRET_KEY", strings.Repeat("x", 40)}, {"IOLINK_PG_DSN", "postgres://user:very-secret@[bad/test"}, {"IOLINK_HTTP_ADDR", ":0"}, {"IOLINK_REPORT_INTERVAL", "5"}, {"IOLINK_OFFLINE_GRACE", "0"}, {"IOLINK_WX_APPID", "appid"}, {"IOLINK_WX_TEMPLATE_ID", "template"}} {
		t.Run(tc.k+tc.v, func(t *testing.T) {
			env := validEnv()
			env[tc.k] = tc.v
			_, err := LoadConfig(func(k string) string { return env[k] }, true)
			if err == nil {
				t.Fatal("bad config accepted")
			}
			if strings.Contains(err.Error(), "very-secret") {
				t.Fatal("credential leaked")
			}
		})
	}
}
func TestAdminCommandsDoNotRequireServingKey(t *testing.T) {
	env := validEnv()
	delete(env, "IOLINK_SECRET_KEY")
	if _, err := LoadConfig(func(k string) string { return env[k] }, false); err != nil {
		t.Fatal(err)
	}
}
func TestPasswordHash(t *testing.T) {
	a, b := HashPassword("strong-password"), HashPassword("strong-password")
	if a == b {
		t.Fatal("password hashes must have random salts")
	}
	if !CheckPassword(a, "strong-password") || CheckPassword(a, "wrong") {
		t.Fatal("password verification failed")
	}
	if !CheckPassword("1cd663ce3300b9f52a357c4ae4e114064b0fa066071728aca1d7a98f5916f2e0", "admin123") {
		t.Fatal("legacy upgrade verifier failed")
	}
	for _, bad := range []string{"", "$argon2id$v=19$m=999999999,t=3,p=2$x$y", strings.Repeat("0", 63)} {
		if CheckPassword(bad, "x") {
			t.Fatal("corrupt hash accepted")
		}
	}
}
