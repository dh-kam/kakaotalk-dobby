package friends

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFriendsService struct {
	friendsResp *kakao.FriendsResponse
	friendsErr  error

	sentTextCalls []struct {
		UUIDs []string
		Text  string
	}
	sendTextResult *kakao.MessageResult
	sendTextErr    error
}

func (m *mockFriendsService) GetFriends(ctx context.Context, opts kakao.FriendsQueryOptions) (*kakao.FriendsResponse, error) {
	if m.friendsErr != nil {
		return nil, m.friendsErr
	}
	return m.friendsResp, nil
}

func (m *mockFriendsService) SendText(ctx context.Context, receiverUUIDs []string, req kakao.TextMessageRequest) (*kakao.MessageResult, error) {
	m.sentTextCalls = append(m.sentTextCalls, struct {
		UUIDs []string
		Text  string
	}{
		UUIDs: receiverUUIDs,
		Text:  req.Text,
	})
	if m.sendTextErr != nil {
		return nil, m.sendTextErr
	}
	return m.sendTextResult, nil
}

func (m *mockFriendsService) SendFeed(ctx context.Context, receiverUUIDs []string, feed kakao.FeedTemplate) (*kakao.MessageResult, error) {
	return nil, nil
}

func (m *mockFriendsService) SendList(ctx context.Context, receiverUUIDs []string, list kakao.ListTemplate) (*kakao.MessageResult, error) {
	return nil, nil
}

func (m *mockFriendsService) SendCommerce(ctx context.Context, receiverUUIDs []string, commerce kakao.CommerceTemplate) (*kakao.MessageResult, error) {
	return nil, nil
}

func (m *mockFriendsService) SendLocation(ctx context.Context, receiverUUIDs []string, location kakao.LocationTemplate) (*kakao.MessageResult, error) {
	return nil, nil
}

func (m *mockFriendsService) SendScrap(ctx context.Context, receiverUUIDs []string, requestURL string, templateID int64, args map[string]string) (*kakao.MessageResult, error) {
	return nil, nil
}

func (m *mockFriendsService) SendCustom(ctx context.Context, receiverUUIDs []string, templateID int64, args map[string]string) (*kakao.MessageResult, error) {
	return nil, nil
}

type mockClient struct {
	friends *mockFriendsService
}

func (m *mockClient) Auth() kakao.AuthService           { return nil }
func (m *mockClient) User() kakao.UserService           { return nil }
func (m *mockClient) Memo() kakao.MemoService           { return nil }
func (m *mockClient) Friends() kakao.FriendsService     { return m.friends }
func (m *mockClient) Storage() kakao.StorageService     { return nil }
func (m *mockClient) GetValidAccessToken(ctx context.Context) (string, error) {
	return "test-token", nil
}
func (m *mockClient) GetTokenStore() kakao.TokenStore { return nil }

func TestFriendWatcher_CheckOnce_AllAuthorized(t *testing.T) {
	tmpDir := t.TempDir()
	mapPath := filepath.Join(tmpDir, "friends_map.json")
	statePath := filepath.Join(tmpDir, "friends_alert_state.json")

	dir := NewFileFriendDirectory(mapPath)
	mockFriends := &mockFriendsService{
		friendsResp: &kakao.FriendsResponse{
			TotalCount: 2,
			Elements: []kakao.Friend{
				{
					ID:              5064290474,
					UUID:            "MAQyBjICMQE5FSQUJB0sFCMWIQ08CToONwAzdA",
					ProfileNickname: "감동희",
					AllowedMsg:      true,
				},
				{
					ID:              5140656190,
					UUID:            "MAc2AzsLPQ82GigfKBEkHCsaIw8-CzgMNQIxZw",
					ProfileNickname: "감윤재",
					AllowedMsg:      true,
				},
			},
		},
	}
	client := &mockClient{friends: mockFriends}

	var buf bytes.Buffer
	watcher := NewFriendWatcher(FriendWatcherConfig{
		Client:         client,
		Directory:      dir,
		StatePath:      statePath,
		AdminEmail:     "inzero@hanmail.net",
		AlertOnUnknown: true,
		Out:            &buf,
	})

	detections, err := watcher.CheckOnce(context.Background())
	require.NoError(t, err)
	assert.Empty(t, detections)
	assert.Empty(t, mockFriends.sentTextCalls)
}

