package notify

import (
	"strings"

	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
)

// TargetType defines the recipient category for messages.
type TargetType string

const (
	TargetMemo    TargetType = "memo"    // Personal talk memo (나에게 보내기)
	TargetFriends TargetType = "friends" // KakaoTalk friends (친구에게 보내기)
	TargetFamily  TargetType = "family"  // Family broadcast (나 + 등록된 가족 친구들 전체)
	TargetChannel TargetType = "channel" // KakaoTalk channel message (채널 메시지)
)

// MessageType defines the format template of the KakaoTalk message.
type MessageType string

const (
	MessageTypeText   MessageType = "text"
	MessageTypeFeed   MessageType = "feed"
	MessageTypeList   MessageType = "list"
	MessageTypeCustom MessageType = "custom"
)

// SendMessageRequest defines the input payload for the message dispatch API.
type SendMessageRequest struct {
	Bot           string            `json:"bot,omitempty"`            // Name of the bot to send from ("dobby", "infra", or empty for default)
	Target        TargetType        `json:"target,omitempty"`         // "memo" (default) | "friends" | "channel"
	Type          MessageType       `json:"type,omitempty"`           // "text" (default) | "feed" | "list" | "custom"
	Text          string            `json:"text,omitempty"`           // Message text content
	Title         string            `json:"title,omitempty"`          // Feed or List title
	Description   string            `json:"description,omitempty"`    // Feed description
	ImageURL      string            `json:"image_url,omitempty"`       // Image URL for card messages
	WebURL        string            `json:"web_url,omitempty"`        // Button click URL
	MobileURL     string            `json:"mobile_url,omitempty"`     // Mobile fallback URL
	ButtonTitle   string            `json:"button_title,omitempty"`   // Action button text
	ReceiverUUIDs []string          `json:"receiver_uuids,omitempty"` // Recipient UUIDs for friends target
	ChannelID     string            `json:"channel_id,omitempty"`     // Channel ID or slug for channel target
	TemplateID    int64             `json:"template_id,omitempty"`    // Custom template ID
	TemplateArgs  map[string]string `json:"template_args,omitempty"`  // Custom template variable mapping
}

// SendMessageResponse defines the unified output payload from message dispatch.
type SendMessageResponse struct {
	Success        bool                `json:"success"`
	Bot            string              `json:"bot"`
	Target         TargetType          `json:"target"`
	Type           MessageType         `json:"type"`
	Message        string              `json:"message,omitempty"`
	DeliveredCount int                 `json:"delivered_count"`
	FailedCount    int                 `json:"failed_count"`
	Details        *SendMessageDetails `json:"details,omitempty"`
}

// SendMessageDetails holds recipient-level delivery results.
type SendMessageDetails struct {
	SuccessfulReceivers []string            `json:"successful_receivers,omitempty"`
	Failures            []kakao.FailureInfo `json:"failures,omitempty"`
	ChannelInfo         map[string]any      `json:"channel_info,omitempty"`
}

// NormalizeTarget canonicalizes incoming target strings.
func NormalizeTarget(raw TargetType) (TargetType, bool) {
	s := strings.ToLower(strings.TrimSpace(string(raw)))
	switch s {
	case "", "memo", "me", "self":
		return TargetMemo, true
	case "friends", "friend":
		return TargetFriends, true
	case "family", "all":
		return TargetFamily, true
	case "channel":
		return TargetChannel, true
	default:
		return TargetType(s), false
	}
}
