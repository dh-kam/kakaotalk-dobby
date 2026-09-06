package stt

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuotaTracker_Limits(t *testing.T) {
	currTime := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	nowFn := func() time.Time { return currTime }

	cfg := QuotaConfig{
		MaxRequestsPerMinute:  2,
		MaxRequestsPerDay:     5,
		MaxAudioSecondsPerDay: 100,
		MaxFileSizeBytes:      1000,
		NowFunc:               nowFn,
	}
	tracker := NewQuotaTracker(cfg)

	// 1. File size limit
	can, reason := tracker.CanTranscribe(1001, 10)
	assert.False(t, can)
	assert.Contains(t, reason, "file size")

	// 2. Normal requests within limits
	can, _ = tracker.CanTranscribe(500, 10)
	assert.True(t, can)
	tracker.RecordUsage(true, 200, 10, 0)

	can, _ = tracker.CanTranscribe(500, 10)
	assert.True(t, can)
	tracker.RecordUsage(true, 200, 10, 0)

	// 3. RPM limit hit (2 reqs in same minute)
	can, reason = tracker.CanTranscribe(500, 10)
	assert.False(t, can)
	assert.Contains(t, reason, "RPM limit reached")

	// Advance time by 61 seconds -> RPM window cleared
	currTime = currTime.Add(61 * time.Second)
	can, _ = tracker.CanTranscribe(500, 10)
	assert.True(t, can)

	// 4. Rate limit 429 backoff
	tracker.RecordUsage(false, 429, 10, 30*time.Second)
	can, reason = tracker.CanTranscribe(500, 10)
	assert.False(t, can)
	assert.Contains(t, reason, "rate-limit backoff")

	// Advance past backoff
	currTime = currTime.Add(31 * time.Second)
	can, _ = tracker.CanTranscribe(500, 10)
	assert.True(t, can)

	// 5. Daily audio seconds limit
	tracker.RecordUsage(true, 200, 85, 0) // Total so far: 10 + 10 + 85 = 105 > 100
	can, reason = tracker.CanTranscribe(500, 1)
	assert.False(t, can)
	assert.Contains(t, reason, "daily audio seconds quota reached")

	// Advance past 24 hours -> daily reset
	currTime = currTime.Add(25 * time.Hour)
	can, _ = tracker.CanTranscribe(500, 10)
	assert.True(t, can)
}

func TestGroqTranscriber_MockHTTP(t *testing.T) {
	tempDir := t.TempDir()
	audioPath := filepath.Join(tempDir, "sample.mp3")
	require.NoError(t, os.WriteFile(audioPath, []byte("fake-audio-bytes"), 0644))

	// 1. Success case
	tsSuccess := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "안녕하세요. 음성 인식 테스트입니다.")
	}))
	defer tsSuccess.Close()

	groq := NewGroqTranscriber(GroqConfig{
		APIKey:     "test-key",
		BaseURL:    tsSuccess.URL,
		HTTPClient: tsSuccess.Client(),
	})

	res, err := groq.Transcribe(t.Context(), audioPath, TranscribeOptions{Language: "ko"})
	require.NoError(t, err)
	assert.Equal(t, "안녕하세요. 음성 인식 테스트입니다.", res.Text)
	assert.Equal(t, "groq", res.Provider)
	assert.False(t, res.Fallback)

	// 2. 429 RateLimit case with Retry-After header
	ts429 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "15")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error": "rate limit exceeded"}`)
	}))
	defer ts429.Close()

	groq429 := NewGroqTranscriber(GroqConfig{
		APIKey:     "test-key",
		BaseURL:    ts429.URL,
		HTTPClient: ts429.Client(),
	})

	_, err429 := groq429.Transcribe(t.Context(), audioPath, TranscribeOptions{})
	require.Error(t, err429)
	var apiErr *APIError
	require.True(t, assert.ErrorAs(t, err429, &apiErr))
	assert.Equal(t, 429, apiErr.StatusCode)
	assert.Equal(t, 15*time.Second, apiErr.RetryAfter)

	// 3. Missing API key
	groqNoKey := NewGroqTranscriber(GroqConfig{})
	_, errNoKey := groqNoKey.Transcribe(t.Context(), audioPath, TranscribeOptions{})
	assert.ErrorIs(t, errNoKey, ErrInvalidAPIKey)
}

type mockTranscriber struct {
	name   string
	result *TranscribeResult
	err    error
}

func (m *mockTranscriber) Name() string {
	return m.name
}

