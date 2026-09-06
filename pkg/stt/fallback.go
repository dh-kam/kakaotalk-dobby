package stt

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// OrchestratorConfig configures the hybrid STT service.
type OrchestratorConfig struct {
	Groq          Transcriber
	Local         Transcriber
	Quota         *QuotaTracker
	AllowFallback bool // If true, falls back to local when Groq quota is exceeded or fails
}

// Orchestrator coordinates primary cloud STT (Groq) and local fallback (Whisper Go).
type Orchestrator struct {
	groq          Transcriber
	local         Transcriber
	quota         *QuotaTracker
	mu            sync.RWMutex
	allowFallback bool
}

// NewOrchestrator creates a new STT orchestrator.
func NewOrchestrator(cfg OrchestratorConfig) *Orchestrator {
	quota := cfg.Quota
	if quota == nil {
		quota = NewQuotaTracker(DefaultFreeQuotaConfig())
	}

	return &Orchestrator{
		groq:          cfg.Groq,
		local:         cfg.Local,
		quota:         quota,
		allowFallback: cfg.AllowFallback,
	}
}

func (o *Orchestrator) Name() string {
	return "orchestrator"
}

// SetFallbackEnabled toggles whether local Whisper should be used when Groq is throttled or errors.
func (o *Orchestrator) SetFallbackEnabled(enabled bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.allowFallback = enabled
}

// IsFallbackEnabled returns whether fallback is currently active.
func (o *Orchestrator) IsFallbackEnabled() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.allowFallback
}

// GetQuotaStats returns the current quota usage statistics.
func (o *Orchestrator) GetQuotaStats() QuotaStats {
	return o.quota.GetStats()
}

// estimateDurationSeconds estimates audio length from file size and extension.
func estimateDurationSeconds(path string, sizeBytes int64) int {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".wav":
		// 16kHz 16-bit mono is 32,000 bytes/sec
		secs := int(sizeBytes / 32000)
		if secs < 1 {
			return 1
		}
		return secs
	case ".mp3":
		// 48kbps is ~6,000 bytes/sec, 128kbps is ~16,000 bytes/sec
		secs := int(sizeBytes / 8000)
		if secs < 1 {
			return 1
		}
		return secs
	default:
		// Default safe estimation: 15 seconds
		return 15
	}
}

func (o *Orchestrator) Transcribe(ctx context.Context, audioPath string, opts TranscribeOptions) (*TranscribeResult, error) {
	fileInfo, err := os.Stat(audioPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrAudioFileNotFound
		}
		return nil, err
	}
	fileSize := fileInfo.Size()
	estSeconds := estimateDurationSeconds(audioPath, fileSize)

	o.mu.RLock()
	canFallback := o.allowFallback && o.local != nil
	o.mu.RUnlock()

	// 1. If Groq is not configured or nil
	if o.groq == nil {
		if canFallback {
			res, err := o.local.Transcribe(ctx, audioPath, opts)
			if err != nil {
				return nil, fmt.Errorf("local whisper error: %w", err)
			}
			res.Fallback = true
			res.Reason = "Groq cloud provider not configured"
			return res, nil
		}
		return nil, ErrInvalidAPIKey
	}

	// 2. Check Quota policy (RPM, daily seconds, 429 backoff)
	canUseGroq, quotaReason := o.quota.CanTranscribe(fileSize, estSeconds)
	if !canUseGroq {
		if canFallback {
			res, err := o.local.Transcribe(ctx, audioPath, opts)
			if err != nil {
				return nil, fmt.Errorf("quota exceeded (%s) and local fallback failed: %w", quotaReason, err)
			}
			res.Fallback = true
			res.Reason = fmt.Sprintf("Fallback to local Whisper (Groq quota policy: %s)", quotaReason)
			return res, nil
		}
		return nil, fmt.Errorf("%w: %s", ErrFallbackDisabled, quotaReason)
	}

	// 3. Attempt primary Groq transcription
	res, groqErr := o.groq.Transcribe(ctx, audioPath, opts)
	if groqErr == nil {
		o.quota.RecordUsage(true, 200, estSeconds, 0)
		return res, nil
	}

	// 4. Handle Groq Error
	var apiErr *APIError
	statusCode := 500
	var retryAfter time.Duration
	if errors.As(groqErr, &apiErr) {
		statusCode = apiErr.StatusCode
		retryAfter = apiErr.RetryAfter
	}
	o.quota.RecordUsage(false, statusCode, estSeconds, retryAfter)

	// If fallback is enabled, delegate to local Whisper on 429, 5xx, or network timeout
	if canFallback {
		fallbackRes, fallbackErr := o.local.Transcribe(ctx, audioPath, opts)
		if fallbackErr == nil {
			fallbackRes.Fallback = true
			fallbackRes.Reason = fmt.Sprintf("Groq failed (%v); fell back to local Whisper", groqErr)
			return fallbackRes, nil
		}
		return nil, fmt.Errorf("groq failed (%w) and local fallback also failed (%v)", groqErr, fallbackErr)
	}

	return nil, groqErr
}
