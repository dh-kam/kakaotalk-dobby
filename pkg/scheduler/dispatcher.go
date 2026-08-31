package scheduler

import (
	"context"
	"fmt"
	"os"

	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
)

// Dispatcher handles delivery of fired scheduled jobs to KakaoTalk.
type Dispatcher struct {
	kakaoClient kakao.Client
}

// NewDispatcher creates a NotificationDispatcher.
func NewDispatcher(kakaoClient kakao.Client) *Dispatcher {
	return &Dispatcher{kakaoClient: kakaoClient}
}

// HandleJob executes when a schedule fires.
func (d *Dispatcher) HandleJob(ctx context.Context, job *Job) error {
	fmt.Printf("⏰ [Scheduler] Job fired! ID=%s | Title=%s | Message=%s\n", job.ID, job.Title, job.Message)

	if d.kakaoClient == nil {
		fmt.Printf("ℹ️ [Scheduler] Kakao client not configured, logged notification to stdout.\n")
		return nil
	}

	text := fmt.Sprintf("⏰ [알림] %s\n\n%s", job.Title, job.Message)

	req := kakao.TextMessageRequest{
		Text:        text,
		WebURL:      "https://outline.0xc0de1ab.dev",
		MobileURL:   "https://outline.0xc0de1ab.dev",
		ButtonTitle: "확인 / 블로그",
	}

	if err := d.kakaoClient.Memo().SendText(ctx, req); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ [Scheduler] Failed to send Kakao Talk notification for job %s: %v\n", job.ID, err)
		return err
	}

	fmt.Printf("✅ [Scheduler] KakaoTalk notification sent successfully for job %s\n", job.ID)
	return nil
}
