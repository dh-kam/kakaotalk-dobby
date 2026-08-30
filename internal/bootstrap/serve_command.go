package bootstrap

import (
	"context"

	"github.com/dh-kam/kakao-bot/internal/config"
	"github.com/dh-kam/kakao-bot/internal/usecase/webhook"
	"github.com/dh-kam/refutils/flagsbinder"
	"github.com/spf13/cobra"
)

func newServeCommand(ctx context.Context) *cobra.Command {
	cfg := config.Load()

	opts := struct {
		ListenAddr   string `flag:"listen" usage:"Address to listen on for incoming HTTP webhooks"`
		ClientID     string `flag:"client-id" usage:"Kakao REST API Key"`
		ClientSecret string `flag:"client-secret" usage:"Kakao Client Secret"`
		RedirectURI  string `flag:"redirect-uri" usage:"Kakao OAuth Redirect URI"`
		TokenPath    string `flag:"token-path" usage:"Path to token JSON file"`
		SecretToken  string `flag:"secret-token" usage:"Optional secret token required in X-Webhook-Token header"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("listen", "l", "127.0.0.1:8080", "Address to listen on").
		StringP("client-id", "c", cfg.ClientID, "Kakao REST API Key").
		String("client-secret", cfg.ClientSecret, "Kakao Client Secret (optional)").
		StringP("redirect-uri", "r", cfg.RedirectURI, "Kakao OAuth Redirect URI").
		StringP("token-path", "t", cfg.TokenPath, "Path to token JSON file").
		String("secret-token", "", "Optional secret token required in X-Webhook-Token header")

	cmd := &cobra.Command{
		Use:           "serve",
		Short:         "Start HTTP webhook server to relay alerts and messages to KakaoTalk",
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
			return webhook.NewServeUseCase().Execute(cmd.Context(), webhook.ServeRequest{
				ListenAddr:   opts.ListenAddr,
				ClientID:     opts.ClientID,
				ClientSecret: opts.ClientSecret,
				RedirectURI:  opts.RedirectURI,
				TokenPath:    opts.TokenPath,
				SecretToken:  opts.SecretToken,
				Out:          cmd.OutOrStdout(),
			})
		},
	}

	binder.SetTo(cmd.Flags())
	return cmd
}