func (m *mockTranscriber) Transcribe(ctx context.Context, audioPath string, opts TranscribeOptions) (*TranscribeResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestOrchestrator_FallbackPolicies(t *testing.T) {
	tempDir := t.TempDir()
	audioPath := filepath.Join(tempDir, "sample.wav")
	require.NoError(t, os.WriteFile(audioPath, make([]byte, 32000), 0644)) // 1 second wav

	groqSuccess := &mockTranscriber{
		name: "groq",
		result: &TranscribeResult{
			Text:     "Groq Result",
			Provider: "groq",
		},
	}

	groqFail429 := &mockTranscriber{
		name: "groq",
		err: &APIError{
			StatusCode: 429,
			Message:    "rate limited",
			RetryAfter: 10 * time.Second,
		},
	}

	localMock := &mockTranscriber{
		name: "local_whisper",
		result: &TranscribeResult{
			Text:     "Local Whisper Result",
			Provider: "local_whisper",
		},
	}

	// Case 1: Groq Normal, Fallback disabled -> Uses Groq
	tracker1 := NewQuotaTracker(DefaultFreeQuotaConfig())
	orch1 := NewOrchestrator(OrchestratorConfig{
		Groq:          groqSuccess,
		Local:         localMock,
		Quota:         tracker1,
		AllowFallback: false,
	})

	res1, err := orch1.Transcribe(t.Context(), audioPath, TranscribeOptions{})
	require.NoError(t, err)
	assert.Equal(t, "Groq Result", res1.Text)
	assert.False(t, res1.Fallback)

	// Case 2: Groq 429, Fallback disabled -> Returns error (does not fallback)
	tracker2 := NewQuotaTracker(DefaultFreeQuotaConfig())
	orch2 := NewOrchestrator(OrchestratorConfig{
		Groq:          groqFail429,
		Local:         localMock,
		Quota:         tracker2,
		AllowFallback: false,
	})

	_, err2 := orch2.Transcribe(t.Context(), audioPath, TranscribeOptions{})
	require.Error(t, err2)
	assert.Contains(t, err2.Error(), "rate limited")

	// Case 3: Groq 429, Fallback enabled -> Falls back to Local Whisper
	orch2.SetFallbackEnabled(true)
	assert.True(t, orch2.IsFallbackEnabled())

	res3, err := orch2.Transcribe(t.Context(), audioPath, TranscribeOptions{})
	require.NoError(t, err)
	assert.Equal(t, "Local Whisper Result", res3.Text)
	assert.True(t, res3.Fallback)
	assert.Contains(t, res3.Reason, "local Whisper")

	// Case 4: Quota exceeded, Fallback disabled -> Returns ErrFallbackDisabled
	tracker4 := NewQuotaTracker(QuotaConfig{
		MaxRequestsPerMinute: 1,
	})
	tracker4.RecordUsage(true, 200, 10, 0) // Max RPM reached

	orch4 := NewOrchestrator(OrchestratorConfig{
		Groq:          groqSuccess,
		Local:         localMock,
		Quota:         tracker4,
		AllowFallback: false,
	})

	_, err4 := orch4.Transcribe(t.Context(), audioPath, TranscribeOptions{})
	require.Error(t, err4)
	assert.ErrorIs(t, err4, ErrFallbackDisabled)

	// Case 5: Quota exceeded, Fallback enabled -> Automatically falls back to Local Whisper
	orch4.SetFallbackEnabled(true)
	res5, err := orch4.Transcribe(t.Context(), audioPath, TranscribeOptions{})
	require.NoError(t, err)
	assert.Equal(t, "Local Whisper Result", res5.Text)
	assert.True(t, res5.Fallback)
	assert.Contains(t, res5.Reason, "Groq quota policy")
}

func TestOrchestrator_Concurrency(t *testing.T) {
	tempDir := t.TempDir()
	audioPath := filepath.Join(tempDir, "sample.wav")
	require.NoError(t, os.WriteFile(audioPath, make([]byte, 1000), 0644))

	groqMock := &mockTranscriber{
		name:   "groq",
		result: &TranscribeResult{Text: "Concurrent Reply", Provider: "groq"},
	}
	localMock := &mockTranscriber{
		name:   "local_whisper",
		result: &TranscribeResult{Text: "Local Reply", Provider: "local_whisper"},
	}

	orch := NewOrchestrator(OrchestratorConfig{
		Groq:          groqMock,
		Local:         localMock,
		Quota:         NewQuotaTracker(DefaultFreeQuotaConfig()),
		AllowFallback: false,
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				orch.SetFallbackEnabled(true)
			} else {
				orch.SetFallbackEnabled(false)
			}
			_, _ = orch.Transcribe(context.Background(), audioPath, TranscribeOptions{})
			_ = orch.GetQuotaStats()
		}(i)
	}
	wg.Wait()
}
