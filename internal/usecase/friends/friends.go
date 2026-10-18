package friends

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
	"github.com/samber/lo"
)

// ListFriendsRequest holds options for querying friend list.
type ListFriendsRequest struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	TokenPath    string
	Offset       int
	Limit        int
	RawMarkdown  bool
	Out          io.Writer
}

// ListFriendsUseCase retrieves and displays Kakao friends.
type ListFriendsUseCase struct {
	presenter *FriendsPresenter
}

func NewListFriendsUseCase() *ListFriendsUseCase {
	return &ListFriendsUseCase{
		presenter: NewFriendsPresenter(),
	}
}

func (uc *ListFriendsUseCase) Execute(ctx context.Context, req ListFriendsRequest) error {
	out := req.Out
	if out == nil {
		out = os.Stdout
	}

	tokenStore := kakao.NewFileTokenStore(req.TokenPath)
	client := kakao.NewClient(kakao.ClientConfig{
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		RedirectURI:  req.RedirectURI,
		TokenStore:   tokenStore,
	})

	// 1. Retrieve Service Account Info (best-effort)
	var saInfo *ServiceAccountInfo
	if profile, err := client.User().GetMe(ctx); err == nil && profile != nil {
		saInfo = &ServiceAccountInfo{
			ID:          profile.ID,
			ConnectedAt: profile.ConnectedAt,
		}
		if profile.KakaoAccount != nil {
			saInfo.Email = profile.KakaoAccount.Email
			if profile.KakaoAccount.Profile != nil {
				saInfo.Nickname = profile.KakaoAccount.Profile.Nickname
			}
		}
	}

	if token, err := tokenStore.Load(ctx); err == nil && token != nil {
		if saInfo == nil {
			saInfo = &ServiceAccountInfo{}
		}
		saInfo.Scope = token.Scope
	}

	// 2. Retrieve KakaoTalk Friends
	friendsResp, err := client.Friends().GetFriends(ctx, kakao.FriendsQueryOptions{
		Offset: req.Offset,
		Limit:  req.Limit,
	})
	if err != nil {
		return fmt.Errorf("get friends: %w", err)
	}

	friendViews := lo.Map(friendsResp.Elements, func(friend kakao.Friend, idx int) FriendItemView {
		return FriendItemView{
			Index:       idx + 1 + req.Offset,
			UUID:        friend.UUID,
			Nickname:    friend.ProfileNickname,
			Messageable: friend.AllowedMsg,
			Favorite:    friend.Favorite,
		}
	})

	viewData := FriendsViewData{
		ServiceAccount: saInfo,
		Friends:        friendViews,
		TotalCount:     friendsResp.TotalCount,
	}

	return uc.presenter.Render(out, viewData, req.RawMarkdown)
}