func TestFriendWatcher_CheckOnce_DetectAndAlertUnauthorized(t *testing.T) {
	tmpDir := t.TempDir()
	mapPath := filepath.Join(tmpDir, "friends_map.json")
	statePath := filepath.Join(tmpDir, "friends_alert_state.json")

	dir := NewFileFriendDirectory(mapPath)
	mockFriends := &mockFriendsService{
		friendsResp: &kakao.FriendsResponse{
			TotalCount: 2,
			Elements: []kakao.Friend{
				{
					ID:              5064290474,
					UUID:            "MAQyBjICMQE5FSQUJB0sFCMWIQ08CToONwAzdA",
					ProfileNickname: "감동희",
					AllowedMsg:      true,
				},
				{
					ID:              9999999999,
					UUID:            "UNKNOWN-HACKER-UUID",
					ProfileNickname: "수상한사용자",
					AllowedMsg:      true,
				},
			},
		},
		sendTextResult: &kakao.MessageResult{
			SuccessfulReceiverUUIDs: []string{"MAQyBjICMQE5FSQUJB0sFCMWIQ08CToONwAzdA"},
		},
	}
	client := &mockClient{friends: mockFriends}

	var buf bytes.Buffer
	watcher := NewFriendWatcher(FriendWatcherConfig{
		Client:         client,
		Directory:      dir,
		StatePath:      statePath,
		AdminEmail:     "inzero@hanmail.net",
		AlertOnUnknown: true,
		Out:            &buf,
	})

	// 1. First run: should detect and send alert to admin
	detections, err := watcher.CheckOnce(context.Background())
	require.NoError(t, err)
	require.Len(t, detections, 1)

	d := detections[0]
	assert.Equal(t, "UNKNOWN-HACKER-UUID", d.Friend.UUID)
	assert.True(t, d.IsNewAlert)
	assert.True(t, d.AlertSent)
	assert.NoError(t, d.AlertError)

	require.Len(t, mockFriends.sentTextCalls, 1)
	assert.Equal(t, []string{"MAQyBjICMQE5FSQUJB0sFCMWIQ08CToONwAzdA"}, mockFriends.sentTextCalls[0].UUIDs)
	assert.Contains(t, mockFriends.sentTextCalls[0].Text, "보안 경고")
	assert.Contains(t, mockFriends.sentTextCalls[0].Text, "수상한사용자")
	assert.Contains(t, mockFriends.sentTextCalls[0].Text, "UNKNOWN-HACKER-UUID")

	// 2. Second run: should detect as unauthorized but suppress alert (IsNewAlert: false)
	detections2, err := watcher.CheckOnce(context.Background())
	require.NoError(t, err)
	require.Len(t, detections2, 1)
	assert.False(t, detections2[0].IsNewAlert)
	assert.False(t, detections2[0].AlertSent)
	// Still 1 call because duplicate alert was suppressed
	assert.Len(t, mockFriends.sentTextCalls, 1)
}

func TestCheckFriendsUseCase(t *testing.T) {
	tmpDir := t.TempDir()
	mapPath := filepath.Join(tmpDir, "friends_map.json")
	statePath := filepath.Join(tmpDir, "friends_alert_state.json")
	tokenPath := filepath.Join(tmpDir, "non_existent_tokens.json")

	// Seed directory
	_ = NewFileFriendDirectory(mapPath)

	var buf bytes.Buffer
	uc := NewCheckFriendsUseCase()

	// Calling with non-existent token will return error from Kakao client, but we can verify request handling
	err := uc.Execute(context.Background(), CheckFriendsRequest{
		TokenPath:      tokenPath,
		FriendsMapPath: mapPath,
		StatePath:      statePath,
		AdminEmail:     "inzero@hanmail.net",
		Out:            &buf,
	})
	// Error expected because kakao API cannot be reached without valid token
	assert.Error(t, err)
}
