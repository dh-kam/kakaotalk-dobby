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
