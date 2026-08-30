package kakao

import "time"

// TokenInfo holds OAuth token response data from Kakao.
type TokenInfo struct {
	AccessToken           string    `json:"access_token"`
	TokenType             string    `json:"token_type"`
	RefreshToken          string    `json:"refresh_token"`
	ExpiresIn             int64     `json:"expires_in"`
	Scope                 string    `json:"scope,omitempty"`
	RefreshTokenExpiresIn int64     `json:"refresh_token_expires_in,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
}

// IsExpired checks if the access token has expired (or within 60s of expiring).
func (t *TokenInfo) IsExpired() bool {
	if t == nil || t.AccessToken == "" {
		return true
	}
	if t.CreatedAt.IsZero() {
		return false
	}
	return time.Now().After(t.CreatedAt.Add(time.Duration(t.ExpiresIn-60) * time.Second))
}

// IsRefreshTokenExpired checks if the refresh token has expired.
func (t *TokenInfo) IsRefreshTokenExpired() bool {
	if t == nil || t.RefreshToken == "" {
		return true
	}
	if t.CreatedAt.IsZero() || t.RefreshTokenExpiresIn == 0 {
		return false
	}
	return time.Now().After(t.CreatedAt.Add(time.Duration(t.RefreshTokenExpiresIn) * time.Second))
}

// UserProfile represents Kakao user profile from /v2/user/me.
type UserProfile struct {
	ID           int64         `json:"id"`
	KakaoAccount *KakaoAccount `json:"kakao_account,omitempty"`
}

// KakaoAccount represents user account details.
type KakaoAccount struct {
	ProfileNicknameNeedsAgreement bool          `json:"profile_nickname_needs_agreement"`
	ProfileImageNeedsAgreement    bool          `json:"profile_image_needs_agreement"`
	Profile                       *KakaoProfile `json:"profile,omitempty"`
	NameNeedsAgreement            bool          `json:"name_needs_agreement"`
	Name                          string        `json:"name,omitempty"`
	EmailNeedsAgreement           bool          `json:"email_needs_agreement"`
	IsEmailValid                  bool          `json:"is_email_valid"`
	IsEmailVerified               bool          `json:"is_email_verified"`
	Email                         string        `json:"email,omitempty"`
}

// KakaoProfile represents user profile details.
type KakaoProfile struct {
	Nickname             string `json:"nickname"`
	ThumbnailImageURL    string `json:"thumbnail_image_url,omitempty"`
	ProfileImageURL      string `json:"profile_image_url,omitempty"`
	IsDefaultImage       bool   `json:"is_default_image"`
	IsDefaultNickname    bool   `json:"is_default_nickname"`
}

// Friend represents a KakaoTalk friend.
type Friend struct {
	ID                    int64  `json:"id"`
	UUID                  string `json:"uuid"`
	Favorite              bool   `json:"favorite"`
	ProfileNickname       string `json:"profile_nickname"`
	ProfileThumbnailImage string `json:"profile_thumbnail_image"`
	AllowedMsg            bool   `json:"allowed_msg"`
}

// FriendsResponse represents friends list response.
type FriendsResponse struct {
	Elements   []Friend `json:"elements"`
	TotalCount int      `json:"total_count"`
	BeforeURL  string   `json:"before_url,omitempty"`
	AfterURL   string   `json:"after_url,omitempty"`
}

// MessageResult represents response for sending messages to friends.
type MessageResult struct {
	SuccessfulReceiverUUIDs []string      `json:"successful_receiver_uuids"`
	FailureInfo             []FailureInfo `json:"failure_info,omitempty"`
}

// FailureInfo contains detail about recipient delivery failure.
type FailureInfo struct {
	Code          int      `json:"code"`
	Msg           string   `json:"msg"`
	ReceiverUUIDs []string `json:"receiver_uuids"`
}

// APIError represents an error response from Kakao API.
type APIError struct {
	Msg              string `json:"msg,omitempty"`
	Code             int    `json:"code,omitempty"`
	ErrorStr         string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

func (e *APIError) Error() string {
	if e.ErrorDescription != "" {
		return e.ErrorDescription
	}
	if e.Msg != "" {
		return e.Msg
	}
	if e.ErrorStr != "" {
		return e.ErrorStr
	}
	return "unknown kakao api error"
}
