package bootstrap

import (
	"context"
	"time"

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
		newFriendsCheckCommand(ctx),
		newFriendsWatchCommand(ctx),
	)

	return cmd
}

func newFriendsListCommand(ctx context.Context) *cobra.Command {
	cfg := config.Load()

	opts := struct {
		ClientID       string `flag:"client-id" usage:"Kakao REST API Key"`
		ClientSecret   string `flag:"client-secret" usage:"Kakao Client Secret"`
		RedirectURI    string `flag:"redirect-uri" usage:"Kakao OAuth Redirect URI"`
		TokenPath      string `flag:"token-path" usage:"Path to token JSON file"`
		FriendsMapPath string `flag:"friends-map" usage:"Path to friends mapping JSON file"`
		Offset         int    `flag:"offset" usage:"Pagination offset"`
		Limit          int    `flag:"limit" usage:"Number of friends to retrieve (max 100)"`
		Raw            bool   `flag:"raw" usage:"Output raw markdown instead of glamour-styled terminal view"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("client-id", "c", cfg.ClientID, "Kakao REST API Key").
		String("client-secret", cfg.ClientSecret, "Kakao Client Secret (optional)").
		StringP("redirect-uri", "r", cfg.RedirectURI, "Kakao OAuth Redirect URI").
		StringP("token-path", "t", cfg.TokenPath, "Path to token JSON file").
		StringP("friends-map", "m", cfg.FriendsMapPath, "Path to friends mapping JSON file").
		IntP("offset", "o", 0, "Pagination offset").
		IntP("limit", "l", 10, "Number of friends to retrieve (max 100)").
		Bool("raw", false, "Output raw markdown instead of glamour-styled terminal view")

	cmd := &cobra.Command{
		Use:           "list",
		Short:         "List KakaoTalk friends with receiver UUIDs and mapped identities",
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
				ClientID:       opts.ClientID,
				ClientSecret:   opts.ClientSecret,
				RedirectURI:    opts.RedirectURI,
				TokenPath:      opts.TokenPath,
				FriendsMapPath: opts.FriendsMapPath,
				Offset:         opts.Offset,
				Limit:          opts.Limit,
				RawMarkdown:    opts.Raw,
				Out:            cmd.OutOrStdout(),
			})
		},
	}

	binder.SetTo(cmd.Flags())
	maskFlagDefaults(cmd, "client-secret")
	return cmd
}

func newFriendsCheckCommand(ctx context.Context) *cobra.Command {
	cfg := config.Load()

	opts := struct {
		ClientID       string `flag:"client-id" usage:"Kakao REST API Key"`
		ClientSecret   string `flag:"client-secret" usage:"Kakao Client Secret"`
		RedirectURI    string `flag:"redirect-uri" usage:"Kakao OAuth Redirect URI"`
		TokenPath      string `flag:"token-path" usage:"Path to token JSON file"`
		FriendsMapPath string `flag:"friends-map" usage:"Path to friends mapping JSON file"`
		StatePath      string `flag:"state-path" usage:"Path to friends alert state JSON file"`
		AdminEmail     string `flag:"admin-email" usage:"Admin email to receive security alerts (default: inzero@hanmail.net)"`
		Alert          bool   `flag:"alert" usage:"Send KakaoTalk alert to admin when unauthorized friend is detected"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("client-id", "c", cfg.ClientID, "Kakao REST API Key").
		String("client-secret", cfg.ClientSecret, "Kakao Client Secret (optional)").
		StringP("redirect-uri", "r", cfg.RedirectURI, "Kakao OAuth Redirect URI").
		StringP("token-path", "t", cfg.TokenPath, "Path to token JSON file").
		StringP("friends-map", "m", cfg.FriendsMapPath, "Path to friends mapping JSON file").
		String("state-path", cfg.FriendsAlertStatePath, "Path to friends alert state JSON file").
		StringP("admin-email", "a", "inzero@hanmail.net", "Admin email to receive security alerts").
		Bool("alert", true, "Send KakaoTalk alert to admin when unauthorized friend is detected")

	cmd := &cobra.Command{
		Use:           "check",
		Short:         "Check KakaoTalk friends against whitelist and alert admin of unauthorized friends",
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
			return friends.NewCheckFriendsUseCase().Execute(cmd.Context(), friends.CheckFriendsRequest{
				ClientID:       opts.ClientID,
				ClientSecret:   opts.ClientSecret,
				RedirectURI:    opts.RedirectURI,
				TokenPath:      opts.TokenPath,
				FriendsMapPath: opts.FriendsMapPath,
				StatePath:      opts.StatePath,
				AdminEmail:     opts.AdminEmail,
				Alert:          opts.Alert,
				Out:            cmd.OutOrStdout(),
			})
		},
	}

	binder.SetTo(cmd.Flags())
	maskFlagDefaults(cmd, "client-secret")
	return cmd
}

func newFriendsWatchCommand(ctx context.Context) *cobra.Command {
	cfg := config.Load()

	opts := struct {
		ClientID       string        `flag:"client-id" usage:"Kakao REST API Key"`
		ClientSecret   string        `flag:"client-secret" usage:"Kakao Client Secret"`
		RedirectURI    string        `flag:"redirect-uri" usage:"Kakao OAuth Redirect URI"`
		TokenPath      string        `flag:"token-path" usage:"Path to token JSON file"`
		FriendsMapPath string        `flag:"friends-map" usage:"Path to friends mapping JSON file"`
		StatePath      string        `flag:"state-path" usage:"Path to friends alert state JSON file"`
		AdminEmail     string        `flag:"admin-email" usage:"Admin email to receive security alerts"`
		Interval       time.Duration `flag:"interval" usage:"Scan interval (e.g. 1h, 30m)"`
		Alert          bool          `flag:"alert" usage:"Send KakaoTalk alert to admin when unauthorized friend is detected"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("client-id", "c", cfg.ClientID, "Kakao REST API Key").
		String("client-secret", cfg.ClientSecret, "Kakao Client Secret (optional)").
		StringP("redirect-uri", "r", cfg.RedirectURI, "Kakao OAuth Redirect URI").
		StringP("token-path", "t", cfg.TokenPath, "Path to token JSON file").
		StringP("friends-map", "m", cfg.FriendsMapPath, "Path to friends mapping JSON file").
		String("state-path", cfg.FriendsAlertStatePath, "Path to friends alert state JSON file").
		StringP("admin-email", "a", "inzero@hanmail.net", "Admin email to receive security alerts").
		DurationP("interval", "i", 1*time.Hour, "Scan interval").
		Bool("alert", true, "Send KakaoTalk alert to admin when unauthorized friend is detected")

	cmd := &cobra.Command{
		Use:           "watch",
		Short:         "Periodically monitor KakaoTalk friends and alert admin of unauthorized additions",
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
			return friends.NewWatchFriendsUseCase().Execute(cmd.Context(), friends.WatchFriendsRequest{
				ClientID:       opts.ClientID,
				ClientSecret:   opts.ClientSecret,
				RedirectURI:    opts.RedirectURI,
				TokenPath:      opts.TokenPath,
				FriendsMapPath: opts.FriendsMapPath,
				StatePath:      opts.StatePath,
				AdminEmail:     opts.AdminEmail,
				Interval:       opts.Interval,
				Alert:          opts.Alert,
				Out:            cmd.OutOrStdout(),
			})
		},
	}

	binder.SetTo(cmd.Flags())
	maskFlagDefaults(cmd, "client-secret")
	return cmd
}
