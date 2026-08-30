package bootstrap

import (
	"context"
	"fmt"

	"github.com/dh-kam/kakao-bot/internal/config"
	"github.com/dh-kam/kakao-bot/internal/usecase/storage"
	"github.com/dh-kam/refutils/flagsbinder"
	"github.com/spf13/cobra"
)

func newStorageCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "storage",
		Short:         "Manage Talk Message image uploads on Kakao CDN",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(
		newStorageUploadCommand(ctx),
		newStorageDeleteCommand(ctx),
	)

	return cmd
}

func newStorageUploadCommand(ctx context.Context) *cobra.Command {
	cfg := config.Load()

	opts := struct {
		ClientID     string `flag:"client-id" usage:"Kakao REST API Key"`
		ClientSecret string `flag:"client-secret" usage:"Kakao Client Secret"`
		RedirectURI  string `flag:"redirect-uri" usage:"Kakao OAuth Redirect URI"`
		TokenPath    string `flag:"token-path" usage:"Path to token JSON file"`
		FilePath     string `flag:"file" usage:"Path to image file"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("client-id", "c", cfg.ClientID, "Kakao REST API Key").
		String("client-secret", cfg.ClientSecret, "Kakao Client Secret (optional)").
		StringP("redirect-uri", "r", cfg.RedirectURI, "Kakao OAuth Redirect URI").
		StringP("token-path", "t", cfg.TokenPath, "Path to token JSON file").
		StringP("file", "f", "", "Path to image file to upload")

	cmd := &cobra.Command{
		Use:           "upload [image_file]",
		Short:         "Upload image file to Kakao CDN for use in messages",
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := binder.BindCommand(cmd, &opts, args...); err != nil {
				_ = cmd.Usage()
				return err
			}
			if len(args) > 0 && opts.FilePath == "" {
				opts.FilePath = args[0]
			}
			if opts.FilePath == "" {
				_ = cmd.Usage()
				return fmt.Errorf("image file path is required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return storage.NewUploadUseCase().Execute(cmd.Context(), storage.UploadRequest{
				ClientID:     opts.ClientID,
				ClientSecret: opts.ClientSecret,
				RedirectURI:  opts.RedirectURI,
				TokenPath:    opts.TokenPath,
				FilePath:     opts.FilePath,
				Out:          cmd.OutOrStdout(),
			})
		},
	}

	binder.SetTo(cmd.Flags())
	return cmd
}

func newStorageDeleteCommand(ctx context.Context) *cobra.Command {
	cfg := config.Load()

	opts := struct {
		ClientID     string `flag:"client-id" usage:"Kakao REST API Key"`
		ClientSecret string `flag:"client-secret" usage:"Kakao Client Secret"`
		RedirectURI  string `flag:"redirect-uri" usage:"Kakao OAuth Redirect URI"`
		TokenPath    string `flag:"token-path" usage:"Path to token JSON file"`
		ImageURL     string `flag:"image-url" usage:"Kakao CDN image URL to delete"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("client-id", "c", cfg.ClientID, "Kakao REST API Key").
		String("client-secret", cfg.ClientSecret, "Kakao Client Secret (optional)").
		StringP("redirect-uri", "r", cfg.RedirectURI, "Kakao OAuth Redirect URI").
		StringP("token-path", "t", cfg.TokenPath, "Path to token JSON file").
		StringP("image-url", "u", "", "Kakao CDN image URL to delete")

	cmd := &cobra.Command{
		Use:           "delete [image_url]",
		Short:         "Delete an uploaded image from Kakao CDN",
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := binder.BindCommand(cmd, &opts, args...); err != nil {
				_ = cmd.Usage()
				return err
			}
			if len(args) > 0 && opts.ImageURL == "" {
				opts.ImageURL = args[0]
			}
			if opts.ImageURL == "" {
				_ = cmd.Usage()
				return fmt.Errorf("image URL is required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return storage.NewDeleteUseCase().Execute(cmd.Context(), storage.DeleteRequest{
				ClientID:     opts.ClientID,
				ClientSecret: opts.ClientSecret,
				RedirectURI:  opts.RedirectURI,
				TokenPath:    opts.TokenPath,
				ImageURL:     opts.ImageURL,
				Out:          cmd.OutOrStdout(),
			})
		},
	}

	binder.SetTo(cmd.Flags())
	return cmd
}
