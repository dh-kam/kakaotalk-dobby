package friends

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFriendsPresenter_BuildMarkdown_WithServiceAccountAndFriends(t *testing.T) {
	presenter := NewFriendsPresenter()

	data := FriendsViewData{
		ServiceAccount: &ServiceAccountInfo{
			ID:          5064288351,
			Nickname:    "Donghee Kam",
			Email:       "donghee.kam@gmail.com",
			ConnectedAt: "2026-08-30T10:15:44Z",
			Scope:       "talk_message friends profile_nickname",
		},
		Friends: []FriendItemView{
			{
				Index:        1,
				UUID:         "UUID-1111",
				Nickname:     "감동희",
				Email:        "inzero@hanmail.net",
				MappedName:   "아빠 (감동희)",
				IsAdmin:      true,
				IsRegistered: true,
				Messageable:  true,
				Favorite:     false,
			},
			{
				Index:        2,
				UUID:         "UUID-2222",
				Nickname:     "감윤재",
				Email:        "kamtoyun@gmail.com",
				MappedName:   "아들 (감윤재)",
				IsAdmin:      false,
				IsRegistered: true,
				Messageable:  true,
				Favorite:     true,
			},
			{
				Index:        3,
				UUID:         "UUID-3333",
				Nickname:     "미등록친구",
				Email:        "",
				MappedName:   "",
				IsAdmin:      false,
				IsRegistered: false,
				Messageable:  false,
				Favorite:     false,
			},
		},
		TotalCount: 3,
	}

	md := presenter.BuildMarkdown(data)

	assert.Contains(t, md, "# 🤖 KakaoTalk Service Account & Friends")
	assert.Contains(t, md, "### 👤 Active Service Account")
	assert.Contains(t, md, "- **User ID**: `5064288351`")
	assert.Contains(t, md, "- **Nickname**: Donghee Kam")
	assert.Contains(t, md, "- **Email**: `donghee.kam@gmail.com`")
	assert.Contains(t, md, "`talk_message`, `friends`, `profile_nickname`")
	assert.Contains(t, md, "### 👥 Authorized Friends (3 / Total 3)")
	assert.Contains(t, md, "| # | Nickname | Email | Status | Receiver UUID |")
	assert.Contains(t, md, "**감동희 (아빠 (감동희)) 👑**")
	assert.Contains(t, md, "**감윤재 (아들 (감윤재)) ⭐**")
	assert.Contains(t, md, "`inzero@hanmail.net`")
	assert.Contains(t, md, "kamtoyun@gmail.com")
	assert.Contains(t, md, "⚠️ 미등록")
	assert.Contains(t, md, "✅ Yes")
	assert.Contains(t, md, "❌ No")
	assert.Contains(t, md, "UUID-1111")
	assert.Contains(t, md, "🚨 **보안 주의**: 등록되지 않은 미승인 친구(1명)가 감지되었습니다.")
}

func TestFriendsPresenter_BuildMarkdown_EmptyFriends(t *testing.T) {
	presenter := NewFriendsPresenter()

	data := FriendsViewData{
		ServiceAccount: &ServiceAccountInfo{
			ID: 5064288351,
		},
		Friends:    nil,
		TotalCount: 0,
	}

	md := presenter.BuildMarkdown(data)

	assert.Contains(t, md, "### 👤 Active Service Account")
	assert.Contains(t, md, "### 👥 Authorized Friends (0)")
	assert.Contains(t, md, "No friends found.")
}

func TestFriendsPresenter_Render_Raw(t *testing.T) {
	presenter := NewFriendsPresenter()
	data := FriendsViewData{
		ServiceAccount: &ServiceAccountInfo{
			ID: 12345,
		},
		Friends: []FriendItemView{
			{Index: 1, UUID: "U-1", Nickname: "Friend1", Messageable: true},
		},
		TotalCount: 1,
	}

	var buf bytes.Buffer
	err := presenter.Render(&buf, data, true)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "# 🤖 KakaoTalk Service Account & Friends")
	assert.Contains(t, out, "Friend1")
}

func TestFriendsPresenter_Render_Glamour(t *testing.T) {
	presenter := NewFriendsPresenter()
	data := FriendsViewData{
		ServiceAccount: &ServiceAccountInfo{
			ID: 12345,
		},
		Friends: []FriendItemView{
			{Index: 1, UUID: "U-1", Nickname: "Friend1", Messageable: true},
		},
		TotalCount: 1,
	}

	var buf bytes.Buffer
	err := presenter.Render(&buf, data, false)
	require.NoError(t, err)

	out := buf.String()
	// Glamour renders styled output
	assert.NotEmpty(t, out)
	assert.Contains(t, out, "Friend1")
}

func TestGetTerminalWidth_Fallback(t *testing.T) {
	var buf bytes.Buffer
	width := getTerminalWidth(&buf)
	assert.Equal(t, 110, width)
}
