package bootstrap

import (
	"context"

	"github.com/dh-kam/kakao-bot/internal/usecase/skill"
	"github.com/dh-kam/refutils/flagsbinder"
	"github.com/spf13/cobra"
)

func newSkillCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "skill",
		Short:         "Manage and run Kakao i OpenBuilder chatbot skill webhook server",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(
		newSkillServeCommand(ctx),
	)

	return cmd
}

func newSkillServeCommand(ctx context.Context) *cobra.Command {
	opts := struct {
		ListenAddr string `flag:"listen" usage:"Address to listen on for Kakao OpenBuilder skill requests"`
		ChannelID  string `flag:"channel-id" usage:"KakaoTalk channel search ID"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("listen", "l", ":8080", "Address to listen on").
		String("channel-id", "0xc0de1ab", "KakaoTalk channel search ID")

	cmd := &cobra.Command{
		Use:           "serve",
		Short:         "Start Kakao i OpenBuilder chatbot skill webhook server",
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
			return skill.NewSkillServeUseCase().Execute(cmd.Context(), skill.SkillServeRequest{
				ListenAddr: opts.ListenAddr,
				ChannelID:  opts.ChannelID,
				Out:        cmd.OutOrStdout(),
			})
		},
	}

	binder.SetTo(cmd.Flags())
	return cmd
}
