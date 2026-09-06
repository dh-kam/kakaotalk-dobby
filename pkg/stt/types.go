package stt

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrQuotaExceeded is returned when Groq quota or rate limits are reached.
	ErrQuotaExceeded = errors.New("stt: quota exceeded")
	// ErrInvalidAPIKey is returned when Groq API key is missing or invalid.
	ErrInvalidAPIKey = errors.New("stt: invalid or missing API key")
	// ErrFallbackDisabled is returned when Groq fails/throttles and fallback to local is disabled.
	ErrFallbackDisabled = errors.New("stt: groq quota or rate limit exceeded and local fallback is disabled")
	// ErrAudioFileNotFound is returned when the audio file cannot be found.
	ErrAudioFileNotFound = errors.New("stt: audio file not found")
)

// TranscribeOptions specifies parameters for speech-to-text recognition.
type TranscribeOptions struct {
	Language       string  // e.g. "ko", "en"
	Prompt         string  // Optional context or vocabulary hint
	Temperature    float64 // Sampling temperature (0.0 to 1.0)
	ResponseFormat string  // "text", "json", "verbose_json"
}

// TranscribeResult holds the transcribed text and execution metadata.
type TranscribeResult struct {
	Text      string        `json:"text"`
	Duration  time.Duration `json:"duration,omitempty"`
	Provider  string        `json:"provider"` // "groq" or "local_whisper"
	Model     string        `json:"model"`
	Fallback  bool          `json:"fallback"`
	Reason    string        `json:"reason,omitempty"`
}

// Transcriber defines the contract for speech-to-text recognition.
type Transcriber interface {
	Name() string
	Transcribe(ctx context.Context, audioPath string, opts TranscribeOptions) (*TranscribeResult, error)
}
