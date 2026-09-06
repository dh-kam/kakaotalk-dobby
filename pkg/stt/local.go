package stt

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// LocalWhisperConfig holds configuration for the local Whisper STT engine.
type LocalWhisperConfig struct {
	BinaryPath string // Path to whisper-cli or go-whisper
	ModelPath  string // Path to ggml model (e.g. ggml-base.bin, ggml-small.bin)
	Threads    int    // Number of CPU threads (default 4)
	Language   string // Default language (default "ko")
}

type localWhisperTranscriber struct {
	binaryPath string
	modelPath  string
	threads    int
	language   string
}

// NewLocalWhisperTranscriber creates a new local Whisper engine.
func NewLocalWhisperTranscriber(cfg LocalWhisperConfig) Transcriber {
	bin := cfg.BinaryPath
	if bin == "" {
		// Auto detect binary
		candidates := []string{
			"/tmp/whisper.cpp/build/bin/whisper-cli",
			"/tmp/whisper.cpp/bindings/go/build_go/go-whisper",
			"whisper-cli",
			"go-whisper",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				bin = c
				break
			}
			if p, err := exec.LookPath(c); err == nil {
				bin = p
				break
			}
		}
	}

	model := cfg.ModelPath
	if model == "" {
		candidates := []string{
			"/tmp/whisper.cpp/models/ggml-base.bin",
			"/tmp/whisper.cpp/models/ggml-small.bin",
			"/workspace/kakao-bot/models/ggml-base.bin",
		}
		for _, m := range candidates {
			if _, err := os.Stat(m); err == nil {
				model = m
				break
			}
		}
	}

	threads := cfg.Threads
	if threads <= 0 {
		threads = 4
	}

	lang := cfg.Language
	if lang == "" {
		lang = "ko"
	}

	return &localWhisperTranscriber{
		binaryPath: bin,
		modelPath:  model,
		threads:    threads,
		language:   lang,
	}
}

func (l *localWhisperTranscriber) Name() string {
	return "local_whisper"
}

var timestampRegex = regexp.MustCompile(`\[\s*\d+[^\]]*\]\s*`)

func (l *localWhisperTranscriber) Transcribe(ctx context.Context, audioPath string, opts TranscribeOptions) (*TranscribeResult, error) {
	if _, err := os.Stat(audioPath); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrAudioFileNotFound
		}
		return nil, err
	}

	if l.binaryPath == "" {
		return nil, fmt.Errorf("local whisper binary not found")
	}
	if l.modelPath == "" {
		return nil, fmt.Errorf("local whisper model not found")
	}

	// 1. Prepare 16kHz mono WAV file for whisper.cpp
	wavPath := audioPath
	needCleanup := false
	ext := strings.ToLower(filepath.Ext(audioPath))
	if ext != ".wav" {
		tempWav, err := os.CreateTemp("", "whisper_input_*.wav")
		if err != nil {
			return nil, fmt.Errorf("create temp wav file: %w", err)
		}
		tempWav.Close()
		wavPath = tempWav.Name()
		needCleanup = true
		defer func() {
			if needCleanup {
				_ = os.Remove(wavPath)
			}
		}()

		// ffmpeg conversion to 16kHz 16-bit mono PCM
		convCmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", audioPath, "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", wavPath)
		if output, err := convCmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("ffmpeg convert audio: %w (output: %s)", err, string(output))
		}
	}

	lang := opts.Language
	if lang == "" {
		lang = l.language
	}

	// 2. Execute local whisper binary
	// whisper-cli arguments: -m <model> -l <lang> -t <threads> -f <wavPath> --no-prints
	start := time.Now()
	var cmd *exec.Cmd
	if strings.Contains(l.binaryPath, "go-whisper") {
		cmd = exec.CommandContext(ctx, l.binaryPath,
			"-model", l.modelPath,
			"-language", lang,
			"-threads", fmt.Sprintf("%d", l.threads),
			wavPath,
		)
	} else {
		cmd = exec.CommandContext(ctx, l.binaryPath,
			"-m", l.modelPath,
			"-l", lang,
			"-t", fmt.Sprintf("%d", l.threads),
			"-f", wavPath,
			"--no-prints",
		)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("local whisper failed: %w (stderr: %s)", err, stderr.String())
	}
	elapsed := time.Since(start)

	// 3. Clean and parse output lines
	lines := strings.Split(stdout.String(), "\n")
	var cleanedText strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip info/progress lines if any
		if strings.HasPrefix(line, "whisper_") || strings.HasPrefix(line, "system_info") || strings.HasPrefix(line, "Loading ") || strings.HasPrefix(line, "read_audio_data") {
			continue
		}
		textOnly := timestampRegex.ReplaceAllString(line, "")
		textOnly = strings.TrimSpace(textOnly)
		if textOnly != "" {
			if cleanedText.Len() > 0 {
				cleanedText.WriteString(" ")
			}
			cleanedText.WriteString(textOnly)
		}
	}

	return &TranscribeResult{
		Text:     cleanedText.String(),
		Duration: elapsed,
		Provider: "local_whisper",
		Model:    filepath.Base(l.modelPath),
		Fallback: false,
	}, nil
}
