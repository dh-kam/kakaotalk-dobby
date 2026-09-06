package notify

import (
	"context"
	"fmt"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
)

// ChannelSender abstracts delivery of messages destined for KakaoTalk channels.
type ChannelSender interface {
	Send(ctx context.Context, client kakao.Client, req SendMessageRequest) (*SendMessageDetails, error)
}

// DefaultChannelSender formats channel notifications with channel chat links and dispatches via talk memo.
type DefaultChannelSender struct {
	DefaultChannelID string
}

// NewDefaultChannelSender creates a ChannelSender with an optional default channel ID.
func NewDefaultChannelSender(defaultChannelID string) *DefaultChannelSender {
	return &DefaultChannelSender{
		DefaultChannelID: defaultChannelID,
	}
}

func (s *DefaultChannelSender) Send(ctx context.Context, client kakao.Client, req SendMessageRequest) (*SendMessageDetails, error) {
	chID := req.ChannelID
	if chID == "" {
		chID = s.DefaultChannelID
	}

	channelChatURL := req.WebURL
	if channelChatURL == "" && chID != "" {
		channelChatURL = fmt.Sprintf("https://pf.kakao.com/%s/chat", chID)
	}

	buttonTitle := req.ButtonTitle
	if buttonTitle == "" {
		buttonTitle = "채널 채팅 바로가기"
	}

	title := req.Title
	if title == "" {
		if chID != "" {
			title = fmt.Sprintf("[채널 @%s] 알림", chID)
		} else {
			title = "[채널 알림]"
		}
	}

	feed := kakao.NewFeedTemplate(title, req.Text, req.ImageURL, channelChatURL, buttonTitle)
	if err := client.Memo().SendFeed(ctx, *feed); err != nil {
		return nil, fmt.Errorf("channel memo delivery failed: %w", err)
	}

	return &SendMessageDetails{
		ChannelInfo: map[string]any{
			"channel_id":   chID,
			"chat_url":     channelChatURL,
			"delivered_at": time.Now().UTC().Format(time.RFC3339),
		},
	}, nil
}
