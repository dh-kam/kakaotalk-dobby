package bootstrap

import (
	"context"

	"github.com/dh-kam/kakaotalk-dobby/internal/config"
	"github.com/dh-kam/kakaotalk-dobby/internal/usecase/user"
	"github.com/dh-kam/refutils/flagsbinder"
	"github.com/spf13/cobra"
)

func newUserCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "user",
		Short:         "Retrieve Kakao user profile and account details",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(
		newUserMeCommand(ctx),
		newUserAddressCommand(ctx),
	)

	return cmd
}

func newUserMeCommand(ctx context.Context) *cobra.Command {
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
		Use:           "me",
		Short:         "Show authenticated user profile",
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
			return user.NewMeUseCase().Execute(cmd.Context(), user.MeRequest{
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

func newUserAddressCommand(ctx context.Context) *cobra.Command {
	cfg := config.Load()

	opts := struct {
		ClientID     string `flag:"client-id" usage:"Kakao REST API Key"`
		ClientSecret string `flag:"client-secret" usage:"Kakao Client Secret"`
		RedirectURI  string `flag:"redirect-uri" usage:"Kakao OAuth Redirect URI"`
		TokenPath    string `flag:"token-path" usage:"Path to token JSON file"`
		FromUpdated  int    `flag:"from-updated" usage:"Fetch addresses updated after this timestamp"`
		PageSize     int    `flag:"page-size" usage:"Number of addresses per page"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("client-id", "c", cfg.ClientID, "Kakao REST API Key").
		String("client-secret", cfg.ClientSecret, "Kakao Client Secret (optional)").
		StringP("redirect-uri", "r", cfg.RedirectURI, "Kakao OAuth Redirect URI").
		StringP("token-path", "t", cfg.TokenPath, "Path to token JSON file").
		Int("from-updated", 0, "Fetch addresses updated after timestamp").
		Int("page-size", 10, "Number of addresses per page")

	cmd := &cobra.Command{
		Use:           "address",
		Short:         "Query registered shipping addresses for user",
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
			return user.NewShippingAddressUseCase().Execute(cmd.Context(), user.ShippingAddressRequest{
				ClientID:     opts.ClientID,
				ClientSecret: opts.ClientSecret,
				RedirectURI:  opts.RedirectURI,
				TokenPath:    opts.TokenPath,
				FromUpdated:  opts.FromUpdated,
				PageSize:     opts.PageSize,
				Out:          cmd.OutOrStdout(),
			})
		},
	}

	binder.SetTo(cmd.Flags())
	return cmd
}
