package friends

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFriendDirectory struct {
	mappings []FriendMapping
}

func (m *mockFriendDirectory) All() []FriendMapping {
	return m.mappings
}

func (m *mockFriendDirectory) FindByUUID(uuid string) (*FriendMapping, bool) {
	for _, mapping := range m.mappings {
		if mapping.UUID == uuid {
			cp := mapping
			return &cp, true
		}
	}
	return nil, false
}

func (m *mockFriendDirectory) FindByEmail(email string) (*FriendMapping, bool) {
	for _, mapping := range m.mappings {
		if strings.EqualFold(mapping.Email, email) {
			cp := mapping
			return &cp, true
		}
	}
	return nil, false
}

func (m *mockFriendDirectory) GetAdmin() (*FriendMapping, bool) {
	for _, mapping := range m.mappings {
		if mapping.IsAdmin {
			cp := mapping
			return &cp, true
		}
	}
	return m.FindByEmail("inzero@hanmail.net")
}

func (m *mockFriendDirectory) AddOrUpdate(mapping FriendMapping) error {
	m.mappings = append(m.mappings, mapping)
	return nil
}

func (m *mockFriendDirectory) Save() error {
	return nil
}

type capturedAlert struct {
	AdminUUID string
	Text      string
}

type mockAlertSender struct {
	mu      sync.Mutex
	alerts  []capturedAlert
	failErr error
}

func (m *mockAlertSender) SendAlert(ctx context.Context, adminUUID string, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failErr != nil {
		return m.failErr
	}
	m.alerts = append(m.alerts, capturedAlert{
		AdminUUID: adminUUID,
		Text:      text,
	})
	return nil
}

func (m *mockAlertSender) getAlerts() []capturedAlert {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]capturedAlert, len(m.alerts))
	copy(cp, m.alerts)
	return cp
}

func setupTestDirectory() *mockFriendDirectory {
	return &mockFriendDirectory{
		mappings: []FriendMapping{
			{
				Name:    "아빠 (감동희)",
				Email:   "inzero@hanmail.net",
				UUID:    "ADMIN_UUID_123",
				IsAdmin: true,
			},
			{
				Name:    "아들 (감윤재)",
				Email:   "kamtoyun@gmail.com",
				UUID:    "SON_UUID_456",
				IsAdmin: false,
			},
			{
				Name:    "신원 미확인 친구",
				Email:   "", // Empty email -> unverified!
				UUID:    "UNVERIFIED_UUID_789",
				IsAdmin: false,
			},
		},
	}
}

func TestDirectoryFriendGuard_VerifiedRecipients(t *testing.T) {
	dir := setupTestDirectory()
	alertSender := &mockAlertSender{}
	guard := NewDirectoryFriendGuard(DirectoryFriendGuardConfig{
		Directory:   dir,
		AlertSender: alertSender,
	})

	ctx := context.Background()
	allowed, err := guard.ValidateSend(ctx, []string{"ADMIN_UUID_123", "SON_UUID_456"}, "안녕하세요!")

	require.NoError(t, err)
	assert.Equal(t, []string{"ADMIN_UUID_123", "SON_UUID_456"}, allowed)
	assert.Empty(t, alertSender.getAlerts(), "No alerts should be sent for verified recipients")
}

func TestDirectoryFriendGuard_UnregisteredRecipient(t *testing.T) {
	dir := setupTestDirectory()
	alertSender := &mockAlertSender{}
	var callbackCalled bool
	var callbackUnverified []UnverifiedRecipientInfo

	guard := NewDirectoryFriendGuard(DirectoryFriendGuardConfig{
		Directory:   dir,
		AlertSender: alertSender,
		OnAlertSent: func(unverified []UnverifiedRecipientInfo, summary string) {
			callbackCalled = true
			callbackUnverified = unverified
		},
	})

	ctx := context.Background()
	allowed, err := guard.ValidateSend(ctx, []string{"UNKNOWN_UUID_999"}, "비밀 메시지")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnverifiedFriendBlocked), "error must match ErrUnverifiedFriendBlocked")
	assert.Nil(t, allowed)

	alerts := alertSender.getAlerts()
	require.Len(t, alerts, 1)
	assert.Equal(t, "ADMIN_UUID_123", alerts[0].AdminUUID)
	assert.Contains(t, alerts[0].Text, "🚨 [보안 경고]")
	assert.Contains(t, alerts[0].Text, "UNKNOWN_UUID_999")
	assert.Contains(t, alerts[0].Text, "화이트리스트(friends_map.json) 미등록 계정")
	assert.Contains(t, alerts[0].Text, "비밀 메시지")

	assert.True(t, callbackCalled)
	require.Len(t, callbackUnverified, 1)
	assert.Equal(t, "UNKNOWN_UUID_999", callbackUnverified[0].UUID)
	assert.False(t, callbackUnverified[0].Registered)
}

