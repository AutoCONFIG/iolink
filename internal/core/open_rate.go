package core

import (
	"errors"
	"math"
	"time"
)

// ConfigureOpenRateLimit is called by composition before serving requests.
func (s *Service) ConfigureOpenRateLimit(perMinute, burst int) error {
	if perMinute < 1 || perMinute > 1_000_000 || burst < 1 || burst > 1_000_000 {
		return errors.New("open api rate and burst must be between 1 and 1000000")
	}
	s.openRatePerMinute, s.openRateBurst = perMinute, burst
	return nil
}

func consumeOpenRate(tokens float64, last, now time.Time, perMinute, burst int) (float64, int) {
	rate := float64(perMinute) / 60
	elapsed := math.Max(0, now.Sub(last).Seconds())
	tokens = math.Min(float64(burst), tokens+elapsed*rate)
	if tokens < 1 {
		return tokens, int(math.Ceil((1 - tokens) / rate))
	}
	return tokens - 1, 0
}
