package openbuilder

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSimpleTextResponse(t *testing.T) {
	resp := NewSimpleTextResponse("Hello from OpenBuilder")
	resp.AddQuickReply("Help", "help")

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"version":"2.0"`)
	assert.Contains(t, jsonStr, `"text":"Hello from OpenBuilder"`)
	assert.Contains(t, jsonStr, `"label":"Help"`)
}

func TestNewBasicCardResponse(t *testing.T) {
	btn := NewWebButton("Visit", "https://0xc0de1ab.dev")
	resp := NewBasicCardResponse("Card Title", "Card Description", "https://example.com/thumb.png", btn)

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"title":"Card Title"`)
	assert.Contains(t, jsonStr, `"description":"Card Description"`)
	assert.Contains(t, jsonStr, `"webLinkUrl":"https://0xc0de1ab.dev"`)
}

func TestSkillResponse_ValidateAndNormalize(t *testing.T) {
	// 1. TextCard with over 500 characters description
	longText := ""
	for i := 0; i < 600; i++ {
		longText += "가"
	}
	resp := NewTextCardResponse("타이틀", longText)
	for i := 0; i < 15; i++ {
		resp.AddQuickReply("엄청나게긴퀵리플라이버튼라벨입니다", "msg")
	}

	resp.ValidateAndNormalize()

	tc := resp.Template.Outputs[0].TextCard
	assert.NotNil(t, tc)
	assert.LessOrEqual(t, len([]rune(tc.Title))+len([]rune(tc.Description)), 400)
	assert.True(t, len([]rune(tc.Description)) > 0)

	// QuickReplies count max 10, label max 14
	assert.LessOrEqual(t, len(resp.Template.QuickReplies), 10)
	for _, qr := range resp.Template.QuickReplies {
		assert.LessOrEqual(t, len([]rune(qr.Label)), 14)
	}
}

