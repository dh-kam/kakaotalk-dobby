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
	Out          io.Writer
}

// ListFriendsUseCase retrieves and displays Kakao friends.
type ListFriendsUseCase struct{}

func NewListFriendsUseCase() *ListFriendsUseCase {
	return &ListFriendsUseCase{}
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

	friendsResp, err := client.Friends().GetFriends(ctx, kakao.FriendsQueryOptions{
		Offset: req.Offset,
		Limit:  req.Limit,
	})
	if err != nil {
		return fmt.Errorf("get friends: %w", err)
	}

	if len(friendsResp.Elements) == 0 {
		fmt.Fprintln(out, "No friends found (or friends have not granted messaging consent to this app).")
		return nil
	}

	fmt.Fprintf(out, "Found %d KakaoTalk Friend(s) (Total: %d):\n", len(friendsResp.Elements), friendsResp.TotalCount)
	fmt.Fprintf(out, "%-36s | %-20s | %-10s\n", "UUID", "Nickname", "Messageable")
	fmt.Fprintln(out, "-------------------------------------+----------------------+------------")

	lo.ForEach(friendsResp.Elements, func(friend kakao.Friend, _ int) {
		msgAllowed := "Yes"
		if !friend.AllowedMsg {
			msgAllowed = "No"
		}
		fmt.Fprintf(out, "%-36s | %-20s | %-10s\n", friend.UUID, friend.ProfileNickname, msgAllowed)
	})

	return nil
}
