package bootstrap

import (
	"context"
	"fmt"

	"github.com/dh-kam/kakaotalk-dobby/internal/config"
	"github.com/dh-kam/kakaotalk-dobby/internal/usecase/auth"
	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
	"github.com/dh-kam/refutils/flagsbinder"
	"github.com/spf13/cobra"
)

func newAuthCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "auth",
		Short:         "Manage KakaoTalk OAuth authentication and tokens",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(
		newAuthLoginCommand(ctx),
		newAuthCheckCommand(ctx),
		newAuthStatusCommand(ctx),
		newAuthRefreshCommand(ctx),
		newAuthLogoutCommand(ctx),
		newAuthUnlinkCommand(ctx),
	)

	return cmd
}

func newAuthLoginCommand(ctx context.Context) *cobra.Command {
	cfg := config.Load()

	opts := struct {
		ClientID     string   `flag:"client-id" usage:"Kakao REST API Key"`
		ClientSecret string   `flag:"client-secret" usage:"Kakao Client Secret"`
		RedirectURI  string   `flag:"redirect-uri" usage:"Kakao OAuth Redirect URI"`
		TokenPath    string   `flag:"token-path" usage:"Path to save token JSON file"`
		Code         string   `flag:"code" usage:"Kakao authorization code"`
		Scopes       []string `flag:"scopes" usage:"OAuth scopes (optional)"`
		Manual       bool     `flag:"manual" usage:"Force manual code input without starting local callback server"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("client-id", "c", cfg.ClientID, "Kakao REST API Key").
		String("client-secret", cfg.ClientSecret, "Kakao Client Secret (optional)").
		StringP("redirect-uri", "r", cfg.RedirectURI, "Kakao OAuth Redirect URI").
		StringP("token-path", "t", cfg.TokenPath, "Path to save token JSON file").
		String("code", "", "Kakao authorization code").
		StringSlice("scopes", nil, "OAuth scopes (optional)").
		Bool("manual", false, "Force manual code input without starting local callback server")

	cmd := &cobra.Command{
		Use:           "login",
		Short:         "Authenticate with Kakao via OAuth 2.0 and save tokens",
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := binder.BindCommand(cmd, &opts, args...); err != nil {
				_ = cmd.Usage()
				return err
			}
			if opts.ClientID == "" {
				_ = cmd.Usage()
				return fmt.Errorf("--client-id is required (or set KAKAO_REST_API_KEY environment variable)")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return auth.NewLoginUseCase().Execute(cmd.Context(), auth.LoginRequest{
				ClientID:     opts.ClientID,
				ClientSecret: opts.ClientSecret,
				RedirectURI:  opts.RedirectURI,
				TokenPath:    opts.TokenPath,
				Code:         opts.Code,
				Scopes:       opts.Scopes,
				Manual:       opts.Manual,
				Out:          cmd.OutOrStdout(),
			})
		},
	}

	binder.SetTo(cmd.Flags())
	return cmd
}

func newAuthCheckCommand(ctx context.Context) *cobra.Command {
	cfg := config.Load()

	opts := struct {
		ClientID    string `flag:"client-id" usage:"Kakao REST API Key"`
		RedirectURI string `flag:"redirect-uri" usage:"Kakao OAuth Redirect URI"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("client-id", "c", cfg.ClientID, "Kakao REST API Key").
		StringP("redirect-uri", "r", cfg.RedirectURI, "Kakao OAuth Redirect URI")

	cmd := &cobra.Command{
		Use:           "check",
		Short:         "Validate Kakao REST API Key and verify Developer settings",
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := binder.BindCommand(cmd, &opts, args...); err != nil {
				_ = cmd.Usage()
				return err
			}
			if opts.ClientID == "" {
				_ = cmd.Usage()
				return fmt.Errorf("--client-id is required (or set KAKAO_REST_API_KEY in .env)")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return auth.NewCheckUseCase().Execute(cmd.Context(), auth.CheckRequest{
				ClientID:    opts.ClientID,
				RedirectURI: opts.RedirectURI,
				Out:         cmd.OutOrStdout(),
			})
		},
	}

	binder.SetTo(cmd.Flags())
	return cmd
}

func newAuthStatusCommand(ctx context.Context) *cobra.Command {
	cfg := config.Load()

	opts := struct {
		ClientID     string `flag:"client-id" usage:"Kakao REST API Key"`
		ClientSecret string `flag:"client-secret" usage:"Kakao Client Secret"`
		RedirectURI  string `flag:"redirect-uri" usage:"Kakao OAuth Redirect URI"`
		TokenPath    string `flag:"token-path" usage:"Path to token JSON file"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("client-id", "c", cfg.ClientID, "Kakao REST API Key").
		String("client-secret", cfg.ClientSecret, "Kakao Client Secret (optional)").
		StringP("redirect-uri", "r", cfg.RedirectURI, "Kakao OAuth Redirect URI").
		StringP("token-path", "t", cfg.TokenPath, "Path to token JSON file")

	cmd := &cobra.Command{
		Use:           "status",
		Short:         "Check KakaoTalk authentication status and current user profile",
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
			return auth.NewStatusUseCase().Execute(cmd.Context(), auth.StatusRequest{
				ClientID:     opts.ClientID,
				ClientSecret: opts.ClientSecret,
				RedirectURI:  opts.RedirectURI,
				TokenPath:    opts.TokenPath,
				Out:          cmd.OutOrStdout(),
			})
		},
	}

	binder.SetTo(cmd.Flags())
	return cmd
}

func newAuthRefreshCommand(ctx context.Context) *cobra.Command {
	cfg := config.Load()

	opts := struct {
		ClientID     string `flag:"client-id" usage:"Kakao REST API Key"`
		ClientSecret string `flag:"client-secret" usage:"Kakao Client Secret"`
		TokenPath    string `flag:"token-path" usage:"Path to token JSON file"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("client-id", "c", cfg.ClientID, "Kakao REST API Key").
		String("client-secret", cfg.ClientSecret, "Kakao Client Secret (optional)").
		StringP("token-path", "t", cfg.TokenPath, "Path to token JSON file")

	cmd := &cobra.Command{
		Use:           "refresh",
		Short:         "Force refresh KakaoTalk access token",
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := binder.BindCommand(cmd, &opts, args...); err != nil {
				_ = cmd.Usage()
				return err
			}
			if opts.ClientID == "" {
				_ = cmd.Usage()
				return fmt.Errorf("--client-id is required (or set KAKAO_REST_API_KEY environment variable)")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return auth.NewRefreshUseCase().Execute(cmd.Context(), auth.RefreshRequest{
				ClientID:     opts.ClientID,
				ClientSecret: opts.ClientSecret,
				TokenPath:    opts.TokenPath,
				Out:          cmd.OutOrStdout(),
			})
		},
	}

	binder.SetTo(cmd.Flags())
	return cmd
}

func newAuthLogoutCommand(ctx context.Context) *cobra.Command {
	cfg := config.Load()

	opts := struct {
		ClientID     string `flag:"client-id" usage:"Kakao REST API Key"`
		ClientSecret string `flag:"client-secret" usage:"Kakao Client Secret"`
		TokenPath    string `flag:"token-path" usage:"Path to token JSON file"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("client-id", "c", cfg.ClientID, "Kakao REST API Key").
		String("client-secret", cfg.ClientSecret, "Kakao Client Secret (optional)").
		StringP("token-path", "t", cfg.TokenPath, "Path to token JSON file")

	cmd := &cobra.Command{
		Use:           "logout",
		Short:         "Expire current Kakao access token session and clear local tokens",
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
			tokenStore := kakao.NewFileTokenStore(opts.TokenPath)
			client := kakao.NewClient(kakao.ClientConfig{
				ClientID:     opts.ClientID,
				ClientSecret: opts.ClientSecret,
				TokenStore:   tokenStore,
			})
			userID, err := client.Auth().Logout(cmd.Context())
			if err != nil {
				return fmt.Errorf("logout failed: %w", err)
			}
			_ = tokenStore.Clear(cmd.Context())
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully logged out user %d and cleared local tokens.\n", userID)
			return nil
		},
	}

	binder.SetTo(cmd.Flags())
	return cmd
}

func newAuthUnlinkCommand(ctx context.Context) *cobra.Command {
	cfg := config.Load()

	opts := struct {
		ClientID     string `flag:"client-id" usage:"Kakao REST API Key"`
		ClientSecret string `flag:"client-secret" usage:"Kakao Client Secret"`
		TokenPath    string `flag:"token-path" usage:"Path to token JSON file"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("client-id", "c", cfg.ClientID, "Kakao REST API Key").
		String("client-secret", cfg.ClientSecret, "Kakao Client Secret (optional)").
		StringP("token-path", "t", cfg.TokenPath, "Path to token JSON file")

	cmd := &cobra.Command{
		Use:           "unlink",
		Short:         "Unlink app connection from Kakao user account",
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
			tokenStore := kakao.NewFileTokenStore(opts.TokenPath)
			client := kakao.NewClient(kakao.ClientConfig{
				ClientID:     opts.ClientID,
				ClientSecret: opts.ClientSecret,
				TokenStore:   tokenStore,
			})
			userID, err := client.Auth().Unlink(cmd.Context())
			if err != nil {
				return fmt.Errorf("unlink failed: %w", err)
			}
			_ = tokenStore.Clear(cmd.Context())
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully unlinked user %d and cleared local tokens.\n", userID)
			return nil
		},
	}

	binder.SetTo(cmd.Flags())
	return cmd
}
