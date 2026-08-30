package kakao

import (
	"context"
	"io"
)

// TokenStore abstracts token persistence.
type TokenStore interface {
	Load(ctx context.Context) (*TokenInfo, error)
	Save(ctx context.Context, token *TokenInfo) error
	Clear(ctx context.Context) error
}

// AuthService handles authentication, token lifecycle, and account session.
type AuthService interface {
	GetAuthURL(scopes []string) string
	ExchangeCode(ctx context.Context, code string) (*TokenInfo, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenInfo, error)
	GetAccessTokenInfo(ctx context.Context) (*AccessTokenInfo, error)
	Logout(ctx context.Context) (int64, error)
	Unlink(ctx context.Context) (int64, error)
}

// UserService retrieves Kakao user and account information.
type UserService interface {
	GetMe(ctx context.Context) (*UserProfile, error)
	GetShippingAddresses(ctx context.Context, fromUpdated, pageSize int) (*ShippingAddressesResponse, error)
}

// MemoService handles sending messages to the authenticated user's own chatroom.
type MemoService interface {
	SendText(ctx context.Context, req TextMessageRequest) error
	SendFeed(ctx context.Context, feed FeedTemplate) error
	SendList(ctx context.Context, list ListTemplate) error
	SendCommerce(ctx context.Context, commerce CommerceTemplate) error
	SendLocation(ctx context.Context, location LocationTemplate) error
	SendScrap(ctx context.Context, requestURL string, templateID int64, args map[string]string) error
	SendCustom(ctx context.Context, templateID int64, args map[string]string) error
}

// FriendsService handles friend discovery and sending messages to friends.
type FriendsService interface {
	GetFriends(ctx context.Context, opts FriendsQueryOptions) (*FriendsResponse, error)
	SendText(ctx context.Context, receiverUUIDs []string, req TextMessageRequest) (*MessageResult, error)
	SendFeed(ctx context.Context, receiverUUIDs []string, feed FeedTemplate) (*MessageResult, error)
	SendList(ctx context.Context, receiverUUIDs []string, list ListTemplate) (*MessageResult, error)
	SendCommerce(ctx context.Context, receiverUUIDs []string, commerce CommerceTemplate) (*MessageResult, error)
	SendLocation(ctx context.Context, receiverUUIDs []string, location LocationTemplate) (*MessageResult, error)
	SendScrap(ctx context.Context, receiverUUIDs []string, requestURL string, templateID int64, args map[string]string) (*MessageResult, error)
	SendCustom(ctx context.Context, receiverUUIDs []string, templateID int64, args map[string]string) (*MessageResult, error)
}

// StorageService manages image uploads and scraping for KakaoTalk messages.
type StorageService interface {
	UploadImage(ctx context.Context, reader io.Reader, filename string) (*UploadedImageInfo, error)
	ScrapImage(ctx context.Context, imageURL string) (*UploadedImageInfo, error)
	DeleteImage(ctx context.Context, imageURL string) error
}

// Client is the primary composite interface for interacting with Kakao APIs.
type Client interface {
	Auth() AuthService
	User() UserService
	Memo() MemoService
	Friends() FriendsService
	Storage() StorageService

	GetValidAccessToken(ctx context.Context) (string, error)
	GetTokenStore() TokenStore
}
