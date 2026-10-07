package platform

import "testing"

func TestOpenAPIRateConfig_defaultsAndOverrides(t *testing.T) {
	env := validEnv()
	cfg, err := LoadConfig(func(key string) string { return env[key] }, true)
	if err != nil || cfg.OpenAPIRatePerMinute != 60 || cfg.OpenAPIBurst != 10 {
		t.Fatalf("unexpected defaults: %v", err)
	}
	env["IOLINK_OPENAPI_RATE_PER_MINUTE"], env["IOLINK_OPENAPI_BURST"] = "30", "2"
	cfg, err = LoadConfig(func(key string) string { return env[key] }, true)
	if err != nil || cfg.OpenAPIRatePerMinute != 30 || cfg.OpenAPIBurst != 2 {
		t.Fatalf("overrides not loaded: %v", err)
	}
}

func TestOpenAPIRateConfig_rejectsInvalid(t *testing.T) {
	for _, key := range []string{"IOLINK_OPENAPI_RATE_PER_MINUTE", "IOLINK_OPENAPI_BURST"} {
		for _, value := range []string{"0", "-1", "1000001", "1.5", "invalid"} {
			env := validEnv()
			env[key] = value
			if _, err := LoadConfig(func(key string) string { return env[key] }, true); err == nil {
				t.Fatal("invalid open API rate configuration accepted")
			}
		}
	}
}