func TestDirectoryFriendGuard_RegisteredWithoutEmail(t *testing.T) {
	dir := setupTestDirectory()
	alertSender := &mockAlertSender{}

	guard := NewDirectoryFriendGuard(DirectoryFriendGuardConfig{
		Directory:   dir,
		AlertSender: alertSender,
	})

	ctx := context.Background()
	allowed, err := guard.ValidateSend(ctx, []string{"UNVERIFIED_UUID_789"}, "테스트 알림")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnverifiedFriendBlocked))
	assert.Nil(t, allowed)

	alerts := alertSender.getAlerts()
	require.Len(t, alerts, 1)
	assert.Equal(t, "ADMIN_UUID_123", alerts[0].AdminUUID)
	assert.Contains(t, alerts[0].Text, "UNVERIFIED_UUID_789")
	assert.Contains(t, alerts[0].Text, "신원 미확인 친구")
	assert.Contains(t, alerts[0].Text, "이메일 미등록 (신원확인 미완료)")
}

func TestDirectoryFriendGuard_MixedRecipients(t *testing.T) {
	dir := setupTestDirectory()
	alertSender := &mockAlertSender{}

	guard := NewDirectoryFriendGuard(DirectoryFriendGuardConfig{
		Directory:   dir,
		AlertSender: alertSender,
	})

	ctx := context.Background()
	// 1 verified (SON_UUID_456) and 1 unverified (UNKNOWN_UUID_999)
	allowed, err := guard.ValidateSend(ctx, []string{"SON_UUID_456", "UNKNOWN_UUID_999"}, "공지사항입니다.")

	// Allowed slice should contain only the verified user
	require.NoError(t, err, "Mixed dispatch should not return error if at least one verified user exists")
	assert.Equal(t, []string{"SON_UUID_456"}, allowed)

	// Admin must still be alerted about the unverified recipient!
	alerts := alertSender.getAlerts()
	require.Len(t, alerts, 1)
	assert.Contains(t, alerts[0].Text, "UNKNOWN_UUID_999")
	assert.Contains(t, alerts[0].Text, "공지사항입니다.")
}

func TestDirectoryFriendGuard_AlertSenderFailureDoesNotUnblock(t *testing.T) {
	dir := setupTestDirectory()
	alertSender := &mockAlertSender{
		failErr: errors.New("network timeout when sending alert"),
	}

	guard := NewDirectoryFriendGuard(DirectoryFriendGuardConfig{
		Directory:   dir,
		AlertSender: alertSender,
	})

	ctx := context.Background()
	allowed, err := guard.ValidateSend(ctx, []string{"UNKNOWN_UUID_999"}, "중요 메시지")

	// Even if alert delivery failed, unverified friend must STILL be blocked!
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnverifiedFriendBlocked))
	assert.Nil(t, allowed)
}

func TestDirectoryFriendGuard_FallbackAdminUUID(t *testing.T) {
	emptyDir := &mockFriendDirectory{}
	alertSender := &mockAlertSender{}

	guard := NewDirectoryFriendGuard(DirectoryFriendGuardConfig{
		Directory:   emptyDir,
		AlertSender: alertSender,
		AdminUUID:   "CUSTOM_FALLBACK_ADMIN_UUID",
	})

	ctx := context.Background()
	_, err := guard.ValidateSend(ctx, []string{"SOME_RANDOM_UUID"}, "테스트")

	require.Error(t, err)
	alerts := alertSender.getAlerts()
	require.Len(t, alerts, 1)
	assert.Equal(t, "CUSTOM_FALLBACK_ADMIN_UUID", alerts[0].AdminUUID)
}

func TestDirectoryFriendGuard_Deduplication(t *testing.T) {
	dir := setupTestDirectory()
	alertSender := &mockAlertSender{}

	guard := NewDirectoryFriendGuard(DirectoryFriendGuardConfig{
		Directory:   dir,
		AlertSender: alertSender,
	})

	ctx := context.Background()
	allowed, err := guard.ValidateSend(ctx, []string{"SON_UUID_456", "SON_UUID_456", "SON_UUID_456"}, "중복 테스트")

	require.NoError(t, err)
	assert.Equal(t, []string{"SON_UUID_456"}, allowed)
	assert.Empty(t, alertSender.getAlerts())
}
