package mediajobs

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/media"
)

type JobStore interface {
	ClaimJob(context.Context, time.Time, time.Duration, string) (media.Lease, bool, error)
	RevalidateJob(context.Context, media.Lease) (media.JobAuthorization, error)
	FinishJob(context.Context, media.Lease, media.JobResult, time.Time) error
}

type EnqueueInput struct {
	Job           media.Job
	AvailableAt   time.Time
	Authorization media.JobAuthorization
}

type Snapshot struct {
	Job           media.Job
	Status        string
	AvailableAt   time.Time
	LeasedUntil   time.Time
	LeaseOwner    string
	Result        media.JobResult
	Authorization media.JobAuthorization
}

type entry struct {
	job           media.Job
	status        string
	availableAt   time.Time
	leasedUntil   time.Time
	leaseOwner    string
	leaseToken    string
	result        media.JobResult
	authorization media.JobAuthorization
}

type MemoryStore struct {
	mu    sync.Mutex
	clock func() time.Time
	jobs  map[string]*entry
}

var _ JobStore = (*MemoryStore)(nil)

func NewMemoryStore(clock func() time.Time) *MemoryStore {
	if clock == nil {
		clock = time.Now
	}
	return &MemoryStore{clock: clock, jobs: make(map[string]*entry)}
}

func (s *MemoryStore) Enqueue(ctx context.Context, in EnqueueInput) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if s == nil || in.Job.ID == "" || in.Job.Kind == "" || in.Job.CameraID <= 0 || in.Job.SourceVersion <= 0 || in.Job.StreamID == "" || in.Job.Attempts < 0 || in.Job.Attempts >= MaxAttempts {
		return ErrInvalidJob
	}
	when := in.AvailableAt
	if when.IsZero() {
		when = s.clock().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.jobs[in.Job.ID]; exists {
		return media.ErrConflict
	}
	s.jobs[in.Job.ID] = &entry{job: in.Job, status: "scheduled", availableAt: when.UTC(), authorization: in.Authorization}
	return nil
}

func (s *MemoryStore) SetAuthorization(ctx context.Context, id string, auth media.JobAuthorization) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if s == nil || id == "" {
		return ErrInvalidJob
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return ErrNotFound
	}
	j.authorization = auth
	return nil
}

func (s *MemoryStore) ClaimJob(ctx context.Context, now time.Time, lease time.Duration, workerID string) (media.Lease, bool, error) {
	if err := contextError(ctx); err != nil {
		return media.Lease{}, false, err
	}
	if s == nil || now.IsZero() || lease <= 0 || lease > maxLease || strings.TrimSpace(workerID) == "" || len(strings.TrimSpace(workerID)) > maxWorkerID {
		return media.Lease{}, false, ErrInvalidJob
	}
	now = s.clock().UTC()
	if now.IsZero() {
		return media.Lease{}, false, ErrInvalidJob
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var selected *entry
	ids := make([]string, 0, len(s.jobs))
	for id := range s.jobs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := s.jobs[ids[i]], s.jobs[ids[j]]
		if !a.availableAt.Equal(b.availableAt) {
			return a.availableAt.Before(b.availableAt)
		}
		return ids[i] < ids[j]
	})
	for _, id := range ids {
		j := s.jobs[id]
		if j.job.Attempts >= MaxAttempts {
			continue
		}
		eligible := (j.status == "scheduled" || j.status == "retryable") && !j.availableAt.After(now)
		if j.status == "running" && (j.leasedUntil.IsZero() || !j.leasedUntil.After(now)) {
			eligible = true
		}
		if eligible {
			selected = j
			break
		}
	}
	if selected == nil {
		return media.Lease{}, false, nil
	}
	token, err := leaseToken()
	if err != nil {
		return media.Lease{}, false, err
	}
	selected.job.Attempts++
	selected.status = "running"
	selected.leaseOwner = workerID
	selected.leaseToken = token
	selected.leasedUntil = now.Add(lease)
	return media.Lease{Job: selected.job, LeaseToken: token, LeasedUntil: selected.leasedUntil}, true, nil
}

func (s *MemoryStore) RevalidateJob(ctx context.Context, lease media.Lease) (media.JobAuthorization, error) {
	if err := contextError(ctx); err != nil {
		return media.JobAuthorization{}, err
	}
	if s == nil || !validLease(lease) {
		return media.JobAuthorization{}, media.ErrLeaseLost
	}
	now := s.clock().UTC()
	if now.IsZero() {
		return media.JobAuthorization{}, media.ErrLeaseLost
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[lease.ID]
	if !ok || !leaseMatches(j, lease) || j.status != "running" || !j.leasedUntil.After(now) {
		return media.JobAuthorization{}, media.ErrLeaseLost
	}
	auth := j.authorization
	auth.SourceCurrent = auth.SourceCurrent && auth.Target.SourceVersion == lease.SourceVersion && auth.Target.CameraID == lease.CameraID && (auth.Target.StreamID == "" || auth.Target.StreamID == lease.StreamID)
	return auth, nil
}

func (s *MemoryStore) FinishJob(ctx context.Context, lease media.Lease, result media.JobResult, now time.Time) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if s == nil || !validLease(lease) || now.IsZero() || !validResult(result) {
		return ErrInvalidJob
	}
	now = now.UTC()
	clockNow := s.clock().UTC()
	if clockNow.IsZero() {
		return media.ErrLeaseLost
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[lease.ID]
	if !ok || !leaseMatches(j, lease) || j.status != "running" || !j.leasedUntil.After(now) || !j.leasedUntil.After(clockNow) {
		return media.ErrLeaseLost
	}
	if result.Status != media.JobSkipped && ((j.authorization.Target.SourceVersion != 0 && j.authorization.Target.SourceVersion != lease.SourceVersion) || (j.authorization.Target.CameraID != 0 && j.authorization.Target.CameraID != lease.CameraID) || (j.authorization.Target.StreamID != "" && j.authorization.Target.StreamID != lease.StreamID)) {
		return media.ErrLeaseLost
	}
	j.result = result
	j.status = resultState(result.Status)
	j.availableAt = result.NextAt.UTC()
	j.leasedUntil = time.Time{}
	j.leaseOwner = ""
	j.leaseToken = ""
	return nil
}

func (s *MemoryStore) Snapshot(ctx context.Context, id string) (Snapshot, bool, error) {
	if err := contextError(ctx); err != nil {
		return Snapshot{}, false, err
	}
	if s == nil {
		return Snapshot{}, false, ErrInvalidJob
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return Snapshot{}, false, nil
	}
	return Snapshot{Job: j.job, Status: j.status, AvailableAt: j.availableAt, LeasedUntil: j.leasedUntil, LeaseOwner: j.leaseOwner, Result: j.result, Authorization: j.authorization}, true, nil
}
