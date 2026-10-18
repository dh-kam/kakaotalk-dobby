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
				Index:       1,
				UUID:        "UUID-1111",
				Nickname:    "감동희",
				Messageable: true,
				Favorite:    false,
			},
			{
				Index:       2,
				UUID:        "UUID-2222",
				Nickname:    "감윤재",
				Messageable: true,
				Favorite:    true,
			},
			{
				Index:       3,
				UUID:        "UUID-3333",
				Nickname:    "차단친구",
				Messageable: false,
				Favorite:    false,
			},
		},
		TotalCount: 3,
	}

	md := presenter.BuildMarkdown(data)

	assert.Contains(t, md, "# 🤖 KakaoTalk Service Account & Friends")
	assert.Contains(t, md, "### 👤 Active Service Account")
	assert.Contains(t, md, "5064288351")
	assert.Contains(t, md, "Donghee Kam")
	assert.Contains(t, md, "donghee.kam@gmail.com")
	assert.Contains(t, md, "`talk_message`, `friends`, `profile_nickname`")
	assert.Contains(t, md, "### 👥 Authorized Friends (3 / Total 3)")
	assert.Contains(t, md, "**감동희**")
	assert.Contains(t, md, "**감윤재**")
	assert.Contains(t, md, "✅ Yes")
	assert.Contains(t, md, "❌ No")
	assert.Contains(t, md, "⭐")
	assert.Contains(t, md, "UUID-1111")
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
