package mediajobs

import (
	"sync"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/media"
)

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Set(now time.Time) {
	c.mu.Lock()
	c.now = now
	c.mu.Unlock()
}

func sampleInput(now time.Time) EnqueueInput {
	return EnqueueInput{
		Job:         media.Job{ID: "job-1", Kind: media.JobStart, CameraID: 9, SourceVersion: 7, StreamID: "stream-7", SessionID: "session-1"},
		AvailableAt: now,
		Authorization: media.JobAuthorization{
			Active:        true,
			SourceCurrent: true,
			Target: media.PlaybackTarget{
				TenantID:      21,
				CameraID:      9,
				SourceVersion: 7,
				StreamID:      "stream-7",
			},
		},
	}
}
