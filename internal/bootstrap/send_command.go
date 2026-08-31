package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/dh-kam/kakaotalk-dobby/internal/config"
	"github.com/dh-kam/kakaotalk-dobby/internal/usecase/send"
	"github.com/dh-kam/refutils/flagsbinder"
	"github.com/spf13/cobra"
)

func newSendCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "send",
		Short:         "Send KakaoTalk messages to yourself or friends",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(
		newSendMeCommand(ctx),
		newSendFriendCommand(ctx),
	)

	return cmd
}

func newSendMeCommand(ctx context.Context) *cobra.Command {
	cfg := config.Load()

	opts := struct {
		ClientID     string `flag:"client-id" usage:"Kakao REST API Key"`
		ClientSecret string `flag:"client-secret" usage:"Kakao Client Secret"`
		RedirectURI  string `flag:"redirect-uri" usage:"Kakao OAuth Redirect URI"`
		TokenPath    string `flag:"token-path" usage:"Path to token JSON file"`
		Text         string `flag:"text" usage:"Message text content (can also be passed as arguments or piped)"`
		WebURL       string `flag:"url" usage:"Target Web URL when clicking message or button"`
		ButtonTitle  string `flag:"button" usage:"Button title text"`
		Title        string `flag:"title" usage:"Title for feed template message"`
		Description  string `flag:"description" usage:"Description for feed template message"`
		ImageURL     string `flag:"image-url" usage:"Image URL for feed template message"`
		TemplateID   int    `flag:"template-id" usage:"Custom message template ID registered in Kakao Developers"`
		TemplateArgs string `flag:"template-args" usage:"JSON string of arguments for custom template"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("client-id", "c", cfg.ClientID, "Kakao REST API Key").
		String("client-secret", cfg.ClientSecret, "Kakao Client Secret (optional)").
		StringP("redirect-uri", "r", cfg.RedirectURI, "Kakao OAuth Redirect URI").
		StringP("token-path", "t", cfg.TokenPath, "Path to token JSON file").
		StringP("text", "m", "", "Message text content").
		StringP("url", "u", "", "Target Web URL").
		StringP("button", "b", "", "Button title text").
		String("title", "", "Title for feed template").
		String("description", "", "Description for feed template").
		String("image-url", "", "Image URL for feed template").
		Int("template-id", 0, "Custom message template ID").
		String("template-args", "", "JSON string of arguments for custom template")

	cmd := &cobra.Command{
		Use:           "me [message]",
		Short:         "Send a message to yourself (나에게 보내기)",
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := binder.BindCommand(cmd, &opts, args...); err != nil {
				_ = cmd.Usage()
				return err
			}
			if len(args) > 0 && opts.Text == "" {
				opts.Text = strings.Join(args, " ")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return send.NewSendMeUseCase().Execute(cmd.Context(), send.SendMeRequest{
				ClientID:     opts.ClientID,
				ClientSecret: opts.ClientSecret,
				RedirectURI:  opts.RedirectURI,
				TokenPath:    opts.TokenPath,
				Text:         opts.Text,
				WebURL:       opts.WebURL,
				ButtonTitle:  opts.ButtonTitle,
				Title:        opts.Title,
				Description:  opts.Description,
				ImageURL:     opts.ImageURL,
				TemplateID:   int64(opts.TemplateID),
				TemplateArgs: opts.TemplateArgs,
				In:           cmd.InOrStdin(),
				Out:          cmd.OutOrStdout(),
			})
		},
	}

	binder.SetTo(cmd.Flags())
	return cmd
}

func newSendFriendCommand(ctx context.Context) *cobra.Command {
	cfg := config.Load()

	opts := struct {
		ClientID      string   `flag:"client-id" usage:"Kakao REST API Key"`
		ClientSecret  string   `flag:"client-secret" usage:"Kakao Client Secret"`
		RedirectURI   string   `flag:"redirect-uri" usage:"Kakao OAuth Redirect URI"`
		TokenPath     string   `flag:"token-path" usage:"Path to token JSON file"`
		ReceiverUUIDs []string `flag:"receiver-uuids" usage:"List of Kakao friend receiver UUIDs"`
		Text          string   `flag:"text" usage:"Message text content"`
		WebURL        string   `flag:"url" usage:"Target Web URL"`
		ButtonTitle   string   `flag:"button" usage:"Button title text"`
		Title         string   `flag:"title" usage:"Title for feed template"`
		Description   string   `flag:"description" usage:"Description for feed template"`
		ImageURL      string   `flag:"image-url" usage:"Image URL for feed template"`
		TemplateID    int      `flag:"template-id" usage:"Custom message template ID"`
		TemplateArgs  string   `flag:"template-args" usage:"JSON string of arguments for custom template"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("client-id", "c", cfg.ClientID, "Kakao REST API Key").
		String("client-secret", cfg.ClientSecret, "Kakao Client Secret (optional)").
		StringP("redirect-uri", "r", cfg.RedirectURI, "Kakao OAuth Redirect URI").
		StringP("token-path", "t", cfg.TokenPath, "Path to token JSON file").
		StringSliceP("receiver-uuids", "R", nil, "List of Kakao friend receiver UUIDs").
		StringP("text", "m", "", "Message text content").
		StringP("url", "u", "", "Target Web URL").
		StringP("button", "b", "", "Button title text").
		String("title", "", "Title for feed template").
		String("description", "", "Description for feed template").
		String("image-url", "", "Image URL for feed template").
		Int("template-id", 0, "Custom message template ID").
		String("template-args", "", "JSON string of arguments for custom template")

	cmd := &cobra.Command{
		Use:           "friend [message]",
		Short:         "Send a message to friends (친구에게 보내기)",
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := binder.BindCommand(cmd, &opts, args...); err != nil {
				_ = cmd.Usage()
				return err
			}
			if len(args) > 0 && opts.Text == "" {
				opts.Text = strings.Join(args, " ")
			}
			if len(opts.ReceiverUUIDs) == 0 {
				_ = cmd.Usage()
				return fmt.Errorf("--receiver-uuids (-R) is required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return send.NewSendFriendUseCase().Execute(cmd.Context(), send.SendFriendRequest{
				ClientID:      opts.ClientID,
				ClientSecret:  opts.ClientSecret,
				RedirectURI:   opts.RedirectURI,
				TokenPath:     opts.TokenPath,
				ReceiverUUIDs: opts.ReceiverUUIDs,
				Text:          opts.Text,
				WebURL:        opts.WebURL,
				ButtonTitle:   opts.ButtonTitle,
				Title:         opts.Title,
				Description:   opts.Description,
				ImageURL:      opts.ImageURL,
				TemplateID:    int64(opts.TemplateID),
				TemplateArgs:  opts.TemplateArgs,
				In:            cmd.InOrStdin(),
				Out:           cmd.OutOrStdout(),
			})
		},
	}

	binder.SetTo(cmd.Flags())
	return cmd
}
