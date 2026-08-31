package bootstrap

import (
	"context"

	"github.com/dh-kam/kakaotalk-dobby/internal/config"
	"github.com/dh-kam/kakaotalk-dobby/internal/usecase/friends"
	"github.com/dh-kam/refutils/flagsbinder"
	"github.com/spf13/cobra"
)

func newFriendsCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "friends",
		Short:         "Inspect KakaoTalk friends who authorized the application",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(
		newFriendsListCommand(ctx),
	)

	return cmd
}

func newFriendsListCommand(ctx context.Context) *cobra.Command {
	cfg := config.Load()

	opts := struct {
		ClientID     string `flag:"client-id" usage:"Kakao REST API Key"`
		ClientSecret string `flag:"client-secret" usage:"Kakao Client Secret"`
		RedirectURI  string `flag:"redirect-uri" usage:"Kakao OAuth Redirect URI"`
		TokenPath    string `flag:"token-path" usage:"Path to token JSON file"`
		Offset       int    `flag:"offset" usage:"Pagination offset"`
		Limit        int    `flag:"limit" usage:"Number of friends to retrieve (max 100)"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("client-id", "c", cfg.ClientID, "Kakao REST API Key").
		String("client-secret", cfg.ClientSecret, "Kakao Client Secret (optional)").
		StringP("redirect-uri", "r", cfg.RedirectURI, "Kakao OAuth Redirect URI").
		StringP("token-path", "t", cfg.TokenPath, "Path to token JSON file").
		IntP("offset", "o", 0, "Pagination offset").
		IntP("limit", "l", 10, "Number of friends to retrieve (max 100)")

	cmd := &cobra.Command{
		Use:           "list",
		Short:         "List KakaoTalk friends with receiver UUIDs",
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := binder.BindCommand(cmd, &opts, args...); err != nil {
				_ = cmd.Usage()
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return friends.NewListFriendsUseCase().Execute(cmd.Context(), friends.ListFriendsRequest{
				ClientID:     opts.ClientID,
				ClientSecret: opts.ClientSecret,
				RedirectURI:  opts.RedirectURI,
				TokenPath:    opts.TokenPath,
				Offset:       opts.Offset,
				Limit:        opts.Limit,
				Out:          cmd.OutOrStdout(),
			})
		},
	}

	binder.SetTo(cmd.Flags())
	return cmd
}
