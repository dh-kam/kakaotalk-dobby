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

// IsExpired checks if the access token has expired (with a 60s buffer).
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

// AccessTokenInfo contains token metadata returned from /v1/user/access_token_info.
type AccessTokenInfo struct {
	ID        int64 `json:"id"`
	ExpiresIn int64 `json:"expires_in"`
	AppID     int64 `json:"app_id"`
}

// UserProfile represents user profile from /v2/user/me.
type UserProfile struct {
	ID           int64         `json:"id"`
	KakaoAccount *KakaoAccount `json:"kakao_account,omitempty"`
	ConnectedAt  string        `json:"connected_at,omitempty"`
	SynchedAt    string        `json:"synched_at,omitempty"`
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
	PhoneNumberNeedsAgreement     bool          `json:"phone_number_needs_agreement"`
	PhoneNumber                   string        `json:"phone_number,omitempty"`
	BirthyearNeedsAgreement       bool          `json:"birthyear_needs_agreement"`
	Birthyear                     string        `json:"birthyear,omitempty"`
	BirthdayNeedsAgreement        bool          `json:"birthday_needs_agreement"`
	Birthday                      string        `json:"birthday,omitempty"`
	GenderNeedsAgreement          bool          `json:"gender_needs_agreement"`
	Gender                        string        `json:"gender,omitempty"`
}

// KakaoProfile represents user profile details.
type KakaoProfile struct {
	Nickname             string `json:"nickname"`
	ThumbnailImageURL    string `json:"thumbnail_image_url,omitempty"`
	ProfileImageURL      string `json:"profile_image_url,omitempty"`
	IsDefaultImage       bool   `json:"is_default_image"`
	IsDefaultNickname    bool   `json:"is_default_nickname"`
}

// ShippingAddressesResponse represents response from /v1/user/shipping_address.
type ShippingAddressesResponse struct {
	UserID            int64             `json:"user_id"`
	ShippingAddresses []ShippingAddress `json:"shipping_addresses"`
}

// ShippingAddress represents a single user shipping address.
type ShippingAddress struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	IsDefault      bool   `json:"is_default"`
	UpdatedAt      int64  `json:"updated_at"`
	Type           string `json:"type"`
	BaseAddress    string `json:"base_address"`
	DetailAddress  string `json:"detail_address"`
	ReceiverName   string `json:"receiver_name"`
	ReceiverPhone1 string `json:"receiver_phone_number1"`
	ReceiverPhone2 string `json:"receiver_phone_number2"`
	ZoneNumber     string `json:"zone_number"`
	ZipCode        string `json:"zip_code"`
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

// FriendsQueryOptions holds pagination and sorting params for /v1/api/talk/friends.
type FriendsQueryOptions struct {
	Offset      int    `json:"offset,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Order       string `json:"order,omitempty"`        // "asc" or "desc"
	FriendOrder string `json:"friend_order,omitempty"` // "favorite" or "nickname"
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

// UploadedImageInfo represents result of image upload from /v2/api/talk/message/image/upload.
type UploadedImageInfo struct {
	Infos ImageInfos `json:"infos"`
}

// ImageInfos contains original and thumbnail URLs.
type ImageInfos struct {
	Original  ImageDetail `json:"original"`
	Thumbnail ImageDetail `json:"thumbnail,omitempty"`
}

// ImageDetail contains image properties.
type ImageDetail struct {
	URL      string `json:"url"`
	Length   int64  `json:"length"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Format   string `json:"content_type"`
	Expires  int64  `json:"expires_at"`
}

// TextMessageRequest is a helper request struct for sending simple text messages.
type TextMessageRequest struct {
	Text        string
	WebURL      string
	MobileURL   string
	ButtonTitle string
}
