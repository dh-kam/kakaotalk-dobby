package notify

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
	"github.com/samber/lo"
)

var (
	ErrInvalidRequest = errors.New("invalid request")
)

// NotifyUseCase manages multi-bot message routing and delivery.
type NotifyUseCase struct {
	registry      kakao.ClientRegistry
	channelSender ChannelSender
}

// NewNotifyUseCase constructs a NotifyUseCase with registry and channel sender.
func NewNotifyUseCase(registry kakao.ClientRegistry, channelSender ChannelSender) *NotifyUseCase {
	if channelSender == nil {
		channelSender = NewDefaultChannelSender("")
	}
	return &NotifyUseCase{
		registry:      registry,
		channelSender: channelSender,
	}
}

// Send dispatches a message to the specified target using the selected bot.
func (uc *NotifyUseCase) Send(ctx context.Context, req SendMessageRequest) (*SendMessageResponse, error) {
	if uc.registry == nil {
		return nil, errors.New("kakao client registry is not configured")
	}

	target, ok := NormalizeTarget(req.Target)
	if !ok {
		return nil, fmt.Errorf("%w: unsupported target %q (supported: memo, friends, channel)", ErrInvalidRequest, req.Target)
	}

	if target == TargetFriends && len(req.ReceiverUUIDs) == 0 {
		return nil, fmt.Errorf("%w: receiver_uuids is required when target is friends", ErrInvalidRequest)
	}

	if req.TemplateID <= 0 && strings.TrimSpace(req.Text) == "" && strings.TrimSpace(req.Title) == "" && strings.TrimSpace(req.ImageURL) == "" {
		return nil, fmt.Errorf("%w: text, title, or template_id is required", ErrInvalidRequest)
	}

	client, err := uc.registry.Get(req.Bot)
	if err != nil {
		return nil, err
	}

	resolvedBot := strings.ToLower(strings.TrimSpace(req.Bot))
	if resolvedBot == "" || resolvedBot == "default" {
		infoList := uc.registry.Info()
		defInfo, found := lo.Find(infoList, func(b kakao.BotInfo) bool { return b.IsDefault })
		if found {
			resolvedBot = defInfo.Name
		} else if len(infoList) > 0 {
			resolvedBot = infoList[0].Name
		} else {
			resolvedBot = "default"
		}
	}

	msgType := req.Type
	if msgType == "" {
		if req.TemplateID > 0 {
			msgType = MessageTypeCustom
		} else if req.Title != "" || req.ImageURL != "" {
			msgType = MessageTypeFeed
		} else {
			msgType = MessageTypeText
		}
	}

	switch target {
	case TargetMemo:
		if req.TemplateID > 0 {
			if err := client.Memo().SendCustom(ctx, req.TemplateID, req.TemplateArgs); err != nil {
				return nil, fmt.Errorf("send custom memo message: %w", err)
			}
		} else if msgType == MessageTypeFeed || req.Title != "" || req.ImageURL != "" {
			feed := kakao.NewFeedTemplate(req.Title, req.Description, req.ImageURL, req.WebURL, req.ButtonTitle)
			if err := client.Memo().SendFeed(ctx, *feed); err != nil {
				return nil, fmt.Errorf("send feed memo message: %w", err)
			}
		} else {
			textReq := kakao.TextMessageRequest{
				Text:        req.Text,
				WebURL:      req.WebURL,
				MobileURL:   req.MobileURL,
				ButtonTitle: req.ButtonTitle,
			}
			if err := client.Memo().SendText(ctx, textReq); err != nil {
				return nil, fmt.Errorf("send text memo message: %w", err)
			}
		}

		return &SendMessageResponse{
			Success:        true,
			Bot:            resolvedBot,
			Target:         TargetMemo,
			Type:           msgType,
			Message:        "Message delivered successfully to yourself",
			DeliveredCount: 1,
			FailedCount:    0,
		}, nil

	case TargetFriends:
		var result *kakao.MessageResult
		if req.TemplateID > 0 {
			result, err = client.Friends().SendCustom(ctx, req.ReceiverUUIDs, req.TemplateID, req.TemplateArgs)
		} else if msgType == MessageTypeFeed || req.Title != "" || req.ImageURL != "" {
			feed := kakao.NewFeedTemplate(req.Title, req.Description, req.ImageURL, req.WebURL, req.ButtonTitle)
			result, err = client.Friends().SendFeed(ctx, req.ReceiverUUIDs, *feed)
		} else {
			textReq := kakao.TextMessageRequest{
				Text:        req.Text,
				WebURL:      req.WebURL,
				MobileURL:   req.MobileURL,
				ButtonTitle: req.ButtonTitle,
			}
			result, err = client.Friends().SendText(ctx, req.ReceiverUUIDs, textReq)
		}

		if err != nil {
			return nil, fmt.Errorf("send friends message: %w", err)
		}

		delivered := len(result.SuccessfulReceiverUUIDs)
		failed := len(result.FailureInfo)
		return &SendMessageResponse{
			Success:        delivered > 0,
			Bot:            resolvedBot,
			Target:         TargetFriends,
			Type:           msgType,
			Message:        fmt.Sprintf("Friends message processed: %d delivered, %d failed", delivered, failed),
			DeliveredCount: delivered,
			FailedCount:    failed,
			Details: &SendMessageDetails{
				SuccessfulReceivers: result.SuccessfulReceiverUUIDs,
				Failures:            result.FailureInfo,
			},
		}, nil

	case TargetChannel:
		if uc.channelSender == nil {
			return nil, errors.New("channel sender is not configured")
		}
		details, err := uc.channelSender.Send(ctx, client, req)
		if err != nil {
			return nil, fmt.Errorf("send channel message: %w", err)
		}
		return &SendMessageResponse{
			Success:        true,
			Bot:            resolvedBot,
			Target:         TargetChannel,
			Type:           msgType,
			Message:        "Channel message delivered successfully",
			DeliveredCount: 1,
			FailedCount:    0,
			Details:        details,
		}, nil

	default:
		return nil, fmt.Errorf("%w: unsupported target %q", ErrInvalidRequest, target)
	}
}
