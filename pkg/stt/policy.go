package stt

import (
	"fmt"
	"sync"
	"time"
)

// QuotaConfig defines the rate and volume limits for a cloud STT provider.
type QuotaConfig struct {
	// MaxRequestsPerMinute is the maximum number of requests allowed in a rolling 1-minute window.
	// Default on Groq Free Tier: 25 RPM.
	MaxRequestsPerMinute int

	// MaxRequestsPerDay is the maximum number of requests allowed in a 24-hour day window.
	// Default on Groq Free Tier: 2000 RPD.
	MaxRequestsPerDay int

	// MaxAudioSecondsPerDay is the maximum audio length (in seconds) allowed per day.
	// Default on Groq Free Tier: 7200 seconds (~2 hours).
	MaxAudioSecondsPerDay int

	// MaxFileSizeBytes is the maximum audio file size allowed per single request.
	// Default on Groq Cloud: 25 MB.
	MaxFileSizeBytes int64

	// NowFunc provides current time, useful for mock clocks in tests.
	NowFunc func() time.Time
}

// DefaultFreeQuotaConfig returns the standard limits for Groq Free Tier.
func DefaultFreeQuotaConfig() QuotaConfig {
	return QuotaConfig{
		MaxRequestsPerMinute:  25,
		MaxRequestsPerDay:     2000,
		MaxAudioSecondsPerDay: 7200,
		MaxFileSizeBytes:      25 * 1024 * 1024,
		NowFunc:               time.Now,
	}
}

// QuotaStats reports the current usage metrics.
type QuotaStats struct {
	RequestsLastMinute int       `json:"requests_last_minute"`
	MaxRPM             int       `json:"max_rpm"`
	DailyRequests      int       `json:"daily_requests"`
	MaxRPD             int       `json:"max_rpd"`
	DailyAudioSeconds  int       `json:"daily_audio_seconds"`
	MaxDailySeconds    int       `json:"max_daily_seconds"`
	BackoffActive      bool      `json:"backoff_active"`
	BackoffUntil       time.Time `json:"backoff_until,omitempty"`
}

// QuotaTracker tracks rate limits and volume quotas in a thread-safe manner.
type QuotaTracker struct {
	mu           sync.Mutex
	cfg          QuotaConfig
	minuteReqs   []time.Time
	dailyReqs    int
	dailySeconds int
	dayStart     time.Time
	backoffUntil time.Time
	nowFunc      func() time.Time
}

// NewQuotaTracker creates a new tracker with the specified config.
func NewQuotaTracker(cfg QuotaConfig) *QuotaTracker {
	if cfg.MaxRequestsPerMinute <= 0 {
		cfg.MaxRequestsPerMinute = 25
	}
	if cfg.MaxRequestsPerDay <= 0 {
		cfg.MaxRequestsPerDay = 2000
	}
	if cfg.MaxAudioSecondsPerDay <= 0 {
		cfg.MaxAudioSecondsPerDay = 7200
	}
	if cfg.MaxFileSizeBytes <= 0 {
		cfg.MaxFileSizeBytes = 25 * 1024 * 1024
	}
	nowFn := cfg.NowFunc
	if nowFn == nil {
		nowFn = time.Now
	}

	return &QuotaTracker{
		cfg:      cfg,
		dayStart: nowFn(),
		nowFunc:  nowFn,
	}
}

// CanTranscribe checks whether a request with the given file size and estimated audio length
// is allowed under the current quota policies.
func (q *QuotaTracker) CanTranscribe(fileSizeBytes int64, estimatedAudioSeconds int) (bool, string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := q.nowFunc()

	// 1. File size limit
	if fileSizeBytes > q.cfg.MaxFileSizeBytes {
		return false, fmt.Sprintf("file size (%d bytes) exceeds maximum allowed (%d bytes / 25MB)", fileSizeBytes, q.cfg.MaxFileSizeBytes)
	}

	// 2. Active 429 RateLimit backoff
	if now.Before(q.backoffUntil) {
		remaining := q.backoffUntil.Sub(now).Round(time.Second)
		return false, fmt.Sprintf("provider is in rate-limit backoff for another %v", remaining)
	}

	// 3. Sliding 1-minute window check
	cutoff := now.Add(-1 * time.Minute)
	valid := q.minuteReqs[:0]
	for _, t := range q.minuteReqs {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	q.minuteReqs = valid

	if len(q.minuteReqs) >= q.cfg.MaxRequestsPerMinute {
		return false, fmt.Sprintf("RPM limit reached (%d/%d requests in the last minute)", len(q.minuteReqs), q.cfg.MaxRequestsPerMinute)
	}

	// 4. Daily window reset
	if now.Sub(q.dayStart) >= 24*time.Hour {
		q.dayStart = now
		q.dailyReqs = 0
		q.dailySeconds = 0
	}

	// 5. Daily requests check
	if q.dailyReqs >= q.cfg.MaxRequestsPerDay {
		return false, fmt.Sprintf("daily request limit reached (%d/%d requests today)", q.dailyReqs, q.cfg.MaxRequestsPerDay)
	}

	// 6. Daily audio seconds check
	if q.dailySeconds+estimatedAudioSeconds > q.cfg.MaxAudioSecondsPerDay {
		return false, fmt.Sprintf("daily audio seconds quota reached (%d + %d > %d seconds today)", q.dailySeconds, estimatedAudioSeconds, q.cfg.MaxAudioSecondsPerDay)
	}

	return true, ""
}

// RecordUsage updates the tracker state after a transcription attempt.
func (q *QuotaTracker) RecordUsage(success bool, statusCode int, audioSeconds int, backoffDuration time.Duration) {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := q.nowFunc()

	if statusCode == 429 {
		// Set backoff
		if backoffDuration <= 0 {
			backoffDuration = 30 * time.Second
		}
		q.backoffUntil = now.Add(backoffDuration)
		return
	}

	if success {
		q.minuteReqs = append(q.minuteReqs, now)
		q.dailyReqs++
		if audioSeconds > 0 {
			q.dailySeconds += audioSeconds
		}
	}
}

// GetStats returns current snapshot of quota utilization.
func (q *QuotaTracker) GetStats() QuotaStats {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := q.nowFunc()
	cutoff := now.Add(-1 * time.Minute)
	validCount := 0
	for _, t := range q.minuteReqs {
		if t.After(cutoff) {
			validCount++
		}
	}

	backoffActive := now.Before(q.backoffUntil)

	return QuotaStats{
		RequestsLastMinute: validCount,
		MaxRPM:             q.cfg.MaxRequestsPerMinute,
		DailyRequests:      q.dailyReqs,
		MaxRPD:             q.cfg.MaxRequestsPerDay,
		DailyAudioSeconds:  q.dailySeconds,
		MaxDailySeconds:    q.cfg.MaxAudioSecondsPerDay,
		BackoffActive:      backoffActive,
		BackoffUntil:       q.backoffUntil,
	}
}
